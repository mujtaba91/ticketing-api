package organization

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/mujtaba91/ticketing-api/internal/auth"
	"github.com/mujtaba91/ticketing-api/internal/httpx"
)

type Handler struct {
	service *orgService
}

func NewHandler(db *pgxpool.Pool) *Handler {
	return &Handler{service: newService(db)}
}

// Routes must be mounted behind auth.RequireAuth.
func (h *Handler) Routes(r chi.Router) {
	r.Post("/", h.create)
	r.Get("/", h.listMine)
	r.Post("/{id}/members", h.addStaff)
}

type createRequest struct {
	Name string `json:"name"`
}

type addStaffRequest struct {
	Email string `json:"email"`
}

type organizationResponse struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role Role   `json:"role"`
}

type listResponse struct {
	Items []organizationResponse `json:"items"`
}

type memberResponse struct {
	OrganizationID string `json:"organization_id"`
	UserID         string `json:"user_id"`
	Role           Role   `json:"role"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var req createRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	m, err := h.service.Create(ctx, caller, req.Name)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, toResponse(m))
}

func (h *Handler) listMine(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	list, err := h.service.ListMine(ctx, caller)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	// make, not nil, so an empty list encodes as [] instead of null.
	items := make([]organizationResponse, 0, len(list))
	for _, m := range list {
		items = append(items, toResponse(m))
	}
	httpx.WriteJSON(w, http.StatusOK, listResponse{Items: items})
}

func (h *Handler) addStaff(w http.ResponseWriter, r *http.Request) {
	var req addStaffRequest
	if err := httpx.DecodeJSON(w, r, &req); err != nil {
		httpx.WriteError(w, r, err)
		return
	}

	ctx := r.Context()
	caller := auth.CallerFrom(ctx)
	m, err := h.service.AddStaff(ctx, caller, chi.URLParam(r, "id"), req.Email)
	if err != nil {
		httpx.WriteError(w, r, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, memberResponse{OrganizationID: m.OrganizationID, UserID: m.UserID, Role: m.Role})
}

func toResponse(m Membership) organizationResponse {
	return organizationResponse{ID: m.ID, Name: m.Name, Role: m.Role}
}
