package routes

import (
	"errors"
	"net/http"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/port"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// AuthHandler ครอบ service.ConsoleAuth ให้เป็น HTTP handler ของ /api/ai/admin/auth + /users
type AuthHandler struct {
	auth  *service.ConsoleAuth
	perms *service.PermissionService
}

func NewAuthHandler(auth *service.ConsoleAuth, perms *service.PermissionService) *AuthHandler {
	return &AuthHandler{auth: auth, perms: perms}
}

// อ่าน ctx key ที่ ConsoleAuth middleware แปะไว้ — ประกาศเป็น literal ที่นี่เพราะ
// import gin (middleware) กลับมาที่ routes จะเกิด import cycle (gin.go นำเข้า routes อยู่แล้ว)
// ค่าต้องตรงกับ ConsoleUserIDKey/ConsoleUsernameKey ใน internal/adapter/handler/gin/middleware.go เป๊ะ
const (
	ctxConsoleUserID   = "console_user_id"
	ctxConsoleUsername = "console_username"
)

func consoleUserID(c *gin.Context) string {
	v, _ := c.Get(ctxConsoleUserID)
	s, _ := v.(string)
	return s
}

func consoleUsername(c *gin.Context) string {
	v, _ := c.Get(ctxConsoleUsername)
	s, _ := v.(string)
	return s
}

// stagePayload ส่งผลลัพธ์แต่ละขั้นของ 2-step login/enroll กลับไปแบบเดียวกันทุก endpoint
func stagePayload(c *gin.Context, lr service.LoginResult) {
	switch lr.Stage {
	case "enroll":
		ResData(c, http.StatusOK, "OK", "", gin.H{
			"stage":       "enroll",
			"ticket":      lr.Ticket,
			"otpauth_uri": lr.OTPAuthURI,
			"secret":      lr.Secret,
		})
	case "totp":
		ResData(c, http.StatusOK, "OK", "", gin.H{"stage": "totp", "ticket": lr.Ticket})
	default:
		ResData(c, http.StatusOK, "OK", "", gin.H{"stage": lr.Stage, "token": lr.Token, "user": lr.User})
	}
}

// fail แปล sentinel error ของ console auth service เป็น HTTP response (โค้ด/ข้อความ/error-code)
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrSetupDone):
		ResData(c, http.StatusConflict, "ตั้งค่าระบบไปแล้ว", "SETUP_DONE", nil)
	case errors.Is(err, service.ErrInvalidCredentials):
		ResData(c, http.StatusUnauthorized, "username หรือรหัสผ่านไม่ถูกต้อง", "INVALID_CREDENTIALS", nil)
	case errors.Is(err, service.ErrAccountLocked):
		ResData(c, http.StatusLocked, "บัญชีถูกล็อกชั่วคราว", "ACCOUNT_LOCKED", nil)
	case errors.Is(err, service.ErrAccountDisabled):
		ResData(c, http.StatusForbidden, "บัญชีถูกปิดใช้งาน", "ACCOUNT_DISABLED", nil)
	case errors.Is(err, service.ErrInvalidPassword):
		ResData(c, http.StatusBadRequest, "รหัสผ่านต้องมีอย่างน้อย 8 ตัว", "INVALID_PASSWORD", nil)
	case errors.Is(err, service.ErrInvalidRole):
		ResData(c, http.StatusBadRequest, "role ไม่ถูกต้อง", "INVALID_ROLE", nil)
	case errors.Is(err, service.ErrInvalid2FA):
		ResData(c, http.StatusUnauthorized, "รหัส 2FA ไม่ถูกต้อง", "INVALID_2FA", nil)
	case errors.Is(err, service.ErrInvalidTicket):
		ResData(c, http.StatusUnauthorized, "ticket หมดอายุ เริ่ม login ใหม่", "INVALID_TICKET", nil)
	case errors.Is(err, port.ErrUsernameTaken):
		ResData(c, http.StatusConflict, "username นี้ถูกใช้แล้ว", "USERNAME_TAKEN", nil)
	case errors.Is(err, service.ErrLastAdmin):
		ResData(c, http.StatusConflict, "ต้องมี admin ที่ใช้งานได้อย่างน้อย 1 คน", "LAST_ADMIN", nil)
	case errors.Is(err, service.ErrCannotDeleteSelf):
		ResData(c, http.StatusConflict, "ลบบัญชีตัวเองไม่ได้", "CANNOT_DELETE_SELF", nil)
	case errors.Is(err, port.ErrUserNotFound):
		ResData(c, http.StatusNotFound, "ไม่พบผู้ใช้", "NOT_FOUND", nil)
	default:
		ResData(c, http.StatusInternalServerError, "เกิดข้อผิดพลาด", "INTERNAL", nil)
	}
}

func badRequest(c *gin.Context) {
	ResData(c, http.StatusBadRequest, "ข้อมูลไม่ถูกต้อง", "BAD_REQUEST", nil)
}

// ---- public (ไม่ต้อง auth) ----

