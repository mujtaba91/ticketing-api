package auth

import (
	"errors"
	"strings"
	"testing"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
)

func TestRegisterRequestValidate(t *testing.T) {
	valid := registerRequest{Name: "Sara", Email: "sara@example.com", Password: "12345678"}

	tests := []struct {
		name      string
		change    func(*registerRequest)
		wantField string
	}{
		{"valid", func(*registerRequest) {}, ""},
		{"password of 64 characters", func(r *registerRequest) { r.Password = strings.Repeat("a", 64) }, ""},
		{"password of 8 two-byte characters", func(r *registerRequest) { r.Password = strings.Repeat("é", 8) }, ""},
		{"missing name", func(r *registerRequest) { r.Name = "" }, "name"},
		{"long name", func(r *registerRequest) { r.Name = strings.Repeat("a", 101) }, "name"},
		{"missing email", func(r *registerRequest) { r.Email = "" }, "email"},
		{"email without domain", func(r *registerRequest) { r.Email = "sara" }, "email"},
		{"email with display name", func(r *registerRequest) { r.Email = "Sara <sara@example.com>" }, "email"},
		{"missing password", func(r *registerRequest) { r.Password = "" }, "password"},
		{"password of 7 characters", func(r *registerRequest) { r.Password = "1234567" }, "password"},
		{"password of 4 two-byte characters", func(r *registerRequest) { r.Password = strings.Repeat("é", 4) }, "password"},
		{"password of 65 characters", func(r *registerRequest) { r.Password = strings.Repeat("a", 65) }, "password"},
		{"password of 37 two-byte characters", func(r *registerRequest) { r.Password = strings.Repeat("é", 37) }, "password"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := valid
			tt.change(&req)
			err := req.validate()

			if tt.wantField == "" {
				if err != nil {
					t.Fatalf("want no error, got %v", err)
				}
				return
			}
			var appErr *apperr.Error
			if !errors.As(err, &appErr) || appErr.Kind != apperr.KindInvalid {
				t.Fatalf("want an invalid error, got %v", err)
			}
			if _, ok := appErr.Fields[tt.wantField]; !ok || len(appErr.Fields) != 1 {
				t.Errorf("fields = %v, want only %q", appErr.Fields, tt.wantField)
			}
		})
	}
}
