// Package svc is the service-to-service HTTP client. Cross-service calls are
// plain REST with a shared X-Internal-Token header (plan §2) — even inside the
// monorepo, so splitting repos later costs nothing.
package svc

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"
)

type Client struct {
	base  string
	token string
	http  *http.Client
}

func New(baseURL, internalToken string) *Client {
	return &Client{
		base:  baseURL,
		token: internalToken,
		http:  &http.Client{Timeout: 5 * time.Second},
	}
}

// Do performs a request and decodes the JSON response into out (may be nil).
// A non-2xx status is returned as *Error so callers can inspect the upstream
// contract error code.
func (c *Client) Do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Internal-Token", c.token)

	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return parseError(resp)
	}
	if out == nil || resp.StatusCode == http.StatusNoContent {
		return nil
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.Do(ctx, http.MethodGet, path, nil, out)
}

func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.Do(ctx, http.MethodPost, path, body, out)
}

// Fire is the fire-and-forget form used for notifications: a failed event must
// never fail the order update that triggered it (plan §2). It runs on its own
// goroutine and background context so the caller's request can return.
func (c *Client) Fire(path string, body any) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		if err := c.Do(ctx, http.MethodPost, path, body, nil); err != nil {
			slog.Warn("internal call failed (ignored)", "path", path, "err", err)
		}
	}()
}

// Error carries an upstream service's contract error response.
type Error struct {
	Status  int
	Code    string
	Message string
}

func (e *Error) Error() string {
	return fmt.Sprintf("upstream %d %s: %s", e.Status, e.Code, e.Message)
}

// CodeOf returns the contract error code of err, or "" if err is not an
// upstream error — lets a caller re-surface e.g. PAYMENT_DECLINED verbatim.
func CodeOf(err error) string {
	if e, ok := err.(*Error); ok {
		return e.Code
	}
	return ""
}

func parseError(resp *http.Response) error {
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 8<<10))
	_ = json.Unmarshal(body, &envelope)
	if envelope.Error.Code == "" {
		envelope.Error.Code = "UPSTREAM_ERROR"
		envelope.Error.Message = string(body)
	}
	return &Error{Status: resp.StatusCode, Code: envelope.Error.Code, Message: envelope.Error.Message}
}
