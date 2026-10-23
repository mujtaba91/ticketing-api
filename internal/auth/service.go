package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
	"github.com/mujtaba91/ticketing-api/internal/database"
)

// One error for unknown email and wrong password, so login cannot reveal which emails exist.
var ErrInvalidCredentials = apperr.Unauthorized("invalid email or password")

var ErrInvalidRefreshToken = apperr.Unauthorized("invalid or expired refresh token")

type authService struct {
	db              *pgxpool.Pool
	store           authStore
	jwtSecret       string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func newService(db *pgxpool.Pool, jwtSecret string, accessTokenTTL, refreshTokenTTL time.Duration) *authService {
	return &authService{
		db:              db,
		store:           *NewStore(db),
		jwtSecret:       jwtSecret,
		accessTokenTTL:  accessTokenTTL,
		refreshTokenTTL: refreshTokenTTL,
	}
}

func (s *authService) Login(ctx context.Context, email string, password string) (accessToken string, refreshToken string, err error) {
	user, err := s.store.UserByEmail(ctx, email)
	if errors.Is(err, ErrUserNotFound) {
		return "", "", ErrInvalidCredentials
	}
	if err != nil {
		return "", "", err
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		return "", "", ErrInvalidCredentials
	}
	return s.issueTokens(ctx, &s.store, user)
}

func (s *authService) Register(ctx context.Context, name, email, password string) (accessToken string, refreshToken string, err error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", "", err
	}
	user, err := s.store.CreateUser(ctx, name, email, string(hash))
	if err != nil {
		return "", "", err
	}
	return s.issueTokens(ctx, &s.store, user)
}

func (s *authService) Refresh(ctx context.Context, token string) (accessToken string, refreshToken string, err error) {
	hashedToken := hashToken(token)
	tokenInDb, err := s.store.RefreshTokenByHash(ctx, hashedToken)
	if err != nil {
		if errors.Is(err, ErrRefreshTokenNotFound) {
			return "", "", ErrInvalidRefreshToken
		}
		return "", "", err
	}

	if tokenInDb.RevokedAt != nil {
		return "", "", s.revokeSessions(ctx, tokenInDb.UserID)
	}
	err = database.WithTx(ctx, s.db, func(tx pgx.Tx) error {
		txStore := NewStore(tx)

		user, err := txStore.UserByID(ctx, tokenInDb.UserID)
		if errors.Is(err, ErrUserNotFound) {
			return ErrInvalidRefreshToken
		}
		if err != nil {
			return err
		}
		if user.TokenVersion != tokenInDb.Version || tokenInDb.ExpiresAt.Before(time.Now()) {
			return ErrInvalidRefreshToken
		}
		_, err = txStore.RevokeRefreshToken(ctx, tokenInDb.ID)
		if err != nil {
			return err
		}
		accessToken, refreshToken, err = s.issueTokens(ctx, txStore, user)
		return err
	})
	if err != nil {
		if errors.Is(err, ErrRefreshTokenRevoked) {
			return "", "", s.revokeSessions(ctx, tokenInDb.UserID)
		}
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

func (s *authService) revokeSessions(ctx context.Context, userID string) error {
	_, err := s.store.BumpTokenVersion(ctx, userID)
	if err != nil {
		return err
	}
	return ErrInvalidRefreshToken
}

func (s *authService) issueTokens(ctx context.Context, store *authStore, user User) (accessToken string, refreshToken string, err error) {
	accessToken, err = newAccessToken(user.ID, user.TokenVersion, s.jwtSecret, s.accessTokenTTL)
	if err != nil {
		return "", "", err
	}
	refreshToken, hash, err := newRefreshToken()
	if err != nil {
		return "", "", err
	}
	err = store.CreateRefreshToken(ctx, user.ID, hash, user.TokenVersion, time.Now().Add(s.refreshTokenTTL))
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

type accessClaims struct {
	TokenVersion int `json:"tv"`
	jwt.RegisteredClaims
}

func newAccessToken(userID string, tokenVersion int, secret string, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := accessClaims{
		TokenVersion: tokenVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

func newRefreshToken() (raw, hash string, err error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", "", err
	}
	raw = base64.RawURLEncoding.EncodeToString(b)
	return raw, hashToken(raw), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
