package httpx

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseLimit(t *testing.T) {
	tests := []struct {
		query   string
		want    int
		wantErr bool
	}{
		{"", DefaultLimit, false},
		{"?limit=1", 1, false},
		{"?limit=100", 100, false},
		{"?limit=0", 0, true},
		{"?limit=101", 0, true},
		{"?limit=-5", 0, true},
		{"?limit=ten", 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.query, func(t *testing.T) {
			got, err := ParseLimit(httptest.NewRequest(http.MethodGet, "/events"+tt.query, nil))
			if (err != nil) != tt.wantErr {
				t.Fatalf("err = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Errorf("limit = %d, want %d", got, tt.want)
			}
		})
	}
}
