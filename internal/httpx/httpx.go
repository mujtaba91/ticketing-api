package httpx

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
)

const maxBodyBytes = 1 << 20

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("write response", "error", err)
	}
}

// DecodeJSON caps the body size and rejects unknown fields, so a typo in a field name is not silently ignored.
func DecodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	r.Body = http.MaxBytesReader(w, r.Body, maxBodyBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return apperr.Invalid("request body must be valid JSON with only the documented fields")
	}
	return nil
}

type errorBody struct {
	Error errorDetail `json:"error"`
}

type errorDetail struct {
	Code    string            `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

var statusByKind = map[apperr.Kind]int{
	apperr.KindInvalid:      http.StatusBadRequest,
	apperr.KindUnauthorized: http.StatusUnauthorized,
	apperr.KindForbidden:    http.StatusForbidden,
	apperr.KindNotFound:     http.StatusNotFound,
	apperr.KindConflict:     http.StatusConflict,
}

// WriteError answers with the status of a known error kind; anything else is logged and hidden behind a generic 500.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	var appErr *apperr.Error
	if errors.As(err, &appErr) {
		if status, ok := statusByKind[appErr.Kind]; ok {
			WriteJSON(w, status, errorBody{errorDetail{string(appErr.Kind), appErr.Message, appErr.Fields}})
			return
		}
	}
	slog.ErrorContext(r.Context(), "unhandled error", requestAttrs(r, "error", err)...)
	writeInternal(w)
}

// Recoverer turns a panic into the same logged 500 as any other unknown error.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// ErrAbortHandler is net/http's own signal to drop the connection.
			if rec == http.ErrAbortHandler {
				panic(rec)
			}
			slog.ErrorContext(r.Context(), "panic recovered", requestAttrs(r, "panic", rec, "stack", string(debug.Stack()))...)
			writeInternal(w)
		}()
		next.ServeHTTP(w, r)
	})
}

// WriteMethodNotAllowed has no apperr kind, since 405 is about routing, not the domain.
func WriteMethodNotAllowed(w http.ResponseWriter) {
	WriteJSON(w, http.StatusMethodNotAllowed, errorBody{errorDetail{Code: "method_not_allowed", Message: "method not allowed"}})
}

func writeInternal(w http.ResponseWriter) {
	WriteJSON(w, http.StatusInternalServerError, errorBody{errorDetail{Code: "internal", Message: "internal server error"}})
}

func requestAttrs(r *http.Request, attrs ...any) []any {
	return append(attrs, "method", r.Method, "path", r.URL.Path, "request_id", middleware.GetReqID(r.Context()))
}
