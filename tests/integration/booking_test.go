//go:build integration

package integration

import (
	"net/http"
	"sync"
	"testing"
)

func TestConcurrentBookingsNeverOversell(t *testing.T) {
	const (
		n     = 50
		seats = 10
	)

	first := registerUser(t)
	organization := createOrg(t, first.AccessToken)
	event := createEvent(t, first.AccessToken, organization, seats)
	var users []user
	for range n {
		users = append(users, registerUser(t))
	}

	start := make(chan struct{})
	results := make(chan int, n)
	var wg sync.WaitGroup
	for _, u := range users {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			status, err := book(u.AccessToken, event)
			if err != nil {
				t.Errorf("book: %v", err)
			}
			results <- status
		}()
	}
	close(start)
	wg.Wait()
	close(results)

	var created, soldOut, unknown int
	for status := range results {
		switch status {
		case http.StatusCreated:
			created++
		case http.StatusConflict:
			soldOut++
		default:
			unknown++
		}
	}
	if created != seats || soldOut != n-seats || unknown != 0 {
		t.Fatalf("got %d created, %d sold out, %d other; want %d, %d, 0", created, soldOut, unknown, seats, n-seats)
	}

	if active := countActiveBookings(t, event); active != seats {
		t.Fatalf("active bookings in database: %d, want %d", active, seats)
	}
}
