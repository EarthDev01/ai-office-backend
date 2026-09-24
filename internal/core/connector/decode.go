package connector

import (
	"encoding/json"
	"fmt"
)

// Decoded = ผลของการแกะ response ของ host 1 ครั้ง (ใช้ทั้งตอนรันจริงและ contract test — ต้องเป็นโค้ดเดียวกัน)
type Decoded struct {
	Data     any
	NotFound bool
	ErrCode  string // "" = สำเร็จ · denied | unauthorized | busy | http_error | bad_json | schema_mismatch | host_error
	Err      error
}

// DecodeResponse ตัด envelope · ตัดสิน "ไม่พบ" · ตรวจ schema — ไม่มี I/O
//
//	HTTP 401/403/429 → unauthorized/denied/busy
//	HTTP อื่นนอก 2xx → not_found ถ้า status อยู่ใน not_found_codes (host ตอบ 404 = ค้นแล้วไม่พบ · B-11) ไม่งั้น http_error
//	2xx → envelope: code ต้องอยู่ใน ok_codes (หรือ not_found_codes = ไม่พบ) แล้วเอา data_field · raw = ทั้งก้อน
//	แล้วตรวจ schema (P-11) — ผิด = schema_mismatch (ห้ามโชว์ตัวเลขบางส่วน)
func DecodeResponse(h HostAPI, cl Call, status int, body []byte) Decoded {
	fail := func(code string, err error) Decoded { return Decoded{ErrCode: code, Err: err} }
	switch {
	case status == 403:
		return fail("denied", fmt.Errorf("host ปฏิเสธสิทธิ์"))
	case status == 401:
		return fail("unauthorized", fmt.Errorf("host ไม่รับกุญแจ"))
	case status == 429:
		return fail("busy", fmt.Errorf("host ตอบ 429"))
	case status < 200 || status >= 300:
		if containsCode(cl.NotFoundCodes, status) {
			return Decoded{NotFound: true}
		}
		return fail("http_error", fmt.Errorf("host ตอบ %d", status))
	}
	var parsed any
	if err := json.Unmarshal(body, &parsed); err != nil {
		return fail("bad_json", err)
	}
	data := parsed
	if cl.Envelope != "raw" {
		env := h.Envelope
		code, ok := ToFloat(GetPath(parsed, env.CodeField))
		if !ok {
			return fail("schema_mismatch", fmt.Errorf("ไม่มี %s ใน response", env.CodeField))
		}
		if containsCode(cl.NotFoundCodes, int(code)) {
			return Decoded{NotFound: true}
		}
		if !containsCode(env.OkCodes, int(code)) {
			return fail("host_error", fmt.Errorf("host code %v", code))
		}
		data = GetPath(parsed, env.DataField)
	}
	if cl.Schema != nil {
		if err := cl.Schema.Validate(data); err != nil {
			return fail("schema_mismatch", err)
		}
	}
	return Decoded{Data: data}
}

func containsCode(list []int, v int) bool {
	for _, x := range list {
		if x == v {
			return true
		}
	}
	return false
}
