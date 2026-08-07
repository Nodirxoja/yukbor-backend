// Package models is the single source of truth for the wire types shared by
// all YUK BOR services. Every field MUST stay 1:1 with the iOS Swift models
// (Core/Model/*.swift) and docs/API_CONTRACT.md. Change the contract first,
// then this file, then the services.
package models

import "time"

// ---- Enums ------------------------------------------------------------

type UserRole string

const (
	RoleClient            UserRole = "client"
	RoleDriver            UserRole = "driver"
	RoleEquipmentProvider UserRole = "equipmentProvider"
	RoleLaborProvider     UserRole = "laborProvider"
	RoleFleetAdmin        UserRole = "fleetAdmin"
	RoleAdmin             UserRole = "admin"
)

type VerificationStatus string

const (
	VerificationPending  VerificationStatus = "pending"
	VerificationApproved VerificationStatus = "approved"
	VerificationRejected VerificationStatus = "rejected"
)

type OrderType string

const (
	OrderTransportOnly        OrderType = "transportOnly"
	OrderTransportWithOptions OrderType = "transportWithOptions"
	OrderEquipmentOnly        OrderType = "equipmentOnly"
	OrderLaborOnly            OrderType = "laborOnly"
)

// OrderStatus is shared by all three legs (transport/equipment/labor).
type OrderStatus string

const (
	StatusDraft             OrderStatus = "draft"
	StatusPublished         OrderStatus = "published"
	StatusMatched           OrderStatus = "matched"
	StatusAccepted          OrderStatus = "accepted"
	StatusInProgress        OrderStatus = "inProgress"
	StatusLoadingInProgress OrderStatus = "loadingInProgress"
	StatusInTransit         OrderStatus = "inTransit"
	StatusDelivered         OrderStatus = "delivered"
	StatusCompleted         OrderStatus = "completed"
	StatusCancelled         OrderStatus = "cancelled"
	StatusDisputed          OrderStatus = "disputed"
)

type Leg string

const (
	LegTransport Leg = "transport"
	LegEquipment Leg = "equipment"
	LegLabor     Leg = "labor"
)

type EquipmentType string

const (
	EquipmentExcavator EquipmentType = "excavator"
	EquipmentCrane     EquipmentType = "crane"
	EquipmentForklift  EquipmentType = "forklift"
	EquipmentLoader    EquipmentType = "loader"
)

type VehicleType string

const (
	VehicleTractorTrailer VehicleType = "tractorTrailer"
	VehicleFlatbed        VehicleType = "flatbed"
	VehicleRefrigerated   VehicleType = "refrigerated"
	VehicleTanker         VehicleType = "tanker"
	VehicleDumpTruck      VehicleType = "dumpTruck"
	VehicleBoxTruck       VehicleType = "boxTruck"
)

type PaymentMethod string

const (
	PaymentPayme  PaymentMethod = "payme"
	PaymentClick  PaymentMethod = "click"
	PaymentUzcard PaymentMethod = "uzcard"
)

type TransactionStatus string

const (
	TxHeld     TransactionStatus = "held"
	TxReleased TransactionStatus = "released"
	TxRefunded TransactionStatus = "refunded"
)

type NotificationType string

const (
	NotifNewOrderMatch      NotificationType = "newOrderMatch"
	NotifOrderStatusChanged NotificationType = "orderStatusChanged"
	NotifPaymentReleased    NotificationType = "paymentReleased"
	NotifBackhaulSuggestion NotificationType = "backhaulSuggestion"
	NotifReviewReceived     NotificationType = "reviewReceived"
)

// ---- DTOs -------------------------------------------------------------

