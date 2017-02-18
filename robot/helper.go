package robot

import "net/http"

func getRemoteIp(r *http.Request) string {
	if r.Header.Get("X-Forwarded-For") != "" {
		return r.Header.Get("X-Forwarded-For")
	}
	return r.RemoteAddr
}