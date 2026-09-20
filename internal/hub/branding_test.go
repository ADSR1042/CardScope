package hub

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestLoginCopyPermissionsAndPersistence(t *testing.T) {
	h := testHub(t)
	if w := request(h, "GET", "/login-copy", nil, "", "", ""); w.Code != 200 {
		t.Fatal(w.Code)
	}
	admin, csrf := login(t, h, "admin")
	viewer, vcsrf := login(t, h, "viewer")
	value := LoginCopy{"实验室", "GPU\n资源面板", "<script>alert(1)</script>", ""}
	for _, auth := range []struct {
		cookie, csrf string
		code         int
	}{{"", "", 401}, {viewer, vcsrf, 403}, {admin, "", 403}} {
		if w := request(h, "PUT", "/login-copy", value, auth.cookie, auth.csrf, ""); w.Code != auth.code {
			t.Fatalf("got %d want %d", w.Code, auth.code)
		}
	}
	if w := request(h, "PUT", "/login-copy", value, admin, csrf, ""); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	var raw string
	if err := h.DB.QueryRow("SELECT value FROM settings WHERE key='login_copy'").Scan(&raw); err != nil {
		t.Fatal(err)
	}
	var stored LoginCopy
	if err := json.Unmarshal([]byte(raw), &stored); err != nil || stored != value {
		t.Fatal("not persisted", err)
	}
	w := request(h, "GET", "/login-copy", nil, "", "", "")
	var public LoginCopy
	if err := json.Unmarshal(w.Body.Bytes(), &public); err != nil || public != value {
		t.Fatal("public copy mismatch", err)
	}
	value.Title = strings.Repeat("字", 121)
	if w := request(h, "PUT", "/login-copy", value, admin, csrf, ""); w.Code != 400 {
		t.Fatal("unbounded title", w.Code)
	}
}
