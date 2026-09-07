package services

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"

	"github.com/ambiguity-lab/booking-service/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

var (
	_ bookingRepository = (*fakeBookingRepo)(nil)
	_ txBeginner        = fakePool{}
)

// nopTx satisfies pgx.Tx while only implementing the operations the service
// actually exercises. Any other embedded method call panics.
type nopTx struct {
	pgx.Tx
}

func (nopTx) Commit(context.Context) error   { return nil }
func (nopTx) Rollback(context.Context) error { return nil }

// fakePool is a transaction starter that hands out nop transactions.
type fakePool struct{}

func (fakePool) Begin(context.Context) (pgx.Tx, error) { return &nopTx{}, nil }

// fakeBookingRepo is a configurable booking data layer. Every method delegates
// to a function field so tests can script per-call behavior.
type fakeBookingRepo struct {
	lockPropertyFn          func(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID) (*models.Property, error)
	countOverlappingRoomsFn func(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID, checkIn, checkOut time.Time) (int, error)
	createFn                func(ctx context.Context, tx pgx.Tx, b *models.Booking) error
	getByIDFn               func(ctx context.Context, id uuid.UUID) (*models.Booking, error)
	updateStatusFn          func(ctx context.Context, id uuid.UUID, status string) error
	listByUserFn            func(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error)

	createCalled     bool
	updateCalled     bool
	lastUpdateID     uuid.UUID
	lastUpdateStatus string
}

func (f *fakeBookingRepo) LockProperty(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID) (*models.Property, error) {
	return f.lockPropertyFn(ctx, tx, propertyID)
}

func (f *fakeBookingRepo) CountOverlappingRooms(ctx context.Context, tx pgx.Tx, propertyID uuid.UUID, checkIn, checkOut time.Time) (int, error) {
	return f.countOverlappingRoomsFn(ctx, tx, propertyID, checkIn, checkOut)
}

func (f *fakeBookingRepo) Create(ctx context.Context, tx pgx.Tx, b *models.Booking) error {
	return f.createFn(ctx, tx, b)
}

func (f *fakeBookingRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Booking, error) {
	return f.getByIDFn(ctx, id)
}

func (f *fakeBookingRepo) UpdateStatus(ctx context.Context, id uuid.UUID, status string) error {
	return f.updateStatusFn(ctx, id, status)
}

func (f *fakeBookingRepo) ListByUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error) {
	return f.listByUserFn(ctx, userID, limit, offset)
}

func newFakeBookingRepo() *fakeBookingRepo {
	f := &fakeBookingRepo{}
	f.lockPropertyFn = func(_ context.Context, _ pgx.Tx, _ uuid.UUID) (*models.Property, error) {
		return &models.Property{}, nil
	}
	f.countOverlappingRoomsFn = func(context.Context, pgx.Tx, uuid.UUID, time.Time, time.Time) (int, error) {
		return 0, nil
	}
	f.createFn = func(_ context.Context, _ pgx.Tx, b *models.Booking) error {
		f.createCalled = true
		b.ID = uuid.New()
		b.CreatedAt = time.Now().UTC()
		b.UpdatedAt = time.Now().UTC()
		return nil
	}
	f.getByIDFn = func(context.Context, uuid.UUID) (*models.Booking, error) {
		return &models.Booking{}, nil
	}
	f.updateStatusFn = func(_ context.Context, id uuid.UUID, status string) error {
		f.updateCalled = true
		f.lastUpdateID = id
		f.lastUpdateStatus = status
		return nil
	}
	f.listByUserFn = func(context.Context, uuid.UUID, int, int) ([]models.Booking, error) {
		return nil, nil
	}
	return f
}

type createCase struct {
	name        string
	req         CreateBookingRequest
	basePrice   int
	totalRooms  int
	overlapping int
	wantErr     error
	wantFields  []models.FieldError
	wantPrice   int
	wantCreate  bool
}

