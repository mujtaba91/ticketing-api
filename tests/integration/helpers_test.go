//go:build integration

package integration

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
	"time"
)

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// doJSON is sendJSON for the test goroutine: any transport or decode error fails the test at once.
func doJSON(t *testing.T, method, path, accessToken string, body, out any) int {
	t.Helper()

	status, err := sendJSON(method, path, accessToken, body, out)
	if err != nil {
		t.Fatal(err)
	}
	return status
}

// sendJSON returns errors instead of calling t.Fatal, so goroutines can use it; t.Fatal works only on the test goroutine.
func sendJSON(method, path, accessToken string, body, out any) (int, error) {
	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			return 0, fmt.Errorf("encode request: %w", err)
		}
	}
	req, err := http.NewRequest(method, baseURL+path, &reqBody)
	if err != nil {
		return 0, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return 0, fmt.Errorf("%s %s: decode response with status %d: %w", method, path, resp.StatusCode, err)
		}
	}
	return resp.StatusCode, nil
}

var emailCounter atomic.Int64

// uniqueEmail lets tests share one database without cleaning it between tests.
func uniqueEmail() string {
	return fmt.Sprintf("user%d@example.com", emailCounter.Add(1))
}

// user embeds tokens, so u.AccessToken works directly.
type user struct {
	tokens
	Email string
}

// registerUser creates a new user and fails the test unless it gets a token pair back.
func registerUser(t *testing.T) user {
	t.Helper()

	u := user{Email: uniqueEmail()}
	status := doJSON(t, http.MethodPost, "/auth/register", "", map[string]string{
		"name":     "Test User",
		"email":    u.Email,
		"password": "password123",
	}, &u.tokens)
	if status != http.StatusCreated {
		t.Fatalf("register: status %d, want %d", status, http.StatusCreated)
	}
	return u
}

// refresh calls POST /auth/refresh; tokens are empty when the status is not 200.
func refresh(t *testing.T, refreshToken string) (int, tokens) {
	t.Helper()

	status, got, err := tryRefresh(refreshToken)
	if err != nil {
		t.Fatal(err)
	}
	return status, got
}

// tryRefresh is refresh for goroutines.
func tryRefresh(refreshToken string) (int, tokens, error) {
	var got tokens
	status, err := sendJSON(http.MethodPost, "/auth/refresh", "", map[string]string{"refresh_token": refreshToken}, &got)
	return status, got, err
}

type idResponse struct {
	ID string `json:"id"`
}

// createOrg makes the token's user the Owner of a new organization and returns its id.
func createOrg(t *testing.T, accessToken string) string {
	t.Helper()

	var got idResponse
	status := doJSON(t, http.MethodPost, "/organizations", accessToken, map[string]string{"name": "Test Org"}, &got)
	if status != http.StatusCreated {
		t.Fatalf("create organization: status %d, want %d", status, http.StatusCreated)
	}
	return got.ID
}

func addStaff(t *testing.T, ownerToken, orgID, email string) {
	t.Helper()

	status := doJSON(t, http.MethodPost, "/organizations/"+orgID+"/members", ownerToken, map[string]string{"email": email}, nil)
	if status != http.StatusCreated {
		t.Fatalf("add staff: status %d, want %d", status, http.StatusCreated)
	}
}

// createEvent creates an event starting tomorrow and returns its id.
func createEvent(t *testing.T, accessToken, orgID string, maxSeats int) string {
	t.Helper()

	return createEventAt(t, accessToken, orgID, maxSeats, time.Now().Add(24*time.Hour))
}

func createEventAt(t *testing.T, accessToken, orgID string, maxSeats int, startsAt time.Time) string {
	t.Helper()

	var got idResponse
	status := doJSON(t, http.MethodPost, "/organizations/"+orgID+"/events", accessToken, map[string]any{
		"name":      "Test Event",
		"max_seats": maxSeats,
		"starts_at": startsAt,
	}, &got)
	if status != http.StatusCreated {
		t.Fatalf("create event: status %d, want %d", status, http.StatusCreated)
	}
	return got.ID
}

// book takes no t and returns the error, because the concurrency test calls it from goroutines.
func book(accessToken, eventID string) (int, error) {
	return sendJSON(http.MethodPost, "/events/"+eventID+"/bookings", accessToken, nil, nil)
}

// countActiveBookings reads the database directly, so it does not trust the API's own answers.
func countActiveBookings(t *testing.T, eventID string) int {
	t.Helper()

	var n int
	err := testPool.QueryRow(context.Background(),
		"SELECT count(*) FROM bookings WHERE event_id = $1 AND cancelled_at IS NULL", eventID).Scan(&n)
	if err != nil {
		t.Fatalf("count active bookings: %v", err)
	}
	return n
}
