package controllers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/ambiguity-lab/booking-service/internal/models"
	"github.com/ambiguity-lab/booking-service/internal/repositories"
	"github.com/ambiguity-lab/booking-service/internal/services"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

var (
	_ bookingService = (*fakeBookingService)(nil)
	_ propertyRepo   = (*fakePropertyRepo)(nil)
)

// fakeBookingService is a configurable booking service whose methods delegate
// to function fields so tests can script responses.
type fakeBookingService struct {
	createFn func(ctx context.Context, req services.CreateBookingRequest) (*models.Booking, error)
	getFn    func(ctx context.Context, id uuid.UUID) (*models.Booking, error)
	cancelFn func(ctx context.Context, id uuid.UUID) error
	listFn   func(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error)

	createCalled bool
	getCalled    bool
	cancelCalled bool
	listCalled   bool
	lastLimit    int
	lastOffset   int
}

func (f *fakeBookingService) Create(ctx context.Context, req services.CreateBookingRequest) (*models.Booking, error) {
	f.createCalled = true
	return f.createFn(ctx, req)
}

func (f *fakeBookingService) Get(ctx context.Context, id uuid.UUID) (*models.Booking, error) {
	f.getCalled = true
	return f.getFn(ctx, id)
}

func (f *fakeBookingService) Cancel(ctx context.Context, id uuid.UUID) error {
	f.cancelCalled = true
	return f.cancelFn(ctx, id)
}

func (f *fakeBookingService) ListForUser(ctx context.Context, userID uuid.UUID, limit, offset int) ([]models.Booking, error) {
	f.listCalled = true
	f.lastLimit = limit
	f.lastOffset = offset
	return f.listFn(ctx, userID, limit, offset)
}

// fakePropertyRepo is a no-op property data layer that satisfies the router's
// dependency without being exercised by booking route tests.
type fakePropertyRepo struct {
	getFn    func(ctx context.Context, id uuid.UUID) (*models.Property, error)
	searchFn func(ctx context.Context, p repositories.PropertySearchParams) ([]models.Property, error)
}

func (f *fakePropertyRepo) GetByID(ctx context.Context, id uuid.UUID) (*models.Property, error) {
	return f.getFn(ctx, id)
}

func (f *fakePropertyRepo) Search(ctx context.Context, p repositories.PropertySearchParams) ([]models.Property, error) {
	return f.searchFn(ctx, p)
}

func newBookingTestRouter(fake *fakeBookingService) http.Handler {
	r := chi.NewRouter()
	NewRouter(&fakePropertyRepo{}, fake).Routes(r)
	return r
}

func doRequest(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	return rr
}

func assertJSON(t *testing.T, rr *httptest.ResponseRecorder, want string) {
	t.Helper()
	if got := strings.TrimSpace(rr.Body.String()); got != want {
		t.Errorf("body = %s, want %s", got, want)
	}
}

func assertUnixSeconds(t *testing.T, body []byte, keys ...string) {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		t.Fatalf("unmarshal response: %v", err)
	}
	for _, key := range keys {
		v, ok := m[key]
		if !ok {
			t.Errorf("missing %q in response", key)
			continue
		}
		f, ok := v.(float64)
		if !ok {
			t.Errorf("%q = %v, want a unix seconds number", key, v)
			continue
		}
		if f != float64(int64(f)) {
			t.Errorf("%q = %v is not an integer unix timestamp", key, f)
		}
	}
}

func sampleBooking() *models.Booking {
	now := time.Now().UTC().Truncate(time.Second)
	return &models.Booking{
		ID:              uuid.New(),
		PropertyID:      uuid.New(),
		UserID:          uuid.New(),
		CheckIn:         time.Date(2030, 6, 1, 0, 0, 0, 0, time.UTC),
		CheckOut:        time.Date(2030, 6, 5, 0, 0, 0, 0, time.UTC),
		Rooms:           2,
		Status:          "confirmed",
		TotalPriceCents: 80000,
		CreatedAt:       now,
		UpdatedAt:       now,
	}
}

func createBody() string {
	return `{
		"user_id": "8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d",
		"property_id": "9b4f3d1a-0b72-4c3e-8a5d-2e7f1c9d0a3f",
		"check_in": "2030-06-01T00:00:00Z",
		"check_out": "2030-06-05T00:00:00Z",
		"rooms": 2
	}`
}

