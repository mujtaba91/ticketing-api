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

// @Summary Create an organization; the caller becomes its owner
// @Tags organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body createRequest true "Organization"
// @Success 201 {object} organizationResponse
// @Failure 400 {object} httpx.errorBody
// @Failure 401 {object} httpx.errorBody
// @Router /organizations [post]
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

// @Summary List the caller's organizations with their role
// @Tags organizations
// @Produce json
// @Security BearerAuth
// @Success 200 {object} listResponse
// @Failure 401 {object} httpx.errorBody
// @Router /organizations [get]
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

// @Summary Add a staff member by email (owner or admin)
// @Tags organizations
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Organization ID"
// @Param body body addStaffRequest true "Staff email"
// @Success 201 {object} memberResponse
// @Failure 400 {object} httpx.errorBody
// @Failure 401 {object} httpx.errorBody
// @Failure 403 {object} httpx.errorBody
// @Failure 404 {object} httpx.errorBody
// @Failure 409 {object} httpx.errorBody
// @Router /organizations/{id}/members [post]
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
