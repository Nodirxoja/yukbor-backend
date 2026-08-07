package orders

import (
	"testing"

	"github.com/aventiseld/yukbor-backend/pkg/models"
)

func TestLegsFor(t *testing.T) {
	cases := []struct {
		name         string
		typ          models.OrderType
		hasEq, hasLb bool
		want         int
	}{
		{"transportOnly", models.OrderTransportOnly, false, false, 1},
		{"equipmentOnly", models.OrderEquipmentOnly, false, false, 1},
		{"laborOnly", models.OrderLaborOnly, false, false, 1},
		{"combo transport only", models.OrderTransportWithOptions, false, false, 1},
		{"combo +equipment", models.OrderTransportWithOptions, true, false, 2},
		{"combo +labor", models.OrderTransportWithOptions, false, true, 2},
		{"combo full", models.OrderTransportWithOptions, true, true, 3},
	}
	for _, c := range cases {
		if got := len(LegsFor(c.typ, c.hasEq, c.hasLb)); got != c.want {
			t.Errorf("%s: got %d legs, want %d", c.name, got, c.want)
		}
	}
}

func TestCanTransition(t *testing.T) {
	if !CanTransition(models.StatusAccepted, models.StatusLoadingInProgress) {
		t.Error("accepted → loadingInProgress must be allowed")
	}
	if CanTransition(models.StatusDelivered, models.StatusInTransit) {
		t.Error("backward transition must be rejected")
	}
	if CanTransition(models.StatusCompleted, models.StatusPublished) {
		t.Error("completed is terminal")
	}
}

func TestIsReadyForClientConfirmation(t *testing.T) {
	if !IsReadyForClientConfirmation([]models.OrderStatus{models.StatusDelivered, models.StatusCompleted}) {
		t.Error("all legs delivered/completed ⇒ ready")
	}
	if IsReadyForClientConfirmation([]models.OrderStatus{models.StatusDelivered, models.StatusInTransit}) {
		t.Error("a leg still in transit ⇒ not ready")
	}
	if IsReadyForClientConfirmation(nil) {
		t.Error("no legs ⇒ not ready")
	}
}
