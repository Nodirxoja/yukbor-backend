package auth

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Uzbek driver licence and vehicle plate issuance.
//
// The API contract is frozen — iOS never sends a licence number or a plate —
// so the registry client issues them itself from the applicant's PINFL. Every
// value is DERIVED, not random: the same PINFL always yields the same licence
// and plate, so a rehearsed demo shows the same data every run, and the seed
// script produces stable fixtures.
//
// Formats (as used in Uzbekistan):
//
//	licence number  AB1234567          two letters + seven digits
//	plate, form A   01 123 ABC         region + three digits + three letters
//	plate, form B   01 A 123 BC        region + letter + three digits + two letters
//
// Which plate form a driver gets is itself derived from the PINFL, so the demo
// shows both shapes across the seeded fleet.

// plateAlphabet omits I, O, Q and W — they are not used on Uzbek plates
// because they read as 1/0 or do not appear in the Latin transliteration set.
const plateAlphabet = "ABCDEFGHJKLMNPRSTUVXYZ"

// plateRegions are the real region codes printed on Uzbek plates.
var plateRegions = []string{
	"01", // Toshkent shahri
	"10", // Toshkent viloyati
	"20", // Buxoro
	"25", // Jizzax
	"30", // Qashqadaryo
	"40", // Navoiy
	"50", // Namangan
	"60", // Samarqand
	"70", // Surxondaryo
	"75", // Sirdaryo
	"80", // Farg'ona
	"85", // Xorazm
	"90", // Qoraqalpog'iston
}

// derive turns a PINFL plus a salt into a stable stream of numbers. Distinct
// salts keep the licence number and the plate from correlating visibly.
func derive(pinfl, salt string) func(n int) int {
	sum := sha256.Sum256([]byte(salt + ":" + pinfl))
	i := 0
	return func(n int) int {
		if n <= 0 {
			return 0
		}
		// Walk the digest in 4-byte windows, wrapping when exhausted.
		off := (i * 4) % (len(sum) - 4)
		i++
		return int(binary.BigEndian.Uint32(sum[off:off+4]) % uint32(n))
	}
}

// GenerateLicenseNumber issues a licence number in the AB1234567 form.
func GenerateLicenseNumber(pinfl string) string {
	next := derive(pinfl, "license")
	letters := []byte{
		plateAlphabet[next(len(plateAlphabet))],
		plateAlphabet[next(len(plateAlphabet))],
	}
	return fmt.Sprintf("%s%07d", letters, next(10_000_000))
}

// GenerateVehiclePlate issues a plate in one of the two Uzbek forms.
func GenerateVehiclePlate(pinfl string) string {
	next := derive(pinfl, "plate")
	region := plateRegions[next(len(plateRegions))]
	digits := next(1000)

	if next(2) == 0 {
		// Form A: 01 123 ABC
		return fmt.Sprintf("%s %03d %c%c%c", region, digits,
			plateAlphabet[next(len(plateAlphabet))],
			plateAlphabet[next(len(plateAlphabet))],
			plateAlphabet[next(len(plateAlphabet))])
	}
	// Form B: 01 A 123 BC
	return fmt.Sprintf("%s %c %03d %c%c", region,
		plateAlphabet[next(len(plateAlphabet))], digits,
		plateAlphabet[next(len(plateAlphabet))],
		plateAlphabet[next(len(plateAlphabet))])
}

// RequiredCategoryForRole is the category checked at REGISTRATION time. The
// contract gives us the role but not the specific vehicle, so we gate on the
// broadest requirement the role implies:
//
//	driver            → C   (any truck over 3.5t)
//	equipmentProvider → C   (self-propelled equipment)
//
// The narrower CE check happens later, when a driver accepts a leg whose cargo
// actually requires a tractor-trailer — see RequiredLicenseCategory.
func RequiredCategoryForRole(role models.UserRole) string {
	switch role {
	case models.RoleDriver, models.RoleEquipmentProvider:
		return "C"
	default:
		return "" // clients and labor providers need no licence
	}
}

// NeedsLicenseCheck reports whether a role must clear the licence registry.
func NeedsLicenseCheck(role models.UserRole) bool {
	return RequiredCategoryForRole(role) != ""
}
