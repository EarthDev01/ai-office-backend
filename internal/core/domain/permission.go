package domain

import "regexp"

// Permission keys — string คงที่ที่ทุกชั้น (service/middleware/FE) ใช้ตรงกัน
//
// อย่าเปลี่ยนค่า string พวกนี้โดยไม่แก้ FE ด้วย เพราะ /role-permissions และ
// /auth/me ส่งค่าพวกนี้ตรง ๆ ให้ FE ใช้เทียบสิทธิ์
const (
	PermOfficeView   = "office.view"
	PermOfficeEdit   = "office.edit"
	PermOfficeDelete = "office.delete"
	PermUserManage   = "user.manage"
	PermAuditView    = "audit.view"

	// แชทของแอดมินเว็บ — อ่านข้ามทุกเว็บได้ ทุกการเปิดอ่านถูกบันทึกลง access_log
	PermConversationRead  = "conversation.read"
	PermVerificationWrite = "verification.write"

	PermUsageView      = "usage.view"      // ดูการใช้ token รายเดือน/รายวัน
	PermDeletionManage = "deletion.manage" // ลบข้อมูลแชทตามคำขอ (PDPA)
	PermSettingsManage = "settings.manage" // แก้ตั้งค่าระบบ (ดูได้ด้วย office.view)
	PermAccessLogView  = "accesslog.view"  // ดูบันทึกการเข้าถึงข้อมูลแชท
)

