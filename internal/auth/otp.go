package auth

import (
	"context"
	"log/slog"
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

// OTP policy for the real implementation (TODO day-1):
//   - 4-digit code, TTL 120s (contract: expiresInSeconds)
//   - store SHA-256 hash of the code, never plaintext
//   - max 5 verify attempts per verificationId → OTP_INVALID
//   - rate limit: max 3 requests per phone per 10 minutes
//   - DEMO MODE (non-prod only): also accept master code "7777" so on-stage
//     registration never stalls on SMS delivery (plan §10)
