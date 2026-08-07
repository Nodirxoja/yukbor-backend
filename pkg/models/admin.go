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

// AdminStats backs the dashboard's stats row (plan §11).
//
// The money figures are aggregated in the wallet's own SQL; the counts come
// from orders and auth over internal HTTP. Wallet could read the other schemas
// directly — it is one database — but services owning their own data is the
// property that makes splitting the repo later a non-event, and a back-office
// screen polled every 10s can afford two extra calls.
type AdminStats struct {
	TotalOrders     int `json:"totalOrders"`
	ActiveOrders    int `json:"activeOrders"`
	CompletedOrders int `json:"completedOrders"`
	RegisteredUsers int `json:"registeredUsers"`

	CreditedToExecutors string `json:"creditedToExecutors"` // Σ released (amount − commission)
	ServiceFeesCharged  string `json:"serviceFeesCharged"`  // Σ commission of released
	HeldInEscrow        string `json:"heldInEscrow"`        // Σ held amounts
}

// OrderStats is GET /internal/orders/stats.
type OrderStats struct {
	Total     int `json:"total"`
	Active    int `json:"active"`
	Completed int `json:"completed"`
}

// UserStats is GET /internal/users/stats.
type UserStats struct {
	Total int `json:"total"`
}
