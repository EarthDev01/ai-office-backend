package test

import (
	"net/http"
	"net/url"
	"testing"

	"ai-office-backend/internal/core/domain"
)

func TestGroups_CreateDomainInGroup(t *testing.T) {
	r := newRouter(t)

	code, g := consoleJSON(t, r, "POST", "/api/ai/admin/groups", `{"name":"office-v10"}`, consoleToken)
	if code != http.StatusCreated || g["name"] != "office-v10" {
		t.Fatalf("สร้างกลุ่ม = %d %v", code, g)
	}
	gid := g["id"].(string)
	if code, _ := consoleJSON(t, r, "POST", "/api/ai/admin/groups", `{"name":"OFFICE-V10"}`, consoleToken); code != http.StatusBadRequest {
		t.Fatalf("ชื่อกลุ่มซ้ำ (ไม่สนตัวพิมพ์) ต้องได้ 400 ได้ %d", code)
	}

	// domain ต้องอยู่ในกลุ่มที่มีจริง
	for name, body := range map[string]string{
		"ไม่ระบุกลุ่ม":   `{"id":"staging","origin":"https://staging.example"}`,
		"กลุ่มไม่มีจริง": `{"id":"staging","group_id":"nope","origin":"https://staging.example"}`,
		"URL ผิดรูป":     `{"id":"staging","group_id":"` + gid + `","origin":"staging.example"}`,
		"URL ซ้ำ":        `{"id":"staging","group_id":"` + gid + `","origin":"` + officeOrigin + `"}`,
	} {
		if code, _ := admin(t, r, http.MethodPost, "/api/ai/admin/offices", body); code == http.StatusCreated {
			t.Errorf("%s: ต้องสร้างไม่ได้", name)
		}
	}
	code, o := admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"id":"staging","group_id":"`+gid+`","origin":"https://Staging.Example/"}`)
	if code != http.StatusCreated || o.GroupID != gid || len(o.AllowedOrigins) != 1 || o.AllowedOrigins[0] != "https://staging.example" {
		t.Fatalf("สร้าง domain = %d %+v", code, o)
	}

	// ย้าย domain ไปกลุ่มอื่นได้ · ย้ายไปกลุ่มที่ไม่มีไม่ได้
	if code, o := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/staging", `{"group_id":"`+testGroupID+`"}`); code != 200 || o.GroupID != testGroupID {
		t.Fatalf("ย้ายกลุ่ม = %d %+v", code, o)
	}
	if code, _ := admin(t, r, http.MethodPatch, "/api/ai/admin/offices/staging", `{"group_id":"nope"}`); code != http.StatusBadRequest {
		t.Fatalf("ย้ายไปกลุ่มที่ไม่มีต้องได้ 400 ได้ %d", code)
	}
}

func TestGroups_RenameDeleteAudit(t *testing.T) {
	r := newRouter(t)
	_, g := consoleJSON(t, r, "POST", "/api/ai/admin/groups", `{"name":"เดิม"}`, consoleToken)
	gid := g["id"].(string)

	if code, g := consoleJSON(t, r, "PATCH", "/api/ai/admin/groups/"+gid, `{"name":"ใหม่"}`, consoleToken); code != 200 || g["name"] != "ใหม่" {
		t.Fatalf("เปลี่ยนชื่อ = %d %v", code, g)
	}
	// กลุ่มที่ยังมี domain ลบไม่ได้
	admin(t, r, http.MethodPost, "/api/ai/admin/offices", `{"id":"d1","group_id":"`+gid+`"}`)
	if code, _ := consoleJSON(t, r, "DELETE", "/api/ai/admin/groups/"+gid, "", consoleToken); code != http.StatusBadRequest {
		t.Fatalf("ลบกลุ่มที่มี domain ต้องได้ 400 ได้ %d", code)
	}
	admin(t, r, http.MethodDelete, "/api/ai/admin/offices/d1", "")
	if code, _ := consoleJSON(t, r, "DELETE", "/api/ai/admin/groups/"+gid, "", consoleToken); code != 200 {
		t.Fatalf("ลบกลุ่มว่าง = %d", code)
	}
	_, list := consoleJSON(t, r, "GET", "/api/ai/admin/groups", "", consoleToken)
	if len(list["data"].([]any)) != 1 { // เหลือกลุ่มตั้งต้นของเทส
		t.Fatalf("กลุ่มที่เหลือ = %v", list)
	}
	for _, a := range []string{domain.AuditGroupCreate, domain.AuditGroupUpdate, domain.AuditGroupDelete} {
		if page := queryAudit(t, r, consoleToken, url.Values{"action": {a}}); page.Total != 1 {
			t.Errorf("audit %s = %d รายการ", a, page.Total)
		}
	}
}

func TestGroups_ViewerCannotEdit(t *testing.T) {
	r := newRouter(t)
	tok := viewerToken(t)
	if code, _ := consoleJSON(t, r, "GET", "/api/ai/admin/groups", "", tok); code != 200 {
		t.Fatalf("viewer อ่านกลุ่มได้ ได้ %d", code)
	}
	if code, _ := consoleJSON(t, r, "POST", "/api/ai/admin/groups", `{"name":"x"}`, tok); code != http.StatusForbidden {
		t.Fatalf("viewer สร้างกลุ่มไม่ได้ ได้ %d", code)
	}
}
