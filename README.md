# booking-service

booking-service is a Go HTTP API for a travel booking platform. It lets clients
search properties, place bookings with automatic room-availability checks and
price calculation, retrieve and cancel bookings, and list a user's bookings.
The API is organized around a small set of REST routes under `/v1`, structured
JSON responses, and a single, consistent error envelope for everything that
fails. Data lives in a local PostgreSQL database.

## Requirements

- Go 1.23
- PostgreSQL 16 or later running locally

## Getting started

```sh
createdb booking
cp .env.example .env
make db-load
make build
./bin/api
```

`make db-load` executes the schema and seed data through `psql` using the
`DATABASE_URL` from your shell environment, so export the values from `.env`
first (e.g. `set -a; source .env; set +a`). The server itself loads `.env`
automatically at startup.

## Configuration

| Variable      | Description                                          |
| ------------- | ---------------------------------------------------- |
| `DATABASE_URL` | PostgreSQL connection string, e.g. `postgres://postgres:postgres@localhost:5432/booking?sslmode=disable` |
| `PORT`         | HTTP listen port. Defaults to `8080`.                |

## API reference

All routes are at `/v1` except the health check. Request bodies and responses
are JSON; timestamps use RFC3339 strings.

| Method | Path                          | Request body                     | Success response        |
| ------ | ----------------------------- | -------------------------------- | ----------------------- |
| GET    | `/healthz`                    | —                                | `200` health status     |
| GET    | `/v1/properties`              | — (query params)                 | `200` list of properties |
| GET    | `/v1/properties/{id}`         | —                                | `200` property          |
| POST   | `/v1/bookings`                | booking                          | `201` booking           |
| GET    | `/v1/bookings/{id}`           | —                                | `200` booking           |
| POST   | `/v1/bookings/{id}/cancel`    | —                                | `204` empty body        |
| GET    | `/v1/users/{id}/bookings`     | — (query params)                 | `200` list of bookings  |

### GET /healthz

Returns the service's liveness status.

```json
{
  "status": "ok"
}
```

### GET /v1/properties

Lists and searches properties. Accepts the query parameters `q`, `city`,
`sort`, `limit`, and `offset`. `city` filters results to a single city; `sort`
controls result ordering; `limit` and `offset` paginate the results.

```json
[
  {
    "id": "c2e3f4a5-b6c7-4d8e-9f0a-1b2c3d4e5f60",
    "name": "Seaside Resort",
    "city": "Barcelona",
    "country": "Spain",
    "description": "A beautiful seaside resort with stunning views of the Mediterranean.",
    "base_price_cents": 12000,
    "rating": 4.5,
    "total_rooms": 50,
    "created_at": "2026-01-01T12:00:00Z"
  }
]
```

### GET /v1/properties/{id}

Returns a single property by id. `rank` is omitted from the response when it is
zero.

### POST /v1/bookings

Creates a booking for a property and a user. Rooms are reserved atomically and
the total price is computed from the property's nightly base price, the number
of rooms, and the number of nights between `check_in` and `check_out`.

Request body:

```json
{
  "user_id": "8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d",
  "property_id": "9b4f3d1a-0b72-4c3e-8a5d-2e7f1c9d0a3f",
  "check_in": "2030-06-01T00:00:00Z",
  "check_out": "2030-06-05T00:00:00Z",
  "rooms": 2
}
```

Success response (`201`):

```json
{
  "id": "1f2e3d4c-5b6a-4f3e-8d2c-1a2b3c4d5e6f",
  "property_id": "9b4f3d1a-0b72-4c3e-8a5d-2e7f1c9d0a3f",
  "user_id": "8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d",
  "check_in": "2030-06-01T00:00:00Z",
  "check_out": "2030-06-05T00:00:00Z",
  "rooms": 2,
  "status": "confirmed",
  "total_price_cents": 80000,
  "created_at": "2026-09-08T10:30:00Z",
  "updated_at": "2026-09-08T10:30:00Z"
}
```

### GET /v1/bookings/{id}

Returns a single booking by id.

### POST /v1/bookings/{id}/cancel

Cancels a booking. Has no request body and no response body on success.

### GET /v1/users/{id}/bookings

Lists the bookings belonging to a user, newest first. Accepts the `limit` and
`offset` query parameters for pagination.

```json
[
  {
    "id": "1f2e3d4c-5b6a-4f3e-8d2c-1a2b3c4d5e6f",
    "property_id": "9b4f3d1a-0b72-4c3e-8a5d-2e7f1c9d0a3f",
    "user_id": "8a5e28ec-457a-4a1c-b11f-6e0b6a5e1c2d",
    "check_in": "2030-06-01T00:00:00Z",
    "check_out": "2030-06-05T00:00:00Z",
    "rooms": 2,
    "status": "confirmed",
    "total_price_cents": 80000,
    "created_at": "2026-09-08T10:30:00Z",
    "updated_at": "2026-09-08T10:30:00Z"
  }
]
```

## Error handling

Every non-2xx response uses the same envelope. The error code, a human-readable
message, and (for validation failures only) the offending fields are nested
under `error`:

```json
{
  "error": {
    "code": "validation_failed",
    "message": "validation failed: check_in: cannot be in the past;",
    "details": [
      {
        "field": "check_in",
        "issue": "cannot be in the past"
      }
    ]
  }
}
```

`details` is present only when the code is `validation_failed`; all other codes
omit it. `internal_error` responses always use the generic message `internal
server error`.

### Status codes by route

| Route                        | Status codes                      |
| ---------------------------- | --------------------------------- |
| `GET /healthz`               | `200`                             |
| `GET /v1/properties`         | `200`, `400`, `500`               |
| `GET /v1/properties/{id}`    | `200`, `400`, `404`, `500`        |
| `POST /v1/bookings`          | `201`, `400`, `404`, `409`, `500` |
| `GET /v1/bookings/{id}`      | `200`, `400`, `404`, `500`        |
| `POST /v1/bookings/{id}/cancel` | `204`, `400`, `404`, `409`, `500` |
| `GET /v1/users/{id}/bookings` | `200`, `400`, `500`               |

### Error codes

| Code                | HTTP status | Description                                              |
| ------------------- | ----------- | -------------------------------------------------------- |
| `validation_failed` | `400`       | The request body, path, or query parameters failed validation. |
| `not_found`         | `404`       | The requested entity does not exist.                     |
| `conflict`          | `409`       | The operation conflicts with current state, e.g. cancelling an already-cancelled booking. |
| `no_availability`   | `409`       | The requested rooms are not available for the given dates. |
| `internal_error`    | `500`       | An unexpected server error occurred.                     |

## Project layout

```
booking-service/
├── cmd/
│   └── api/            # entrypoint: config, database pool, HTTP server
├── db/
│   ├── schema.sql      # tables, indexes, generated search vector
│   └── seed.sql        # sample users and properties
├── internal/
│   ├── controllers/    # HTTP routing, handlers, error envelope
│   ├── middleware/     # request id, structured logging, panic recovery
│   ├── models/         # domain types and sentinel errors
│   ├── repositories/   # PostgreSQL data access
│   └── services/       # booking orchestration and business rules
├── Makefile
├── .env.example
├── README.md
└── CHANGELOG.md
```

## Conventions

- Repository methods wrap every database error with `%w`
  (`fmt.Errorf("create booking: %w", err)`) so callers can use `errors.Is` and
  `errors.As`.
- `pgx.ErrNoRows` is translated to `models.ErrNotFound` at the repository
  boundary; it never propagates beyond it.
- Controllers never return raw database errors. Any error that does not map to
  a known sentinel becomes `internal_error` with a generic message.