func TestBookingService_Create(t *testing.T) {
	userID := uuid.New()
	propertyID := uuid.New()

	tests := []createCase{
		{
			name: "rejects check_out not after check_in",
			req: CreateBookingRequest{
				UserID:     userID,
				PropertyID: propertyID,
				CheckIn:    time.Now().AddDate(0, 0, 5),
				CheckOut:   time.Now().AddDate(0, 0, 4),
				Rooms:      1,
			},
			wantErr:    models.ErrValidation,
			wantFields: []models.FieldError{{Field: "check_out", Issue: "must be after check_in"}},
		},
		{
			name: "rejects check_in in the past",
			req: CreateBookingRequest{
				UserID:     userID,
				PropertyID: propertyID,
				CheckIn:    time.Now().AddDate(0, 0, -1),
				CheckOut:   time.Now().AddDate(0, 0, 2),
				Rooms:      1,
			},
			wantErr:    models.ErrValidation,
			wantFields: []models.FieldError{{Field: "check_in", Issue: "cannot be in the past"}},
		},
		{
			name: "rejects fewer than one room",
			req: CreateBookingRequest{
				UserID:     userID,
				PropertyID: propertyID,
				CheckIn:    time.Now().AddDate(0, 0, 1),
				CheckOut:   time.Now().AddDate(0, 0, 3),
				Rooms:      0,
			},
			wantErr:    models.ErrValidation,
			wantFields: []models.FieldError{{Field: "rooms", Issue: "must be at least 1"}},
		},
		{
			name: "reports every invalid field",
			req: CreateBookingRequest{
				UserID:     userID,
				PropertyID: propertyID,
				CheckIn:    time.Now().AddDate(0, 0, -3),
				CheckOut:   time.Now().AddDate(0, 0, -4),
				Rooms:      0,
			},
			wantErr: models.ErrValidation,
			wantFields: []models.FieldError{
				{Field: "check_out", Issue: "must be after check_in"},
				{Field: "check_in", Issue: "cannot be in the past"},
				{Field: "rooms", Issue: "must be at least 1"},
			},
		},
		{
			name: "rejects when demand exceeds capacity",
			req: CreateBookingRequest{
				UserID:     userID,
				PropertyID: propertyID,
				CheckIn:    time.Now().AddDate(0, 0, 1),
				CheckOut:   time.Now().AddDate(0, 0, 4),
				Rooms:      3,
			},
			basePrice:   10000,
			totalRooms:  10,
			overlapping: 9,
			wantErr:     models.ErrUnavailable,
		},
		{
			name: "prices multiple rooms across multiple nights",
			req: CreateBookingRequest{
				UserID:     userID,
				PropertyID: propertyID,
				CheckIn:    time.Date(2030, 1, 10, 0, 0, 0, 0, time.UTC),
				CheckOut:   time.Date(2030, 1, 13, 0, 0, 0, 0, time.UTC),
				Rooms:      2,
			},
			basePrice:  10000,
			totalRooms: 10,
			wantPrice:  10000 * 2 * 3,
			wantCreate: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeBookingRepo()
			fake.lockPropertyFn = func(_ context.Context, _ pgx.Tx, id uuid.UUID) (*models.Property, error) {
				return &models.Property{ID: id, TotalRooms: tc.totalRooms, BasePriceCents: tc.basePrice}, nil
			}
			fake.countOverlappingRoomsFn = func(context.Context, pgx.Tx, uuid.UUID, time.Time, time.Time) (int, error) {
				return tc.overlapping, nil
			}

			svc := NewBookingService(fakePool{}, fake)
			booking, err := svc.Create(context.Background(), tc.req)

			if tc.wantErr != nil {
				if err == nil {
					t.Fatalf("want error %v, got nil", tc.wantErr)
				}
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("want error %v, got %v", tc.wantErr, err)
				}
				if tc.wantFields != nil {
					assertValidation(t, err, tc.wantFields)
				}
				if fake.createCalled {
					t.Errorf("repository Create should not be called")
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !fake.createCalled {
				t.Errorf("repository Create was not called")
			}
			if booking.TotalPriceCents != tc.wantPrice {
				t.Errorf("TotalPriceCents = %d, want %d", booking.TotalPriceCents, tc.wantPrice)
			}
			if booking.Rooms != tc.req.Rooms {
				t.Errorf("Rooms = %d, want %d", booking.Rooms, tc.req.Rooms)
			}
			if booking.Status != "confirmed" {
				t.Errorf("Status = %q, want confirmed", booking.Status)
			}
			if !booking.CheckIn.Equal(tc.req.CheckIn) || !booking.CheckOut.Equal(tc.req.CheckOut) {
				t.Errorf("dates not preserved: got %v..%v, want %v..%v", booking.CheckIn, booking.CheckOut, tc.req.CheckIn, tc.req.CheckOut)
			}
		})
	}
}

type cancelCase struct {
	name       string
	status     string
	getErr     error
	wantErr    error
	wantUpdate bool
}

func TestBookingService_Cancel(t *testing.T) {
	id := uuid.New()

	tests := []cancelCase{
		{
			name:       "cancels a confirmed booking",
			status:     "confirmed",
			wantUpdate: true,
		},
		{
			name:    "rejects double cancel",
			status:  "cancelled",
			wantErr: models.ErrConflict,
		},
		{
			name:    "returns not found for unknown booking",
			getErr:  models.ErrNotFound,
			wantErr: models.ErrNotFound,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := newFakeBookingRepo()
			fake.getByIDFn = func(_ context.Context, _ uuid.UUID) (*models.Booking, error) {
				if tc.getErr != nil {
					return nil, tc.getErr
				}
				return &models.Booking{ID: id, Status: tc.status}, nil
			}

			svc := NewBookingService(fakePool{}, fake)
			err := svc.Cancel(context.Background(), id)

			if tc.wantErr != nil {
				if err == nil || !errors.Is(err, tc.wantErr) {
					t.Errorf("want error %v, got %v", tc.wantErr, err)
				}
				if fake.updateCalled {
					t.Errorf("repository UpdateStatus should not be called")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !fake.updateCalled {
				t.Errorf("repository UpdateStatus was not called")
			}
			if fake.lastUpdateID != id {
				t.Errorf("UpdateStatus id = %v, want %v", fake.lastUpdateID, id)
			}
			if fake.lastUpdateStatus != "cancelled" {
				t.Errorf("UpdateStatus status = %q, want cancelled", fake.lastUpdateStatus)
			}
		})
	}
}

func assertValidation(t *testing.T, err error, want []models.FieldError) {
	t.Helper()
	var ve models.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want models.ValidationError, got %T", err)
	}
	if !reflect.DeepEqual(ve.Errors, want) {
		t.Errorf("field errors = %v, want %v", ve.Errors, want)
	}
}
