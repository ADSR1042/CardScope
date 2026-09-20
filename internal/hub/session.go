package hub

import (
	"net/http"
	"net/url"
	"time"
)

func sameOrigin(r *http.Request) bool {
	o := r.Header.Get("Origin")
	if o == "" {
		return true
	}
	u, e := url.Parse(o)
	return e == nil && u.Host == r.Host && (u.Scheme == "http" || u.Scheme == "https")
}
func (h *Hub) session(r *http.Request) (user, error) {
	var u user
	c, e := r.Cookie("gpu_session")
	if e != nil {
		return u, e
	}
	e = h.DB.QueryRow(`
		SELECT a.username, a.role, s.csrf
		FROM sessions AS s
		JOIN accounts AS a ON s.username = a.username
		WHERE s.hash = ? AND s.expires > ?
	`, hash(c.Value), time.Now().UnixMilli()).Scan(&u.Name, &u.Role, &u.CSRF)
	return u, e
}
