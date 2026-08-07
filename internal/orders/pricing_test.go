package orders

import (
	"testing"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

func testTariffs() Tariffs {
	return Tariffs{
		TariffTransportPerTon:      80_000,
		"equipment_hour_crane":     450_000,
		"equipment_hour_excavator": 400_000,
		TariffLaborHourPerWorker:   45_000,
		TariffMinimumOrder:         100_000,
	}
}

func TestEstimateMatchesFormula(t *testing.T) {
	tar := testTariffs()
	in := PriceInput{
		Cargo:     &models.CargoDetails{WeightTons: 12.5},
		Equipment: &models.EquipmentRequest{EquipmentType: models.EquipmentCrane, DurationHours: 4},
		Labor:     &models.LaborRequest{WorkersCount: 3, DurationHours: 2},
	}
	b := Estimate(tar, in)

	// weight × tariff/t + hours × rate + workers × hours × rate
	wantTransport := int64(12.5 * 80_000) // 1_000_000
	wantEquipment := int64(4 * 450_000)   // 1_800_000
	wantLabor := int64(3 * 2 * 45_000)    // 270_000
	if b.Transport != wantTransport || b.Equipment != wantEquipment || b.Labor != wantLabor {
		t.Fatalf("breakdown %+v, want transport=%d equipment=%d labor=%d",
			b, wantTransport, wantEquipment, wantLabor)
	}
	if got, want := b.Sum(), wantTransport+wantEquipment+wantLabor; got != want {
		t.Errorf("sum %d, want %d", got, want)
	}
}

func TestTotalAppliesMinimum(t *testing.T) {
	tar := testTariffs()
	tiny := Estimate(tar, PriceInput{Cargo: &models.CargoDetails{WeightTons: 0.1}}) // 8_000
	if got := Total(tar, tiny); got != 100_000 {
		t.Errorf("below-minimum order priced at %d, want the 100000 floor", got)
	}
	big := Estimate(tar, PriceInput{Cargo: &models.CargoDetails{WeightTons: 10}}) // 800_000
	if got := Total(tar, big); got != 800_000 {
		t.Errorf("above-minimum order priced at %d, want 800000", got)
	}
}

// The split is where money can silently vanish: the parts must always add up
// to the total EXACTLY, whatever the rounding.
func TestSplitTotalIsExact(t *testing.T) {
	tar := testTariffs()
	cases := []struct {
		name  string
		in    PriceInput
		typ   models.OrderType
		total int64
	}{
		{"transport only", PriceInput{Cargo: &models.CargoDetails{WeightTons: 12.5}},
			models.OrderTransportOnly, 1_000_000},
		{"combo three legs", PriceInput{
			Cargo:     &models.CargoDetails{WeightTons: 12.5},
			Equipment: &models.EquipmentRequest{EquipmentType: models.EquipmentCrane, DurationHours: 4},
			Labor:     &models.LaborRequest{WorkersCount: 3, DurationHours: 2},
		}, models.OrderTransportWithOptions, 3_070_000},
		// A total that does not divide evenly by the weights — the remainder
		// has to land somewhere rather than being lost to integer division.
		{"awkward total", PriceInput{
			Cargo:     &models.CargoDetails{WeightTons: 7},
			Equipment: &models.EquipmentRequest{EquipmentType: models.EquipmentExcavator, DurationHours: 3},
			Labor:     &models.LaborRequest{WorkersCount: 5, DurationHours: 7},
		}, models.OrderTransportWithOptions, 1_000_001},
		{"floor-priced order", PriceInput{
			Labor: &models.LaborRequest{WorkersCount: 1, DurationHours: 1},
		}, models.OrderLaborOnly, 100_000},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			b := Estimate(tar, c.in)
			legs := LegsFor(c.typ, c.in.Equipment != nil, c.in.Labor != nil)
			split := SplitTotal(b, c.total, legs)

			if len(split) != len(legs) {
				t.Fatalf("split has %d entries for %d legs", len(split), len(legs))
			}
			var sum int64
			for leg, amount := range split {
				if amount < 0 {
					t.Errorf("leg %s got a negative amount %d", leg, amount)
				}
				sum += amount
			}
			if sum != c.total {
				t.Errorf("legs sum to %d, order total is %d — %d UZS unaccounted for",
					sum, c.total, c.total-sum)
			}
		})
	}
}

func TestSplitTotalIsProportional(t *testing.T) {
	b := Breakdown{Transport: 1_000_000, Equipment: 1_800_000, Labor: 200_000}
	legs := []models.Leg{models.LegTransport, models.LegEquipment, models.LegLabor}
	split := SplitTotal(b, 3_000_000, legs)

	// Equipment carries the most weight, so it must be paid the most.
	if split[models.LegEquipment] <= split[models.LegTransport] {
		t.Errorf("equipment %d should exceed transport %d",
			split[models.LegEquipment], split[models.LegTransport])
	}
	if split[models.LegLabor] >= split[models.LegTransport] {
		t.Errorf("labor %d should be below transport %d",
			split[models.LegLabor], split[models.LegTransport])
	}
	// 1.8/3.0 of 3_000_000 = 1_800_000
	if got := split[models.LegEquipment]; got != 1_800_000 {
		t.Errorf("equipment share %d, want 1800000", got)
	}
}

// A zero-weight order must not pay one executor everything and another nothing.
func TestSplitTotalDegenerate(t *testing.T) {
	legs := []models.Leg{models.LegTransport, models.LegEquipment}
	split := SplitTotal(Breakdown{}, 100_001, legs)

	var sum int64
	for _, amount := range split {
		if amount == 0 {
			t.Error("a leg was allocated nothing in an even split")
		}
		sum += amount
	}
	if sum != 100_001 {
		t.Errorf("even split sums to %d, want 100001", sum)
	}
}