type User struct {
	ID                 string             `json:"id"`
	Role               UserRole           `json:"role"`
	FullName           string             `json:"fullName"`
	PhoneNumber        string             `json:"phoneNumber"`
	Email              *string            `json:"email"`
	IsVerified         bool               `json:"isVerified"`
	VerificationStatus VerificationStatus `json:"verificationStatus"`
	Rating             float64            `json:"rating"`
	RatingsCount       int                `json:"ratingsCount"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

type CargoDetails struct {
	CargoType             string      `json:"cargoType"`
	WeightTons            float64     `json:"weightTons"`
	RequiresRefrigeration bool        `json:"requiresRefrigeration"`
	RequiredVehicleType   VehicleType `json:"requiredVehicleType"`
	SpecialInstructions   *string     `json:"specialInstructions,omitempty"`
}

type EquipmentRequest struct {
	EquipmentType EquipmentType `json:"equipmentType"`
	DurationHours int           `json:"durationHours"`
	Notes         *string       `json:"notes,omitempty"`
}

type LaborRequest struct {
	WorkersCount    int     `json:"workersCount"`
	DurationHours   int     `json:"durationHours"`
	TaskDescription *string `json:"taskDescription,omitempty"`
}

// Order is the flattened wire representation expected by iOS. Internally the
// orders service stores legs as rows and flattens them into these fields.
type Order struct {
	ID         string    `json:"id"`
	ClientID   string    `json:"clientId"`
	ClientName string    `json:"clientName"`
	Type       OrderType `json:"type"`
	// Status is the transport leg's status for transport orders, or the
	// single leg's status for equipmentOnly/laborOnly.
	Status OrderStatus `json:"status"`

	Cargo            *CargoDetails     `json:"cargo,omitempty"`
	EquipmentRequest *EquipmentRequest `json:"equipmentRequest,omitempty"`
	LaborRequest     *LaborRequest     `json:"laborRequest,omitempty"`

	PickupAddress   string   `json:"pickupAddress"`
	PickupLocation  Location `json:"pickupLocation"`
	DropoffAddress  string   `json:"dropoffAddress"`
	DropoffLocation Location `json:"dropoffLocation"`

	ScheduledDate time.Time `json:"scheduledDate"`
	PriceEstimate string    `json:"priceEstimate"` // decimal string, UZS
	Currency      string    `json:"currency"`

	AssignedDriverID   *string `json:"assignedDriverId"`
	AssignedDriverName *string `json:"assignedDriverName"`

	AssignedEquipmentProviderID   *string      `json:"assignedEquipmentProviderId"`
	AssignedEquipmentProviderName *string      `json:"assignedEquipmentProviderName"`
	EquipmentStatus               *OrderStatus `json:"equipmentStatus"`

	AssignedLaborProviderID   *string      `json:"assignedLaborProviderId"`
	AssignedLaborProviderName *string      `json:"assignedLaborProviderName"`
	LaborStatus               *OrderStatus `json:"laborStatus"`

	CreatedAt time.Time `json:"createdAt"`
	UpdatedAt time.Time `json:"updatedAt"`
}

type Transaction struct {
	ID                 string            `json:"id"`
	OrderID            string            `json:"orderId"`
	OrderTitle         string            `json:"orderTitle"`
	PayerID            string            `json:"payerId"`
	PayeeID            string            `json:"payeeId"`
	Amount             string            `json:"amount"`             // decimal string, UZS
	PlatformCommission string            `json:"platformCommission"` // decimal string, UZS
	PaymentMethod      PaymentMethod     `json:"paymentMethod"`
	Status             TransactionStatus `json:"status"`
	CreatedAt          time.Time         `json:"createdAt"`
	ReleasedAt         *time.Time        `json:"releasedAt"`
}

type AppNotification struct {
	ID             string           `json:"id"`
	UserID         string           `json:"userId"`
	Type           NotificationType `json:"type"`
	Title          string           `json:"title"`
	Body           string           `json:"body"`
	RelatedOrderID *string          `json:"relatedOrderId"`
	IsRead         bool             `json:"isRead"`
	CreatedAt      time.Time        `json:"createdAt"`
}

type Review struct {
	ID         string    `json:"id"`
	OrderID    string    `json:"orderId"`
	ReviewerID string    `json:"reviewerId"`
	RevieweeID string    `json:"revieweeId"`
	Rating     int       `json:"rating"`
	Comment    *string   `json:"comment,omitempty"`
	CreatedAt  time.Time `json:"createdAt"`
}

// WSEvent is the realtime envelope: same JSON objects as REST plus "event".
type WSEvent struct {
	Event string `json:"event"` // order.updated | order.created | transaction.updated | notification.created
	Data  any    `json:"data"`
}
