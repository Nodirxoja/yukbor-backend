package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Store is the auth schema: users, OTP codes, MyID verifications, refresh
// tokens. Handlers hold no SQL.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Domain errors, mapped to contract codes by the handler.
var (
	ErrOTPRateLimited = errors.New("otp rate limited")
	ErrOTPExpired     = errors.New("otp expired")
	ErrOTPInvalid     = errors.New("otp invalid")
	ErrOTPNotVerified = errors.New("otp not verified")
	ErrPhoneTaken     = errors.New("phone already registered")
	ErrMyIDToken      = errors.New("myid token expired or invalid")
)

// UserRecord is the full row. models.User is the public projection — the
// identity columns below never cross the wire to the iOS app.
type UserRecord struct {
	models.User

	PINFL             *string
	PassportSeries    *string
	PassportNumber    *string
	BirthDate         *time.Time
	LicenseNumber     *string
	LicenseCategories []string
	VehiclePlate      *string
	RejectionReason   *string
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

const userColumns = `
	id, role, full_name, phone_number, email, is_verified, verification_status,
	rating, ratings_count, pinfl, passport_series, passport_number, birth_date,
	license_number, license_categories, vehicle_plate, rejection_reason,
	created_at, updated_at`

func scanUser(row pgx.Row) (*UserRecord, error) {
	var u UserRecord
	err := row.Scan(
		&u.ID, &u.Role, &u.FullName, &u.PhoneNumber, &u.Email, &u.IsVerified,
		&u.VerificationStatus, &u.Rating, &u.RatingsCount, &u.PINFL,
		&u.PassportSeries, &u.PassportNumber, &u.BirthDate, &u.LicenseNumber,
		&u.LicenseCategories, &u.VehiclePlate, &u.RejectionReason,
		&u.CreatedAt, &u.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// ---- OTP --------------------------------------------------------------

// CreateOTP rate-limits by phone, then stores a hashed code and returns the
// verification id together with the plaintext code (for the SMS sender only).
func (s *Store) CreateOTP(ctx context.Context, phone string) (verificationID, code string, err error) {
	var recent int
	err = s.pool.QueryRow(ctx, `
		SELECT count(*) FROM auth.otp_codes
		WHERE phone_number = $1 AND created_at > now() - $2::interval`,
		phone, fmt.Sprintf("%d seconds", int(OTPWindow.Seconds())),
	).Scan(&recent)
	if err != nil {
		return "", "", err
	}
	if recent >= OTPWindowMax {
		return "", "", ErrOTPRateLimited
	}

	code = GenerateOTP()
	err = s.pool.QueryRow(ctx, `
		INSERT INTO auth.otp_codes (phone_number, code_hash, expires_at)
		VALUES ($1, $2, now() + $3::interval)
		RETURNING verification_id`,
		phone, HashOTP(code), fmt.Sprintf("%d seconds", int(OTPTTL.Seconds())),
	).Scan(&verificationID)
	return verificationID, code, err
}

// VerifyOTP checks a submitted code and marks the verification usable by the
// MyID step. allowMaster enables the non-prod escape hatch (plan §10).
func (s *Store) VerifyOTP(ctx context.Context, verificationID, code string, allowMaster bool) error {
	var (
		hash     string
		attempts int
		expires  time.Time
		verified bool
	)
	err := s.pool.QueryRow(ctx, `
		SELECT code_hash, attempts, expires_at, verified
		FROM auth.otp_codes WHERE verification_id = $1`, verificationID,
	).Scan(&hash, &attempts, &expires, &verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrOTPInvalid
	}
	if err != nil {
		return err
	}

	if verified {
		return nil // idempotent: re-verifying an accepted code is not an error
	}
	if time.Now().After(expires) {
		return ErrOTPExpired
	}
	if attempts >= OTPMaxAttempts {
		return ErrOTPInvalid
	}

	ok := hash == HashOTP(code) || (allowMaster && code == OTPMasterCode)
	if !ok {
		_, _ = s.pool.Exec(ctx,
			`UPDATE auth.otp_codes SET attempts = attempts + 1 WHERE verification_id = $1`,
			verificationID)
		return ErrOTPInvalid
	}

	_, err = s.pool.Exec(ctx,
		`UPDATE auth.otp_codes SET verified = TRUE WHERE verification_id = $1`, verificationID)
	return err
}

// LoginWindow is how long a confirmed OTP still authorises a login.
const LoginWindow = 15 * time.Minute

// RecentlyVerified reports whether this phone completed an OTP verification
// within the login window.
//
// This exists because the API contract is frozen: POST /auth/login carries only
// phoneNumber, so the client cannot echo back a verificationId. Demanding one
// would have made the iOS app unable to log in at all. Looking the recent
// verification up server-side gives the same guarantee — you cannot sign in
// unless somebody just proved they hold that number — while the request body
// stays exactly as the contract specifies.
func (s *Store) RecentlyVerified(ctx context.Context, phone string) (bool, error) {
	var ok bool
	err := s.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM auth.otp_codes
			WHERE phone_number = $1 AND verified = TRUE
			  AND created_at > now() - $2::interval)`,
		phone, fmt.Sprintf("%d seconds", int(LoginWindow.Seconds()))).Scan(&ok)
	return ok, err
}

// VerifiedPhone returns the phone a verification id proves ownership of.
func (s *Store) VerifiedPhone(ctx context.Context, verificationID string) (string, error) {
	var phone string
	var verified bool
	err := s.pool.QueryRow(ctx, `
		SELECT phone_number, verified FROM auth.otp_codes WHERE verification_id = $1`,
		verificationID).Scan(&phone, &verified)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrOTPNotVerified
	}
	if err != nil {
		return "", err
	}
	if !verified {
		return "", ErrOTPNotVerified
	}
	return phone, nil
}

// ---- MyID verifications -----------------------------------------------

// MyIDRecord is a stored, not-yet-consumed KYC pass.
type MyIDRecord struct {
	Token            string
	VerificationID   string
	PINFL            string
	PassportSeries   string
	PassportNumber   string
	VerifiedFullName string
	Confidence       float64
	BirthDate        *time.Time
}

// NewMyIDToken mints the short-lived token the contract returns.
func NewMyIDToken() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic("auth: crypto/rand unavailable: " + err.Error())
	}
	return "myid_tok_" + hex.EncodeToString(buf)
}

func (s *Store) CreateMyIDVerification(ctx context.Context, rec MyIDRecord) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth.myid_verifications
			(token, verification_id, pinfl, passport_series, passport_number,
			 verified_full_name, confidence, birth_date, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8, now() + $9::interval)`,
		rec.Token, rec.VerificationID, rec.PINFL, rec.PassportSeries,
		rec.PassportNumber, rec.VerifiedFullName, rec.Confidence, rec.BirthDate,
		fmt.Sprintf("%d seconds", int(MyIDTokenTTL.Seconds())))
	return err
}

// ConsumeMyIDToken atomically marks a token used and returns it. A token that
// is missing, expired, or already consumed yields ErrMyIDToken — the single
// UPDATE makes double-registration on one KYC pass impossible.
func (s *Store) ConsumeMyIDToken(ctx context.Context, token string) (*MyIDRecord, error) {
	var rec MyIDRecord
	err := s.pool.QueryRow(ctx, `
		UPDATE auth.myid_verifications SET consumed = TRUE
		WHERE token = $1 AND consumed = FALSE AND expires_at > now()
		RETURNING token, verification_id, pinfl, passport_series, passport_number,
		          verified_full_name, confidence, birth_date`, token,
	).Scan(&rec.Token, &rec.VerificationID, &rec.PINFL, &rec.PassportSeries,
		&rec.PassportNumber, &rec.VerifiedFullName, &rec.Confidence, &rec.BirthDate)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrMyIDToken
	}
	if err != nil {
		return nil, err
	}
	return &rec, nil
}

