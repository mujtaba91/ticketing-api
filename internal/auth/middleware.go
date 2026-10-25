package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/golang-jwt/jwt/v5"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
	"github.com/mujtaba91/ticketing-api/internal/database"
	"github.com/mujtaba91/ticketing-api/internal/httpx"
)

// One error for every rejected token, so the response does not reveal why it was rejected.
var ErrInvalidAccessToken = apperr.Unauthorized("invalid or expired access token")

type ctxKey struct{}

// Caller is the authenticated user that RequireAuth puts in the request context.
type Caller struct {
	ID      string
	IsAdmin bool
}

type authMiddleware struct {
	store     *authStore
	jwtSecret string
}

func NewAuthMiddleware(db database.DBTX, jwtSecret string) *authMiddleware {
	return &authMiddleware{store: NewStore(db), jwtSecret: jwtSecret}
}

// CallerFrom panics on routes without RequireAuth, since calling it there is a programming bug.
func CallerFrom(ctx context.Context) Caller {
	c, ok := ctx.Value(ctxKey{}).(Caller)
	if !ok {
		panic("auth.CallerFrom called on a route without RequireAuth")
	}
	return c
}

func (m *authMiddleware) RequireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString, _ := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer ")
		var claims accessClaims
		_, err := jwt.ParseWithClaims(tokenString, &claims,
			func(*jwt.Token) (any, error) { return []byte(m.jwtSecret), nil },
			jwt.WithValidMethods([]string{"HS256"}),
		)
		if err != nil {
			httpx.WriteError(w, r, ErrInvalidAccessToken)
			return
		}
		user, err := m.store.UserByID(r.Context(), claims.Subject)
		if errors.Is(err, ErrUserNotFound) {
			httpx.WriteError(w, r, ErrInvalidAccessToken)
			return
		}
		if err != nil {
			httpx.WriteError(w, r, err)
			return
		}
		if user.TokenVersion != claims.TokenVersion {
			httpx.WriteError(w, r, ErrInvalidAccessToken)
			return
		}
		ctx := context.WithValue(r.Context(), ctxKey{}, Caller{
			ID:      user.ID,
			IsAdmin: user.IsAdmin,
		})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
