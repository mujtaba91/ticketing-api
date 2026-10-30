package event

import (
	"encoding/base64"
	"strings"
	"time"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
)

var ErrInvalidCursor = apperr.Validation(map[string]string{"cursor": "is not a cursor returned by this API"})

// cursor is the sort key of the last event on a page; id breaks ties between events that start at the same time.
type cursor struct {
	StartsAt time.Time
	ID       string
}

// encode is base64 so clients treat the cursor as opaque and don't build their own.
func (c cursor) encode() string {
	return base64.RawURLEncoding.EncodeToString([]byte(c.StartsAt.Format(time.RFC3339Nano) + "|" + c.ID))
}

func decodeCursor(s string) (cursor, error) {
	raw, err := base64.RawURLEncoding.DecodeString(s)
	if err != nil {
		return cursor{}, ErrInvalidCursor
	}
	ts, id, ok := strings.Cut(string(raw), "|")
	if !ok || id == "" {
		return cursor{}, ErrInvalidCursor
	}
	startsAt, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return cursor{}, ErrInvalidCursor
	}
	return cursor{StartsAt: startsAt, ID: id}, nil
}
