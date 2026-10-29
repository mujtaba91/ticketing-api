//go:build integration

package integration

import (
	"net/http"
	"sync"
	"testing"
)

func TestRefreshTokenReuseRevokesSession(t *testing.T) {
	first := registerUser(t)

	status, rotated := refresh(t, first.RefreshToken)
	if status != http.StatusOK {
		t.Fatalf("first refresh: status %d, want %d", status, http.StatusOK)
	}

	status, _ = refresh(t, first.RefreshToken)
	if status != http.StatusUnauthorized {
		t.Fatalf("reusing the old token: status %d, want %d", status, http.StatusUnauthorized)
	}

	// The rotated token was never reused, so this only fails if the whole session was revoked.
	status, _ = refresh(t, rotated.RefreshToken)
	if status != http.StatusUnauthorized {
		t.Fatalf("rotated token after reuse: status %d, want %d", status, http.StatusUnauthorized)
	}
}

func TestRefreshTokenConcurrentRefreshes(t *testing.T) {
	const n = 2

	type result struct {
		status int
		tokens tokens
	}

	start := make(chan struct{})
	results := make(chan result, n)
	var wg sync.WaitGroup

	first := registerUser(t)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status, got := refresh(t, first.RefreshToken)
			results <- result{status, got}
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var winner tokens
	var ok, unauthorized int
	for r := range results {
		switch r.status {
		case http.StatusOK:
			ok++
			winner = r.tokens
		case http.StatusUnauthorized:
			unauthorized++
		default:
			t.Errorf("unexpected status %d", r.status)
		}
	}
	if ok != 1 || unauthorized != n-1 {
		t.Fatalf("got %d OK and %d unauthorized, want 1 and %d", ok, unauthorized, n-1)
	}

	// The losing request counts as reuse, so even the winner's new token must be revoked.
	status, _ := refresh(t, winner.RefreshToken)
	if status != http.StatusUnauthorized {
		t.Fatalf("winner's token after concurrent reuse: status %d, want %d", status, http.StatusUnauthorized)
	}
}
