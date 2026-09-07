package repositories

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/ambiguity-lab/booking-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// BookingRepository provides data access for bookings backed by a PostgreSQL pool.
type BookingRepository struct {
	pool *pgxpool.Pool
}

// NewBookingRepository creates a BookingRepository backed by the given connection pool.
func NewBookingRepository(pool *pgxpool.Pool) *BookingRepository {
	return &BookingRepository{pool: pool}
}

// Create inserts a new booking within the given transaction.
func (r *BookingRepository) Create(ctx context.Context, tx pgx.Tx, b *models.Booking) error {
	q := `INSERT INTO bookings (property_id, user_id, check_in, check_out, rooms, status, total_price_cents)
	      VALUES ($1, $2, $3, $4, $5, $6, $7)
	      RETURNING id, created_at, updated_at`

	err := tx.QueryRow(ctx, q,
		b.PropertyID, b.UserID, b.CheckIn, b.CheckOut, b.Rooms, b.Status, b.TotalPriceCents,
	).Scan(&b.ID, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return fmt.Errorf("create booking: %w", err)
	}
	return nil
}

// GetByID returns the booking with the given id, or models.ErrNotFound if it does not exist.
func (r *BookingRepository) GetByID(ctx context.Context, id uuid.UUID) (*models.Booking, error) {
	q := `SELECT id, property_id, user_id, check_in, check_out, rooms, status, total_price_cents, created_at, updated_at
	      FROM bookings WHERE id = $1`

	row := r.pool.QueryRow(ctx, q, id)
	b, err := scanBooking(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("get booking by id: %w", err)
	}
	return b, nil
}

// ListByUser returns bookings for the given user in reverse chronological order.
func (r *BookingRepository) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error) {
	q := `SELECT id, property_id, user_id, check_in, check_out, rooms, status, total_price_cents, created_at, updated_at
	      FROM bookings WHERE user_id = $1
	      ORDER BY created_at DESC
	      LIMIT $2 OFFSET $3`

	rows, err := r.pool.Query(ctx, q, userID, limit, offset)
	if err != nil {
		return nil, fmt.Errorf("list bookings by user: %w", err)
	}
	defer rows.Close()

	bookings := make([]models.Booking, 0)
	for rows.Next() {
		b, err := scanBooking(rows)
		if err != nil {
			return nil, fmt.Errorf("scan booking: %w", err)
		}
		bookings = append(bookings, *b)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate bookings: %w", err)
	}
	return bookings, nil
}

// CountOverlappingRooms returns the total number of rooms already booked by confirmed
// bookings whose date range overlaps [checkIn, checkOut).
func (r *BookingRepository) CountOverlappingRooms(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID, checkIn, checkOut time.Time) (int, error) {
	q := `SELECT COALESCE(SUM(rooms), 0)
	      FROM bookings
	      WHERE property_id = $1
	        AND status = 'confirmed'
	        AND check_in < $3
	        AND check_out > $2`

	var count int
	err := tx.QueryRow(ctx, q, propertyID, checkIn, checkOut).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count overlapping rooms: %w", err)
	}
	return count, nil
}

// UpdateStatus sets the status of the booking with the given id.
func (r *BookingRepository) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	q := `UPDATE bookings SET status = $2, updated_at = now() WHERE id = $1`

	tag, err := r.pool.Exec(ctx, q, id, status)
	if err != nil {
		return fmt.Errorf("update booking status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return models.ErrNotFound
	}
	return nil
}

// LockProperty locks the given property row FOR UPDATE within the transaction and returns it.
// The caller is responsible for holding the transaction until the placement is decided.
func (r *BookingRepository) LockProperty(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID) (*models.Property, error) {
	q := `SELECT id, name, city, country, description, base_price_cents, rating, total_rooms, created_at
	      FROM properties WHERE id = $1 FOR UPDATE`

	row := tx.QueryRow(ctx, q, propertyID)
	p, err := scanProperty(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, models.ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("lock property: %w", err)
	}
	return p, nil
}

// bookingScanner is implemented by both pgx.Row and pgx.Rows to scan a single booking.
type bookingScanner interface {
	Scan(dest ...any) error
}

// scanBooking scans one row into a Booking.
func scanBooking(s bookingScanner) (*models.Booking, error) {
	var b models.Booking
	err := s.Scan(&b.ID, &b.PropertyID, &b.UserID, &b.CheckIn, &b.CheckOut,
		&b.Rooms, &b.Status, &b.TotalPriceCents, &b.CreatedAt, &b.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &b, nil
}
