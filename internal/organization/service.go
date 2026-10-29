package organization

import (
	"context"
	"errors"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
	"github.com/mujtaba91/ticketing-api/internal/auth"
	"github.com/mujtaba91/ticketing-api/internal/database"
)

const maxNameChars = 100

type orgService struct {
	db    *pgxpool.Pool
	store *Store
}

func newService(db *pgxpool.Pool) *orgService {
	return &orgService{db: db, store: NewStore(db)}
}

// Create runs in one transaction so an organization never exists without its owner.
func (s *orgService) Create(ctx context.Context, caller auth.Caller, name string) (Membership, error) {
	name = strings.TrimSpace(name)
	if err := validateName(name); err != nil {
		return Membership{}, err
	}

	var created Membership
	err := database.WithTx(ctx, s.db, func(tx pgx.Tx) error {
		txStore := NewStore(tx)
		org, err := txStore.CreateOrganization(ctx, name)
		if err != nil {
			return err
		}
		if _, err := txStore.AddMember(ctx, org.ID, caller.ID, RoleOwner); err != nil {
			return err
		}
		created = Membership{Organization: org, Role: RoleOwner}
		return nil
	})
	return created, err
}

func (s *orgService) ListMine(ctx context.Context, caller auth.Caller) ([]Membership, error) {
	return s.store.MembershipsByUser(ctx, caller.ID)
}

func (s *orgService) AddStaff(ctx context.Context, caller auth.Caller, orgID, email string) (Member, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		return Member{}, apperr.Validation(map[string]string{"email": "is required"})
	}
	if !caller.IsAdmin {
		role, err := s.store.MemberRole(ctx, orgID, caller.ID)
		if errors.Is(err, ErrNotMember) {
			return Member{}, apperr.Forbidden("you are not a member of this organization")
		}
		if err != nil {
			return Member{}, err
		}
		if role != RoleOwner {
			return Member{}, apperr.Forbidden("only the organization owner can add staff")
		}
	}
	return s.store.AddMemberByEmail(ctx, orgID, email, RoleStaff)
}

func validateName(name string) error {
	switch {
	case name == "":
		return apperr.Validation(map[string]string{"name": "is required"})
	case utf8.RuneCountInString(name) > maxNameChars:
		return apperr.Validation(map[string]string{"name": "must be at most 100 characters"})
	}
	return nil
}
