package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
