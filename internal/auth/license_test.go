package auth

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

var (
	reLicense = regexp.MustCompile(`^[A-Z]{2}\d{7}$`)
	rePlateA  = regexp.MustCompile(`^\d{2} \d{3} [A-Z]{3}$`) // 01 123 ABC
	rePlateB  = regexp.MustCompile(`^\d{2} [A-Z] \d{3} [A-Z]{2}$`)
)

// pinfls spans the sex digit (1-6) and both licence-trigger parities.
func pinfls() []string {
	out := make([]string, 0, 60)
	for lead := 1; lead <= 6; lead++ {
		for i := 0; i < 10; i++ {
			out = append(out, fmt.Sprintf("%d234567890123%d", lead, i))
		}
	}
	return out
}

func TestGenerateLicenseNumber(t *testing.T) {
	for _, p := range pinfls() {
		got := GenerateLicenseNumber(p)
		if !reLicense.MatchString(got) {
			t.Fatalf("pinfl %s: licence %q is not AB1234567 form", p, got)
		}
		if again := GenerateLicenseNumber(p); again != got {
			t.Fatalf("pinfl %s: licence not deterministic (%q vs %q)", p, got, again)
		}
	}
}

func TestGenerateVehiclePlate(t *testing.T) {
	var sawA, sawB bool
	for _, p := range pinfls() {
		got := GenerateVehiclePlate(p)
		switch {
		case rePlateA.MatchString(got):
			sawA = true
		case rePlateB.MatchString(got):
			sawB = true
		default:
			t.Fatalf("pinfl %s: plate %q matches neither Uzbek form", p, got)
		}
		if !slices.Contains(plateRegions, got[:2]) {
			t.Fatalf("pinfl %s: plate %q has region %q that is not a real code", p, got, got[:2])
		}
		if again := GenerateVehiclePlate(p); again != got {
			t.Fatalf("pinfl %s: plate not deterministic (%q vs %q)", p, got, again)
		}
	}
	// Both plate shapes must occur, otherwise the demo only ever shows one.
	if !sawA || !sawB {
		t.Errorf("expected both plate forms across the sample, got A=%v B=%v", sawA, sawB)
	}
}

func TestSimulatedFullName(t *testing.T) {
	for _, p := range pinfls() {
		got := SimulatedFullName(p)
		parts := strings.Fields(got)
		if len(parts) != 3 {
			t.Fatalf("pinfl %s: %q is not Surname Name Patronymic", p, got)
		}
		// Sex digit (first) must agree with the name's grammatical gender.
		female := isFemalePINFL(p)
		gotFemale := strings.HasSuffix(parts[0], "ova") && strings.HasSuffix(parts[2], "ovna")
		gotMale := strings.HasSuffix(parts[0], "ov") && strings.HasSuffix(parts[2], "ovich")
		if female && !gotFemale {
			t.Errorf("pinfl %s (female): %q has male endings", p, got)
		}
		if !female && !gotMale {
			t.Errorf("pinfl %s (male): %q has female endings", p, got)
		}
		if again := SimulatedFullName(p); again != got {
			t.Errorf("pinfl %s: name not deterministic", p)
		}
	}
}

func TestRequiredCategoryForRole(t *testing.T) {
	cases := map[models.UserRole]string{
		models.RoleDriver:            "C",
		models.RoleEquipmentProvider: "C",
		models.RoleClient:            "",
		models.RoleLaborProvider:     "",
	}
	for role, want := range cases {
		if got := RequiredCategoryForRole(role); got != want {
			t.Errorf("role %s: required category %q, want %q", role, got, want)
		}
		if got := NeedsLicenseCheck(role); got != (want != "") {
			t.Errorf("role %s: NeedsLicenseCheck %v", role, got)
		}
	}
}

// The registration-time gate uses RequiredCategory; the accept-time gate uses
// VehicleType. Both must reach the same verdict for the same licence.
func TestLicenseGateBothForms(t *testing.T) {
	v := SimulatedLicenseVerifier{}
	oddPINFL := "32109876543211"  // → categories ["B"]
	evenPINFL := "32109876543212" // → ["B","C","CE"]
	tractor := models.VehicleTractorTrailer

	byRole, _ := v.VerifyDriverLicense(t.Context(), "", LicenseVerificationRequest{
		PINFL: oddPINFL, RequiredCategory: "C"})
	byVehicle, _ := v.VerifyDriverLicense(t.Context(), "", LicenseVerificationRequest{
		PINFL: oddPINFL, VehicleType: &tractor})
	if byRole.Status != models.VerificationRejected || byVehicle.Status != models.VerificationRejected {
		t.Errorf("odd PINFL must be rejected either way: role=%v vehicle=%v",
			byRole.Status, byVehicle.Status)
	}

	ok, _ := v.VerifyDriverLicense(t.Context(), "", LicenseVerificationRequest{
		PINFL: evenPINFL, RequiredCategory: "C"})
	if ok.Status != models.VerificationApproved {
		t.Errorf("even PINFL must be approved, got %v (%s)", ok.Status, ok.Reason)
	}
	// The registry issues both artefacts on every successful lookup.
	if !reLicense.MatchString(ok.LicenseNumber) || ok.VehiclePlate == "" {
		t.Errorf("approved lookup must issue licence + plate, got %q / %q",
			ok.LicenseNumber, ok.VehiclePlate)
	}
}
