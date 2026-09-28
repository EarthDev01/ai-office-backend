package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/connector"
	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// identityResolver อ่านจาก token ว่าเป็นใคร มีสิทธิ์อะไร เข้า service ไหนได้
//
// วิธีอ่านมาจาก page_auth.identity ของ connector ตาม kind ของ office (โค้ดนี้ไม่รู้จักรูป JWT ของหลังบ้านใด)
// มี cache สั้น ๆ เพราะทุกข้อความในแชทจะต้อง resolve ใหม่
type identityResolver struct {
	ttl   time.Duration
	conns connector.Set

	mu    sync.RWMutex
	cache map[string]cachedCaller
}

type cachedCaller struct {
	caller domain.Caller
	until  time.Time
}

// รับเฉพาะ JWT จริงที่ officeลูกค้า ออกให้แอดมินตอน login — ไม่มีโหมด token ทดสอบ
func NewIdentityResolver(ttl time.Duration, conns connector.Set) port.IdentityResolver {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &identityResolver{ttl: ttl, conns: conns, cache: map[string]cachedCaller{}}
}

func (r *identityResolver) Resolve(ctx context.Context, office domain.Office, cred domain.Credential) (domain.Caller, error) {
	if cred.Token == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}
	conn, ok := r.conns.For(office.Kind)
	if !ok || conn.Host.PageAuth.Identity == nil || conn.Host.PageAuth.Identity.Source != "jwt" {
		// office ตั้ง kind ที่ไม่มี connector / connector ไม่ได้อ่านตัวตนจาก JWT — ตรวจตัวตนไม่ได้
		return domain.Caller{}, domain.ErrNotAuthenticated
	}

	key := office.ID + "|" + cred.Token
	if c, ok := r.get(key); ok {
		return c, nil
	}

	// ตัวตนจริงถูกเข้ารหัสอยู่ใน JWT อยู่แล้ว จึง decode ตรง ๆ
	// ไม่เรียก backoffice API เพราะอยู่หลัง Cloudflare + IP-whitelist ที่ server-to-server
	// call ผ่านไม่ได้ · อีกทั้ง secret ของ JWT ผูกกับ UA/IP/Host ของหลังบ้าน จึง verify signature ฝั่ง
	// server ไม่ได้ — เชื่อ token ที่ browser ล็อกอินมาแล้ว (ข้อมูลจริงยังคุมโดยหลังบ้านทุกครั้งที่ยิง)
	caller, err := parseOfficeJWT(cred.Token, conn.Host.PageAuth.Identity)
	if err != nil {
		return domain.Caller{}, err
	}
	caller.OfficeID = office.ID
	r.put(key, caller)
	return caller, nil
}

func (r *identityResolver) get(key string) (domain.Caller, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.cache[key]
	if !ok || time.Now().After(c.until) {
		return domain.Caller{}, false
	}
	return c.caller, true
}

func (r *identityResolver) put(key string, caller domain.Caller) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if len(r.cache) > 5000 { // กัน map โตไม่จำกัด
		r.cache = map[string]cachedCaller{}
	}
	r.cache[key] = cachedCaller{caller: caller, until: time.Now().Add(r.ttl)}
}

// parseOfficeJWT อ่านตัวตนจาก payload ของ JWT หลังบ้านตรง ๆ ตาม spec ของ connector
//
// ██ เราอ่านเฉพาะ payload · ไม่ verify signature (secret ของหลังบ้านคำนวณซ้ำฝั่ง server ไม่ได้/ไม่ใช่ความลับจริง)
// ██ ทางที่ถูกระยะยาวคือ service token แยก (spec §7.3)
func parseOfficeJWT(token string, spec *connector.IdentitySpec) (domain.Caller, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		// เผื่อ token ที่ใส่ padding มาแบบ std URL encoding
		if payload, err = base64.URLEncoding.DecodeString(parts[1]); err != nil {
			return domain.Caller{}, domain.ErrNotAuthenticated
		}
	}
	var claims map[string]any
	if err := json.Unmarshal(payload, &claims); err != nil {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}
	user, ok := connector.GetPath(claims, spec.Root).(map[string]any)
	if !ok {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}

	caller := domain.Caller{
		AdminID:  claimStr(connector.GetPath(user, spec.ID)),
		Username: claimStr(connector.GetPath(user, spec.Username)),
		Level:    int32(claimNum(connector.GetPath(user, spec.Level))),
	}
	if caller.AdminID == "" && caller.Username == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}
	if exp := claimNum(claims["exp"]); exp > 0 && float64(time.Now().Unix()) >= exp {
		return domain.Caller{}, domain.ErrSessionExpired
	}
	if caller.AdminID == "" {
		caller.AdminID = caller.Username
	}
	// สิทธิ์ติดมากับ JWT ใบนี้เฉพาะเมื่อ connector ไม่ได้บอกให้ widget อ่านจากที่อื่น
	if spec.PermissionsRequest == "" && spec.PermissionsToken == nil {
		caller.Permissions = spec.Permissions.Strings(user)
	}

	// เว็บที่เข้าได้ — ตัดสินตาม connector (ไม่ตั้ง services = ไม่จำกัด)
	switch {
	case spec.AllServices(caller.Level, claimStr(connector.GetPath(user, spec.Dept))):
		caller.AllServices = true
	case spec.Services != nil:
		caller.Services = spec.Services.Strings(user)
		caller.ServicesStrict = spec.ServicesEmpty == "none"
	default:
		caller.AllServices = true
	}
	return caller, nil
}

func claimStr(v any) string {
	s, _ := v.(string)
	return s
}

func claimNum(v any) float64 {
	f, _ := v.(float64)
	return f
}
