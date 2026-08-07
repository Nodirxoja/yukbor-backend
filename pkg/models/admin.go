package models

import "time"

// UserDetail is User plus the operational fields the iOS contract does not
// carry: registration time, the licence issued by the registry, and why an
// applicant was rejected.
//
// It serves two consumers, neither of which is the iOS app:
//   - GET /admin/users — the dashboard's users table (plan §11)
//   - GET /internal/users/{id} — the orders service, which re-checks a
//     driver's licence category before letting them take a tractor-trailer leg
//
// PINFL and passport data deliberately stay out: nothing downstream needs
// them, so they never leave the auth service.
type UserDetail struct {
	User

	CreatedAt         time.Time `json:"createdAt"`
	LicenseNumber     *string   `json:"licenseNumber"`
	LicenseCategories []string  `json:"licenseCategories"`
	VehiclePlate      *string   `json:"vehiclePlate"`
	RejectionReason   *string   `json:"rejectionReason"`
}

// HasCategory reports whether the user's licence carries a category.
func (u UserDetail) HasCategory(category string) bool {
	for _, c := range u.LicenseCategories {
		if c == category {
			return true
		}
	}
	return false
}

// RatingUpdate is the body of POST /internal/users/{id}/rating — the reviews
// service pushing a recomputed aggregate back into the users table (§6).
type RatingUpdate struct {
	Rating float64 `json:"rating"`
	Count  int     `json:"count"`
}
