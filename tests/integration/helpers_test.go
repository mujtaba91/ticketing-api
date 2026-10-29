//go:build integration

package integration

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync/atomic"
	"testing"
)

type tokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// doJSON sends body as JSON and decodes the response into out when out is not nil; it returns the status code.
func doJSON(t *testing.T, method, path, accessToken string, body, out any) int {
	t.Helper()

	var reqBody bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&reqBody).Encode(body); err != nil {
			t.Fatalf("encode request: %v", err)
		}
	}
	req, err := http.NewRequest(method, baseURL+path, &reqBody)
	if err != nil {
		t.Fatalf("new request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if accessToken != "" {
		req.Header.Set("Authorization", "Bearer "+accessToken)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("%s %s: %v", method, path, err)
	}
	defer resp.Body.Close()

	if out != nil {
		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			t.Fatalf("%s %s: decode response with status %d: %v", method, path, resp.StatusCode, err)
		}
	}
	return resp.StatusCode
}

var emailCounter atomic.Int64

// uniqueEmail lets tests share one database without cleaning it between tests.
func uniqueEmail() string {
	return fmt.Sprintf("user%d@example.com", emailCounter.Add(1))
}

// registerUser creates a new user and fails the test unless it gets a token pair back.
func registerUser(t *testing.T) tokens {
	t.Helper()

	var got tokens
	status := doJSON(t, http.MethodPost, "/auth/register", "", map[string]string{
		"name":     "Test User",
		"email":    uniqueEmail(),
		"password": "password123",
	}, &got)
	if status != http.StatusCreated {
		t.Fatalf("register: status %d, want %d", status, http.StatusCreated)
	}
	return got
}

// refresh calls POST /auth/refresh; tokens are empty when the status is not 200.
func refresh(t *testing.T, refreshToken string) (int, tokens) {
	t.Helper()

	var got tokens
	status := doJSON(t, http.MethodPost, "/auth/refresh", "", map[string]string{"refresh_token": refreshToken}, &got)
	return status, got
}
