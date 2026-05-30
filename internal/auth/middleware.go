package auth

import (
	"net/http"
	"strings"
)

func RequireVirtualKey(store VirtualKeyStore, unauthorized func(http.ResponseWriter, *http.Request)) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			token := extractGatewayToken(r)
			if token == "" {
				unauthorized(w, r)
				return
			}

			subject, authenticated, err := AuthenticateVirtualKey(r.Context(), store, token)
			if err != nil {
				http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
				return
			}
			if !authenticated {
				unauthorized(w, r)
				return
			}

			next.ServeHTTP(w, r.WithContext(WithSubject(r.Context(), subject)))
		})
	}
}

func extractGatewayToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if token, ok := strings.CutPrefix(header, "Bearer "); ok {
		token = strings.TrimSpace(token)
		if token != "" {
			return token
		}
	}
	return strings.TrimSpace(r.Header.Get("x-api-key"))
}
