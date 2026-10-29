package event

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
	ErrEventNotFound        = apperr.NotFound("event not found")
	ErrOrganizationNotFound = apperr.NotFound("organization not found")
)

type Event struct {
	ID             string
	OrganizationID string
	Name           string
	MaxSeats       int
	StartsAt       time.Time
}

type eventStore struct {
	db database.DBTX
}

func newStore(db database.DBTX) *eventStore {
	return &eventStore{db: db}
}

const eventColumns = "id, organization_id, name, max_seats, starts_at"

func (s *eventStore) CreateEvent(ctx context.Context, orgID, name string, maxSeats int, startsAt time.Time) (Event, error) {
	e, err := scanEvent(s.db.QueryRow(ctx,
		"INSERT INTO events (organization_id, name, max_seats, starts_at) VALUES ($1, $2, $3, $4) RETURNING "+eventColumns,
		orgID, name, maxSeats, startsAt))
	// 23503 is foreign_key_violation; organization_id is the only foreign key.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && (pgErr.Code == "23503" || pgErr.Code == "22P02") {
		return Event{}, ErrOrganizationNotFound
	}
	return e, err
}

func (s *eventStore) EventByID(ctx context.Context, id string) (Event, error) {
	return scanEvent(s.db.QueryRow(ctx, "SELECT "+eventColumns+" FROM events WHERE id = $1", id))
}

// UpdateEvent keeps a column unchanged when its argument is nil.
func (s *eventStore) UpdateEvent(ctx context.Context, id string, name *string, startsAt *time.Time) (Event, error) {
	return scanEvent(s.db.QueryRow(ctx,
		`UPDATE events SET name = COALESCE($2, name), starts_at = COALESCE($3, starts_at)
		 WHERE id = $1 RETURNING `+eventColumns,
		id, name, startsAt))
}

func (s *eventStore) ListEvents(ctx context.Context) ([]Event, error) {
	rows, err := s.db.Query(ctx, "SELECT "+eventColumns+" FROM events ORDER BY starts_at, id")
	if err != nil {
		return nil, fmt.Errorf("select events: %w", err)
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Event, error) {
		var e Event
		err := row.Scan(&e.ID, &e.OrganizationID, &e.Name, &e.MaxSeats, &e.StartsAt)
		return e, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan events: %w", err)
	}
	return list, nil
}

func scanEvent(row pgx.Row) (Event, error) {
	var e Event
	err := row.Scan(&e.ID, &e.OrganizationID, &e.Name, &e.MaxSeats, &e.StartsAt)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return Event{}, ErrEventNotFound
	}
	if err != nil {
		return Event{}, fmt.Errorf("scan event: %w", err)
	}
	return e, nil
}

// 22P02 is invalid_text_representation, which Postgres raises for a malformed uuid.
func isInvalidUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