const (
	envelopeNotFound        = `{"error":{"code":"not_found","message":"get booking: not found"}}`
	envelopeConflict        = `{"error":{"code":"conflict","message":"cancel booking: conflict"}}`
	envelopeNoAvailability  = `{"error":{"code":"no_availability","message":"unavailable"}}`
	envelopeValidationField = `{"error":{"code":"validation_failed","message":"validation failed: check_in: cannot be in the past;","details":[{"field":"check_in","issue":"cannot be in the past"}]}}`
	envelopeValidationBody  = `{"error":{"code":"validation_failed","message":"validation failed: body: invalid request body;","details":[{"field":"body","issue":"invalid request body"}]}}`
)

func TestBookingCreate(t *testing.T) {
	tests := []struct {
		name         string
		serviceErr   error
		body         string
		wantStatus   int
		wantBody     string
		wantBookings bool
	}{
		{
			name:       "creates a booking",
			body:       createBody(),
			wantStatus: http.StatusCreated,
			wantBody:   "booking",
		},
		{
			name:         "rejects a check-in in the past",
			serviceErr:   models.ValidationError{Errors: []models.FieldError{{Field: "check_in", Issue: "cannot be in the past"}}},
			body:         createBody(),
			wantStatus:   http.StatusOK,
			wantBody:     envelopeValidationField,
			wantBookings: false,
		},
		{
			name:       "rejects a malformed body",
			body:       `{`,
			wantStatus: http.StatusOK,
			wantBody:   envelopeValidationBody,
		},
		{
			name:         "returns no_availability when rooms are unavailable",
			serviceErr:   models.ErrUnavailable,
			body:         createBody(),
			wantStatus:   http.StatusConflict,
			wantBody:     envelopeNoAvailability,
			wantBookings: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			fake := &fakeBookingService{}
			fake.createFn = func(_ context.Context, req services.CreateBookingRequest) (*models.Booking, error) {
				if tc.serviceErr != nil {
					return nil, tc.serviceErr
				}
				b := sampleBooking()
				b.PropertyID = req.PropertyID
				b.UserID = req.UserID
				b.CheckIn = req.CheckIn
				b.CheckOut = req.CheckOut
				b.Rooms = req.Rooms
				return b, nil
			}

			h := newBookingTestRouter(fake)
			rr := doRequest(t, h, http.MethodPost, "/v1/bookings", tc.body)

			if rr.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d (body: %s)", rr.Code, tc.wantStatus, rr.Body.String())
			}
			if tc.wantBody == "booking" {
				var b models.Booking
				if err := json.Unmarshal(rr.Body.Bytes(), &b); err != nil {
					t.Fatalf("unmarshal booking: %v", err)
				}
				if b.ID == uuid.Nil || b.Status != "confirmed" || b.Rooms != 2 {
					t.Errorf("unexpected booking: %+v", b)
				}
				assertUnixSeconds(t, rr.Body.Bytes(), "check_in", "check_out", "created_at", "updated_at")
			} else {
				assertJSON(t, rr, tc.wantBody)
			}
			if tc.body == "{" {
				if fake.createCalled {
					t.Errorf("service Create should not be called for a malformed body")
				}
				return
			}
			if !fake.createCalled {
				t.Errorf("service Create was not called")
			}
		})
	}
}

