CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- -------------------------------------------------------------------
-- Users
-- -------------------------------------------------------------------
CREATE TABLE users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    email         text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now()
);

-- -------------------------------------------------------------------
-- Properties
-- -------------------------------------------------------------------
CREATE TABLE properties (
    id               uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name             text NOT NULL,
    city             text NOT NULL,
    country          text NOT NULL,
    description      text NOT NULL DEFAULT '',
    base_price_cents integer NOT NULL,
    rating           numeric(2,1) NOT NULL DEFAULT 0.0,
    total_rooms      integer NOT NULL,
    created_at       timestamptz NOT NULL DEFAULT now(),
    search_vector    tsvector GENERATED ALWAYS AS (
                         to_tsvector('english', name || ' ' || description)
                     ) STORED
);

CREATE INDEX idx_properties_search ON properties USING GIN (search_vector);

-- -------------------------------------------------------------------
-- Bookings
-- -------------------------------------------------------------------
CREATE TABLE bookings (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    property_id       uuid NOT NULL REFERENCES properties(id),
    user_id           uuid NOT NULL REFERENCES users(id),
    check_in          date NOT NULL,
    check_out         date NOT NULL,
    rooms             integer NOT NULL,
    status            text NOT NULL DEFAULT 'confirmed'
                        CHECK (status IN ('confirmed', 'cancelled')),
    total_price_cents integer NOT NULL,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX idx_bookings_property_dates
    ON bookings (property_id, check_in, check_out);
