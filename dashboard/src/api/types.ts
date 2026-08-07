// Wire types — mirror pkg/models (Go) and Core/Model/*.swift (iOS) 1:1.
// The contract (docs/API_CONTRACT.md) is the source of truth.

export type UserRole =
  | 'client'
  | 'driver'
  | 'equipmentProvider'
  | 'laborProvider'
  | 'fleetAdmin'
  | 'admin'

export type VerificationStatus = 'pending' | 'approved' | 'rejected'

export type OrderType =
  | 'transportOnly'
  | 'transportWithOptions'
  | 'equipmentOnly'
  | 'laborOnly'

export type OrderStatus =
  | 'draft'
  | 'published'
  | 'matched'
  | 'accepted'
  | 'inProgress'
  | 'loadingInProgress'
  | 'inTransit'
  | 'delivered'
  | 'completed'
  | 'cancelled'
  | 'disputed'

export type TransactionStatus = 'held' | 'released' | 'refunded'

// GET /admin/users returns UserDetail: the contract's User plus the fields the
// back office needs — when they registered, the licence the registry issued,
// and why an applicant was rejected. PINFL and passport data never leave auth.
export interface User {
  id: string
  role: UserRole
  fullName: string
  phoneNumber: string
  email: string | null
  isVerified: boolean
  verificationStatus: VerificationStatus
  rating: number
  ratingsCount: number
  createdAt?: string
  licenseNumber?: string | null
  licenseCategories?: string[] | null
  vehiclePlate?: string | null
  rejectionReason?: string | null
}

export interface Location {
  latitude: number
  longitude: number
}

export interface Order {
  id: string
  clientId: string
  clientName: string
  type: OrderType
  status: OrderStatus
  pickupAddress: string
  pickupLocation: Location
  dropoffAddress: string
  dropoffLocation: Location
  scheduledDate: string
  priceEstimate: string // decimal string, UZS
  currency: string
  assignedDriverId: string | null
  assignedDriverName: string | null
  assignedEquipmentProviderId: string | null
  assignedEquipmentProviderName: string | null
  equipmentStatus: OrderStatus | null
  assignedLaborProviderId: string | null
  assignedLaborProviderName: string | null
  laborStatus: OrderStatus | null
  createdAt: string
  updatedAt: string
}

export interface Transaction {
  id: string
  orderId: string
  orderTitle: string
  payerId: string
  payeeId: string
  amount: string // decimal string, UZS
  platformCommission: string // decimal string, UZS
  paymentMethod: 'payme' | 'click' | 'uzcard'
  status: TransactionStatus
  createdAt: string
  releasedAt: string | null
}

// GET /admin/stats — aggregated in SQL on the wallet service.
export interface AdminStats {
  totalOrders: number
  activeOrders: number
  completedOrders: number
  registeredUsers: number
  creditedToExecutors: string // Σ released (amount − commission), UZS
  serviceFeesCharged: string // Σ platformCommission of released, UZS
  heldInEscrow: string // Σ held amounts, UZS
}
