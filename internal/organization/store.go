package organization

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
	"github.com/mujtaba91/ticketing-api/internal/database"
)

type Role string

const (
	RoleOwner Role = "owner"
	RoleStaff Role = "staff"
)

var (
	ErrNotMember            = errors.New("not a member of the organization")
	ErrOrganizationNotFound = apperr.NotFound("organization not found")
	ErrUserNotFound         = apperr.NotFound("no user with this email")
	ErrAlreadyMember        = apperr.Conflict("user is already a member of this organization")
)

type Organization struct {
	ID   string
	Name string
}

// Membership is an organization together with the caller's role in it.
type Membership struct {
	Organization
	Role Role
}

type Member struct {
	OrganizationID string
	UserID         string
	Role           Role
}

type Store struct {
	db database.DBTX
}

func NewStore(db database.DBTX) *Store {
	return &Store{db: db}
}

func (s *Store) CreateOrganization(ctx context.Context, name string) (Organization, error) {
	var o Organization
	err := s.db.QueryRow(ctx, "INSERT INTO organizations (name) VALUES ($1) RETURNING id, name", name).
		Scan(&o.ID, &o.Name)
	if err != nil {
		return Organization{}, fmt.Errorf("insert organization: %w", err)
	}
	return o, nil
}

func (s *Store) AddMember(ctx context.Context, orgID, userID string, role Role) (Member, error) {
	m := Member{OrganizationID: orgID, UserID: userID, Role: role}
	_, err := s.db.Exec(ctx,
		"INSERT INTO organization_members (organization_id, user_id, role) VALUES ($1, $2, $3)",
		orgID, userID, role)
	if err != nil {
		return Member{}, memberInsertError(err)
	}
	return m, nil
}

// AddMemberByEmail resolves the email and inserts in one statement, so no row means no such user.
func (s *Store) AddMemberByEmail(ctx context.Context, orgID, email string, role Role) (Member, error) {
	m := Member{OrganizationID: orgID, Role: role}
	err := s.db.QueryRow(ctx,
		`INSERT INTO organization_members (organization_id, user_id, role)
		 SELECT $1, id, $3 FROM users WHERE email = $2
		 RETURNING user_id`,
		orgID, email, role).Scan(&m.UserID)
	if errors.Is(err, pgx.ErrNoRows) {
		return Member{}, ErrUserNotFound
	}
	if err != nil {
		return Member{}, memberInsertError(err)
	}
	return m, nil
}

func (s *Store) MemberRole(ctx context.Context, orgID, userID string) (Role, error) {
	var role Role
	err := s.db.QueryRow(ctx,
		"SELECT role FROM organization_members WHERE organization_id = $1 AND user_id = $2",
		orgID, userID).Scan(&role)
	if errors.Is(err, pgx.ErrNoRows) || isInvalidUUID(err) {
		return "", ErrNotMember
	}
	if err != nil {
		return "", fmt.Errorf("select member role: %w", err)
	}
	return role, nil
}

func (s *Store) MembershipsByUser(ctx context.Context, userID string) ([]Membership, error) {
	rows, err := s.db.Query(ctx,
		`SELECT o.id, o.name, m.role
		 FROM organizations o
		 JOIN organization_members m ON m.organization_id = o.id
		 WHERE m.user_id = $1
		 ORDER BY o.name, o.id`,
		userID)
	if err != nil {
		return nil, fmt.Errorf("select memberships: %w", err)
	}
	list, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Membership, error) {
		var m Membership
		err := row.Scan(&m.ID, &m.Name, &m.Role)
		return m, err
	})
	if err != nil {
		return nil, fmt.Errorf("scan memberships: %w", err)
	}
	return list, nil
}

func memberInsertError(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		switch pgErr.Code {
		case "23505": // unique_violation on (organization_id, user_id)
			return ErrAlreadyMember
		case "23503", "22P02": // foreign_key_violation or a malformed uuid
			return ErrOrganizationNotFound
		}
	}
	return fmt.Errorf("insert member: %w", err)
}

// 22P02 is invalid_text_representation, which Postgres raises for a malformed uuid.
func isInvalidUUID(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "22P02"
}
