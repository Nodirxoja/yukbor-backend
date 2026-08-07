// Package orders owns the order lifecycle: creation, per-leg accept/status,
// cancellation, client confirmation (which triggers per-payee escrow release
// in the wallet service), and backhaul search.
package orders

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
	"github.com/aventiseld/yukbor-backend/pkg/models"
	"github.com/aventiseld/yukbor-backend/pkg/svc"
)

type Handler struct {
	cfg    config.Config
	store  *Store
	wallet *svc.Client
	auth   *svc.Client
	notify *svc.Client
}

func NewHandler(cfg config.Config, pool *pgxpool.Pool) *Handler {
	return &Handler{
		cfg:    cfg,
		store:  NewStore(pool),
		wallet: svc.New(cfg.WalletURL, cfg.InternalToken),
		auth:   svc.New(cfg.AuthURL, cfg.InternalToken),
		notify: svc.New(cfg.NotificationsURL, cfg.InternalToken),
	}
}

// Routes wires every endpoint from the API contract (§3).
func (h *Handler) Routes() http.Handler {
	secret := h.cfg.Secret()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("orders"))

	mux.HandleFunc("POST /orders", httpx.Authed(secret, h.create))
	mux.HandleFunc("GET /orders", httpx.Authed(secret, h.listByClient))
	mux.HandleFunc("GET /orders/available", httpx.Authed(secret, h.available))
	mux.HandleFunc("GET /orders/backhaul", httpx.Authed(secret, h.backhaul))
	mux.HandleFunc("POST /orders/estimate", httpx.Authed(secret, h.estimate))
	mux.HandleFunc("GET /orders/{id}", httpx.Authed(secret, h.get))
	mux.HandleFunc("POST /orders/{id}/accept", httpx.Authed(secret, h.accept))
	mux.HandleFunc("PATCH /orders/{id}/status", httpx.Authed(secret, h.updateStatus))
	mux.HandleFunc("POST /orders/{id}/cancel", httpx.Authed(secret, h.cancel))
	mux.HandleFunc("POST /orders/{id}/confirm-completion", httpx.Authed(secret, h.confirm))

	// Dashboard (plan §11)
	mux.HandleFunc("GET /admin/orders",
		httpx.AuthedRole(secret, []string{string(models.RoleAdmin)}, h.adminOrders))
	mux.HandleFunc("GET /internal/orders/stats",
		httpx.InternalOnly(h.cfg.InternalToken, h.internalStats))
	// Reviews checks an order is really completed before accepting a review.
	mux.HandleFunc("GET /internal/orders/{id}",
		httpx.InternalOnly(h.cfg.InternalToken, h.internalOrder))

	return httpx.Wrap(mux)
}

// ---- create & read ----------------------------------------------------

// orderInput is the create body: the contract's Order minus the server-owned
// fields. Decoded loosely, so a client that posts the whole object is fine.
type orderInput struct {
	ClientName       string                   `json:"clientName"`
	Type             models.OrderType         `json:"type"`
	Cargo            *models.CargoDetails     `json:"cargo"`
	EquipmentRequest *models.EquipmentRequest `json:"equipmentRequest"`
	LaborRequest     *models.LaborRequest     `json:"laborRequest"`
	PickupAddress    string                   `json:"pickupAddress"`
	PickupLocation   models.Location          `json:"pickupLocation"`
	DropoffAddress   string                   `json:"dropoffAddress"`
	DropoffLocation  models.Location          `json:"dropoffLocation"`
	ScheduledDate    time.Time                `json:"scheduledDate"`
	PriceEstimate    string                   `json:"priceEstimate"`
	Currency         string                   `json:"currency"`
	// Additive and optional: the contract's Order carries no payment method,
	// but escrow needs one when a leg is accepted. Defaults to payme.
	PaymentMethod models.PaymentMethod `json:"paymentMethod"`
}

