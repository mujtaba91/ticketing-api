package httpx

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
)

func TestWriteError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
	}{
		{"invalid", apperr.Invalid("bad input"), http.StatusBadRequest, `"code":"invalid"`},
		{"unauthorized", apperr.Unauthorized("no"), http.StatusUnauthorized, `"code":"unauthorized"`},
		{"forbidden", apperr.Forbidden("no"), http.StatusForbidden, `"code":"forbidden"`},
		{"not found", apperr.NotFound("gone"), http.StatusNotFound, `"code":"not_found"`},
		{"conflict", apperr.Conflict("taken"), http.StatusConflict, `"message":"taken"`},
		{"wrapped", fmt.Errorf("create user: %w", apperr.Conflict("taken")), http.StatusConflict, `"code":"conflict"`},
		{"fields", apperr.Validation(map[string]string{"email": "is required"}), http.StatusBadRequest, `"fields":{"email":"is required"}`},
		{"unknown", errors.New("pq: connection refused"), http.StatusInternalServerError, `"message":"internal server error"`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			WriteError(rec, httptest.NewRequest(http.MethodGet, "/", nil), tt.err)

			if rec.Code != tt.wantStatus {
				t.Errorf("status = %d, want %d", rec.Code, tt.wantStatus)
			}
			if body := rec.Body.String(); !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %s, want it to contain %s", body, tt.wantBody)
			}
			if strings.Contains(rec.Body.String(), "connection refused") {
				t.Error("internal error text leaked to the client")
			}
		})
	}
}

func TestRecoverer(t *testing.T) {
	h := Recoverer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { panic("boom") }))
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("panic value leaked to the client")
	}
}

func TestDecodeJSON(t *testing.T) {
	var dst struct {
		Name string `json:"name"`
	}
	for _, body := range []string{``, `{`, `{"nam":"x"}`} {
		r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
		if err := DecodeJSON(httptest.NewRecorder(), r, &dst); err == nil {
			t.Errorf("body %q: want an error", body)
		}
	}
	r := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(`{"name":"x"}`))
	if err := DecodeJSON(httptest.NewRecorder(), r, &dst); err != nil || dst.Name != "x" {
		t.Errorf("valid body: err = %v, name = %q", err, dst.Name)
	}
}
