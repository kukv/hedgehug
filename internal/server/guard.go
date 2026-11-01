package server

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
)

// guardHost refuses a request whose Host is not 127.0.0.1:<port> or
// localhost:<port>, static files included: a page of another site that
// a DNS name points at 127.0.0.1 cannot reach the server.
func (s *Server) guardHost(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		port := strconv.Itoa(int(s.port.Load()))
		if r.Host != "127.0.0.1:"+port && r.Host != "localhost:"+port {
			writeProblem(w, newProblem(http.StatusForbidden, "host.refused", "the Host header must be 127.0.0.1:"+port+" or localhost:"+port))
			return
		}
		h.ServeHTTP(w, r)
	})
}

// requireToken refuses an API request without Authorization: Bearer <token>.
func (s *Server) requireToken(ctx huma.Context, next func(huma.Context)) {
	got, ok := strings.CutPrefix(ctx.Header("Authorization"), "Bearer ")
	if !ok || subtle.ConstantTimeCompare([]byte(got), []byte(s.opt.Token)) != 1 {
		p := newProblem(http.StatusUnauthorized, "auth.token", "the request needs Authorization: Bearer <token>")
		ctx.SetHeader("Content-Type", "application/problem+json")
		ctx.SetStatus(p.Status)
		_ = json.NewEncoder(ctx.BodyWriter()).Encode(p)
		return
	}
	next(ctx)
}
