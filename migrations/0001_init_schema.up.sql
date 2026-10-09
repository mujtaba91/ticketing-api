-- citext compares case-insensitively, so A@x.com and a@x.com are one account.
CREATE EXTENSION IF NOT EXISTS citext;

CREATE TYPE role_type AS ENUM ('owner', 'staff');

CREATE TABLE users (
    id                    uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name                  text NOT NULL,
    email                 citext NOT NULL UNIQUE,
    password_hash         text NOT NULL,
    is_admin              boolean NOT NULL DEFAULT false,
    token_version         integer NOT NULL DEFAULT 1
);

CREATE TABLE refresh_tokens (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    token_hash    text NOT NULL UNIQUE,
    user_id       uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    version       integer NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    expires_at    timestamptz NOT NULL,
    revoked_at    timestamptz
);

-- Postgres does not index foreign keys on its own.
CREATE INDEX refresh_tokens_user_id_idx ON refresh_tokens (user_id);

CREATE TABLE organizations (
    id   uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name text NOT NULL
);

CREATE TABLE organization_members (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    role            role_type NOT NULL,
    UNIQUE (organization_id, user_id)
);

CREATE TABLE events (
    id              uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name            text NOT NULL,
    organization_id uuid NOT NULL REFERENCES organizations (id) ON DELETE CASCADE,
    max_seats       integer NOT NULL CHECK (max_seats > 0),
    starts_at       timestamptz NOT NULL
);

CREATE INDEX events_organization_id_idx ON events (organization_id);

CREATE TABLE bookings (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id      uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    event_id     uuid NOT NULL REFERENCES events (id) ON DELETE CASCADE,
    created_at   timestamptz NOT NULL DEFAULT now(),
    cancelled_at timestamptz
);

-- Partial so a user can book again after cancelling.
CREATE UNIQUE INDEX bookings_active_event_user_idx ON bookings (event_id, user_id) WHERE cancelled_at IS NULL;
