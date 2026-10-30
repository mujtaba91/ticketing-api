package httpx

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
)

const (
	DefaultLimit = 20
	MaxLimit     = 100
)

// ParseLimit reads ?limit=, falling back to DefaultLimit when it is missing.
func ParseLimit(r *http.Request) (int, error) {
	raw := r.URL.Query().Get("limit")
	if raw == "" {
		return DefaultLimit, nil
	}
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 || n > MaxLimit {
		return 0, apperr.Validation(map[string]string{"limit": fmt.Sprintf("must be a number between 1 and %d", MaxLimit)})
	}
	return n, nil
}
