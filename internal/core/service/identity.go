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

// identityResolver ถาม office-api-v10 ว่า token นี้เป็นใคร มีสิทธิ์อะไร เข้า service ไหนได้
//
// มี cache สั้น ๆ เพราะทุกข้อความในแชทจะต้อง resolve ใหม่ ถ้ายิงทุกครั้งจะไปกิน
// rate limit ของ office-api (survey §6.3)
type identityResolver struct {
	api     port.BackofficeAPI
	ttl     time.Duration
	devMode bool

	mu    sync.RWMutex
	cache map[string]cachedCaller
}

type cachedCaller struct {
	caller domain.Caller
	until  time.Time
}

func NewIdentityResolver(api port.BackofficeAPI, ttl time.Duration, devMode bool) port.IdentityResolver {
	if ttl <= 0 {
		ttl = 60 * time.Second
	}
	return &identityResolver{api: api, ttl: ttl, devMode: devMode, cache: map[string]cachedCaller{}}
}

func (r *identityResolver) Resolve(ctx context.Context, office domain.Office, cred domain.Credential) (domain.Caller, error) {
	if cred.Token == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}

	// office ที่ยังไม่ได้ตั้ง backoffice_api_url ใช้ไม่ได้จริง
	// ยกเว้นโหมด dev ที่ยอมให้ใช้ token ปลอมเพื่อทดสอบหน้าจอ
	if office.BackofficeAPIURL == "" {
		if r.devMode {
			return parseDevToken(office, cred)
		}
		return domain.Caller{}, domain.ErrUpstream
	}

	key := office.ID + "|" + cred.Token
	if c, ok := r.get(key); ok {
		c.Cred = cred // ใช้ UA/IP ของ request ปัจจุบันเสมอ ไม่เอาของที่ cache ไว้
		return c, nil
	}

	// ตัวตนจริงถูกเข้ารหัสอยู่ใน JWT อยู่แล้ว (claim "result" = EmployeeModel)
	// จึง decode ตรง ๆ แบบเดียวกับที่ office-v10x ทำ (jwt_decode(auth_token).result)
	// ไม่เรียก office-api เพราะมันอยู่หลัง Cloudflare + Supercom IP-whitelist ที่
	// server-to-server call ผ่านไม่ได้ · อีกทั้ง secret ของ JWT ผูกกับ IP (O9)
	// จึง verify signature ฝั่ง server ไม่ได้ — เชื่อ token ที่ browser ล็อกอินมาแล้ว
	caller, err := parseOfficeJWT(cred.Token)
	if err != nil {
		return domain.Caller{}, err
	}
	caller.OfficeID = office.ID
	caller.Cred = cred
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

// parseDevToken รับ token ปลอมรูปแบบ "dev:<admin>:<service1,service2>:<role>"
//
// ██ DEV ONLY — ใช้ได้เฉพาะ office ที่ยังไม่ได้ตั้ง backoffice_api_url และ APP_MODE=dev
// ██ มีไว้ทดสอบหน้าจอตอนยังต่อ office-api จริงไม่ได้ (ติด O9)
func parseDevToken(office domain.Office, cred domain.Credential) (domain.Caller, error) {
	if !strings.HasPrefix(cred.Token, "dev:") {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}
	p := strings.Split(strings.TrimPrefix(cred.Token, "dev:"), ":")
	if len(p) < 2 || p[0] == "" || p[1] == "" {
		return domain.Caller{}, domain.ErrNotAuthenticated
	}
	role := "admin"
	if len(p) >= 3 && p[2] != "" {
		role = p[2]
	}
	services := []string{}
	for _, s := range strings.Split(p[1], ",") {
		if s = strings.TrimSpace(s); s != "" {
			services = append(services, s)
		}
	}
	return domain.Caller{
		AdminID:  p[0],
		Username: p[0],
		OfficeID: office.ID,
		RoleName: role,
		Services: services,
		Cred:     cred,
	}, nil
}

// parseOfficeJWT อ่านตัวตนจาก payload ของ office-api JWT ตรง ๆ
//
// JWT ของ office-api = HS256 · payload = { exp, result: EmployeeModel }
// (middlewares/auth.go:31-35 ของ office-api-v10) โดย result มี id/username/role
// ที่มี permission กับ list_service ครบ — เหมือนที่ /api/employees-permission คืน
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
				Name        string `json:"name"`
				Level       int32  `json:"level"`
				Permission  []struct {
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
	// เอาเฉพาะ service ที่ Permission == true (เหมือน EmployeeByID เดิม)
	for _, s := range res.Role.ListService {
		if s.Permission && s.Service != "" {
			caller.Services = append(caller.Services, s.Service)
		}
	}
	return caller, nil
}
