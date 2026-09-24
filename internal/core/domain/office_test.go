package domain

import "testing"

func TestNormalizeOrigin(t *testing.T) {
	ok := map[string]string{
		"https://Admin.K11S.com/": "https://admin.k11s.com",
		" https://a.com:443 ":     "https://a.com",
		"http://a.com:80":         "http://a.com",
		"http://localhost:5173":   "http://localhost:5173",
		"HTTPS://a.com":           "https://a.com",
	}
	for in, want := range ok {
		if got, err := NormalizeOrigin(in); err != nil || got != want {
			t.Fatalf("%q: want %q got %q err=%v", in, want, got, err)
		}
	}
	for _, bad := range []string{"", "a.com", "ftp://a.com", "https://a.com/path", "https://a.com?x=1", "https://u:p@a.com"} {
		if _, err := NormalizeOrigin(bad); err == nil {
			t.Fatalf("%q ต้องไม่ผ่าน", bad)
		}
	}
}
