package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"math/big"
	"time"
)

// SMSSender delivers OTP codes to +998 numbers.
//
// MVP binds LogSender (prints the code — perfect for demos).
// Production: implement EskizSender / PlayMobileSender against the standard
// Uzbek SMS gateways, selected via env. Registration flow is identical.
type SMSSender interface {
	Send(ctx context.Context, phoneNumber, text string) error
}

// LogSender prints the OTP to stdout instead of sending SMS.
type LogSender struct{}

func (LogSender) Send(_ context.Context, phone, text string) error {
	slog.Info("SMS (dev mode, not sent)", "to", phone, "text", text)
	return nil
}

// OTP policy (plan §5, §10).
const (
	OTPTTL         = 120 * time.Second // contract: expiresInSeconds
	OTPMaxAttempts = 5                 // then the code is dead
	OTPWindow      = 10 * time.Minute  // rate-limit window per phone
	OTPWindowMax   = 3                 // codes per phone per window
	OTPMasterCode  = "7777"            // non-prod only: on-stage escape hatch
	OTPMessageForm = "YUK BOR: kod %s. Hech kimga aytmang."
)

// GenerateOTP returns a cryptographically random 4-digit code.
func GenerateOTP() string {
	n, err := rand.Int(rand.Reader, big.NewInt(10000))
	if err != nil {
		// crypto/rand failing is unrecoverable; a predictable code is worse
		// than a loud crash.
		panic("otp: crypto/rand unavailable: " + err.Error())
	}
	return fmt.Sprintf("%04d", n.Int64())
}

// HashOTP is what we persist — codes are never stored in plaintext, even in
// MVP, because it costs nothing to do correctly.
func HashOTP(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

// OTPMessage renders the SMS body.
func OTPMessage(code string) string { return fmt.Sprintf(OTPMessageForm, code) }
