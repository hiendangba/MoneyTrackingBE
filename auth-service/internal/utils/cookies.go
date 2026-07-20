package utils

import (
	"net/http"
	"strings"
	"time"
)

type CookieConfig struct {
	Secure   bool
	SameSite string
	Domain   string
}

func SetTokenCookie(w http.ResponseWriter, name, value string, ttl time.Duration, cfg CookieConfig) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: parseSameSite(cfg.SameSite),
		MaxAge:   int(ttl.Seconds()),
		Domain:   cfg.Domain,
	}
	http.SetCookie(w, cookie)
}

func ClearTokenCookie(w http.ResponseWriter, name string, cfg CookieConfig) {
	cookie := &http.Cookie{
		Name:     name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   cfg.Secure,
		SameSite: parseSameSite(cfg.SameSite),
		MaxAge:   -1,
		Domain:   cfg.Domain,
	}
	http.SetCookie(w, cookie)
}

func GetCookieValue(r *http.Request, name string) string {
	cookie, err := r.Cookie(name)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func parseSameSite(value string) http.SameSite {
	switch strings.ToLower(value) {
	case "strict":
		return http.SameSiteStrictMode
	case "lax":
		return http.SameSiteLaxMode
	case "none":
		return http.SameSiteNoneMode
	default:
		return http.SameSiteDefaultMode
	}
}
