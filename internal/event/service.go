package event

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
	"github.com/mujtaba91/ticketing-api/internal/auth"
	"github.com/mujtaba91/ticketing-api/internal/organization"
)

const (
	maxNameChars = 100
	maxSeats     = 100_000
)

type eventService struct {
	store   *eventStore
	members *organization.Store
}

func newService(db *pgxpool.Pool) *eventService {
	return &eventService{store: newStore(db), members: organization.NewStore(db)}
}

type createInput struct {
	Name     string
	MaxSeats int
	StartsAt time.Time
}

// updateInput leaves max_seats out, since shrinking it under existing bookings would oversell the event.
type updateInput struct {
	Name     *string
	StartsAt *time.Time
}

func (s *eventService) Create(ctx context.Context, caller auth.Caller, orgID string, in createInput) (Event, error) {
	in.Name = strings.TrimSpace(in.Name)
	if err := in.validate(); err != nil {
		return Event{}, err
	}
	if err := s.requireMember(ctx, caller, orgID); err != nil {
		return Event{}, err
	}
	return s.store.CreateEvent(ctx, orgID, in.Name, in.MaxSeats, in.StartsAt)
}

func (s *eventService) Update(ctx context.Context, caller auth.Caller, eventID string, in updateInput) (Event, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		in.Name = &trimmed
	}
	if err := in.validate(); err != nil {
		return Event{}, err
	}
	event, err := s.store.EventByID(ctx, eventID)
	if err != nil {
		return Event{}, err
	}
	if err := s.requireMember(ctx, caller, event.OrganizationID); err != nil {
		return Event{}, err
	}
	return s.store.UpdateEvent(ctx, eventID, in.Name, in.StartsAt)
}

// requireMember allows Admins and any member of orgID; both Owner and Staff manage events.
func (s *eventService) requireMember(ctx context.Context, caller auth.Caller, orgID string) error {
	if caller.IsAdmin {
		return nil
	}
	_, err := s.members.MemberRole(ctx, orgID, caller.ID)
	if errors.Is(err, organization.ErrNotMember) {
		return apperr.Forbidden("you are not a member of this organization")
	}
	return err
}

type page struct {
	Events     []Event
	NextCursor string
}

// List fetches one row more than limit, so it knows whether a next page exists without a count query.
func (s *eventService) List(ctx context.Context, rawCursor string, limit int) (page, error) {
	var after *cursor
	if rawCursor != "" {
		c, err := decodeCursor(rawCursor)
		if err != nil {
			return page{}, err
		}
		after = &c
	}
	events, err := s.store.ListEvents(ctx, after, limit+1)
	if err != nil {
		return page{}, err
	}
	if len(events) <= limit {
		return page{Events: events}, nil
	}
	events = events[:limit]
	last := events[limit-1]
	return page{Events: events, NextCursor: cursor{StartsAt: last.StartsAt, ID: last.ID}.encode()}, nil
}

func (in createInput) validate() error {
	fields := map[string]string{}
	if msg := nameProblem(in.Name); msg != "" {
		fields["name"] = msg
	}
	if in.MaxSeats < 1 || in.MaxSeats > maxSeats {
		fields["max_seats"] = "must be between 1 and 100000"
	}
	if msg := startsAtProblem(in.StartsAt); msg != "" {
		fields["starts_at"] = msg
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	return nil
}

func (in updateInput) validate() error {
	if in.Name == nil && in.StartsAt == nil {
		return apperr.Invalid("provide at least one of name, starts_at")
	}
	fields := map[string]string{}
	if in.Name != nil {
		if msg := nameProblem(*in.Name); msg != "" {
			fields["name"] = msg
		}
	}
	if in.StartsAt != nil {
		if msg := startsAtProblem(*in.StartsAt); msg != "" {
			fields["starts_at"] = msg
		}
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	return nil
}

func nameProblem(name string) string {
	switch {
	case name == "":
		return "is required"
	case utf8.RuneCountInString(name) > maxNameChars:
		return "must be at most 100 characters"
	}
	return ""
}

func startsAtProblem(t time.Time) string {
	switch {
	case t.IsZero():
		return "is required"
	case !t.After(time.Now()):
		return "must be in the future"
	}
	return ""
}
