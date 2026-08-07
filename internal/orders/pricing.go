package orders

import (
	"math"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Server-side pricing (plan §6, §10).
//
// The formula is deliberately identical to the client's: weight × tariff per
// ton + equipment hours × hourly rate + workers × hours × rate, floored at the
// minimum order value. Tariffs come from orders.tariffs so they can change
// without an app release.
//
// The breakdown is what makes combo orders payable. One order carries a single
// priceEstimate but settles with 2-3 executors independently, so each leg needs
// its own amount — the escrow row for (orderId, payeeId) is opened from the
// LEG price, never the order total.
//
// All money is int64 UZS. Uzbek sums have no subunit in practice and floats
// would drift; the wire format stays a decimal string.

// Tariff keys, mirroring migration 0007. The platform commission is NOT here:
// it belongs to the wallet, which is the single owner of that number (0008).
const (
	TariffTransportPerTon    = "transport_per_ton"
	TariffLaborHourPerWorker = "labor_hour_per_worker"
	TariffMinimumOrder       = "minimum_order"
)

// Tariffs is the loaded config table.
type Tariffs map[string]int64

func (t Tariffs) get(key string, fallback int64) int64 {
	if v, ok := t[key]; ok {
		return v
	}
	return fallback
}

// equipmentTariffKey maps an equipment type to its hourly-rate key.
func equipmentTariffKey(e models.EquipmentType) string {
	return "equipment_hour_" + string(e)
}

// PriceInput is everything the formula needs — the subset of an order that
// affects money.
type PriceInput struct {
	Cargo     *models.CargoDetails
	Equipment *models.EquipmentRequest
	Labor     *models.LaborRequest
}

// Breakdown is the per-leg cost, before the order minimum is applied.
type Breakdown struct {
	Transport int64 `json:"transport"`
	Equipment int64 `json:"equipment"`
	Labor     int64 `json:"labor"`
}

func (b Breakdown) Sum() int64 { return b.Transport + b.Equipment + b.Labor }

func (b Breakdown) For(leg models.Leg) int64 {
	switch leg {
	case models.LegTransport:
		return b.Transport
	case models.LegEquipment:
		return b.Equipment
	case models.LegLabor:
		return b.Labor
	}
	return 0
}

// Estimate applies the formula. Components are zero for legs the order lacks.
func Estimate(t Tariffs, in PriceInput) Breakdown {
	var b Breakdown

	if in.Cargo != nil {
		perTon := t.get(TariffTransportPerTon, 80_000)
		b.Transport = int64(math.Round(in.Cargo.WeightTons * float64(perTon)))
	}
	if in.Equipment != nil {
		rate := t.get(equipmentTariffKey(in.Equipment.EquipmentType), 250_000)
		b.Equipment = int64(in.Equipment.DurationHours) * rate
	}
	if in.Labor != nil {
		rate := t.get(TariffLaborHourPerWorker, 45_000)
		b.Labor = int64(in.Labor.WorkersCount) * int64(in.Labor.DurationHours) * rate
	}
	return b
}

// Total is the order price: the sum of components, floored at the minimum.
func Total(t Tariffs, b Breakdown) int64 {
	minimum := t.get(TariffMinimumOrder, 100_000)
	if sum := b.Sum(); sum > minimum {
		return sum
	}
	return minimum
}

// SplitTotal distributes an order total across its legs in proportion to the
// computed breakdown, guaranteeing the parts sum EXACTLY to the total — the
// remainder from integer division lands on the largest leg rather than being
// dropped. Money that does not add up is the one bug nobody forgives.
//
// The total is passed in rather than recomputed so the split honours the price
// the client was actually shown, even if tariffs have moved since.
func SplitTotal(b Breakdown, total int64, legs []models.Leg) map[models.Leg]int64 {
	out := make(map[models.Leg]int64, len(legs))
	if len(legs) == 0 {
		return out
	}

	// Weights of the legs this order actually has.
	weights := make(map[models.Leg]int64, len(legs))
	var weightSum int64
	for _, leg := range legs {
		w := b.For(leg)
		weights[leg] = w
		weightSum += w
	}

	// Degenerate input (a zero-weight order, or a floor-priced one): split
	// evenly rather than paying somebody nothing.
	if weightSum <= 0 {
		share := total / int64(len(legs))
		for _, leg := range legs {
			out[leg] = share
		}
		out[largest(weights, legs)] += total - share*int64(len(legs))
		return out
	}

	var assigned int64
	for _, leg := range legs {
		part := total * weights[leg] / weightSum
		out[leg] = part
		assigned += part
	}
	out[largest(weights, legs)] += total - assigned
	return out
}

// largest returns the leg carrying the most weight; ties break on leg order so
// the result is deterministic.
func largest(weights map[models.Leg]int64, legs []models.Leg) models.Leg {
	best := legs[0]
	for _, leg := range legs[1:] {
		if weights[leg] > weights[best] {
			best = leg
		}
	}
	return best
}
