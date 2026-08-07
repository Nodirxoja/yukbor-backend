// Package auth owns identity: OTP login, registration, tokens, user profile,
// and executor verification (MyID KYC + driver licence registry).
package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/aventiseld/yukbor-backend/pkg/config"
	"github.com/aventiseld/yukbor-backend/pkg/db"
	"github.com/aventiseld/yukbor-backend/pkg/httpx"
	"github.com/aventiseld/yukbor-backend/pkg/jwtx"
	"github.com/aventiseld/yukbor-backend/pkg/models"
)

// Token lifetimes. Access is deliberately generous: a hackathon demo must not
// die mid-presentation because a token aged out between rehearsal and stage.
const (
	AccessTokenTTL  = 24 * time.Hour
	RefreshTokenTTL = 30 * 24 * time.Hour
	maxSelfieBytes  = 10 << 20 // 10 MiB
)

type Handler struct {
	cfg     config.Config
	store   *Store
	sms     SMSSender
	myid    MyIDClient
	license LicenseVerifier
}

func NewHandler(cfg config.Config, pool *pgxpool.Pool) *Handler {
	return &Handler{
		cfg:     cfg,
		store:   NewStore(pool),
		sms:     LogSender{},
		myid:    SimulatedMyIDClient{},
		license: SimulatedLicenseVerifier{},
	}
}

// Routes wires every endpoint from the API contract (§1, §2).
func (h *Handler) Routes() http.Handler {
	secret := h.cfg.Secret()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", httpx.Health("auth"))

	// §1 Auth — open, these are how you get a token in the first place.
	mux.HandleFunc("POST /auth/otp/request", h.otpRequest)
	mux.HandleFunc("POST /auth/otp/verify", h.otpVerify)
	mux.HandleFunc("POST /auth/myid/verify", h.myidVerify)
	mux.HandleFunc("POST /auth/register", h.register)
	mux.HandleFunc("POST /auth/login", h.login)
	mux.HandleFunc("POST /auth/refresh", h.refresh)
	mux.HandleFunc("POST /auth/logout", h.logout)

	// §2 Users
	mux.HandleFunc("GET /users/me", httpx.Authed(secret, h.me))
	mux.HandleFunc("PATCH /users/me", httpx.Authed(secret, h.updateMe))

	// Dashboard (plan §11)
	mux.HandleFunc("GET /admin/users",
		httpx.AuthedRole(secret, []string{string(models.RoleAdmin)}, h.adminUsers))

	// Service-to-service
	mux.HandleFunc("GET /internal/users/stats",
		httpx.InternalOnly(h.cfg.InternalToken, h.internalUserStats))
	mux.HandleFunc("GET /internal/users/{id}",
		httpx.InternalOnly(h.cfg.InternalToken, h.internalUser))
	mux.HandleFunc("POST /internal/users/{id}/rating",
		httpx.InternalOnly(h.cfg.InternalToken, h.internalRating))
	mux.HandleFunc("POST /internal/admin/users/{id}/verification",
		httpx.InternalOnly(h.cfg.InternalToken, h.setVerification))

	return httpx.Wrap(mux)
}

// ---- OTP --------------------------------------------------------------

func (h *Handler) otpRequest(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PhoneNumber string `json:"phoneNumber"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	if !validPhone(req.PhoneNumber) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"phoneNumber must be in +998XXXXXXXXX form")
		return
	}

	verificationID, code, err := h.store.CreateOTP(r.Context(), req.PhoneNumber)
	if errors.Is(err, ErrOTPRateLimited) {
		httpx.WriteError(w, http.StatusTooManyRequests, httpx.CodeOTPRateLimited,
			"too many codes requested, try again later")
		return
	}
	if err != nil {
		h.fail(w, "create otp", err)
		return
	}

	if err := h.sms.Send(r.Context(), req.PhoneNumber, OTPMessage(code)); err != nil {
		slog.Warn("sms delivery failed", "phone", req.PhoneNumber, "err", err)
	}

	resp := struct {
		VerificationID   string `json:"verificationId"`
		ExpiresInSeconds int    `json:"expiresInSeconds"`
		// DevCode is additive and non-prod only: the demo and seed scripts
		// read it instead of an SMS. Swift's Decodable ignores unknown keys,
		// so shipping it never affects the iOS app.
		DevCode string `json:"devCode,omitempty"`
	}{VerificationID: verificationID, ExpiresInSeconds: int(OTPTTL.Seconds())}
	if !h.cfg.IsProd() {
		resp.DevCode = code
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

func (h *Handler) otpVerify(w http.ResponseWriter, r *http.Request) {
	var req struct {
		VerificationID string `json:"verificationId"`
		Code           string `json:"code"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	err := h.store.VerifyOTP(r.Context(), req.VerificationID, req.Code, !h.cfg.IsProd())
	switch {
	case errors.Is(err, ErrOTPExpired):
		httpx.WriteError(w, http.StatusGone, httpx.CodeOTPExpired, "код истёк, запросите новый")
		return
	case errors.Is(err, ErrOTPInvalid):
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeOTPInvalid, "неверный код подтверждения")
		return
	case err != nil:
		h.fail(w, "verify otp", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]bool{"verified": true})
}

