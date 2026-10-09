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
	"golang.org/x/crypto/bcrypt"
)

// One error for unknown email and wrong password, so login cannot reveal which emails exist.
var ErrInvalidCredentials = errors.New("invalid email or password")

type authService struct {
	store           authStore
	jwtSecret       string
	accessTokenTTL  time.Duration
	refreshTokenTTL time.Duration
}

func newService(store authStore, jwtSecret string, accessTokenTTL, refreshTokenTTL time.Duration) *authService {
	return &authService{
		store:           store,
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
	return s.issueTokens(ctx, user)
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
	return s.issueTokens(ctx, user)
}

func (s *authService) issueTokens(ctx context.Context, user User) (accessToken string, refreshToken string, err error) {
	accessToken, err = newAccessToken(user.ID, user.TokenVersion, s.jwtSecret, s.accessTokenTTL)
	if err != nil {
		return "", "", err
	}
	refreshToken, hash, err := newRefreshToken()
	if err != nil {
		return "", "", err
	}
	err = s.store.CreateRefreshToken(ctx, user.ID, hash, user.TokenVersion, time.Now().Add(s.refreshTokenTTL))
	if err != nil {
		return "", "", err
	}
	return accessToken, refreshToken, nil
}

type accessClaims struct {
	TokenVersion int
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

func hashToken(row string) string {
	sum := sha256.Sum256([]byte(row))
	return hex.EncodeToString(sum[:])
}