func (h *AuthHandler) Status(c *gin.Context) {
	needs, err := h.auth.NeedsSetup(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{"needs_setup": needs})
}

type registerReq struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Password    string `json:"password"`
}

func (h *AuthHandler) Register(c *gin.Context) {
	var req registerReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	lr, err := h.auth.Register(c.Request.Context(), req.Username, req.DisplayName, req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	stagePayload(c, lr)
}

type loginReq struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *gin.Context) {
	var req loginReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	lr, err := h.auth.Login(c.Request.Context(), req.Username, req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	stagePayload(c, lr)
}

type verifyTOTPReq struct {
	Ticket string `json:"ticket"`
	Code   string `json:"code"`
}

// VerifyTOTP: ขั้นสองของ login/enroll เสมอจบด้วย Stage=="done" เมื่อไม่ error
// recovery_codes จะไม่ว่างเฉพาะตอน enroll เสร็จรอบแรกเท่านั้น (LoginResult.RecoveryCodes)
func (h *AuthHandler) VerifyTOTP(c *gin.Context) {
	var req verifyTOTPReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	lr, err := h.auth.VerifyTOTP(c.Request.Context(), req.Ticket, req.Code)
	if err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{
		"token":          lr.Token,
		"user":           lr.User,
		"recovery_codes": lr.RecoveryCodes,
	})
}

// ---- authenticated, any role ----

func (h *AuthHandler) Me(c *gin.Context) {
	id := consoleUserID(c)
	if id == "" {
		// break-glass ไม่มี user จริงในฐานข้อมูล — ตอบ identity สังเคราะห์ + สิทธิ์เท่า admin เต็ม
		perms, err := h.perms.For(c.Request.Context(), domain.RoleAdmin)
		if err != nil {
			fail(c, err)
			return
		}
		label, err := h.perms.RoleLabel(c.Request.Context(), domain.RoleAdmin)
		if err != nil {
			fail(c, err)
			return
		}
		ResData(c, http.StatusOK, "OK", "", gin.H{
			"user":        gin.H{"username": "break-glass", "role": domain.RoleAdmin},
			"permissions": perms,
			"role_label":  label,
		})
		return
	}
	u, err := h.auth.Me(c.Request.Context(), id)
	if err != nil {
		fail(c, err)
		return
	}
	perms, err := h.perms.For(c.Request.Context(), u.Role)
	if err != nil {
		fail(c, err)
		return
	}
	label, err := h.perms.RoleLabel(c.Request.Context(), u.Role)
	if err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{"user": u, "permissions": perms, "role_label": label})
}

type changePasswordReq struct {
	OldPassword string `json:"old_password"`
	NewPassword string `json:"new_password"`
}

func (h *AuthHandler) ChangePassword(c *gin.Context) {
	id := consoleUserID(c)
	if id == "" {
		ResData(c, http.StatusBadRequest, "break-glass ไม่มีรหัสผ่าน", "BREAK_GLASS_NO_PASSWORD", nil)
		return
	}
	var req changePasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	if err := h.auth.ChangePassword(c.Request.Context(), id, req.OldPassword, req.NewPassword); err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", nil)
}

// ---- authenticated, admin only ----

func (h *AuthHandler) ListUsers(c *gin.Context) {
	users, err := h.auth.ListUsers(c.Request.Context())
	if err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{"users": users})
}

type createUserReq struct {
	Username    string `json:"username"`
	DisplayName string `json:"display_name"`
	Role        string `json:"role"`
	Password    string `json:"password"`
}

func (h *AuthHandler) CreateUser(c *gin.Context) {
	var req createUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	u, err := h.auth.CreateUser(c.Request.Context(), consoleUsername(c), req.Username, req.DisplayName, domain.Role(req.Role), req.Password)
	if err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{"user": u})
}

type patchUserReq struct {
	DisplayName *string `json:"display_name"`
	Role        *string `json:"role"`
	Status      *string `json:"status"`
}

func (h *AuthHandler) PatchUser(c *gin.Context) {
	var req patchUserReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	var role *domain.Role
	if req.Role != nil {
		r := domain.Role(*req.Role)
		role = &r
	}
	u, err := h.auth.PatchUser(c.Request.Context(), c.Param("id"), req.DisplayName, role, req.Status)
	if err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", gin.H{"user": u})
}

type resetPasswordReq struct {
	Password string `json:"password"`
}

func (h *AuthHandler) ResetPassword(c *gin.Context) {
	var req resetPasswordReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	if err := h.auth.ResetPassword(c.Request.Context(), c.Param("id"), req.Password); err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", nil)
}

func (h *AuthHandler) Reset2FA(c *gin.Context) {
	if err := h.auth.Reset2FA(c.Request.Context(), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", nil)
}

func (h *AuthHandler) DeleteUser(c *gin.Context) {
	if err := h.auth.Delete(c.Request.Context(), consoleUserID(c), c.Param("id")); err != nil {
		fail(c, err)
		return
	}
	ResData(c, http.StatusOK, "OK", "", nil)
}