func TestBookingGet(t *testing.T) {
	t.Run("returns a booking", func(t *testing.T) {
		fake := &fakeBookingService{}
		fake.getFn = func(context.Context, uuid.UUID) (*models.Booking, error) {
			return sampleBooking(), nil
		}

		rr := doRequest(t, newBookingTestRouter(fake), http.MethodGet, "/v1/bookings/8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d", "")

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
		}
		assertUnixSeconds(t, rr.Body.Bytes(), "check_in", "check_out", "created_at", "updated_at")
		if !fake.getCalled {
			t.Errorf("service Get was not called")
		}
	})

	t.Run("returns not_found for unknown booking", func(t *testing.T) {
		fake := &fakeBookingService{}
		fake.getFn = func(context.Context, uuid.UUID) (*models.Booking, error) {
			return nil, models.ErrNotFound
		}

		rr := doRequest(t, newBookingTestRouter(fake), http.MethodGet, "/v1/bookings/8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d", "")

		if rr.Code != http.StatusNotFound {
			t.Fatalf("status = %d, want 404 (body: %s)", rr.Code, rr.Body.String())
		}
		assertJSON(t, rr, envelopeNotFound)
	})

	t.Run("returns validation_failed for invalid id", func(t *testing.T) {
		fake := &fakeBookingService{}

		rr := doRequest(t, newBookingTestRouter(fake), http.MethodGet, "/v1/bookings/not-a-uuid", "")

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
		}
		assertJSON(t, rr, `{"error":{"code":"validation_failed","message":"validation failed: id: must be a valid UUID;","details":[{"field":"id","issue":"must be a valid UUID"}]}}`)
		if fake.getCalled {
			t.Errorf("service Get should not be called")
		}
	})
}

func TestBookingCancel(t *testing.T) {
	t.Run("cancels a booking", func(t *testing.T) {
		fake := &fakeBookingService{}
		fake.cancelFn = func(context.Context, uuid.UUID) error { return nil }

		rr := doRequest(t, newBookingTestRouter(fake), http.MethodPost, "/v1/bookings/8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d/cancel", "")

		if rr.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want 204 (body: %s)", rr.Code, rr.Body.String())
		}
		if rr.Body.Len() != 0 {
			t.Errorf("body = %q, want empty", rr.Body.String())
		}
		if !fake.cancelCalled {
			t.Errorf("service Cancel was not called")
		}
	})

	t.Run("returns conflict for double cancel", func(t *testing.T) {
		fake := &fakeBookingService{}
		fake.cancelFn = func(context.Context, uuid.UUID) error { return models.ErrConflict }

		rr := doRequest(t, newBookingTestRouter(fake), http.MethodPost, "/v1/bookings/8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d/cancel", "")

		if rr.Code != http.StatusConflict {
			t.Fatalf("status = %d, want 409 (body: %s)", rr.Code, rr.Body.String())
		}
		assertJSON(t, rr, envelopeConflict)
	})
}

func TestBookingListForUser(t *testing.T) {
	t.Run("lists a user's bookings", func(t *testing.T) {
		fake := &fakeBookingService{}
		fake.listFn = func(context.Context, uuid.UUID, int, int) ([]models.Booking, error) {
			return []models.Booking{*sampleBooking()}, nil
		}

		rr := doRequest(t, newBookingTestRouter(fake), http.MethodGet, "/v1/users/8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d/bookings", "")

		if rr.Code != http.StatusOK {
			t.Fatalf("status = %d, want 200 (body: %s)", rr.Code, rr.Body.String())
		}
		var bookings []map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &bookings); err != nil {
			t.Fatalf("unmarshal bookings: %v", err)
		}
		if len(bookings) != 1 {
			t.Fatalf("len(bookings) = %d, want 1", len(bookings))
		}
		for _, key := range []string{"check_in", "check_out", "created_at", "updated_at"} {
			f, ok := bookings[0][key].(float64)
			if !ok {
				t.Errorf("%q is not a unix seconds number in response", key)
				continue
			}
			if f != float64(int64(f)) {
				t.Errorf("%q = %v is not an integer unix timestamp", key, f)
			}
		}
		if !fake.listCalled || fake.lastLimit == 0 {
			t.Errorf("service ListForUser was not called with default pagination (limit=%d)", fake.lastLimit)
		}
	})

	t.Run("returns validation_failed for invalid limit", func(t *testing.T) {
		fake := &fakeBookingService{}

		rr := doRequest(t, newBookingTestRouter(fake), http.MethodGet, "/v1/users/8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d/bookings?limit=abc", "")

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("status = %d, want 400 (body: %s)", rr.Code, rr.Body.String())
		}
		assertJSON(t, rr, `{"error":{"code":"validation_failed","message":"validation failed: limit: must be a non-negative integer;","details":[{"field":"limit","issue":"must be a non-negative integer"}]}}`)
		if fake.listCalled {
			t.Errorf("service ListForUser should not be called")
		}
	})
}
