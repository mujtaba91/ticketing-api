package booking

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
	"github.com/mujtaba91/ticketing-api/internal/database"
)

var (
	ErrBookingNotFound = apperr.NotFound("booking not found")
	ErrEventNotFound   = apperr.NotFound("event not found")
	ErrAlreadyBooked   = apperr.Conflict("you already have an active booking for this event")
	ErrSoldOut         = apperr.Conflict("event is sold out")
	ErrEventStarted    = apperr.Conflict("event has already started")
)

type Booking struct {
	ID          string
	UserID      string
	EventID     string
	CreatedAt   time.Time
	CancelledAt *time.Time
}

type bookingStore struct {
	db database.DBTX
}

func newStore(db database.DBTX) *bookingStore {
	return &bookingStore{db: db}
}

const bookingColumns = "id, user_id, event_id, created_at, cancelled_at"

func (s *bookingStore) LockEvent(ctx context.Context, eventID string) (maxSeats int, startsAt time.Time, err error) {
	err = s.db.QueryRow(ctx,
		"SELECT max_seats, starts_at FROM events WHERE id = $1 FOR UPDATE",
		eventID).Scan(&maxSeats, &startsAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
			return 0, time.Time{}, ErrEventNotFound
		}
		return 0, time.Time{}, fmt.Errorf("lock event: %w", err)
	}
	return maxSeats, startsAt, nil
}

func (s *bookingStore) CountActiveBookings(ctx context.Context, eventID string) (int, error) {
	var count int
	err := s.db.QueryRow(ctx,
		"SELECT count(*) FROM bookings WHERE event_id = $1 AND cancelled_at IS NULL",
		eventID).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count active bookings: %w", err)
	}
	return count, nil
}

func (s *bookingStore) CancelBooking(ctx context.Context, bookingID, userID string) (Booking, error) {
	return scanBooking(s.db.QueryRow(ctx,
		"UPDATE bookings SET cancelled_at = now() WHERE id = $1 AND user_id = $2 AND cancelled_at IS NULL RETURNING "+bookingColumns,
		bookingID, userID))
}

func (s *bookingStore) CreateBooking(ctx context.Context, userID, eventID string) (Booking, error) {
	b, err := scanBooking(s.db.QueryRow(ctx,
		"INSERT INTO bookings (user_id, event_id) VALUES ($1, $2) RETURNING "+bookingColumns,
		userID, eventID))
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation on the partial index of active bookings
			return Booking{}, ErrAlreadyBooked
		case "23503": // foreign_key_violation on event_id
			return Booking{}, ErrEventNotFound
		}
	}
	return b, err
}

func (s *bookingStore) BookingByID(ctx context.Context, id string) (Booking, error) {
	return scanBooking(s.db.QueryRow(ctx, "SELECT "+bookingColumns+" FROM bookings WHERE id = $1", id))
}

func (s *bookingStore) BookingsByUser(ctx context.Context, userID string) ([]Booking, error) {
	rows, err := s.db.Query(ctx,
		"SELECT "+bookingColumns+" FROM bookings WHERE user_id = $1 ORDER BY created_at DESC, id",
		userID)
	if err != nil {
		return nil, fmt.Errorf("select bookings: %w", err)
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Booking, error) {
		var b Booking
		err := row.Scan(&b.ID, &b.UserID, &b.EventID, &b.CreatedAt, &b.CancelledAt)
		return b, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan bookings: %w", err)
	}
	return list, nil
}

func scanBooking(row pgx.Row) (Booking, error) {
	var b Booking
	err := row.Scan(&b.ID, &b.UserID, &b.EventID, &b.CreatedAt, &b.CancelledAt)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return Booking{}, ErrBookingNotFound
	}
	if err != nil {
		return Booking{}, fmt.Errorf("scan booking: %w", err)
	}
	return b, nil
}

// 22P02 is invalid_text_representation, which Postgres raises for a malformed uuid.
func isInvalidUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
