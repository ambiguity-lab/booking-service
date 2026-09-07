package services

import (
	"context"
	"time"

	"github.com/ambiguity-lab/booking-service/internal/models"
	"github.com/ambiguity-lab/booking-service/internal/repositories"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// CreateBookingRequest describes a booking the caller wants to place.
type CreateBookingRequest struct {
	UserID     uuid.UUID
	PropertyID uuid.UUID
	CheckIn    time.Time
	CheckOut   time.Time
	Rooms      int
}

// BookingService orchestrates booking lifecycle operations.
type BookingService struct {
	properties *repositories.PropertyRepository
	bookings   *repositories.BookingRepository
	pool       *pgxpool.Pool
}

// NewBookingService wires together the repositories and pool into a single BookingService.
func NewBookingService(pool *pgxpool.Pool, properties *repositories.PropertyRepository, bookings *repositories.BookingRepository) *BookingService {
	return &BookingService{
		properties: properties,
		bookings:   bookings,
		pool:       pool,
	}
}

// Create validates the request, reserves rooms atomically, and persists a booking.
func (s *BookingService) Create(ctx context.Context, req CreateBookingRequest) (*models.Booking, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	prop, err := s.bookings.LockProperty(ctx, tx, req.PropertyID)
	if err != nil {
		return nil, err
	}

	booked, err := s.bookings.CountOverlappingRooms(ctx, tx, req.PropertyID, req.CheckIn, req.CheckOut)
	if err != nil {
		return nil, err
	}
	if booked+req.Rooms > prop.TotalRooms {
		return nil, models.ErrUnavailable
	}

	nights := int(req.CheckOut.Sub(req.CheckIn).Hours() / 24)

	booking := &models.Booking{
		PropertyID:      req.PropertyID,
		UserID:          req.UserID,
		CheckIn:         req.CheckIn,
		CheckOut:        req.CheckOut,
		Rooms:           req.Rooms,
		Status:          "confirmed",
		TotalPriceCents: prop.BasePriceCents * req.Rooms * nights,
	}

	if err := s.bookings.Create(ctx, tx, booking); err != nil {
		return nil, err
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return booking, nil
}

// Get returns a booking by its id, mapping missing bookings to models.ErrNotFound.
func (s *BookingService) Get(ctx context.Context, id uuid.UUID) (*models.Booking, error) {
	return s.bookings.GetByID(ctx, id)
}

// Cancel transitions a booking to cancelled. Cancelling an already-cancelled booking
// yields models.ErrConflict.
func (s *BookingService) Cancel(ctx context.Context, id uuid.UUID) error {
	b, err := s.bookings.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if b.Status == "cancelled" {
		return models.ErrConflict
	}
	return s.bookings.UpdateStatus(ctx, id, "cancelled")
}

// ListForUser returns the bookings belonging to the given user, paginated.
func (s *BookingService) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error) {
	return s.bookings.ListByUser(ctx, userID, limit, offset)
}

// validateCreate enforces the business rules for new bookings.
func validateCreate(req CreateBookingRequest) error {
	var ve models.ValidationError
	if !req.CheckOut.After(req.CheckIn) {
		ve.Errors = append(ve.Errors, models.FieldError{
			Field: "check_out",
			Issue: "must be after check_in",
		})
	}
	if req.CheckIn.Before(time.Now()) {
		ve.Errors = append(ve.Errors, models.FieldError{
			Field: "check_in",
			Issue: "cannot be in the past",
		})
	}
	if req.Rooms < 1 {
		ve.Errors = append(ve.Errors, models.FieldError{
			Field: "rooms",
			Issue: "must be at least 1",
		})
	}
	if len(ve.Errors) > 0 {
		return ve
	}
	return nil
}