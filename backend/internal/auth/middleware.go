package auth

import (
	"context"
	"net/http"
	"strings"
)

type contextKey string

const userContextKey contextKey = "authenticated-user"

type TokenVerifier interface {
	Verify(context.Context, string) (*User, error)
}

func UserFromContext(ctx context.Context) (*User, bool) {
	user, ok := ctx.Value(userContextKey).(*User)
	return user, ok
}

func Middleware(verifier TokenVerifier) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			if authHeader == "" {
				http.Error(w, "missing Authorization header", http.StatusUnauthorized)
				return
			}

			parts := strings.Fields(authHeader)

			if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
				http.Error(w, "invalid Authorization header", http.StatusUnauthorized)
				return
			}

			user, err := verifier.Verify(r.Context(), parts[1])
			if err != nil {
				http.Error(w, "invalid access token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(
				r.Context(),
				userContextKey,
				user,
			)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
