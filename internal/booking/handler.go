package booking

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/auth"
	"github.com/mujtaba91/ticketing-api/internal/httpx"
)

type Handler struct {
	service *bookingService
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{service: newService(db)}
}

// Routes must be mounted behind auth.RequireAuth.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/events/{id}/bookings", h.book)
	r.Delete("/bookings/{id}", h.cancel)
	r.Get("/bookings", h.listMine)
}

type bookingResponse struct {
	ID          string     `json:"id"`
	EventID     string     `json:"event_id"`
	CreatedAt   time.Time  `json:"created_at"`
	CancelledAt *time.Time `json:"cancelled_at"`
}

type listResponse struct {
	Items []bookingResponse `json:"items"`
}

// @Summary Book one seat; 409 when sold out, started or already booked
// @Tags bookings
// @Produce json
// @Security BearerAuth
// @Param id path string true "Event ID"
// @Success 201 {object} bookingResponse
// @Failure 401 {object} httpx.errorBody
// @Failure 404 {object} httpx.errorBody
// @Failure 409 {object} httpx.errorBody
// @Router /events/{id}/bookings [post]
func (h *Handler) book(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	b, err := h.service.Book(ctx, caller, chi.URLParam(r, "id"))
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toResponse(b))
}

// @Summary Cancel one of the caller's bookings
// @Tags bookings
// @Security BearerAuth
// @Param id path string true "Booking ID"
// @Success 204
// @Failure 401 {object} httpx.errorBody
// @Failure 404 {object} httpx.errorBody
// @Router /bookings/{id} [delete]
func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	if err := h.service.Cancel(ctx, caller, chi.URLParam(r, "id")); err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// @Summary List the caller's bookings, cancelled ones included
// @Tags bookings
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listResponse
// @Failure 401 {object} httpx.errorBody
// @Router /bookings [get]
func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	list, err := h.service.ListMine(ctx, caller)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// make, not nil, so an empty list encodes as [] instead of null.
	items := make([]bookingResponse, 0, len(list))
	for _, b := range list {
		items = append(items, toResponse(b))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{Items: items})
}

func toResponse(b Booking) bookingResponse {
	return bookingResponse{ID: b.ID, EventID: b.EventID, CreatedAt: b.CreatedAt, CancelledAt: b.CancelledAt}
}
