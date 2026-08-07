package orders

import "github.com/aventiseld/yukbor-backend/pkg/models"

// LegsFor derives which legs an order has from its type and requests (§3.1).
func LegsFor(t models.OrderType, hasEquipment, hasLabor bool) []models.Leg {
	switch t {
	case models.OrderTransportOnly:
		return []models.Leg{models.LegTransport}
	case models.OrderEquipmentOnly:
		return []models.Leg{models.LegEquipment}
	case models.OrderLaborOnly:
		return []models.Leg{models.LegLabor}
	case models.OrderTransportWithOptions:
		legs := []models.Leg{models.LegTransport}
		if hasEquipment {
			legs = append(legs, models.LegEquipment)
		}
		if hasLabor {
			legs = append(legs, models.LegLabor)
		}
		return legs
	}
	return nil
}

// nextAllowed defines forward-only status transitions per leg. Executors may
// only move along these edges; cancelled/disputed are handled separately.
var nextAllowed = map[models.OrderStatus][]models.OrderStatus{
	models.StatusPublished:         {models.StatusMatched, models.StatusAccepted},
	models.StatusMatched:           {models.StatusAccepted},
	models.StatusAccepted:          {models.StatusInProgress, models.StatusLoadingInProgress},
	models.StatusInProgress:        {models.StatusLoadingInProgress, models.StatusInTransit, models.StatusDelivered},
	models.StatusLoadingInProgress: {models.StatusInTransit, models.StatusDelivered},
	models.StatusInTransit:         {models.StatusDelivered},
	models.StatusDelivered:         {models.StatusCompleted},
}

// CanTransition reports whether an executor may move a leg from → to.
func CanTransition(from, to models.OrderStatus) bool {
	for _, s := range nextAllowed[from] {
		if s == to {
			return true
		}
	}
	return false
}

// IsReadyForClientConfirmation: every leg delivered or completed (§3.2).
func IsReadyForClientConfirmation(legStatuses []models.OrderStatus) bool {
	if len(legStatuses) == 0 {
		return false
	}
	for _, s := range legStatuses {
		if s != models.StatusDelivered && s != models.StatusCompleted {
			return false
		}
	}
	return true
}