// ---- MyID KYC ---------------------------------------------------------

// myidVerify proxies passport data + selfie to MyID (simulated in MVP) and,
// on a match, issues the short-lived token that POST /auth/register consumes.
func (h *Handler) myidVerify(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(maxSelfieBytes); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest,
			"expected multipart/form-data with a selfie")
		return
	}

	req := MyIDVerifyRequest{
		VerificationID: r.FormValue("verificationId"),
		PassportSeries: strings.ToUpper(r.FormValue("passportSeries")),
		PassportNumber: r.FormValue("passportNumber"),
		PINFL:          r.FormValue("pinfl"),
		BirthDate:      r.FormValue("birthDate"),
	}
	if req.VerificationID == "" || req.PINFL == "" || req.PassportNumber == "" {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation,
			"verificationId, passportSeries, passportNumber, pinfl and birthDate are required")
		return
	}

	// KYC is bound to a phone that has already proved ownership (contract §1).
	if _, err := h.store.VerifiedPhone(r.Context(), req.VerificationID); err != nil {
		if errors.Is(err, ErrOTPNotVerified) {
			httpx.WriteError(w, http.StatusForbidden, httpx.CodeOTPNotVerified,
				"phone must be confirmed via OTP before identity verification")
			return
		}
		h.fail(w, "lookup verification", err)
		return
	}

	file, _, err := r.FormFile("selfie")
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "selfie image is required")
		return
	}
	defer file.Close()
	if req.Selfie, err = io.ReadAll(io.LimitReader(file, maxSelfieBytes)); err != nil {
		h.fail(w, "read selfie", err)
		return
	}

	res, err := h.myid.VerifyPerson(r.Context(), req)
	switch {
	case errors.Is(err, ErrPassportNotFound):
		httpx.WriteError(w, http.StatusNotFound, httpx.CodePassportNotFound,
			"паспорт не найден в базе ГЦП")
		return
	case errors.Is(err, ErrMyIDUnavailable):
		httpx.WriteError(w, http.StatusServiceUnavailable, httpx.CodeMyIDUnavailable,
			"сервис MyID недоступен, попробуйте ещё раз")
		return
	case err != nil:
		h.fail(w, "myid verify", err)
		return
	}

	if !res.IsMatched {
		// No token is issued — registration cannot proceed (contract §1).
		httpx.WriteError(w, http.StatusUnprocessableEntity, httpx.CodeFaceMismatch,
			"лицо не совпадает с фото в документе")
		return
	}

	rec := MyIDRecord{
		Token:            NewMyIDToken(),
		VerificationID:   req.VerificationID,
		PINFL:            req.PINFL,
		PassportSeries:   req.PassportSeries,
		PassportNumber:   req.PassportNumber,
		VerifiedFullName: res.VerifiedFullName,
		Confidence:       res.Confidence,
		BirthDate:        parseDate(req.BirthDate),
	}
	if err := h.store.CreateMyIDVerification(r.Context(), rec); err != nil {
		h.fail(w, "store myid verification", err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"myIdVerificationToken": rec.Token,
		"isMatched":             true,
		"confidence":            res.Confidence,
		"verifiedFullName":      res.VerifiedFullName,
	})
}

// ---- Registration & login ---------------------------------------------

