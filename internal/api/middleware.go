package api

import (
	"net/http"
	"strings"
)

var mutateMethods = map[string]bool{
	http.MethodPut:    true,
	http.MethodPatch:  true,
	http.MethodDelete: true,
}

func requireAuth(adminKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !mutateMethods[r.Method] {
				next.ServeHTTP(w, r)
				return
			}

			if adminKey == "" {
				next.ServeHTTP(w, r)
				return
			}

			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			if token != adminKey {
				respond(w, http.StatusUnauthorized, errorBody(errUnauthorized))
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

var errUnauthorized = authError("unauthorized: valid Bearer token required")

type authError string

func (e authError) Error() string { return string(e) }
