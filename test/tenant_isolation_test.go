package test

import (
	"bytes"
	"net/http"
	"testing"
)

// office/service มาทาง path ได้อย่างเดียว
// ยัดมาทาง body/query/header = ความพยายามเลี่ยงการตรวจ → ปฏิเสธทั้ง request
func TestRejectTenantField(t *testing.T) {
	cases := []struct {
		name   string
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
		{name: "body flat", body: `{"service_id":"PG99"}`},
		{name: "body nested", body: `{"ctx":{"service_id":"PG99"}}`},
	}

	r := newRouter(t)
	tok := devToken("adm_ploy", "K11S", "PG99")

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := do(t, r, req{
				method:  http.MethodGet,
				path:    bootPath(demoKey, "K11S") + tc.query,
				body:    tc.body,
				origin:  officeOrigin,
				token:   tok,
				headers: [][2]string{tc.header},
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
	w := do(t, r, req{
		method: http.MethodGet, path: bootPath(demoKey, "K11S"),
		origin: officeOrigin, token: devToken("adm_ploy", "K11S"),
	})
	if w.Code != http.StatusOK {
		t.Fatalf("request ที่สะอาดต้องผ่าน ได้ %d %s", w.Code, w.Body.String())
	}
}
