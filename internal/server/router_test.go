package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"

	"github.com/mujtaba91/ticketing-api/docs"
	"github.com/mujtaba91/ticketing-api/internal/config"
)

func TestRouterErrorsUseUnifiedFormat(t *testing.T) {
	// A nil pool is fine: none of these requests reaches a handler that queries the database.
	router := NewRouter(nil, config.Config{JWTSecret: "test"})

	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantCode   string
		wantAllow  string
	}{
		{"unknown path", http.MethodGet, "/nope", http.StatusNotFound, "not_found", ""},
		{"wrong method", http.MethodDelete, "/health", http.StatusMethodNotAllowed, "method_not_allowed", "GET"},
		{"wrong method on a sub-router", http.MethodGet, "/auth/login", http.StatusMethodNotAllowed, "method_not_allowed", "POST"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, httptest.NewRequest(tt.method, tt.path, nil))

			if rec.Code != tt.wantStatus {
				t.Fatalf("status %d, want %d", rec.Code, tt.wantStatus)
			}
			var body struct {
				Error struct {
					Code string `json:"code"`
				} `json:"error"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("body is not JSON: %q", rec.Body.String())
			}
			if body.Error.Code != tt.wantCode {
				t.Errorf("code %q, want %q", body.Error.Code, tt.wantCode)
			}
			if got := rec.Header().Get("Allow"); got != tt.wantAllow {
				t.Errorf("Allow %q, want %q", got, tt.wantAllow)
			}
		})
	}
}

// The spec is hand-written, so this catches a route added to the router but not to the docs.
func TestEveryRouteIsDocumented(t *testing.T) {
	spec, err := fs.ReadFile(docs.FS, "openapi.yaml")
	if err != nil {
		t.Fatalf("read spec: %v", err)
	}
	router := NewRouter(nil, config.Config{JWTSecret: "test"}).(chi.Routes)

	err = chi.Walk(router, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		route = strings.TrimSuffix(route, "/")
		if strings.HasPrefix(route, "/docs") {
			return nil
		}
		if !strings.Contains(string(spec), "\n  "+route+":\n") {
			t.Errorf("%s %s is not in docs/openapi.yaml", method, route)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("walk routes: %v", err)
	}
}