func (h *Handler) create(w http.ResponseWriter, r *http.Request) {
	var in orderInput
	if err := httpx.ReadJSONLoose(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	legs := LegsFor(in.Type, in.EquipmentRequest != nil, in.LaborRequest != nil)
	if len(legs) == 0 {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"unknown order type: "+string(in.Type))
		return
	}
	if in.PickupAddress == "" || in.DropoffAddress == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"pickupAddress and dropoffAddress are required")
		return
	}

	tariffs, err := h.store.Tariffs(r.Context())
	if err != nil {
		h.fail(w, "load tariffs", err)
		return
	}
	breakdown := Estimate(tariffs, PriceInput{
		Cargo: in.Cargo, Equipment: in.EquipmentRequest, Labor: in.LaborRequest,
	})

	// Honour the price the client displayed when it sends one, so the order
	// never shows a different total than the user agreed to; fall back to the
	// server formula otherwise. The split always uses the server breakdown.
	total := Total(tariffs, breakdown)
	if client, err := strconv.ParseInt(in.PriceEstimate, 10, 64); err == nil && client > 0 {
		total = client
	}

	method := in.PaymentMethod
	if method == "" {
		method = models.PaymentPayme
	}
	currency := in.Currency
	if currency == "" {
		currency = "UZS"
	}
	scheduled := in.ScheduledDate
	if scheduled.IsZero() {
		scheduled = time.Now().Add(24 * time.Hour)
	}

	rec, err := h.store.Create(r.Context(), NewOrder{
		ClientID:      httpx.Claims(r).Sub,
		ClientName:    in.ClientName,
		Type:          in.Type,
		Cargo:         in.Cargo,
		Equipment:     in.EquipmentRequest,
		Labor:         in.LaborRequest,
		PickupAddr:    in.PickupAddress,
		Pickup:        in.PickupLocation,
		DropoffAddr:   in.DropoffAddress,
		Dropoff:       in.DropoffLocation,
		ScheduledDate: scheduled,
		PriceEstimate: total,
		Currency:      currency,
		PaymentMethod: method,
		LegPrices:     SplitTotal(breakdown, total, legs),
	})
	if err != nil {
		h.fail(w, "create order", err)
		return
	}

	order := rec.Flatten()
	h.emit([]string{rec.ClientID}, models.EventOrderCreated, order, nil)
	httpx.WriteJSON(w, http.StatusCreated, order)
}

func (h *Handler) listByClient(w http.ResponseWriter, r *http.Request) {
	claims := httpx.Claims(r)
	clientID := r.URL.Query().Get("clientId")
	if clientID == "" {
		clientID = claims.Sub
	}
	if clientID != claims.Sub && claims.Role != string(models.RoleAdmin) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"cannot read another client's orders")
		return
	}

	recs, err := h.store.ListByClient(r.Context(), clientID)
	if err != nil {
		h.fail(w, "list orders", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, flattenAll(recs))
}

func (h *Handler) available(w http.ResponseWriter, r *http.Request) {
	leg := models.Leg(r.URL.Query().Get("leg"))
	if !validLeg(leg) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"leg must be transport, equipment or labor")
		return
	}
	recs, err := h.store.Available(r.Context(), leg)
	if err != nil {
		h.fail(w, "list available", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, flattenAll(recs))
}

// BackhaulRadiusKM is how far from the drop-off point we look for a return
// load (contract §3.2).
const BackhaulRadiusKM = 15

func (h *Handler) backhaul(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	lat, errLat := strconv.ParseFloat(q.Get("dropoffLat"), 64)
	lng, errLng := strconv.ParseFloat(q.Get("dropoffLng"), 64)
	if errLat != nil || errLng != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"dropoffLat and dropoffLng are required")
		return
	}

	radius := float64(BackhaulRadiusKM)
	if v, err := strconv.ParseFloat(q.Get("radiusKm"), 64); err == nil && v > 0 {
		radius = v
	}

	recs, err := h.store.Backhaul(r.Context(), lat, lng, q.Get("excludeOrderId"), radius)
	if err != nil {
		h.fail(w, "backhaul search", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, flattenAll(recs))
}

func (h *Handler) get(w http.ResponseWriter, r *http.Request) {
	rec, err := h.load(w, r)
	if rec == nil {
		return
	}
	_ = err
	httpx.WriteJSON(w, http.StatusOK, rec.Flatten())
}

