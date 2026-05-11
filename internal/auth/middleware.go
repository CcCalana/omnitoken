package auth

import (
	"net/http"
	"strings"
)

func RequireVirtualKey(store VirtualKeyStore, unauthorized func(http.ResponseWriter)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := strings.TrimSpace(r.Header.Get("Authorization"))
			token, ok := strings.CutPrefix(header, "Bearer ")
			if !ok || strings.TrimSpace(token) == "" {
				unauthorized(w)
				return
			}

			subject, authenticated, err := AuthenticateVirtualKey(r.Context(), store, token)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if !authenticated {
				unauthorized(w)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithSubject(r.Context(), subject)))
		})
	}
}
