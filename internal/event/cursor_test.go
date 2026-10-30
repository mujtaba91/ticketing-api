package event

import (
	"encoding/base64"
	"errors"
	"testing"
	"time"
)

func TestCursorRoundTrip(t *testing.T) {
	want := cursor{StartsAt: time.Date(2026, 11, 1, 18, 30, 0, 123456000, time.UTC), ID: "0b9e6a2c-3f4d-4e5a-9b8c-1d2e3f4a5b6c"}

	got, err := decodeCursor(want.encode())
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !got.StartsAt.Equal(want.StartsAt) || got.ID != want.ID {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

func TestDecodeCursorRejectsGarbage(t *testing.T) {
	b64 := func(s string) string { return base64.RawURLEncoding.EncodeToString([]byte(s)) }
	for _, s := range []string{"%%%", b64("no-separator"), b64("not-a-time|abc"), b64("2026-11-01T18:30:00Z|")} {
		if _, err := decodeCursor(s); !errors.Is(err, ErrInvalidCursor) {
			t.Errorf("decodeCursor(%q) err = %v, want ErrInvalidCursor", s, err)
		}
	}
}