func (h *Handler) register(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FullName              string          `json:"fullName"`
		PhoneNumber           string          `json:"phoneNumber"`
		Role                  models.UserRole `json:"role"`
		MyIDVerificationToken string          `json:"myIdVerificationToken"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	if !validRole(req.Role) {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeValidation, "unknown role")
		return
	}

	// Consuming the token is atomic, so one KYC pass registers exactly once.
	kyc, err := h.store.ConsumeMyIDToken(r.Context(), req.MyIDVerificationToken)
	if errors.Is(err, ErrMyIDToken) {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeMyIDTokenInvalid,
			"MyID verification expired — пройдите верификацию заново")
		return
	}
	if err != nil {
		h.fail(w, "consume myid token", err)
		return
	}

	// The KYC pass is bound to a phone by its verificationId; registering a
	// different number with someone else's identity must not be possible.
	phone, err := h.store.VerifiedPhone(r.Context(), kyc.VerificationID)
	if err != nil || phone != req.PhoneNumber {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeMyIDTokenInvalid,
			"verification does not match this phone number")
		return
	}

	newUser := NewUser{
		Role:               req.Role,
		FullName:           preferVerifiedName(kyc.VerifiedFullName, req.FullName),
		PhoneNumber:        req.PhoneNumber,
		VerificationStatus: models.VerificationApproved,
		PINFL:              kyc.PINFL,
		PassportSeries:     kyc.PassportSeries,
		PassportNumber:     kyc.PassportNumber,
		BirthDate:          kyc.BirthDate,
	}

	// Drivers and equipment providers additionally clear the licence registry.
	// The contract carries no vehicle type, so the gate is the role-level
	// requirement; the narrower CE check happens when a driver accepts a
	// tractor-trailer leg (orders service).
	if NeedsLicenseCheck(req.Role) {
		lic, err := h.license.VerifyDriverLicense(r.Context(), "", LicenseVerificationRequest{
			PINFL:            kyc.PINFL,
			RequiredCategory: RequiredCategoryForRole(req.Role),
		})
		if err != nil {
			h.fail(w, "license verify", err)
			return
		}
		newUser.LicenseNumber = &lic.LicenseNumber
		newUser.LicenseCategories = lic.Categories
		newUser.VehiclePlate = &lic.VehiclePlate

		if lic.Status == models.VerificationRejected {
			// Persist the rejected applicant so the dashboard shows them with
			// a reason, then block registration.
			newUser.VerificationStatus = models.VerificationRejected
			newUser.RejectionReason = &lic.Reason
			if _, err := h.store.CreateUser(r.Context(), newUser); err != nil && !errors.Is(err, ErrPhoneTaken) {
				slog.Warn("could not persist rejected applicant", "err", err)
			}
			httpx.WriteError(w, http.StatusForbidden, httpx.CodeLicenseCategoryMismatch,
				"водительское удостоверение не содержит нужной категории ("+
					RequiredCategoryForRole(req.Role)+")")
			return
		}
	}

	user, err := h.store.CreateUser(r.Context(), newUser)
	if errors.Is(err, ErrPhoneTaken) {
		httpx.WriteError(w, http.StatusConflict, httpx.CodePhoneAlreadyRegistered,
			"этот номер уже зарегистрирован")
		return
	}
	if err != nil {
		h.fail(w, "create user", err)
		return
	}

	h.issueTokens(w, r.Context(), user, http.StatusCreated)
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var req struct {
		PhoneNumber string `json:"phoneNumber"`
		// Optional: the contract (§1) posts only phoneNumber, so this is
		// accepted when sent but not required outside production. Without it
		// a phone number alone would mint tokens for that account.
		VerificationID string `json:"verificationId,omitempty"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}

	if req.VerificationID != "" {
		phone, err := h.store.VerifiedPhone(r.Context(), req.VerificationID)
		if err != nil || phone != req.PhoneNumber {
			httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeOTPNotVerified,
				"phone not confirmed")
			return
		}
	} else if h.cfg.IsProd() {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeOTPNotVerified,
			"verificationId from a confirmed OTP is required")
		return
	}

	user, err := h.store.UserByPhone(r.Context(), req.PhoneNumber)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeUserNotFound, "пользователь не найден")
		return
	}
	if err != nil {
		h.fail(w, "lookup user", err)
		return
	}
	h.issueTokens(w, r.Context(), user, http.StatusOK)
}

func (h *Handler) refresh(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	if _, err := jwtx.Verify(h.cfg.Secret(), req.RefreshToken); err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized, "invalid refresh token")
		return
	}

	// Rotation: the presented token is revoked as it is exchanged, so a
	// captured refresh token is good for at most one use.
	userID, err := h.store.RotateRefreshToken(r.Context(), req.RefreshToken)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusUnauthorized, httpx.CodeUnauthorized,
			"refresh token revoked or expired")
		return
	}
	if err != nil {
		h.fail(w, "rotate refresh token", err)
		return
	}

	user, err := h.store.UserByID(r.Context(), userID)
	if err != nil {
		h.fail(w, "lookup user", err)
		return
	}

	access, refreshTok, err := h.mintPair(r.Context(), user)
	if err != nil {
		h.fail(w, "mint tokens", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]string{
		"accessToken": access, "refreshToken": refreshTok,
	})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	var req struct {
		RefreshToken string `json:"refreshToken"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	if err := h.store.RevokeRefreshToken(r.Context(), req.RefreshToken); err != nil {
		h.fail(w, "revoke refresh token", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---- Profile ----------------------------------------------------------

func (h *Handler) me(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.UserByID(r.Context(), httpx.Claims(r).Sub)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeUserNotFound, "пользователь не найден")
		return
	}
	if err != nil {
		h.fail(w, "lookup user", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user.User)
}

func (h *Handler) updateMe(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FullName *string `json:"fullName,omitempty"`
		Email    *string `json:"email,omitempty"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	user, err := h.store.UpdateProfile(r.Context(), httpx.Claims(r).Sub, req.FullName, req.Email)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeUserNotFound, "пользователь не найден")
		return
	}
	if err != nil {
		h.fail(w, "update profile", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, user.User)
}

