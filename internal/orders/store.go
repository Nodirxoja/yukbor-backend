package orders

import (
	"context"
	"errors"
	"math"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Store is the orders schema. Legs live in their own table (plan §4): accept,
// status and escrow all operate per leg, and an atomic claim is then a single
// conditional UPDATE instead of a transaction dance.
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

var (
	ErrLegAlreadyTaken = errors.New("leg already taken")
	ErrNotPublished    = errors.New("leg not published")
	ErrLegNotFound     = errors.New("leg not found")
)

// LegRecord is one row of orders.order_legs.
type LegRecord struct {
	Leg          models.Leg
	Status       models.OrderStatus
	ExecutorID   *string
	ExecutorName *string
	Price        int64
	UpdatedAt    time.Time
}

// OrderRecord is an order plus its legs. models.Order is the flattened wire
// projection the contract defines; Flatten() produces it.
type OrderRecord struct {
	ID            string
	ClientID      string
	ClientName    string
	Type          models.OrderType
	Cargo         *models.CargoDetails
	Equipment     *models.EquipmentRequest
	Labor         *models.LaborRequest
	Pickup        models.Location
	PickupAddr    string
	Dropoff       models.Location
	DropoffAddr   string
	ScheduledDate time.Time
	PriceEstimate int64
	Currency      string
	PaymentMethod models.PaymentMethod
	CancelledAt   *time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time

	Legs []LegRecord
}

// Leg returns a leg by name.
func (o *OrderRecord) Leg(leg models.Leg) *LegRecord {
	for i := range o.Legs {
		if o.Legs[i].Leg == leg {
			return &o.Legs[i]
		}
	}
	return nil
}

// LegNames lists the legs this order has, in canonical order.
func (o *OrderRecord) LegNames() []models.Leg {
	out := make([]models.Leg, 0, len(o.Legs))
	for _, l := range o.Legs {
		out = append(out, l.Leg)
	}
	return out
}

// primaryLeg is the leg whose status the contract's top-level `status` mirrors:
// transport when there is one, otherwise the order's only leg.
func (o *OrderRecord) primaryLeg() *LegRecord {
	if l := o.Leg(models.LegTransport); l != nil {
		return l
	}
	if len(o.Legs) > 0 {
		return &o.Legs[0]
	}
	return nil
}

// Flatten produces the contract's Order shape: legs collapse back into
// assigned{Driver,EquipmentProvider,LaborProvider}Id/Name and the per-leg
// status fields. A leg the order does not have stays null.
func (o *OrderRecord) Flatten() models.Order {
	out := models.Order{
		ID:               o.ID,
		ClientID:         o.ClientID,
		ClientName:       o.ClientName,
		Type:             o.Type,
		Cargo:            o.Cargo,
		EquipmentRequest: o.Equipment,
		LaborRequest:     o.Labor,
		PickupAddress:    o.PickupAddr,
		PickupLocation:   o.Pickup,
		DropoffAddress:   o.DropoffAddr,
		DropoffLocation:  o.Dropoff,
		ScheduledDate:    o.ScheduledDate,
		PriceEstimate:    strconv.FormatInt(o.PriceEstimate, 10),
		Currency:         o.Currency,
		CreatedAt:        o.CreatedAt,
		UpdatedAt:        o.UpdatedAt,
	}

	if l := o.primaryLeg(); l != nil {
		out.Status = l.Status
	}
	if l := o.Leg(models.LegTransport); l != nil {
		out.AssignedDriverID, out.AssignedDriverName = l.ExecutorID, l.ExecutorName
	}
	if l := o.Leg(models.LegEquipment); l != nil {
		status := l.Status
		out.EquipmentStatus = &status
		out.AssignedEquipmentProviderID, out.AssignedEquipmentProviderName = l.ExecutorID, l.ExecutorName
	}
	if l := o.Leg(models.LegLabor); l != nil {
		status := l.Status
		out.LaborStatus = &status
		out.AssignedLaborProviderID, out.AssignedLaborProviderName = l.ExecutorID, l.ExecutorName
	}
	return out
}

// ---- tariffs ----------------------------------------------------------

func (s *Store) Tariffs(ctx context.Context) (Tariffs, error) {
	rows, err := s.pool.Query(ctx, `SELECT key, value::bigint FROM orders.tariffs`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	t := Tariffs{}
	for rows.Next() {
		var k string
		var v int64
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		t[k] = v
	}
	return t, rows.Err()
}

// ---- reads ------------------------------------------------------------

const orderColumns = `
	id, client_id, client_name, type,
	cargo_type, weight_tons::float8, requires_refrigeration, required_vehicle_type, special_instructions,
	equipment_type, equipment_duration_hours, equipment_notes,
	labor_workers_count, labor_duration_hours, labor_task_description,
	pickup_address, pickup_lat, pickup_lng,
	dropoff_address, dropoff_lat, dropoff_lng,
	scheduled_date, price_estimate::bigint, currency, payment_method, cancelled_at,
	created_at, updated_at`

func scanOrder(row pgx.Row) (*OrderRecord, error) {
	var (
		o           OrderRecord
		cargoType   *string
		weightTons  *float64
		refrig      *bool
		vehicleType *string
		specialIns  *string
		equipType   *string
		equipHours  *int
		equipNotes  *string
		workers     *int
		laborHours  *int
		laborTask   *string
	)
	err := row.Scan(
		&o.ID, &o.ClientID, &o.ClientName, &o.Type,
		&cargoType, &weightTons, &refrig, &vehicleType, &specialIns,
		&equipType, &equipHours, &equipNotes,
		&workers, &laborHours, &laborTask,
		&o.PickupAddr, &o.Pickup.Latitude, &o.Pickup.Longitude,
		&o.DropoffAddr, &o.Dropoff.Latitude, &o.Dropoff.Longitude,
		&o.ScheduledDate, &o.PriceEstimate, &o.Currency, &o.PaymentMethod, &o.CancelledAt,
		&o.CreatedAt, &o.UpdatedAt,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, db.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	if cargoType != nil {
		o.Cargo = &models.CargoDetails{
			CargoType:           *cargoType,
			SpecialInstructions: specialIns,
		}
		if weightTons != nil {
			o.Cargo.WeightTons = *weightTons
		}
		if refrig != nil {
			o.Cargo.RequiresRefrigeration = *refrig
		}
		if vehicleType != nil {
			o.Cargo.RequiredVehicleType = models.VehicleType(*vehicleType)
		}
	}
	if equipType != nil {
		o.Equipment = &models.EquipmentRequest{
			EquipmentType: models.EquipmentType(*equipType),
			Notes:         equipNotes,
		}
		if equipHours != nil {
			o.Equipment.DurationHours = *equipHours
		}
	}
	if workers != nil {
		o.Labor = &models.LaborRequest{TaskDescription: laborTask}
		o.Labor.WorkersCount = *workers
		if laborHours != nil {
			o.Labor.DurationHours = *laborHours
		}
	}
	return &o, nil
}

// attachLegs loads the legs for a set of orders in one round trip.
func (s *Store) attachLegs(ctx context.Context, orders []*OrderRecord) error {
	if len(orders) == 0 {
		return nil
	}
	ids := make([]string, 0, len(orders))
	byID := make(map[string]*OrderRecord, len(orders))
	for _, o := range orders {
		ids = append(ids, o.ID)
		byID[o.ID] = o
	}

	rows, err := s.pool.Query(ctx, `
		SELECT order_id, leg, status, executor_id, executor_name, price::bigint, updated_at
		FROM orders.order_legs
		WHERE order_id = ANY($1)
		ORDER BY CASE leg WHEN 'transport' THEN 0 WHEN 'equipment' THEN 1 ELSE 2 END`, ids)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var orderID string
		var l LegRecord
		if err := rows.Scan(&orderID, &l.Leg, &l.Status, &l.ExecutorID,
			&l.ExecutorName, &l.Price, &l.UpdatedAt); err != nil {
			return err
		}
		if o := byID[orderID]; o != nil {
			o.Legs = append(o.Legs, l)
		}
	}
	return rows.Err()
}

func (s *Store) ByID(ctx context.Context, id string) (*OrderRecord, error) {
	o, err := scanOrder(s.pool.QueryRow(ctx,
		`SELECT `+orderColumns+` FROM orders.orders WHERE id = $1`, id))
	if err != nil {
		return nil, err
	}
	return o, s.attachLegs(ctx, []*OrderRecord{o})
}

func (s *Store) query(ctx context.Context, sql string, args ...any) ([]*OrderRecord, error) {
	rows, err := s.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := []*OrderRecord{}
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, s.attachLegs(ctx, out)
}

func (s *Store) ListByClient(ctx context.Context, clientID string) ([]*OrderRecord, error) {
	return s.query(ctx, `SELECT `+orderColumns+` FROM orders.orders
		WHERE client_id = $1 ORDER BY created_at DESC`, clientID)
}

// Available is the executor feed: orders whose LEG is still published and
// unclaimed (contract §3.2).
func (s *Store) Available(ctx context.Context, leg models.Leg) ([]*OrderRecord, error) {
	return s.query(ctx, `SELECT `+orderColumns+` FROM orders.orders o
		WHERE EXISTS (
			SELECT 1 FROM orders.order_legs l
			WHERE l.order_id = o.id AND l.leg = $1
			  AND l.status = 'published' AND l.executor_id IS NULL)
		ORDER BY o.scheduled_date ASC`, leg)
}

func (s *Store) ListAll(ctx context.Context) ([]*OrderRecord, error) {
	return s.query(ctx, `SELECT `+orderColumns+` FROM orders.orders ORDER BY created_at DESC`)
}

// Backhaul is the differentiator feature (contract §3.2): open transport legs
// with a pickup near where a driver is about to unload, so they do not drive
// home empty.
//
// Distance is haversine in SQL, gated by a bounding box first so the trig only
// runs on plausible rows. At Uzbekistan's order volume this is far cheaper than
// bringing in PostGIS, and it behaves identically.
func (s *Store) Backhaul(ctx context.Context, lat, lng float64, excludeOrderID string, radiusKM float64) ([]*OrderRecord, error) {
	// Degrees of latitude are ~111.32 km everywhere; degrees of longitude
	// shrink with the cosine of the latitude.
	latDelta := radiusKM / 111.32
	lngDelta := radiusKM / (111.32 * math.Max(0.01, math.Cos(lat*math.Pi/180)))

	return s.query(ctx, `
		WITH candidates AS (
			SELECT o.*, 6371 * acos(least(1, greatest(-1,
				cos(radians($1)) * cos(radians(o.pickup_lat)) *
				cos(radians(o.pickup_lng) - radians($2)) +
				sin(radians($1)) * sin(radians(o.pickup_lat))
			))) AS distance_km
			FROM orders.orders o
			WHERE EXISTS (
				SELECT 1 FROM orders.order_legs l
				WHERE l.order_id = o.id AND l.leg = 'transport'
				  AND l.status = 'published' AND l.executor_id IS NULL)
			  AND ($3 = '' OR o.id::text <> $3)
			  AND o.pickup_lat BETWEEN $1 - $4 AND $1 + $4
			  AND o.pickup_lng BETWEEN $2 - $5 AND $2 + $5
		)
		SELECT `+orderColumns+` FROM candidates
		WHERE distance_km <= $6
		ORDER BY distance_km ASC`,
		lat, lng, excludeOrderID, latDelta, lngDelta, radiusKM)
}

// Stats backs the dashboard tiles: an order is active while any leg is still
// open, and completed only when every leg is.
func (s *Store) Stats(ctx context.Context) (models.OrderStats, error) {
	var st models.OrderStats
	err := s.pool.QueryRow(ctx, `
		SELECT
			count(*),
			count(*) FILTER (WHERE EXISTS (
				SELECT 1 FROM orders.order_legs l
				WHERE l.order_id = o.id AND l.status NOT IN ('completed','cancelled'))),
			count(*) FILTER (WHERE NOT EXISTS (
				SELECT 1 FROM orders.order_legs l
				WHERE l.order_id = o.id AND l.status <> 'completed'))
		FROM orders.orders o`).Scan(&st.Total, &st.Active, &st.Completed)
	return st, err
}

// ---- writes -----------------------------------------------------------

// NewOrder is a validated create request plus the server-computed leg prices.
type NewOrder struct {
	ClientID      string
	ClientName    string
	Type          models.OrderType
	Cargo         *models.CargoDetails
	Equipment     *models.EquipmentRequest
	Labor         *models.LaborRequest
	PickupAddr    string
	Pickup        models.Location
	DropoffAddr   string
	Dropoff       models.Location
	ScheduledDate time.Time
	PriceEstimate int64
	Currency      string
	PaymentMethod models.PaymentMethod
	LegPrices     map[models.Leg]int64
}

// Create inserts the order and its legs in one transaction — an order without
// legs is unacceptable, since every read path assumes at least one.
func (s *Store) Create(ctx context.Context, n NewOrder) (*OrderRecord, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	var (
		cargoType, vehicleType, specialIns *string
		weightTons                         *float64
		refrig                             *bool
		equipType, equipNotes              *string
		equipHours                         *int
		workers, laborHours                *int
		laborTask                          *string
	)
	if n.Cargo != nil {
		cargoType = &n.Cargo.CargoType
		weightTons = &n.Cargo.WeightTons
		refrig = &n.Cargo.RequiresRefrigeration
		vt := string(n.Cargo.RequiredVehicleType)
		vehicleType = &vt
		specialIns = n.Cargo.SpecialInstructions
	}
	if n.Equipment != nil {
		et := string(n.Equipment.EquipmentType)
		equipType, equipHours, equipNotes = &et, &n.Equipment.DurationHours, n.Equipment.Notes
	}
	if n.Labor != nil {
		workers, laborHours, laborTask = &n.Labor.WorkersCount, &n.Labor.DurationHours, n.Labor.TaskDescription
	}

	var id string
	err = tx.QueryRow(ctx, `
		INSERT INTO orders.orders (
			client_id, client_name, type,
			cargo_type, weight_tons, requires_refrigeration, required_vehicle_type, special_instructions,
			equipment_type, equipment_duration_hours, equipment_notes,
			labor_workers_count, labor_duration_hours, labor_task_description,
			pickup_address, pickup_lat, pickup_lng,
			dropoff_address, dropoff_lat, dropoff_lng,
			scheduled_date, price_estimate, currency, payment_method)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23,$24)
		RETURNING id`,
		n.ClientID, n.ClientName, n.Type,
		cargoType, weightTons, refrig, vehicleType, specialIns,
		equipType, equipHours, equipNotes,
		workers, laborHours, laborTask,
		n.PickupAddr, n.Pickup.Latitude, n.Pickup.Longitude,
		n.DropoffAddr, n.Dropoff.Latitude, n.Dropoff.Longitude,
		n.ScheduledDate, n.PriceEstimate, n.Currency, n.PaymentMethod,
	).Scan(&id)
	if err != nil {
		return nil, err
	}

	// All legs start published (plan §6).
	for leg, price := range n.LegPrices {
		if _, err := tx.Exec(ctx, `
			INSERT INTO orders.order_legs (order_id, leg, status, price)
			VALUES ($1, $2, 'published', $3)`, id, leg, price); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}
	return s.ByID(ctx, id)
}

// ClaimLeg is the atomic accept (plan §12): a single conditional UPDATE, so two
// executors racing for the same leg cannot both win — the loser sees zero rows
// affected. No transaction, no lock, no race.
func (s *Store) ClaimLeg(ctx context.Context, orderID string, leg models.Leg, executorID, executorName string) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE orders.order_legs
		SET executor_id = $3, executor_name = $4, status = 'accepted', updated_at = now()
		WHERE order_id = $1 AND leg = $2 AND executor_id IS NULL AND status = 'published'`,
		orderID, leg, executorID, executorName)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 1 {
		_, _ = s.pool.Exec(ctx, `UPDATE orders.orders SET updated_at = now() WHERE id = $1`, orderID)
		return nil
	}

	// Zero rows: say precisely why, so the client gets the right contract code.
	var status models.OrderStatus
	var executor *string
	err = s.pool.QueryRow(ctx, `
		SELECT status, executor_id FROM orders.order_legs
		WHERE order_id = $1 AND leg = $2`, orderID, leg).Scan(&status, &executor)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrLegNotFound
	}
	if err != nil {
		return err
	}
	if executor != nil {
		return ErrLegAlreadyTaken
	}
	return ErrNotPublished
}

// ReleaseLegClaim undoes a claim. Used when opening escrow fails: a leg must
// never sit claimed with no money behind it.
func (s *Store) ReleaseLegClaim(ctx context.Context, orderID string, leg models.Leg) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE orders.order_legs
		SET executor_id = NULL, executor_name = NULL, status = 'published', updated_at = now()
		WHERE order_id = $1 AND leg = $2`, orderID, leg)
	return err
}

