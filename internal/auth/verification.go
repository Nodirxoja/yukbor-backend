package auth

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// ---- MyID KYC (contract v1.1, §1 POST /auth/myid/verify) ---------------
//
// The iOS app sends passport data + selfie to OUR backend; we proxy to the
// MyID B2B/B2G API (myid.uz, OAuth2 client-credentials + REST) and return
// the face-match result. The client never talks to MyID directly.
//
// On success we issue a short-lived myIdVerificationToken (TTL ~10 min) that
// the client passes to POST /auth/register instead of re-sending documents.
// After a successful MyID pass, verificationStatus is immediately "approved"
// (no manual document moderation).

// MyIDClient is the seam for the real MyID integration.
// MVP binds MockMyIDClient (always matches, ~like the iOS mock).
// Production: OAuth2 client-credentials client against the MyID partner API.
type MyIDClient interface {
	VerifyPerson(ctx context.Context, req MyIDVerifyRequest) (MyIDVerifyResult, error)
}

type MyIDVerifyRequest struct {
	VerificationID string // same id as in otp/verify — binds KYC to the phone
	PassportSeries string // e.g. "AB"
	PassportNumber string // e.g. "1234567"
	PINFL          string // 14 digits
	BirthDate      string // YYYY-MM-DD
	Selfie         []byte // jpeg/png from multipart form
}

type MyIDVerifyResult struct {
	IsMatched        bool
	Confidence       float64
	VerifiedFullName string
	// Error codes to surface per contract: PASSPORT_NOT_FOUND, FACE_MISMATCH,
	// MYID_SERVICE_UNAVAILABLE.
}

// MyIDTokenTTL is the lifetime of a myIdVerificationToken.
const MyIDTokenTTL = 10 * time.Minute

// Sentinel errors mapped to contract error codes by the handler.
var (
	ErrPassportNotFound = errors.New("PASSPORT_NOT_FOUND")
	ErrFaceMismatch     = errors.New("FACE_MISMATCH")
	ErrMyIDUnavailable  = errors.New("MYID_SERVICE_UNAVAILABLE")
	ErrLicenseCategory  = errors.New("LICENSE_CATEGORY_MISMATCH")
)

// SimulatedMyIDClient is a high-fidelity imitation of the MyID B2B API: the
// demo must look whole, so it has realistic latency, plausible confidence
// scores, AND deterministic failure triggers so the rejection path can be
// shown on stage (see plan §10):
//
//   - passport number "0000000"   → PASSPORT_NOT_FOUND
//   - PINFL starting with "99"    → FACE_MISMATCH (isMatched=false)
//   - anything else               → matched, confidence 0.93–0.99
type SimulatedMyIDClient struct{}

func (SimulatedMyIDClient) VerifyPerson(ctx context.Context, req MyIDVerifyRequest) (MyIDVerifyResult, error) {
	// Realistic upstream latency (~2s, like the iOS mock).
	select {
	case <-time.After(2 * time.Second):
	case <-ctx.Done():
		return MyIDVerifyResult{}, ErrMyIDUnavailable
	}
	if req.PassportNumber == "0000000" {
		return MyIDVerifyResult{}, ErrPassportNotFound
	}
	if strings.HasPrefix(req.PINFL, "99") {
		return MyIDVerifyResult{IsMatched: false, Confidence: 0.41}, nil
	}
	// Deterministic pseudo-confidence in [0.93, 0.99] derived from the PINFL,
	// so repeated demo runs look stable but not hard-coded.
	var sum int
	for _, c := range req.PINFL {
		sum += int(c)
	}
	confidence := 0.93 + float64(sum%7)/100.0
	return MyIDVerifyResult{
		IsMatched:        true,
		Confidence:       confidence,
		VerifiedFullName: SimulatedFullName(req.PINFL),
	}, nil
}

