package auth

import (
	"net/http"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/apperr"
	"github.com/mujtaba91/ticketing-api/internal/httpx"
)

const (
	maxNameChars     = 100
	maxEmailBytes    = 254
	minPasswordChars = 8
	maxPasswordChars = 64
	// bcrypt rejects passwords longer than 72 bytes.
	maxPasswordBytes = 72
)

type Handler struct {
	service *authService
}

func NewHandler(db *pgxpool.Pool, jwtSecret string, accessTokenTTL, refreshTokenTTL time.Duration) *Handler {
	return &Handler{service: newService(db, jwtSecret, accessTokenTTL, refreshTokenTTL)}
}

func (h *Handler) Routes(r chi.Router) {
	r.Post("/register", h.register)
	r.Post("/login", h.login)
	r.Post("/refresh", h.refresh)
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// @Summary Create an account and log in
// @Tags auth
// @Accept json
// @Produce json
// @Param body body registerRequest true "New account"
// @Success 201 {object} tokenResponse
// @Failure 400 {object} httpx.errorBody
// @Failure 409 {object} httpx.errorBody
// @Router /auth/register [post]
func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req registerRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(req.Email)
	if err := req.validate(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	access, refresh, err := h.service.Register(r.Context(), req.Name, req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, tokenResponse{AccessToken: access, RefreshToken: refresh})
}

// @Summary Log in
// @Tags auth
// @Accept json
// @Produce json
// @Param body body loginRequest true "Credentials"
// @Success 200 {object} tokenResponse
// @Failure 400 {object} httpx.errorBody
// @Failure 401 {object} httpx.errorBody
// @Router /auth/login [post]
func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	req.Email = strings.TrimSpace(req.Email)
	if err := req.validate(); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	access, refresh, err := h.service.Login(r.Context(), req.Email, req.Password)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh})
}

// @Summary Rotate the refresh token; reusing an old one revokes every session
// @Tags auth
// @Accept json
// @Produce json
// @Param body body refreshRequest true "Current refresh token"
// @Success 200 {object} tokenResponse
// @Failure 400 {object} httpx.errorBody
// @Failure 401 {object} httpx.errorBody
// @Router /auth/refresh [post]
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req refreshRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	if req.RefreshToken == "" {
		httpx.WriteError(w, r, apperr.Validation(map[string]string{"refresh_token": "is required"}))
		return
	}

	access, refresh, err := h.service.Refresh(r.Context(), req.RefreshToken)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, tokenResponse{AccessToken: access, RefreshToken: refresh})
}

func (r registerRequest) validate() error {
	fields := map[string]string{}

	switch {
	case r.Name == "":
		fields["name"] = "is required"
	case utf8.RuneCountInString(r.Name) > maxNameChars:
		fields["name"] = "must be at most 100 characters"
	}

	switch {
	case r.Email == "":
		fields["email"] = "is required"
	case !validEmail(r.Email):
		fields["email"] = "must be a valid email address"
	}

	switch chars := utf8.RuneCountInString(r.Password); {
	case r.Password == "":
		fields["password"] = "is required"
	case chars < minPasswordChars || chars > maxPasswordChars:
		fields["password"] = "must be between 8 and 64 characters"
	// Non-English letters take 2-4 bytes each, so 64 characters can still pass bcrypt's limit.
	case len(r.Password) > maxPasswordBytes:
		fields["password"] = "is too long, please use a shorter one"
	}

	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	return nil
}

// Login only checks presence; applying the password rules here would tell a guesser what they are.
func (r loginRequest) validate() error {
	fields := map[string]string{}
	if r.Email == "" {
		fields["email"] = "is required"
	}
	if r.Password == "" {
		fields["password"] = "is required"
	}
	if len(fields) > 0 {
		return apperr.Validation(fields)
	}
	return nil
}

func validEmail(email string) bool {
	if len(email) > maxEmailBytes {
		return false
	}
	addr, err := mail.ParseAddress(email)
	// ParseAddress also accepts "Name <a@b.c>", so the parsed address must equal the input.
	return err == nil && addr.Address == email
}
