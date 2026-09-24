package routes

import (
	"errors"
	"net/http"

	"ai-office-backend/internal/core/domain"
	"ai-office-backend/internal/core/service"

	"github.com/gin-gonic/gin"
)

// PermissionHandler ครอบ service.PermissionService ให้เป็น HTTP handler ของ
// GET/PUT /role-permissions + POST/PATCH/DELETE /roles — ทั้งหมดต้องผ่าน
// RequirePermission("user.manage") มาก่อน (ดู gin.go)
//
// ต้องรู้จัก auth (ConsoleAuth) ด้วย เพราะ DELETE /roles/:key ต้องเช็คก่อนว่ามี console
// user คนไหนใช้ role นี้อยู่ไหม — ทำที่ handler ชั้นนี้ ไม่ใช่ใน PermissionService เพื่อไม่ให้
// service นั้นต้องผูกกับ console user repository (ดู service/permission.go)
type PermissionHandler struct {
	perms *service.PermissionService
	auth  *service.ConsoleAuth
}

func NewPermissionHandler(perms *service.PermissionService, auth *service.ConsoleAuth) *PermissionHandler {
	return &PermissionHandler{perms: perms, auth: auth}
}

// configPayload ส่ง role config เต็มชุดกลับไปทุกครั้งที่มีการแก้ไข (roles/matrix/catalog)
// เพื่อให้ FE sync state ได้จากคำตอบเดียว ไม่ต้องยิง GET ซ้ำ
func configPayload(cfg domain.RoleConfig) gin.H {
	return gin.H{
		"roles":   cfg.Roles,
		"matrix":  cfg.Matrix,
		"catalog": domain.PermissionCatalog(),
	}
}

func (h *PermissionHandler) respondConfig(c *gin.Context, code int) {
	cfg, err := h.perms.Config(c.Request.Context())
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL", "อ่านสิทธิ์ไม่สำเร็จ", nil)
		return
	}
	ResData(c, code, "OK", "", configPayload(cfg))
}

// GET /role-permissions
func (h *PermissionHandler) Get(c *gin.Context) {
	h.respondConfig(c, http.StatusOK)
}

type putRolePermissionsReq struct {
	Matrix map[string][]string `json:"matrix"`
}

// PUT /role-permissions — แก้เฉพาะ matrix (ไม่แตะรายชื่อ role — ใช้ /roles สำหรับนั้น)
func (h *PermissionHandler) Put(c *gin.Context) {
	var req putRolePermissionsReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Matrix == nil {
		badRequest(c)
		return
	}
	if err := h.perms.SetMatrix(c.Request.Context(), req.Matrix); err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL", "บันทึกสิทธิ์ไม่สำเร็จ", nil)
		return
	}
	h.respondConfig(c, http.StatusOK)
}

// roleFail แปล sentinel error ของ role mutation เป็น HTTP response
func roleFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, service.ErrRoleExists):
		ResData(c, http.StatusConflict, "role นี้มีอยู่แล้ว", "ROLE_EXISTS", nil)
	case errors.Is(err, service.ErrRoleLocked):
		ResData(c, http.StatusForbidden, "role นี้ถูกล็อกไว้ แก้ไม่ได้", "ROLE_LOCKED", nil)
	case errors.Is(err, service.ErrInvalidRole):
		ResData(c, http.StatusBadRequest, "role key ไม่ถูกต้อง", "INVALID_ROLE", nil)
	default:
		ResData(c, http.StatusInternalServerError, "เกิดข้อผิดพลาด", "INTERNAL", nil)
	}
}

type addRoleReq struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

// POST /roles — สร้าง role ใหม่ เริ่มด้วยสิทธิ์ว่างเปล่า
func (h *PermissionHandler) AddRole(c *gin.Context) {
	var req addRoleReq
	if err := c.ShouldBindJSON(&req); err != nil || req.Key == "" {
		badRequest(c)
		return
	}
	if err := h.perms.AddRole(c.Request.Context(), req.Key, req.Label); err != nil {
		roleFail(c, err)
		return
	}
	h.respondConfig(c, http.StatusCreated)
}

type renameRoleReq struct {
	Label string `json:"label"`
}

// PATCH /roles/:key — เปลี่ยน label เท่านั้น (rename key ไม่รองรับ — key ผูกกับ matrix/user.role อยู่)
func (h *PermissionHandler) RenameRole(c *gin.Context) {
	var req renameRoleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		badRequest(c)
		return
	}
	if err := h.perms.RenameRole(c.Request.Context(), c.Param("key"), req.Label); err != nil {
		roleFail(c, err)
		return
	}
	h.respondConfig(c, http.StatusOK)
}

// DeleteRole ลบ role — ต้องเช็คก่อนว่ามี console user ใช้ role นี้อยู่ไหม (409 ROLE_IN_USE)
// ก่อนจะปล่อยให้ service.DeleteRole เช็คต่อว่า role นี้เป็น builtin ไหม (403 ROLE_LOCKED)
func (h *PermissionHandler) DeleteRole(c *gin.Context) {
	key := c.Param("key")

	// builtin (admin) ล็อกไว้ก่อน — ลบไม่ได้ไม่ว่าจะมี user ใช้หรือไม่
	if cfg, cerr := h.perms.Config(c.Request.Context()); cerr == nil {
		for _, r := range cfg.Roles {
			if r.Key == key && r.Builtin {
				ResData(c, http.StatusForbidden, "role นี้ถูกล็อกไว้ ลบไม่ได้", "ROLE_LOCKED", nil)
				return
			}
		}
	}

	users, err := h.auth.ListUsers(c.Request.Context())
	if err != nil {
		ResData(c, http.StatusInternalServerError, "INTERNAL", "ตรวจสอบผู้ใช้ไม่สำเร็จ", nil)
		return
	}
	for _, u := range users {
		if string(u.Role) == key {
			ResData(c, http.StatusConflict, "ยังมี user ใช้ role นี้ ย้าย user ออกก่อน", "ROLE_IN_USE", nil)
			return
		}
	}

	if err := h.perms.DeleteRole(c.Request.Context(), key); err != nil {
		roleFail(c, err)
		return
	}
	h.respondConfig(c, http.StatusOK)
}
