package test

import (
	"bytes"
	"net/http"
	"testing"
)

// office/service มาทาง path ได้อย่างเดียว · ของผู้ใช้มาจากตั๋ว
// ยัดมาทาง body/query/header = ความพยายามเลี่ยงการตรวจ → ปฏิเสธทั้ง request (AC-29)
func TestRejectTenantField(t *testing.T) {
	cases := []struct {
		name   string
		path   string
		method string
		query  string
		header [2]string
		body   string
	}{
		{name: "query service_id", query: "?service_id=PG99"},
		{name: "query service", query: "?service=PG99"},
		{name: "query office_id", query: "?office_id=other"},
		{name: "query website_id", query: "?website_id=PG99"},
		{name: "query tenant", query: "?tenant=PG99"},
		{name: "header X-Service-Id", header: [2]string{"X-Service-Id", "PG99"}},
		{name: "header X-Website-Id", header: [2]string{"X-Website-Id", "PG99"}},
		{name: "chat body flat", path: "/chat", method: http.MethodPost, body: `{"text":"x","service_id":"PG99"}`},
		{name: "chat body nested", path: "/chat", method: http.MethodPost, body: `{"text":"x","ctx":{"service_id":"PG99"}}`},
		{name: "session body office", path: "/session", method: http.MethodPost, body: `{"kind":"sample-kind","user":{"id":"x","office_id":"acme"},"grant":"g"}`},
	}

	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			path, method := "/bootstrap", http.MethodGet
			if tc.path != "" {
				path, method = tc.path, tc.method
			}
			w := do(t, r, req{
				method: method, path: basePath(demoKey, "K11S") + path + tc.query, body: tc.body,
				origin: officeOrigin, token: tk, headers: [][2]string{tc.header},
			})
			if w.Code != http.StatusBadRequest {
				t.Fatalf("อยากได้ 400 ได้ %d body=%s", w.Code, w.Body.String())
			}
			if !bytes.Contains(w.Body.Bytes(), []byte("UNEXPECTED_TENANT_FIELD")) {
				t.Fatalf("อยากได้ UNEXPECTED_TENANT_FIELD ได้ %s", w.Body.String())
			}
		})
	}
}

func TestAllowsCleanRequest(t *testing.T) {
	r := newRouter(t)
	tk := ticketFor(t, r, demoKey, "K11S", secretK11S, adminUser())
	if code, _, raw := getBoot(t, r, demoKey, "K11S", tk); code != http.StatusOK {
		t.Fatalf("request ที่สะอาดต้องผ่าน ได้ %d %s", code, raw)
	}
}
