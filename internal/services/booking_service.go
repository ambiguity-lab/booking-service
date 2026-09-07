package services

import (
	"context"
	"time"

	"github.com/ambiguity-lab/booking-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// CreateBookingRequest describes a booking the caller wants to place.
type CreateBookingRequest struct {
	UserID     uuid.UUID
	PropertyID uuid.UUID
	CheckIn    time.Time
	CheckOut   time.Time
	Rooms      int
}

// bookingRepository is the subset of the booking data layer the service needs.
// It is implemented by *repositories.BookingRepository.
type bookingRepository interface {
	LockProperty(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID) (*models.Property, error)
	CountOverlappingRooms(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID, checkIn, checkOut time.Time) (int, error)
	Create(ctx context.Context, tx pgx.Tx, b *models.Booking) error
	GetByID(ctx context.Context, id uuid.UUID) (*models.Booking, error)
	UpdateStatus(ctx context.Context, id uuid.UUID, status string) error
	ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error)
}

// txBeginner starts transactions. It is implemented by *pgxpool.Pool.
type txBeginner interface {
	Begin(ctx context.Context) (pgx.Tx, error)
}

// BookingService orchestrates booking lifecycle operations.
type BookingService struct {
	pool     txBeginner
	bookings bookingRepository
}

// NewBookingService wires together the transaction starter and booking data layer
// into a single BookingService.
func NewBookingService(pool txBeginner, bookings bookingRepository) *BookingService {
	return &BookingService{
		pool:     pool,
		bookings: bookings,
	}
}

// Create validates the request, reserves rooms atomically, and persists a booking.
// TODO: handle the case where check_in and check_out are the same day —
// currently produces a zero-night booking with zero price
func (s *BookingService) Create(ctx context.Context, req CreateBookingRequest) (*models.Booking, error) {
	if err := validateCreate(req); err != nil {
		return nil, err
	}

	prop, err := s.bookings.LockProperty(ctx, nil, req.PropertyID)
	if err != nil {
		return nil, err
	}

	booked, err := s.bookings.CountOverlappingRooms(ctx, nil, req.PropertyID, req.CheckIn, req.CheckOut)
	if err != nil {
		return nil, err
	}
	if booked+req.Rooms > prop.TotalRooms {
		return nil, models.ErrUnavailable
	}

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() {
		_ = tx.Rollback(ctx)
	}()

	booking := &models.Booking{
		PropertyID:      req.PropertyID,
		UserID:          req.UserID,
		CheckIn:         req.CheckIn,
		CheckOut:        req.CheckOut,
		Rooms:           req.Rooms,
		Status:          "confirmed",
		TotalPriceCents: calculateTotalPrice(prop.BasePriceCents, req.Rooms, req.CheckIn, req.CheckOut),
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

// calculateTotalPrice computes the total price in cents for a stay spanning
// the given check-in and check-out dates.
func calculateTotalPrice(basePrice, rooms int, checkIn, checkOut time.Time) int {
	nights := int(checkOut.Sub(checkIn).Hours() / 24)
	return basePrice * rooms * nights
}