// ---- Driver license check (drivers & equipment providers) --------------
//
// After MyID confirms identity, drivers/equipmentProviders must additionally
// pass a license check: query the license by PINFL/number against the
// government registry, then validate the category against the vehicle they
// want to operate. Category missing or license invalid → registration
// REJECTED (LICENSE_CATEGORY_MISMATCH).
//
// MVP: ENFORCED for real, with a simulated registry upstream — the rules run,
// only the lookup is imitated. The production registry client replaces the
// lookup, never the rules.
type LicenseVerifier interface {
	VerifyDriverLicense(ctx context.Context, userID string, req LicenseVerificationRequest) (LicenseResult, error)
}

type LicenseVerificationRequest struct {
	PINFL         string
	LicenseNumber string
	// What the applicant wants to operate — drives the category check.
	VehicleType   *models.VehicleType
	EquipmentType *models.EquipmentType
	// RequiredCategory is the direct form, used at registration where the
	// contract gives us a role but not a specific vehicle. Ignored when
	// VehicleType is set.
	RequiredCategory string
}

type LicenseResult struct {
	Status     models.VerificationStatus
	Categories []string // e.g. ["B", "C", "CE"]
	Reason     string   // e.g. "LICENSE_CATEGORY_MISMATCH"

	// Issued by the registry lookup. iOS never sends these — the contract is
	// frozen — so the backend derives them from the PINFL (see license.go).
	LicenseNumber string // AB1234567
	VehiclePlate  string // "01 123 ABC" or "01 A 123 BC"
}

// requiredCategory resolves what the applicant must hold: an explicit vehicle
// type wins, otherwise the role-level requirement passed by the caller.
func (r LicenseVerificationRequest) requiredCategory() string {
	if r.VehicleType != nil {
		return models.RequiredLicenseCategory(*r.VehicleType)
	}
	return r.RequiredCategory
}

// SimulatedLicenseVerifier imitates the government license registry with
// deterministic outcomes, keyed on the LAST digit of the PINFL so every
// enforcement path can be shown on stage (plan §10):
//
//	1,3,5,7,9 → ["B"]           car only      → REJECTED at registration
//	0,2,4     → ["B","C"]       rigid trucks  → registers fine, but REFUSED a
//	                                            tractor-trailer load on accept
//	6,8       → ["B","C","CE"]  everything    → no restrictions
//
// The middle bucket is what makes the second gate real: without a driver who
// holds C but not CE, the accept-time check could never fire, and a rule that
// can never fire is not a rule. The category logic itself is genuine and
// survives the swap to the production registry client — only the lookup is
// imitated.
type SimulatedLicenseVerifier struct{}

// LicenseCategoriesFor is the simulated registry lookup, exposed so the seed
// script can pick PINFLs that produce the drivers a demo needs.
func LicenseCategoriesFor(pinfl string) []string {
	if pinfl == "" {
		return []string{"B", "C", "CE"}
	}
	switch last := pinfl[len(pinfl)-1] - '0'; {
	case last%2 == 1:
		return []string{"B"}
	case last >= 6:
		return []string{"B", "C", "CE"}
	default:
		return []string{"B", "C"}
	}
}

func (SimulatedLicenseVerifier) VerifyDriverLicense(ctx context.Context, _ string, req LicenseVerificationRequest) (LicenseResult, error) {
	// Realistic registry latency.
	select {
	case <-time.After(700 * time.Millisecond):
	case <-ctx.Done():
		return LicenseResult{}, ctx.Err()
	}
	categories := LicenseCategoriesFor(req.PINFL)

	// The registry issues the licence number and the plate of the vehicle it
	// is registered to; both are derived from the PINFL so they stay stable
	// across demo runs (license.go).
	res := LicenseResult{
		Status:        models.VerificationApproved,
		Categories:    categories,
		LicenseNumber: GenerateLicenseNumber(req.PINFL),
		VehiclePlate:  GenerateVehiclePlate(req.PINFL),
	}

	if need := req.requiredCategory(); need != "" && !slices.Contains(categories, need) {
		res.Status = models.VerificationRejected
		res.Reason = ErrLicenseCategory.Error()
	}
	return res, nil
}