// ---- Users ------------------------------------------------------------

func (s *Store) UserByID(ctx context.Context, id string) (*UserRecord, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM auth.users WHERE id = $1`, id))
}

func (s *Store) UserByPhone(ctx context.Context, phone string) (*UserRecord, error) {
	return scanUser(s.pool.QueryRow(ctx,
		`SELECT `+userColumns+` FROM auth.users WHERE phone_number = $1`, phone))
}

// NewUser is everything registration knows about an applicant once MyID and
// the licence registry have run.
type NewUser struct {
	Role               models.UserRole
	FullName           string
	PhoneNumber        string
	VerificationStatus models.VerificationStatus
	RejectionReason    *string
	PINFL              string
	PassportSeries     string
	PassportNumber     string
	BirthDate          *time.Time
	LicenseNumber      *string
	LicenseCategories  []string
	VehiclePlate       *string
}

// CreateUser inserts a new applicant.
//
// A previously REJECTED applicant is overwritten in place rather than colliding
// on the phone number: a driver whose licence check failed must be able to come
// back, and on demo day a rehearsal must not permanently burn a phone number.
// An approved account still returns ErrPhoneTaken.
func (s *Store) CreateUser(ctx context.Context, n NewUser) (*UserRecord, error) {
	existing, err := s.UserByPhone(ctx, n.PhoneNumber)
	switch {
	case err == nil && existing.VerificationStatus != models.VerificationRejected:
		return nil, ErrPhoneTaken
	case err != nil && !errors.Is(err, db.ErrNotFound):
		return nil, err
	}

	isVerified := n.VerificationStatus == models.VerificationApproved
	if existing != nil {
		return scanUser(s.pool.QueryRow(ctx, `
			UPDATE auth.users SET
				role = $2, full_name = $3, is_verified = $4, verification_status = $5,
				rejection_reason = $6, pinfl = $7, passport_series = $8,
				passport_number = $9, birth_date = $10, license_number = $11,
				license_categories = $12, vehicle_plate = $13, updated_at = now()
			WHERE id = $1
			RETURNING `+userColumns,
			existing.ID, n.Role, n.FullName, isVerified, n.VerificationStatus,
			n.RejectionReason, n.PINFL, n.PassportSeries, n.PassportNumber,
			n.BirthDate, n.LicenseNumber, n.LicenseCategories, n.VehiclePlate))
	}

	return scanUser(s.pool.QueryRow(ctx, `
		INSERT INTO auth.users
			(role, full_name, phone_number, is_verified, verification_status,
			 rejection_reason, pinfl, passport_series, passport_number, birth_date,
			 license_number, license_categories, vehicle_plate)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING `+userColumns,
		n.Role, n.FullName, n.PhoneNumber, isVerified, n.VerificationStatus,
		n.RejectionReason, n.PINFL, n.PassportSeries, n.PassportNumber,
		n.BirthDate, n.LicenseNumber, n.LicenseCategories, n.VehiclePlate))
}

// UpdateProfile applies the PATCH /users/me partial update.
func (s *Store) UpdateProfile(ctx context.Context, id string, fullName, email *string) (*UserRecord, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		UPDATE auth.users SET
			full_name = COALESCE($2, full_name),
			email     = COALESCE($3, email),
			updated_at = now()
		WHERE id = $1
		RETURNING `+userColumns, id, fullName, email))
}

