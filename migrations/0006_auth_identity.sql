-- Identity data captured during registration.
--
-- The API contract is frozen: POST /auth/register carries only
-- {fullName, phoneNumber, role, myIdVerificationToken}. Everything below is
-- therefore derived server-side — passport/PINFL come from the consumed MyID
-- verification, and the driver licence + vehicle plate are issued by the
-- licence registry client (simulated in MVP, real registry later). iOS sends
-- nothing new.

ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS passport_series    TEXT;
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS passport_number    TEXT;
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS birth_date         DATE;

-- Driver / equipment provider licence, from the registry lookup.
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS license_number     TEXT;
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS license_categories TEXT[];
-- Uzbek vehicle plate, e.g. '01 123 ABC' or '01 A 123 BC'.
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS vehicle_plate      TEXT;

-- Why a registration was rejected (LICENSE_CATEGORY_MISMATCH), so the admin
-- dashboard can show rejected applicants with a reason instead of a bare flag.
ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS rejection_reason   TEXT;

ALTER TABLE auth.users ADD COLUMN IF NOT EXISTS updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- MyID verification also carries the birth date through to registration.
ALTER TABLE auth.myid_verifications ADD COLUMN IF NOT EXISTS birth_date DATE;

CREATE INDEX IF NOT EXISTS users_role_idx ON auth.users (role, created_at DESC);
CREATE INDEX IF NOT EXISTS myid_verifications_vid_idx ON auth.myid_verifications (verification_id);