// ---- accept -----------------------------------------------------------

func (h *Handler) accept(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Leg          models.Leg `json:"leg"`
		ExecutorID   string     `json:"executorId"`
		ExecutorName string     `json:"executorName"`
	}
	if err := httpx.ReadJSONLoose(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	claims := httpx.Claims(r)
	if req.ExecutorID == "" {
		req.ExecutorID = claims.Sub
	}
	if req.ExecutorID != claims.Sub && claims.Role != string(models.RoleAdmin) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"cannot accept a leg on behalf of another executor")
		return
	}

	rec, err := h.load(w, r)
	if rec == nil {
		return
	}
	leg := rec.Leg(req.Leg)
	if leg == nil {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeLegNotFound,
			"this order has no "+string(req.Leg)+" leg")
		return
	}

	// Second licence gate (plan §5): registration only proved category C. A
	// tractor-trailer load needs CE, so the driver is re-checked here against
	// the licence the registry actually issued.
	if code, msg := h.licenceGate(r.Context(), rec, req.Leg, req.ExecutorID); code != "" {
		httpx.WriteError(w, http.StatusForbidden, code, msg)
		return
	}

	switch err = h.store.ClaimLeg(r.Context(), rec.ID, req.Leg, req.ExecutorID, req.ExecutorName); {
	case errors.Is(err, ErrLegAlreadyTaken):
		httpx.WriteError(w, http.StatusConflict, httpx.CodeLegAlreadyTaken,
			"этот заказ уже принят другим исполнителем")
		return
	case errors.Is(err, ErrNotPublished):
		httpx.WriteError(w, http.StatusConflict, httpx.CodeOrderNotPublished,
			"заказ больше не опубликован")
		return
	case errors.Is(err, ErrLegNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeLegNotFound, "leg not found")
		return
	case err != nil:
		h.fail(w, "claim leg", err)
		return
	}

	// Open escrow for THIS leg's price. If the money cannot be held, the claim
	// is rolled back — a leg must never be assigned with nothing behind it.
	var tx models.Transaction
	err = h.wallet.Post(r.Context(), "/wallet/transactions", map[string]any{
		"orderId":       rec.ID,
		"orderTitle":    title(rec),
		"payerId":       rec.ClientID,
		"payeeId":       req.ExecutorID,
		"amount":        strconv.FormatInt(leg.Price, 10),
		"paymentMethod": rec.PaymentMethod,
	}, &tx)
	if err != nil {
		if rbErr := h.store.ReleaseLegClaim(r.Context(), rec.ID, req.Leg); rbErr != nil {
			slog.Error("could not roll back leg claim", "order", rec.ID, "leg", req.Leg, "err", rbErr)
		}
		if code := svc.CodeOf(err); code == httpx.CodePaymentDeclined {
			httpx.WriteError(w, http.StatusPaymentRequired, code, "платёж отклонён провайдером")
			return
		}
		h.fail(w, "open escrow", err)
		return
	}

	updated, err := h.store.ByID(r.Context(), rec.ID)
	if err != nil {
		h.fail(w, "reload order", err)
		return
	}
	order := updated.Flatten()

	h.emitTo([]string{rec.ClientID, req.ExecutorID}, []string{rec.ClientID},
		models.EventOrderUpdated, order, &models.NotifySpec{
			Type:           models.NotifNewOrderMatch,
			Title:          "Исполнитель найден",
			Body:           req.ExecutorName + " принял ваш заказ",
			RelatedOrderID: &rec.ID,
		})
	h.emit([]string{rec.ClientID, req.ExecutorID}, models.EventTransactionUpdated, tx, nil)

	httpx.WriteJSON(w, http.StatusOK, order)
}