// ---- Admin & internal -------------------------------------------------

func (h *Handler) adminUsers(w http.ResponseWriter, r *http.Request) {
	users, err := h.store.ListUsers(r.Context(), r.URL.Query().Get("role"))
	if err != nil {
		h.fail(w, "list users", err)
		return
	}
	out := make([]models.UserDetail, 0, len(users))
	for _, u := range users {
		out = append(out, detail(u))
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

func (h *Handler) internalUser(w http.ResponseWriter, r *http.Request) {
	user, err := h.store.UserByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeUserNotFound, "пользователь не найден")
		return
	}
	if err != nil {
		h.fail(w, "lookup user", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail(*user))
}

func (h *Handler) internalUserStats(w http.ResponseWriter, r *http.Request) {
	total, err := h.store.CountUsers(r.Context())
	if err != nil {
		h.fail(w, "count users", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, models.UserStats{Total: total})
}

func (h *Handler) internalRating(w http.ResponseWriter, r *http.Request) {
	var req models.RatingUpdate
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	if err := h.store.SetRating(r.Context(), r.PathValue("id"), req.Rating, req.Count); err != nil {
		if errors.Is(err, db.ErrNotFound) {
			httpx.WriteError(w, http.StatusNotFound, httpx.CodeUserNotFound, "пользователь не найден")
			return
		}
		h.fail(w, "set rating", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) setVerification(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Status models.VerificationStatus `json:"status"`
		Reason *string                   `json:"reason,omitempty"`
	}
	if err := httpx.ReadJSON(r, &req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, httpx.CodeBadRequest, "invalid body")
		return
	}
	user, err := h.store.SetVerification(r.Context(), r.PathValue("id"), req.Status, req.Reason)
	if errors.Is(err, db.ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, httpx.CodeUserNotFound, "пользователь не найден")
		return
	}
	if err != nil {
		h.fail(w, "set verification", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, detail(*user))
}

// ---- helpers ----------------------------------------------------------

func (h *Handler) issueTokens(w http.ResponseWriter, ctx context.Context, u *UserRecord, status int) {
	access, refreshTok, err := h.mintPair(ctx, u)
	if err != nil {
		h.fail(w, "mint tokens", err)
		return
	}
	httpx.WriteJSON(w, status, map[string]any{
		"user":         u.User,
		"accessToken":  access,
		"refreshToken": refreshTok,
	})
}

func (h *Handler) mintPair(ctx context.Context, u *UserRecord) (access, refreshTok string, err error) {
	secret := h.cfg.Secret()
	if access, err = jwtx.Sign(secret, jwtx.NewAccess(u.ID, string(u.Role), AccessTokenTTL)); err != nil {
		return "", "", err
	}
	if refreshTok, err = jwtx.Sign(secret, jwtx.NewRefresh(u.ID, string(u.Role), RefreshTokenTTL)); err != nil {
		return "", "", err
	}
	err = h.store.SaveRefreshToken(ctx, refreshTok, u.ID, time.Now().Add(RefreshTokenTTL))
	return access, refreshTok, err
}

func (h *Handler) fail(w http.ResponseWriter, op string, err error) {
	slog.Error("auth", "op", op, "err", err)
	httpx.WriteError(w, http.StatusInternalServerError, httpx.CodeInternal, "internal error")
}

func detail(u UserRecord) models.UserDetail {
	return models.UserDetail{
		User:              u.User,
		CreatedAt:         u.CreatedAt,
		LicenseNumber:     u.LicenseNumber,
		LicenseCategories: u.LicenseCategories,
		VehiclePlate:      u.VehiclePlate,
		RejectionReason:   u.RejectionReason,
	}
}

// preferVerifiedName uses the name MyID returned over what the user typed
// (contract §1), falling back when the upstream gave us nothing.
func preferVerifiedName(verified, typed string) string {
	if strings.TrimSpace(verified) != "" {
		return verified
	}
	return typed
}

func parseDate(s string) *time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return nil
	}
	return &t
}

// validPhone accepts the +998XXXXXXXXX form the contract uses.
func validPhone(p string) bool {
	if !strings.HasPrefix(p, "+998") || len(p) != 13 {
		return false
	}
	for _, c := range p[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func validRole(r models.UserRole) bool {
	switch r {
	case models.RoleClient, models.RoleDriver, models.RoleEquipmentProvider,
		models.RoleLaborProvider, models.RoleFleetAdmin, models.RoleAdmin:
		return true
	}
	return false
}
