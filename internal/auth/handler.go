// Package auth owns identity: OTP login, registration, tokens, user profile,
// and executor verification (MyID / driver license — stubbed for MVP).
package auth

import (
	"net/http"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
)

type Handler struct {
	cfg     config.Config
	sms     SMSSender
	myid    MyIDClient
	license LicenseVerifier
	// store *Store  // TODO(day-1): postgres-backed store (see store.go)
}

func NewHandler(cfg config.Config) *Handler {
	return &Handler{cfg: cfg, sms: LogSender{}, myid: SimulatedMyIDClient{}, license: SimulatedLicenseVerifier{}}
}

// Routes wires every endpoint from the API contract (§1, §2).
func (h *Handler) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("auth"))

	// §1 Auth
	mux.HandleFunc("POST /auth/otp/request", h.otpRequest) // TODO(day-1)
	mux.HandleFunc("POST /auth/otp/verify", h.otpVerify)   // TODO(day-1)
	// MyID KYC (contract v1.1): multipart/form-data with passport data +
	// selfie, proxied to MyID B2B API → myIdVerificationToken (TTL ~10 min).
	// Errors: PASSPORT_NOT_FOUND, FACE_MISMATCH, MYID_SERVICE_UNAVAILABLE.
	mux.HandleFunc("POST /auth/myid/verify", h.myidVerify) // TODO(day-1)
	// register requires a valid myIdVerificationToken (contract v1.1);
	// on success verificationStatus = approved. MYID_TOKEN_EXPIRED_OR_INVALID.
	mux.HandleFunc("POST /auth/register", h.register) // TODO(day-1)
	mux.HandleFunc("POST /auth/login", h.login)       // TODO(day-1)
	mux.HandleFunc("POST /auth/refresh", h.refresh)   // TODO(day-1)
	mux.HandleFunc("POST /auth/logout", h.logout)     // TODO(day-1)

	// §2 Users
	mux.HandleFunc("GET /users/me", h.me)         // TODO(day-1)
	mux.HandleFunc("PATCH /users/me", h.updateMe) // TODO(day-1)

	// Dev-only: flip verificationStatus while MyID integration is stubbed.
	mux.HandleFunc("POST /internal/admin/users/{id}/verification",
		httpx.InternalOnly(h.cfg.InternalToken, h.setVerification))

	return httpx.Logger(mux)
}

func (h *Handler) otpRequest(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
func (h *Handler) otpVerify(w http.ResponseWriter, r *http.Request)  { httpx.NotImplemented(w, r) }
func (h *Handler) myidVerify(w http.ResponseWriter, r *http.Request) { httpx.NotImplemented(w, r) }
func (h *Handler) register(w http.ResponseWriter, r *http.Request)   { httpx.NotImplemented(w, r) }
func (h *Handler) login(w http.ResponseWriter, r *http.Request)      { httpx.NotImplemented(w, r) }
func (h *Handler) refresh(w http.ResponseWriter, r *http.Request)    { httpx.NotImplemented(w, r) }
func (h *Handler) logout(w http.ResponseWriter, r *http.Request)     { httpx.NotImplemented(w, r) }
func (h *Handler) me(w http.ResponseWriter, r *http.Request)         { httpx.NotImplemented(w, r) }
func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request)   { httpx.NotImplemented(w, r) }
func (h *Handler) setVerification(w http.ResponseWriter, r *http.Request) {
	httpx.NotImplemented(w, r)
}
