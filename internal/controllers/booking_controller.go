package controllers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ambiguity-lab/booking-service/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// bookingController serves booking creation, retrieval, cancellation, and user
// listing requests.
type bookingController struct {
	service bookingService
}

// createBookingRequest is the decoded body of a create-booking request. It is
// kept separate from the service request struct so decoding and validation stay
// in the HTTP layer.
type createBookingRequest struct {
	UserID     uuid.UUID `json:"user_id"`
	PropertyID uuid.UUID `json:"property_id"`
	CheckIn    time.Time `json:"check_in"`
	CheckOut   time.Time `json:"check_out"`
	Rooms      int       `json:"rooms"`
}

func (c *bookingController) create(w http.ResponseWriter, r *http.Request) {
	var req createBookingRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, fieldError("body", "invalid request body"))
		return
	}
	booking, err := c.service.Create(r.Context(), services.CreateBookingRequest{
		UserID:     req.UserID,
		PropertyID: req.PropertyID,
		CheckIn:    req.CheckIn,
		CheckOut:   req.CheckOut,
		Rooms:      req.Rooms,
	})
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, booking)
}

func (c *bookingController) get(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fieldError("id", "must be a valid UUID"))
		return
	}
	booking, err := c.service.Get(r.Context(), id)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, booking)
}

func (c *bookingController) cancel(w http.ResponseWriter, r *http.Request) {
	id, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fieldError("id", "must be a valid UUID"))
		return
	}
	if err := c.service.Cancel(r.Context(), id); err != nil {
		writeError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (c *bookingController) listForUser(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "id"))
	if err != nil {
		writeError(w, fieldError("id", "must be a valid UUID"))
		return
	}
	limit, offset, err := parsePagination(r)
	if err != nil {
		writeError(w, err)
		return
	}
	bookings, err := c.service.ListForUser(r.Context(), userID, limit, offset)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, bookings)
}
