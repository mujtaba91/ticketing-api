package booking

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/auth"
	"github.com/mujtaba91/ticketing-api/internal/database"
)

type bookingService struct {
	db    *pgxpool.Pool
	store *bookingStore
}

func newService(db *pgxpool.Pool) *bookingService {
	return &bookingService{db: db, store: newStore(db)}
}

// Book locks the event row first, so concurrent bookings for one event count seats one at a time.
func (s *bookingService) Book(ctx context.Context, caller auth.Caller, eventID string) (Booking, error) {
	var booking Booking
	err := database.WithTx(ctx, s.db, func(tx pgx.Tx) error {
		txStore := newStore(tx)
		maxSeats, startsAt, err := txStore.LockEvent(ctx, eventID)
		if err != nil {
			return err
		}
		if startsAt.Before(time.Now()) {
			return ErrEventStarted
		}
		count, err := txStore.CountActiveBookings(ctx, eventID)
		if err != nil {
			return err
		}
		if count >= maxSeats {
			return ErrSoldOut
		}
		booking, err = txStore.CreateBooking(ctx, caller.ID, eventID)
		return err
	})
	if err != nil {
		return Booking{}, err
	}
	return booking, nil
}

// Cancel needs no event lock, since freeing a seat can never oversell.
func (s *bookingService) Cancel(ctx context.Context, caller auth.Caller, bookingID string) error {
	_, err := s.store.CancelBooking(ctx, bookingID, caller.ID)
	return err
}

func (s *bookingService) ListMine(ctx context.Context, caller auth.Caller) ([]Booking, error) {
	return s.store.BookingsByUser(ctx, caller.ID)
}