// UpdateLegStatus moves one leg forward. The caller has already checked the
// transition is legal and that the requester owns the leg.
func (s *Store) UpdateLegStatus(ctx context.Context, orderID string, leg models.Leg, status models.OrderStatus) error {
	tag, err := s.pool.Exec(ctx, `
		UPDATE orders.order_legs SET status = $3, updated_at = now()
		WHERE order_id = $1 AND leg = $2`, orderID, leg, status)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrLegNotFound
	}
	_, err = s.pool.Exec(ctx, `UPDATE orders.orders SET updated_at = now() WHERE id = $1`, orderID)
	return err
}

// CompleteAllLegs marks every leg completed on client confirmation.
func (s *Store) CompleteAllLegs(ctx context.Context, orderID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE orders.order_legs SET status = 'completed', updated_at = now()
		WHERE order_id = $1`, orderID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `UPDATE orders.orders SET updated_at = now() WHERE id = $1`, orderID)
	return err
}

// Cancel marks every leg cancelled.
func (s *Store) Cancel(ctx context.Context, orderID string) error {
	_, err := s.pool.Exec(ctx, `
		UPDATE orders.order_legs SET status = 'cancelled', updated_at = now()
		WHERE order_id = $1`, orderID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		UPDATE orders.orders SET cancelled_at = now(), updated_at = now() WHERE id = $1`, orderID)
	return err
}
