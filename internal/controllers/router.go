// Package controllers wires the HTTP routes of the booking-service API,
// decodes and validates request input, invokes the service and repository
// layers, and encodes responses using the standard error envelope.
package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/ambiguity-lab/booking-service/internal/models"
	"github.com/ambiguity-lab/booking-service/internal/repositories"
	"github.com/ambiguity-lab/booking-service/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// bookingService is the booking operations the handlers depend on. It is
// implemented by *services.BookingService.
type bookingService interface {
	Create(ctx context.Context, req services.CreateBookingRequest) (*models.Booking, error)
	Get(ctx context.Context, id uuid.UUID) (*models.Booking, error)
	Cancel(ctx context.Context, id uuid.UUID) error
	ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error)
}

// propertyRepo is the property data access the handlers depend on. It is
// implemented by *repositories.PropertyRepository.
type propertyRepo interface {
	GetByID(ctx context.Context, id uuid.UUID) (*models.Property, error)
	Search(ctx context.Context, p repositories.PropertySearchParams) ([]models.Property, error)
}

// Router owns the HTTP handlers and the route table for the API.
type Router struct {
	properties *propertyController
	bookings   *bookingController
}

// NewRouter constructs the Router that serves the booking-service API. It takes
// the property repository and the booking service the handlers operate on.
func NewRouter(properties propertyRepo, bookings bookingService) *Router {
	return &Router{
		properties: &propertyController{repo: properties},
		bookings:   &bookingController{service: bookings},
	}
}

// Routes registers every API route on the given chi router.
func (rt *Router) Routes(r chi.Router) {
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Route("/v1", func(r chi.Router) {
		r.Get("/properties", rt.properties.search)
		r.Get("/properties/{id}", rt.properties.get)
		r.Post("/bookings", rt.bookings.create)
		r.Get("/bookings/{id}", rt.bookings.get)
		r.Post("/bookings/{id}/cancel", rt.bookings.cancel)
		r.Get("/users/{id}/bookings", rt.bookings.listForUser)
	})
}

// errorEnvelope is the top-level shape of every non-2xx response.
type errorEnvelope struct {
	Error errorBody `json:"error"`
}

// errorBody is the payload of an error envelope.
type errorBody struct {
	Code    string       `json:"code"`
	Message string       `json:"message"`
	Details []detailBody `json:"details,omitempty"`
}

// detailBody describes a single field-level validation failure.
type detailBody struct {
	Field string `json:"field"`
	Issue string `json:"issue"`
}

// writeJSON encodes v as JSON with the given status code.
func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v != nil {
		_ = json.NewEncoder(w).Encode(v)
	}
}

// writeError maps err to its HTTP status, error code, message, and details, then
// writes the standard error envelope.
func writeError(w http.ResponseWriter, err error) {
	status, body := errorToResponse(err)
	writeJSON(w, status, errorEnvelope{Error: body})
}

// errorToResponse is the single mapping from a domain error to its HTTP status
// and JSON error body. Every controller uses this mapping.
func errorToResponse(err error) (int, errorBody) {
	var ve models.ValidationError
	switch {
	case errors.As(err, &ve):
		details := make([]detailBody, 0, len(ve.Errors))
		for _, f := range ve.Errors {
			details = append(details, detailBody{Field: f.Field, Issue: f.Issue})
		}
		return http.StatusBadRequest, errorBody{
			Code:    "validation_failed",
			Message: err.Error(),
			Details: details,
		}
	case errors.Is(err, models.ErrNotFound):
		return http.StatusNotFound, errorBody{Code: "not_found", Message: err.Error()}
	case errors.Is(err, models.ErrConflict):
		return http.StatusConflict, errorBody{Code: "conflict", Message: err.Error()}
	case errors.Is(err, models.ErrUnavailable):
		return http.StatusConflict, errorBody{Code: "no_availability", Message: err.Error()}
	default:
		slog.Error("internal error", "error", err)
		return http.StatusInternalServerError, errorBody{
			Code:    "internal_error",
			Message: "internal server error",
		}
	}
}

// parsePagination reads and validates the limit and offset query parameters.
// limit defaults to 20 and is capped at 100; offset defaults to 0.
func parsePagination(r *http.Request) (limit, offset int, err error) {
	limit = 20
	if v := r.URL.Query().Get("limit"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n < 0 {
			return 0, 0, fieldError("limit", "must be a non-negative integer")
		}
		limit = n
	}
	if limit > 100 {
		limit = 100
	}
	if v := r.URL.Query().Get("offset"); v != "" {
		n, convErr := strconv.Atoi(v)
		if convErr != nil || n < 0 {
			return 0, 0, fieldError("offset", "must be a non-negative integer")
		}
		offset = n
	}
	return limit, offset, nil
}

// fieldError builds a single-field validation error.
func fieldError(field, issue string) error {
	return models.ValidationError{Errors: []models.FieldError{{Field: field, Issue: issue}}}
}
