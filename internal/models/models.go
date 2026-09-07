package models

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
)

// User represents a registered user in the system.
type User struct {
	// ID is the unique identifier for the user.
	ID uuid.UUID `json:"id"`
	// Email is the user's email address, unique across all users.
	Email string `json:"email"`
	// PasswordHash is the bcrypt hash of the user's password. Never exposed via JSON.
	PasswordHash string `json:"-"`
	// CreatedAt is the timestamp when the user account was created.
	CreatedAt time.Time `json:"created_at"`
}

// Property represents a bookable property such as a hotel or apartment.
type Property struct {
	// ID is the unique identifier for the property.
	ID uuid.UUID `json:"id"`
	// Name is the display name of the property.
	Name string `json:"name"`
	// City is the city where the property is located.
	City string `json:"city"`
	// Country is the country where the property is located.
	Country string `json:"country"`
	// Description is a free-text description of the property.
	Description string `json:"description"`
	// BasePriceCents is the nightly base price in the smallest currency unit (e.g. cents).
	BasePriceCents int `json:"base_price_cents"`
	// Rating is the aggregate guest rating on a 0.0–5.0 scale.
	Rating float64 `json:"rating"`
	// TotalRooms is the total number of rooms available at the property.
	TotalRooms int `json:"total_rooms"`
	// CreatedAt is the timestamp when the property was created.
	CreatedAt time.Time `json:"created_at"`
	// Rank holds the full-text search relevance score when populated by a search query.
	// Omitted from JSON when zero.
	Rank float64 `json:"rank,omitempty"`
}

// Booking represents a reservation made by a user for a property.
type Booking struct {
	// ID is the unique identifier for the booking.
	ID uuid.UUID `json:"id"`
	// PropertyID references the property being booked.
	PropertyID uuid.UUID `json:"property_id"`
	// UserID references the user who made the booking.
	UserID uuid.UUID `json:"user_id"`
	// CheckIn is the first night of the stay.
	CheckIn time.Time `json:"check_in"`
	// CheckOut is the day of departure (no night is booked for this date).
	CheckOut time.Time `json:"check_out"`
	// Rooms is the number of rooms reserved.
	Rooms int `json:"rooms"`
	// Status is the current status of the booking: "confirmed" or "cancelled".
	Status string `json:"status"`
	// TotalPriceCents is the total cost in the smallest currency unit (e.g. cents).
	TotalPriceCents int `json:"total_price_cents"`
	// CreatedAt is the timestamp when the booking was created.
	CreatedAt time.Time `json:"created_at"`
	// UpdatedAt is the timestamp of the last status change.
	UpdatedAt time.Time `json:"updated_at"`
}

// Sentinel errors returned by the model and repository layers.
var (
	// ErrNotFound indicates that the requested entity does not exist.
	ErrNotFound = errors.New("not found")
	// ErrValidation indicates that the provided data failed validation rules.
	ErrValidation = errors.New("validation failed")
	// ErrConflict indicates a state conflict, such as cancelling an already-cancelled booking.
	ErrConflict = errors.New("conflict")
	// ErrUnavailable indicates that the requested resources (e.g. rooms) are not available for the given dates.
	ErrUnavailable = errors.New("unavailable")
)

// FieldError describes a single validation failure on a specific field.
type FieldError struct {
	// Field is the name of the field that failed validation.
	Field string
	// Issue is a human-readable description of the validation failure.
	Issue string
}

// Error returns a combined message of all field-level validation errors.
func (ve ValidationError) Error() string {
	var b strings.Builder
	b.WriteString("validation failed:")
	for _, f := range ve.Errors {
		fmt.Fprintf(&b, " %s: %s;", f.Field, f.Issue)
	}
	return b.String()
}

// Is reports whether target is ErrValidation.
func (ve ValidationError) Is(target error) bool {
	return target == ErrValidation
}

// ValidationError holds one or more field-level validation failures.
type ValidationError struct {
	// Errors is the list of individual field validation failures.
	Errors []FieldError
}
