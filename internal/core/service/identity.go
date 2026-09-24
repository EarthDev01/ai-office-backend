package service

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
)

// identityResolver อ่านจาก token ว่าเป็นใคร มีสิทธิ์อะไร เข้า service ไหนได้
//
// มี cache สั้น ๆ เพราะทุกข้อความในแชทจะต้อง resolve ใหม่
type identityResolver struct {
	ttl time.Duration

	mu    sync.RWMutex
	cache map[string]cachedCaller
}

type cachedCaller struct {
	caller domain.Caller
	until  time.Time
}

// รับเฉพาะ JWT จริงที่ officeลูกค้า ออกให้แอดมินตอน login — ไม่มีโหมด token ทดสอบ
func NewIdentityResolver(ttl time.Duration) port.IdentityResolver {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &identityResolver{ttl: ttl, cache: map[string]cachedCaller{}}
}

func (r *identityResolver) Resolve(ctx context.Context, office domain.Office, cred domain.Credential) (domain.Caller, error) {
	if cred.Token == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}

	key := office.ID + "|" + cred.Token
	if c, ok := r.get(key); ok {
		return c, nil
	}

	// ตัวตนจริงถูกเข้ารหัสอยู่ใน JWT อยู่แล้ว (claim "result" = EmployeeModel) จึง decode ตรง ๆ
	// ไม่เรียก backoffice API เพราะอยู่หลัง Cloudflare + IP-whitelist ที่ server-to-server
	// call ผ่านไม่ได้ · อีกทั้ง secret ของ JWT ผูกกับ IP (O9) จึง verify signature ฝั่ง
	// server ไม่ได้ — เชื่อ token ที่ browser ล็อกอินมาแล้ว
	caller, err := parseOfficeJWT(cred.Token)
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

// parseOfficeJWT อ่านตัวตนจาก payload ของ JWT หลังบ้านตรง ๆ
//
// JWT = HS256 · payload = { exp, result: EmployeeModel } โดย result มี id/username/role
// ที่มี permission กับ list_service ครบ
//
// ██ เราอ่านเฉพาะ payload · ไม่ verify signature เพราะ secret ผูกกับ UA+IP (O9)
// ██ ที่ฝั่ง server คำนวณซ้ำไม่ได้ ทางที่ถูกระยะยาวคือ service token แยก (survey §2.4 B)
func parseOfficeJWT(token string) (domain.Caller, error) {
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

	var claims struct {
		Exp    int64 `json:"exp"`
		Result struct {
			ID       string `json:"id"`
			Username string `json:"username"`
			Role     struct {
				Name       string `json:"name"`
				Level      int32  `json:"level"`
				Permission []struct {
					Code string `json:"code"`
				} `json:"permission"`
				ListService []struct {
					Service    string `json:"service"`
					Permission bool   `json:"permission"`
				} `json:"list_service"`
			} `json:"role"`
		} `json:"result"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}

	res := claims.Result
	if res.ID == "" && res.Username == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}
	if claims.Exp > 0 && time.Now().Unix() >= claims.Exp {
		return domain.Caller{}, domain.ErrSessionExpired
	}

	caller := domain.Caller{
		AdminID:  res.ID,
		Username: res.Username,
		RoleName: res.Role.Name,
		Level:    res.Role.Level,
	}
	if caller.AdminID == "" {
		caller.AdminID = res.Username
	}
	for _, p := range res.Role.Permission {
		if p.Code != "" {
			caller.Permissions = append(caller.Permissions, p.Code)
		}
	}
	// เอาเฉพาะ service ที่ Permission == true — ตัวที่ false คือมีชื่อแต่ไม่ให้เข้า
	for _, s := range res.Role.ListService {
		if s.Permission && s.Service != "" {
			caller.Services = append(caller.Services, s.Service)
		}
	}
	return caller, nil
}