// licenceGate re-checks a driver's licence for loads that need more than the
// category proved at registration. Returns an error code when the accept must
// be refused; empty means proceed.
func (h *Handler) licenceGate(ctx context.Context, rec *OrderRecord, leg models.Leg, executorID string) (code, message string) {
	if leg != models.LegTransport || rec.Cargo == nil {
		return "", ""
	}
	required := models.RequiredLicenseCategory(rec.Cargo.RequiredVehicleType)
	if required == "" || required == "C" {
		return "", "" // registration already proved C
	}

	var user models.UserDetail
	if err := h.auth.Get(ctx, "/internal/users/"+executorID, &user); err != nil {
		// Do not block a demo on an auth blip; registration already gated C.
		slog.Warn("licence re-check unavailable, allowing accept", "executor", executorID, "err", err)
		return "", ""
	}
	if user.HasCategory(required) {
		return "", ""
	}
	return httpx.CodeLicenseCategoryMismatch,
		"для этого груза нужна категория " + required + ", в вашем удостоверении её нет"
}

// ---- status, cancel, confirm ------------------------------------------

func (h *Handler) updateStatus(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Leg    models.Leg         `json:"leg"`
		Status models.OrderStatus `json:"status"`
	}
	if err := httpx.ReadJSONLoose(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	rec, err := h.load(w, r)
	if rec == nil {
		return
	}
	leg := rec.Leg(req.Leg)
	if leg == nil {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeLegNotFound,
			"this order has no "+string(req.Leg)+" leg")
		return
	}

	// Only the assigned executor drives their own leg (contract §3.2).
	claims := httpx.Claims(r)
	if leg.ExecutorID == nil || (*leg.ExecutorID != claims.Sub && claims.Role != string(models.RoleAdmin)) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"only the assigned executor can update this leg")
		return
	}
	if !CanTransition(leg.Status, req.Status) {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeInvalidStatusTransition,
			"cannot move from "+string(leg.Status)+" to "+string(req.Status))
		return
	}

	if err := h.store.UpdateLegStatus(r.Context(), rec.ID, req.Leg, req.Status); err != nil {
		h.fail(w, "update leg status", err)
		return
	}

	updated, err := h.store.ByID(r.Context(), rec.ID)
	if err != nil {
		h.fail(w, "reload order", err)
		return
	}
	order := updated.Flatten()
	h.emitTo([]string{rec.ClientID, claims.Sub}, []string{rec.ClientID},
		models.EventOrderUpdated, order, &models.NotifySpec{
			Type:           models.NotifOrderStatusChanged,
			Title:          "Статус заказа обновлён",
			Body:           statusText(req.Leg, req.Status),
			RelatedOrderID: &rec.ID,
		})
	httpx.WriteJSON(w, http.StatusOK, order)
}

func (h *Handler) cancel(w http.ResponseWriter, r *http.Request) {
	rec, err := h.load(w, r)
	if rec == nil {
		return
	}
	claims := httpx.Claims(r)
	if rec.ClientID != claims.Sub && claims.Role != string(models.RoleAdmin) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"only the client can cancel this order")
		return
	}

	// Cancellation is allowed while no leg has moved PAST accepted (plan §6):
	// once work has started, cancelling would strand an executor mid-job.
	for _, leg := range rec.Legs {
		if !cancellable(leg.Status) {
			httpx.WriteError(w, http.StatusConflict, httpx.CodeOrderNotCancellable,
				"заказ уже выполняется и не может быть отменён")
			return
		}
	}

	if err := h.store.Cancel(r.Context(), rec.ID); err != nil {
		h.fail(w, "cancel order", err)
		return
	}

	// Legs already accepted have escrow behind them: give the money back.
	var refunded []models.Transaction
	if err := h.wallet.Post(r.Context(), "/internal/transactions/refund",
		map[string]string{"orderId": rec.ID}, &refunded); err != nil {
		slog.Error("refund failed on cancel", "order", rec.ID, "err", err)
	}

	updated, err := h.store.ByID(r.Context(), rec.ID)
	if err != nil {
		h.fail(w, "reload order", err)
		return
	}
	order := updated.Flatten()
	h.emitTo(participants(rec), counterparties(rec, claims.Sub),
		models.EventOrderUpdated, order, &models.NotifySpec{
			Type:           models.NotifOrderStatusChanged,
			Title:          "Заказ отменён",
			Body:           "Клиент отменил заказ" + refundNote(refunded),
			RelatedOrderID: &rec.ID,
		})
	httpx.WriteJSON(w, http.StatusOK, order)
}

