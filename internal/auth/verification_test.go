package auth

import (
	"context"
	"testing"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

func TestSimulatedLicenseVerifier(t *testing.T) {
	v := SimulatedLicenseVerifier{}
	flatbed := models.VehicleFlatbed
	tractor := models.VehicleTractorTrailer

	// PINFL ending 6 or 8 → B,C,CE → approved for any truck.
	res, err := v.VerifyDriverLicense(context.Background(), "u1", LicenseVerificationRequest{
		PINFL: "12345678901238", VehicleType: &tractor,
	})
	if err != nil || res.Status != models.VerificationApproved {
		t.Fatalf("PINFL ending 8 should be approved for CE, got %+v err=%v", res, err)
	}

	// PINFL ending 0/2/4 → B,C → fine as a driver, refused a tractor-trailer.
	res, err = v.VerifyDriverLicense(context.Background(), "u3", LicenseVerificationRequest{
		PINFL: "12345678901234", RequiredCategory: "C",
	})
	if err != nil || res.Status != models.VerificationApproved {
		t.Fatalf("PINFL ending 4 must register fine as a driver, got %+v err=%v", res, err)
	}
	res, err = v.VerifyDriverLicense(context.Background(), "u3", LicenseVerificationRequest{
		PINFL: "12345678901234", VehicleType: &tractor,
	})
	if err != nil || res.Status != models.VerificationRejected {
		t.Fatalf("PINFL ending 4 holds C but not CE and must be refused a tractor-trailer, got %+v", res)
	}

	// Odd-ending PINFL → only B → rejected for trucks.
	res, err = v.VerifyDriverLicense(context.Background(), "u2", LicenseVerificationRequest{
		PINFL: "12345678901231", VehicleType: &flatbed,
	})
	if err != nil {
		t.Fatal(err)
	}
	if res.Status != models.VerificationRejected || res.Reason != ErrLicenseCategory.Error() {
		t.Fatalf("odd PINFL must be rejected with LICENSE_CATEGORY_MISMATCH, got %+v", res)
	}
}

func TestSimulatedMyIDClient(t *testing.T) {
	c := SimulatedMyIDClient{}

	if _, err := c.VerifyPerson(context.Background(), MyIDVerifyRequest{PassportNumber: "0000000", PINFL: "12345678901234"}); err != ErrPassportNotFound {
		t.Fatalf("passport 0000000 must trigger PASSPORT_NOT_FOUND, got %v", err)
	}

	res, err := c.VerifyPerson(context.Background(), MyIDVerifyRequest{PassportNumber: "1234567", PINFL: "99345678901234"})
	if err != nil || res.IsMatched {
		t.Fatalf("PINFL 99... must trigger face mismatch, got %+v err=%v", res, err)
	}

	res, err = c.VerifyPerson(context.Background(), MyIDVerifyRequest{PassportSeries: "AB", PassportNumber: "1234567", PINFL: "12345678901234"})
	if err != nil || !res.IsMatched || res.Confidence < 0.93 || res.Confidence > 0.99 {
		t.Fatalf("normal request must match with confidence in [0.93,0.99], got %+v err=%v", res, err)
	}
}
