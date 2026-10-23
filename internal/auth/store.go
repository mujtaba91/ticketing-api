package auth

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
	ErrEmailTaken           = apperr.Conflict("email already registered")
	ErrUserNotFound         = errors.New("user not found")
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
)

type User struct {
	ID           string
	Name         string
	Email        string
	PasswordHash string
	IsAdmin      bool
	TokenVersion int
}

type authStore struct {
	db database.DBTX
}

func NewStore(db database.DBTX) *authStore {
	return &authStore{db: db}
}

const userColumns = "id, name, email, password_hash, is_admin, token_version"

func (s *authStore) CreateUser(ctx context.Context, name, email, passwordHash string) (User, error) {
	row := s.db.QueryRow(ctx,
		"INSERT INTO users (name, email, password_hash) VALUES ($1, $2, $3) RETURNING "+userColumns,
		name, email, passwordHash)

	u, err := scanUser(row)
	// 23505 is unique_violation; email is the only unique column a caller can set.
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.Code == "23505" {
		return User{}, ErrEmailTaken
	}
	return u, err
}

func (s *authStore) UserByEmail(ctx context.Context, email string) (User, error) {
	return scanUser(s.db.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE email = $1", email))
}

func (s *authStore) UserByID(ctx context.Context, id string) (User, error) {
	return scanUser(s.db.QueryRow(ctx, "SELECT "+userColumns+" FROM users WHERE id = $1", id))
}

func scanUser(row pgx.Row) (User, error) {
	var u User
	err := row.Scan(&u.ID, &u.Name, &u.Email, &u.PasswordHash, &u.IsAdmin, &u.TokenVersion)
	if errors.Is(err, pgx.ErrNoRows) {
		return User{}, ErrUserNotFound
	}
	if err != nil {
		return User{}, fmt.Errorf("scan user: %w", err)
	}
	return u, nil
}

type RefreshToken struct {
	ID        string
	UserID    string
	Version   int
	ExpiresAt time.Time
	RevokedAt *time.Time
}

func (s *authStore) CreateRefreshToken(ctx context.Context, userID, tokenHash string, version int, expiresAt time.Time) error {
	_, err := s.db.Exec(ctx,
		"INSERT INTO refresh_tokens (user_id, token_hash, version, expires_at) VALUES ($1, $2, $3, $4)",
		userID, tokenHash, version, expiresAt)
	if err != nil {
		return fmt.Errorf("insert refresh token: %w", err)
	}
	return nil
}

func (s *authStore) RefreshTokenByHash(ctx context.Context, tokenHash string) (RefreshToken, error) {
	var t RefreshToken
	err := s.db.QueryRow(ctx,
		"SELECT id, user_id, version, expires_at, revoked_at FROM refresh_tokens WHERE token_hash = $1",
		tokenHash).Scan(&t.ID, &t.UserID, &t.Version, &t.ExpiresAt, &t.RevokedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshToken{}, ErrRefreshTokenNotFound
	}
	if err != nil {
		return RefreshToken{}, fmt.Errorf("scan refresh token: %w", err)
	}
	return t, nil
}
