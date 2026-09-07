# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.3.0] - 2026-09-04

### Added

- Property search via `GET /v1/properties`, with text and city filters, result
  ordering, and pagination.
- Property detail endpoint `GET /v1/properties/{id}`.
- Unit tests for the booking service and booking controller.

## [0.2.0] - 2026-08-15

### Added

- Booking creation via `POST /v1/bookings`, including date validation, atomic
  room-availability checks, and automatic price calculation across rooms and
  nights.
- Booking retrieval via `GET /v1/bookings/{id}`.
- Booking cancellation via `POST /v1/bookings/{id}/cancel`, with protection
  against cancelling an already-cancelled booking.
- Per-user booking listing via `GET /v1/users/{id}/bookings` with pagination.
- Standard JSON error envelope with codes `validation_failed`, `not_found`,
  `conflict`, `no_availability`, and `internal_error`.

## [0.1.0] - 2026-07-20

### Added

- Initial Go service scaffold: chi router, environment-based configuration, and
  a pgx connection pool.
- PostgreSQL schema for users, properties, and bookings with seed data and a
  generated search vector.
- Domain models and sentinel errors for bookings, properties, and users.
- Health check endpoint `GET /healthz`.
- HTTP middleware for request IDs, structured JSON request logging, and panic
  recovery.
- Makefile targets for build, run, database load, test, and lint.
- Repository layer with `pgx.ErrNoRows` translated to `models.ErrNotFound`.