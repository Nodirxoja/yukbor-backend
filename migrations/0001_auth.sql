-- auth schema: users, OTP, refresh tokens, verification documents
CREATE SCHEMA IF NOT EXISTS auth;

CREATE TABLE IF NOT EXISTS auth.users (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    role                TEXT NOT NULL CHECK (role IN ('client','driver','equipmentProvider','laborProvider','fleetAdmin','admin')),
    full_name           TEXT NOT NULL,
    phone_number        TEXT NOT NULL UNIQUE,
    email               TEXT,
    is_verified         BOOLEAN NOT NULL DEFAULT FALSE,
    verification_status TEXT NOT NULL DEFAULT 'pending' CHECK (verification_status IN ('pending','approved','rejected')),
    rating              NUMERIC(3,2) NOT NULL DEFAULT 0,
    ratings_count       INTEGER NOT NULL DEFAULT 0,
    pinfl               TEXT, -- filled by MyID integration (post-MVP)
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS auth.otp_codes (
    verification_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    phone_number    TEXT NOT NULL,
    code_hash       TEXT NOT NULL,          -- sha256, never plaintext
    attempts        INTEGER NOT NULL DEFAULT 0,
    verified        BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at      TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS otp_codes_phone_idx ON auth.otp_codes (phone_number, created_at);

CREATE TABLE IF NOT EXISTS auth.refresh_tokens (
    token_hash TEXT PRIMARY KEY,
    user_id    UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ
);

-- MyID KYC (contract v1.1): short-lived tokens issued by POST /auth/myid/verify
-- and consumed by POST /auth/register (MYID_TOKEN_EXPIRED_OR_INVALID otherwise).
CREATE TABLE IF NOT EXISTS auth.myid_verifications (
    token              TEXT PRIMARY KEY,            -- myid_tok_...
    verification_id    UUID NOT NULL,               -- links to otp_codes (phone binding)
    pinfl              TEXT NOT NULL,
    passport_series    TEXT NOT NULL,
    passport_number    TEXT NOT NULL,
    verified_full_name TEXT NOT NULL,
    confidence         NUMERIC(4,3),
    consumed           BOOLEAN NOT NULL DEFAULT FALSE,
    expires_at         TIMESTAMPTZ NOT NULL,        -- created + ~10 min
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Post-MVP: uploaded documents for manual/MyID verification of executors
CREATE TABLE IF NOT EXISTS auth.verification_documents (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    kind       TEXT NOT NULL CHECK (kind IN ('passport','driver_license','vehicle_passport')),
    file_url   TEXT,
    status     TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','approved','rejected')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
