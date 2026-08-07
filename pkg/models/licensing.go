package models

// Driver licence categories required to operate each vehicle type
// (Uzbekistan, Vienna-convention style).
//
// This mapping lives in pkg/models because BOTH services enforce it and
// internal/<svc> may never import another internal/<svc>:
//
//   - auth checks it at registration, against the licence the registry issued
//   - orders re-checks it when a driver accepts a leg, since the cargo decides
//     what the load actually requires
//
// One copy means the two gates cannot drift apart.
const (
	LicenseCategoryTruck        = "C"  // rigid trucks over 3.5t
	LicenseCategoryTruckTrailer = "CE" // truck with trailer
)

// RequiredLicenseCategory returns the category a driver must hold to carry a
// load with the given vehicle type.
func RequiredLicenseCategory(v VehicleType) string {
	switch v {
	case VehicleTractorTrailer:
		return LicenseCategoryTruckTrailer
	case VehicleFlatbed, VehicleRefrigerated, VehicleTanker,
		VehicleDumpTruck, VehicleBoxTruck:
		return LicenseCategoryTruck
	}
	return ""
}