func (h *Handler) confirm(w http.ResponseWriter, r *http.Request) {
	rec, err := h.load(w, r)
	if rec == nil {
		return
	}
	claims := httpx.Claims(r)
	if rec.ClientID != claims.Sub && claims.Role != string(models.RoleAdmin) {
		httpx.WriteError(w, http.StatusForbidden, httpx.CodeForbidden,
			"only the client can confirm completion")
		return
	}

	statuses := make([]models.OrderStatus, 0, len(rec.Legs))
	for _, l := range rec.Legs {
		statuses = append(statuses, l.Status)
	}
	if !IsReadyForClientConfirmation(statuses) {
		httpx.WriteError(w, http.StatusConflict, httpx.CodeOrderNotReady,
			"все плечи заказа должны быть доставлены до подтверждения")
		return
	}

	if err := h.store.CompleteAllLegs(r.Context(), rec.ID); err != nil {
		h.fail(w, "complete legs", err)
		return
	}

	// Release escrow ONCE PER PAYEE (contract §4). Release is idempotent, so a
	// retried confirmation cannot pay anyone twice.
	for _, payee := range payees(rec) {
		var tx models.Transaction
		if err := h.wallet.Post(r.Context(), "/wallet/transactions/release",
			map[string]string{"orderId": rec.ID, "payeeId": payee}, &tx); err != nil {
			slog.Error("escrow release failed", "order", rec.ID, "payee", payee, "err", err)
			continue
		}
		h.emit([]string{payee}, models.EventTransactionUpdated, tx, &models.NotifySpec{
			Type:           models.NotifPaymentReleased,
			Title:          "Оплата получена",
			Body:           "Средства по заказу переведены на ваш счёт",
			RelatedOrderID: &rec.ID,
		})
	}

	updated, err := h.store.ByID(r.Context(), rec.ID)
	if err != nil {
		h.fail(w, "reload order", err)
		return
	}
	order := updated.Flatten()
	h.emit(participants(rec), models.EventOrderUpdated, order, nil)
	httpx.WriteJSON(w, http.StatusOK, order)
}

// ---- estimate ---------------------------------------------------------

func (h *Handler) estimate(w http.ResponseWriter, r *http.Request) {
	var in orderInput
	if err := httpx.ReadJSONLoose(r, &in); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	tariffs, err := h.store.Tariffs(r.Context())
	if err != nil {
		h.fail(w, "load tariffs", err)
		return
	}

	breakdown := Estimate(tariffs, PriceInput{
		Cargo: in.Cargo, Equipment: in.EquipmentRequest, Labor: in.LaborRequest,
	})
	total := Total(tariffs, breakdown)
	legs := LegsFor(in.Type, in.EquipmentRequest != nil, in.LaborRequest != nil)

	// Per-leg amounts are returned too: they are what each executor is
	// actually paid, so the client can show the split before ordering.
	perLeg := map[models.Leg]string{}
	for leg, amount := range SplitTotal(breakdown, total, legs) {
		perLeg[leg] = strconv.FormatInt(amount, 10)
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"priceEstimate": strconv.FormatInt(total, 10),
		"currency":      "UZS",
		"breakdown":     perLeg,
	})
}

// ---- admin & internal -------------------------------------------------

func (h *Handler) adminOrders(w http.ResponseWriter, r *http.Request) {
	recs, err := h.store.ListAll(r.Context())
	if err != nil {
		h.fail(w, "list all orders", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, flattenAll(recs))
}

func (h *Handler) internalOrder(w http.ResponseWriter, r *http.Request) {
	rec, err := h.load(w, r)
	if rec == nil {
		return
	}
	_ = err
	httpx.WriteJSON(w, http.StatusOK, rec.Flatten())
}

func (h *Handler) internalStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.store.Stats(r.Context())
	if err != nil {
		h.fail(w, "order stats", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, stats)
}

// ---- helpers ----------------------------------------------------------

// load fetches the order named in the path, writing the 404 itself when absent.
func (h *Handler) load(w http.ResponseWriter, r *http.Request) (*OrderRecord, error) {
	rec, err := h.store.ByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeOrderNotFound, "заказ не найден")
		return nil, err
	}
	if err != nil {
		h.fail(w, "load order", err)
		return nil, err
	}
	return rec, nil
}

