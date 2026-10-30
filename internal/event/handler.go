package event

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/auth"
	"github.com/mujtaba91/ticketing-api/internal/httpx"
)

type Handler struct {
	service *eventService
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{service: newService(db)}
}

// PublicRoutes need no token.
func (h *Handler) PublicRoutes(r chi.Router) {
	r.Get("/events", h.list)
}

// ProtectedRoutes must be mounted behind auth.RequireAuth.
func (h *Handler) ProtectedRoutes(r chi.Router) {
	r.Post("/organizations/{id}/events", h.create)
	r.Patch("/events/{id}", h.update)
}

type createRequest struct {
	Name     string    `json:"name"`
	MaxSeats int       `json:"max_seats"`
	StartsAt time.Time `json:"starts_at"`
}

// Pointers tell a missing field apart from an empty one.
type updateRequest struct {
	Name     *string    `json:"name"`
	StartsAt *time.Time `json:"starts_at"`
}

type eventResponse struct {
	ID             string    `json:"id"`
	OrganizationID string    `json:"organization_id"`
	Name           string    `json:"name"`
	MaxSeats       int       `json:"max_seats"`
	StartsAt       time.Time `json:"starts_at"`
}

type listResponse struct {
	Items      []eventResponse `json:"items"`
	NextCursor *string         `json:"next_cursor"` // null on the last page
}

// @Summary Create an event (owner, staff or admin of the organization)
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Organization ID"
// @Param body body createRequest true "Event"
// @Success 201 {object} eventResponse
// @Failure 400 {object} httpx.errorBody
// @Failure 401 {object} httpx.errorBody
// @Failure 403 {object} httpx.errorBody
// @Failure 404 {object} httpx.errorBody
// @Router /organizations/{id}/events [post]
func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	in := createInput{Name: req.Name, MaxSeats: req.MaxSeats, StartsAt: req.StartsAt}
	e, err := h.service.Create(ctx, caller, chi.URLParam(r, "id"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toResponse(e))
}

// @Summary Rename or reschedule an event; max_seats cannot change
// @Tags events
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Event ID"
// @Param body body updateRequest true "Fields to change"
// @Success 200 {object} eventResponse
// @Failure 400 {object} httpx.errorBody
// @Failure 401 {object} httpx.errorBody
// @Failure 403 {object} httpx.errorBody
// @Failure 404 {object} httpx.errorBody
// @Router /events/{id} [patch]
func (h *Handler) update(w http.ResponseWriter, r *http.Request) {
	var req updateRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	in := updateInput{Name: req.Name, StartsAt: req.StartsAt}
	e, err := h.service.Update(ctx, caller, chi.URLParam(r, "id"), in)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, toResponse(e))
}

// @Summary List events, soonest first, one page at a time
// @Tags events
// @Produce json
// @Param limit query int false "Page size, 1-100" default(20)
// @Param cursor query string false "next_cursor from the previous page"
// @Success 200 {object} listResponse
// @Failure 400 {object} httpx.errorBody
// @Router /events [get]
func (h *Handler) list(w http.ResponseWriter, r *http.Request) {
	limit, err := httpx.ParseLimit(r)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	p, err := h.service.List(r.Context(), r.URL.Query().Get("cursor"), limit)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// make, not nil, so an empty list encodes as [] instead of null.
	items := make([]eventResponse, 0, len(p.Events))
	for _, e := range p.Events {
		items = append(items, toResponse(e))
	}
	resp := listResponse{Items: items}
	if p.NextCursor != "" {
		resp.NextCursor = &p.NextCursor
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func toResponse(e Event) eventResponse {
	return eventResponse{ID: e.ID, OrganizationID: e.OrganizationID, Name: e.Name, MaxSeats: e.MaxSeats, StartsAt: e.StartsAt}
}
