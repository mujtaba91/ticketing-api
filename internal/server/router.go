package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/auth"
	"github.com/mujtaba91/ticketing-api/internal/booking"
	"github.com/mujtaba91/ticketing-api/internal/config"
	"github.com/mujtaba91/ticketing-api/internal/event"
	"github.com/mujtaba91/ticketing-api/internal/httpx"
	"github.com/mujtaba91/ticketing-api/internal/organization"
)

func NewRouter(pool *pgxpool.Pool, cfg config.Config) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(httpx.Recoverer)

	r.Get("/health", health(pool))
	r.Route("/auth", auth.NewHandler(pool, cfg.JWTSecret, cfg.AccessTokenTTL, cfg.RefreshTokenTTL).Routes)

	events := event.NewHandler(pool)
	events.PublicRoutes(r)

	authMw := auth.NewAuthMiddleware(pool, cfg.JWTSecret)
	r.Group(func(r chi.Router) {
		r.Use(authMw.RequireAuth)
		r.Route("/organizations", organization.NewHandler(pool).Routes)
		events.ProtectedRoutes(r)
		booking.NewHandler(pool).Routes(r)
	})

	return r
}

func health(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		status, code := "ok", http.StatusOK
		if err := pool.Ping(ctx); err != nil {
			slog.ErrorContext(ctx, "health check failed", "error", err)
			status, code = "unavailable", http.StatusServiceUnavailable
		}

		httpx.WriteJSON(w, code, map[string]string{"status": status})
	}
}