// roleKeyPattern: role key ต้องขึ้นต้นด้วยตัวอักษร a-z แล้วตามด้วย a-z0-9_- ยาว 2-30 ตัว
// (ตัวแรก + 1-29 ตัวถัดไป) ห้ามมีตัวพิมพ์ใหญ่/เว้นวรรค/อักขระพิเศษอื่น ๆ เพื่อให้ใช้เป็น
// key ของ JSON/Mongo doc และ path param (/roles/:key) ได้อย่างปลอดภัย
var roleKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_-]{1,29}$`)

// ValidRoleKey เช็ครูปแบบ role key ใหม่ (ใช้ตอน AddRole) — ไม่เช็คว่าซ้ำหรือเป็น "admin" ไหม
// (เรื่องนั้นเป็นหน้าที่ของ service ชั้นบน)
func ValidRoleKey(key string) bool {
	return roleKeyPattern.MatchString(key)
}

// RoleDef คือ role หนึ่งตัวในระบบ — เก็บเป็น "list" ที่แอดมินแก้ได้ (เพิ่ม/เปลี่ยนชื่อ/ลบ)
// ยกเว้น Builtin (ปัจจุบันมีแค่ "admin") ที่ล็อกไว้เปลี่ยนชื่อ/ลบไม่ได้ กันแอดมินล็อกตัวเองออก
type RoleDef struct {
	Key     string `json:"key"`
	Label   string `json:"label"`
	Builtin bool   `json:"builtin"`
}

// RoleConfig คือ config ทั้งหมดของระบบ role — รายชื่อ role ที่มีอยู่ + matrix สิทธิ์ของแต่ละ role
//
// Matrix คีย์ด้วย role key (string) ตรงกับ RoleDef.Key ไม่ใช่ domain.Role คงที่แบบเดิม
// เพราะ role ใหม่แอดมินสร้างเองได้ตอนรันไทม์ ไม่รู้ล่วงหน้าตอน compile
type RoleConfig struct {
	Roles  []RoleDef           `json:"roles"`
	Matrix map[string][]string `json:"matrix"`
}

// DefaultRoleConfig ใช้ seed ตอนยังไม่เคยตั้งค่าอะไรเลย (ดู 02-SPEC RBAC)
//
// admin เป็น builtin ล็อกไว้ (เปลี่ยนชื่อ/ลบไม่ได้ มี user.manage เสมอ) ส่วน operator/viewer
// เป็น role ธรรมดาที่แอดมินแก้ไข/ลบทิ้งได้เหมือน role ที่สร้างเองทีหลังทุกประการ
func DefaultRoleConfig() RoleConfig {
	return RoleConfig{
		Roles: []RoleDef{
			{Key: string(RoleAdmin), Label: "ผู้ดูแล", Builtin: true},
			{Key: string(RoleOperator), Label: "ผู้ปฏิบัติงาน", Builtin: false},
			{Key: string(RoleViewer), Label: "ผู้ชม", Builtin: false},
		},
		Matrix: map[string][]string{
			string(RoleAdmin): {
				PermOfficeView, PermOfficeEdit, PermOfficeDelete, PermUserManage, PermAuditView,
				PermConversationRead, PermVerificationWrite,
				PermUsageView, PermDeletionManage, PermSettingsManage, PermAccessLogView,
			},
			string(RoleOperator): {
				PermOfficeView, PermOfficeEdit, PermOfficeDelete, PermConversationRead, PermVerificationWrite, PermUsageView,
			},
			string(RoleViewer): {
				PermOfficeView, PermUsageView,
			},
		},
	}
}

// PermissionCatalog คือรายการ permission key ทั้งหมดที่ระบบรู้จัก พร้อม label ภาษาไทย
// ให้หน้าคอนโซลแสดงตอนแก้ matrix — ค่า Key ต้องตรงกับ const ด้านบนเป๊ะ
func PermissionCatalog() []struct {
	Key   string `json:"key"`
	Label string `json:"label"`
} {
	return []struct {
		Key   string `json:"key"`
		Label string `json:"label"`
	}{
		{Key: PermOfficeView, Label: "ดู office/service"},
		{Key: PermOfficeEdit, Label: "สร้าง/แก้ office และ service"},
		{Key: PermOfficeDelete, Label: "ลบ office/service"},
		{Key: PermUserManage, Label: "จัดการผู้ใช้ และตั้งสิทธิ์ role"},
		{Key: PermAuditView, Label: "ดูประวัติการทำงานของผู้ใช้"},
		{Key: PermConversationRead, Label: "อ่านประวัติแชท (ทุกเว็บ · บันทึกการเปิดอ่านทุกครั้ง)"},
		{Key: PermVerificationWrite, Label: "ตรวจคำตอบของ AI"},
		{Key: PermUsageView, Label: "ดูการใช้งาน token"},
		{Key: PermDeletionManage, Label: "ลบข้อมูลแชทตามคำขอ (PDPA)"},
		{Key: PermSettingsManage, Label: "แก้ตั้งค่าระบบ"},
		{Key: PermAccessLogView, Label: "ดูบันทึกการเข้าถึงข้อมูลแชท"},
	}
}

// Can เช็คว่า role (key) นี้มี permission key นี้ไหม
func (c RoleConfig) Can(role string, perm string) bool {
	for _, p := range c.Matrix[role] {
		if p == perm {
			return true
		}
	}
	return false
}

// For คืน permission key ทั้งหมดของ role นี้ (ใช้ตอบ /auth/me)
func (c RoleConfig) For(role string) []string {
	out := c.Matrix[role]
	if out == nil {
		return []string{}
	}
	return out
}

// RoleExists เช็คว่า role key นี้อยู่ใน roles list ไหม (นี่คือความหมายใหม่ของ "role ถูกต้อง")
func (c RoleConfig) RoleExists(role string) bool {
	for _, r := range c.Roles {
		if r.Key == role {
			return true
		}
	}
	return false
}

// RoleDefByKey คืน RoleDef ของ key นี้ ถ้ามี
func (c RoleConfig) RoleDefByKey(role string) (RoleDef, bool) {
	for _, r := range c.Roles {
		if r.Key == role {
			return r, true
		}
	}
	return RoleDef{}, false
}

// Label คืนชื่อแสดงผลของ role — ถ้าไม่รู้จัก คืน role key ดิบเป็น fallback
func (c RoleConfig) Label(role string) string {
	if r, ok := c.RoleDefByKey(role); ok {
		return r.Label
	}
	return role
}