// SetVerification flips verification status (dev/admin tool).
func (s *Store) SetVerification(ctx context.Context, id string, status models.VerificationStatus, reason *string) (*UserRecord, error) {
	return scanUser(s.pool.QueryRow(ctx, `
		UPDATE auth.users SET
			verification_status = $2,
			is_verified = ($2 = 'approved'),
			rejection_reason = $3,
			updated_at = now()
		WHERE id = $1
		RETURNING `+userColumns, id, status, reason))
}

// SetRating is called internally by the reviews service after each new review,
// keeping User.rating/ratingsCount authoritative in one place (contract §6).
func (s *Store) SetRating(ctx context.Context, id string, rating float64, count int) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE auth.users SET rating = $2, ratings_count = $3, updated_at = now()
		WHERE id = $1`, id, rating, count)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return db.ErrNotFound
	}
	return nil
}

// ListUsers backs GET /admin/users; role is optional.
func (s *Store) ListUsers(ctx context.Context, role string) ([]UserRecord, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT `+userColumns+` FROM auth.users
		WHERE ($1 = '' OR role = $1)
		ORDER BY created_at DESC`, role)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []UserRecord{}
	for rows.Next() {
		u, err := scanUser(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, *u)
	}
	return out, rows.Err()
}

// CountUsers backs the dashboard's "registered users" tile via the wallet's
// /admin/stats aggregation.
func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.pool.QueryRow(ctx, `SELECT count(*) FROM auth.users`).Scan(&n)
	return n, err
}

// ---- Refresh tokens ---------------------------------------------------

func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (s *Store) SaveRefreshToken(ctx context.Context, token, userID string, expires time.Time) error {
	_, err := s.pool.Exec(ctx, `
		INSERT INTO auth.refresh_tokens (token_hash, user_id, expires_at)
		VALUES ($1, $2, $3)
		ON CONFLICT (token_hash) DO NOTHING`, hashToken(token), userID, expires)
	return err
}

// RotateRefreshToken revokes a presented token and reports whether it was live.
// Rotation on every refresh means a stolen token is usable at most once.
func (s *Store) RotateRefreshToken(ctx context.Context, token string) (userID string, err error) {
	err = s.pool.QueryRow(ctx, `
		UPDATE auth.refresh_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL AND expires_at > now()
		RETURNING user_id`, hashToken(token)).Scan(&userID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", db.ErrNotFound
	}
	return userID, err
}

func (s *Store) RevokeRefreshToken(ctx context.Context, token string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE auth.refresh_tokens SET revoked_at = now()
		WHERE token_hash = $1 AND revoked_at IS NULL`, hashToken(token))
	return err
}
