//go:build integration

package integration

import (
	"encoding/base64"
	"net/http"
	"net/url"
	"testing"
	"time"
)

type eventPage struct {
	Items []struct {
		ID       string    `json:"id"`
		StartsAt time.Time `json:"starts_at"`
	} `json:"items"`
	NextCursor *string `json:"next_cursor"`
}

func TestListEventsPagesWithoutGapsOrDuplicates(t *testing.T) {
	owner := registerUser(t)
	orgID := createOrg(t, owner.AccessToken)

	// 20 events share one start time, so a page of 10 always ends inside the tie and needs the id tie-break.
	base := time.Now().Add(48 * time.Hour).Truncate(time.Second)
	seenCount := map[string]int{}
	for i := range 25 {
		startsAt := base
		if i >= 20 {
			startsAt = base.Add(time.Minute)
		}
		seenCount[createEventAt(t, owner.AccessToken, orgID, 10, startsAt)] = 0
	}

	const limit = 10
	path := "/events?limit=10"
	var prev struct {
		at time.Time
		id string
	}
	for pages := 0; ; pages++ {
		if pages > 1000 {
			t.Fatal("paging did not end")
		}
		var p eventPage
		if status := doJSON(t, http.MethodGet, path, "", nil, &p); status != http.StatusOK {
			t.Fatalf("GET %s: status %d, want 200", path, status)
		}
		if p.NextCursor != nil && len(p.Items) != limit {
			t.Fatalf("page %d has %d items and a next cursor, want %d", pages, len(p.Items), limit)
		}
		for _, e := range p.Items {
			if e.StartsAt.Before(prev.at) || (e.StartsAt.Equal(prev.at) && e.ID <= prev.id) {
				t.Fatalf("event %s at %s is out of order after %s at %s", e.ID, e.StartsAt, prev.id, prev.at)
			}
			prev.at, prev.id = e.StartsAt, e.ID
			if _, ok := seenCount[e.ID]; ok {
				seenCount[e.ID]++
			}
		}
		if p.NextCursor == nil {
			break
		}
		path = "/events?limit=10&cursor=" + url.QueryEscape(*p.NextCursor)
	}

	for id, seen := range seenCount {
		if seen != 1 {
			t.Errorf("event %s seen %d times, want 1", id, seen)
		}
	}
}

func TestListEventsRejectsBadInput(t *testing.T) {
	forged := base64.RawURLEncoding.EncodeToString([]byte("2026-11-01T18:30:00Z|not-a-uuid"))
	for _, query := range []string{"limit=0", "limit=101", "limit=abc", "cursor=%25%25", "cursor=" + forged} {
		if status := doJSON(t, http.MethodGet, "/events?"+query, "", nil, nil); status != http.StatusBadRequest {
			t.Errorf("GET /events?%s: status %d, want 400", query, status)
		}
	}
}