// emit pushes a realtime event, and optionally a stored notification. It is
// fire-and-forget: a notification failure must never fail an order update.
func (h *Handler) emit(userIDs []string, event string, data any, notify *models.NotifySpec) {
	h.emitTo(userIDs, nil, event, data, notify)
}

// emitTo separates the two audiences: everyone in userIDs gets the state event
// so their screen updates, but only notifyUserIDs get the notification. The
// actor should not be told about their own action.
func (h *Handler) emitTo(userIDs, notifyUserIDs []string, event string, data any, notify *models.NotifySpec) {
	h.notify.Fire("/internal/events", models.EmitEventRequest{
		UserIDs:       dedupe(userIDs),
		Event:         event,
		Data:          data,
		Notify:        notify,
		NotifyUserIDs: dedupe(notifyUserIDs),
	})
}

// counterparties is everyone on an order EXCEPT the person who acted.
func counterparties(rec *OrderRecord, actor string) []string {
	out := []string{}
	for _, id := range participants(rec) {
		if id != actor {
			out = append(out, id)
		}
	}
	return out
}

func (h *Handler) fail(w http.ResponseWriter, op string, err error) {
	slog.Error("orders", "op", op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal error")
}

func flattenAll(recs []*OrderRecord) []models.Order {
	out := make([]models.Order, 0, len(recs))
	for _, rec := range recs {
		out = append(out, rec.Flatten())
	}
	return out
}

// title names the order in the wallet ledger and in notifications.
func title(rec *OrderRecord) string {
	switch {
	case rec.Cargo != nil && rec.Cargo.CargoType != "":
		return rec.Cargo.CargoType
	case rec.Equipment != nil:
		return "Спецтехника: " + string(rec.Equipment.EquipmentType)
	case rec.Labor != nil:
		return "Рабочая сила"
	}
	return "Заказ"
}

// payees lists the distinct executors owed money on an order.
func payees(rec *OrderRecord) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, leg := range rec.Legs {
		if leg.ExecutorID != nil && !seen[*leg.ExecutorID] {
			seen[*leg.ExecutorID] = true
			out = append(out, *leg.ExecutorID)
		}
	}
	return out
}

// participants is everyone who should hear about a change: client + executors.
func participants(rec *OrderRecord) []string {
	return append([]string{rec.ClientID}, payees(rec)...)
}

func dedupe(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id != "" && !seen[id] {
			seen[id] = true
			out = append(out, id)
		}
	}
	return out
}

// cancellable reports whether a leg is still early enough to call off.
func cancellable(s models.OrderStatus) bool {
	switch s {
	case models.StatusDraft, models.StatusPublished, models.StatusMatched,
		models.StatusAccepted, models.StatusCancelled:
		return true
	}
	return false
}

func refundNote(refunded []models.Transaction) string {
	if len(refunded) == 0 {
		return ""
	}
	return ", средства возвращены"
}

// statusText renders the leg-aware wording the UI uses (contract §3.1 note).
func statusText(leg models.Leg, s models.OrderStatus) string {
	if leg == models.LegEquipment || leg == models.LegLabor {
		switch s {
		case models.StatusLoadingInProgress:
			return "Исполнитель на месте, работы начаты"
		case models.StatusInTransit:
			return "Работы выполняются"
		case models.StatusDelivered:
			return "Работы завершены"
		}
	}
	switch s {
	case models.StatusLoadingInProgress:
		return "Погрузка началась"
	case models.StatusInTransit:
		return "Груз в пути"
	case models.StatusDelivered:
		return "Груз доставлен"
	case models.StatusCompleted:
		return "Заказ завершён"
	}
	return "Новый статус: " + string(s)
}

func validLeg(l models.Leg) bool {
	switch l {
	case models.LegTransport, models.LegEquipment, models.LegLabor:
		return true
	}
	return false
}
