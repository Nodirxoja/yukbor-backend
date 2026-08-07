package httpx

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// contractCodes is every error code docs/API_CONTRACT.md promises the iOS app,
// plus the two the plan adds (§5 licence gate, §10 payments). The app branches
// on these strings, so a typo here is a silent production bug — this test is
// the "error codes match the contract exactly" check from plan §9.
var contractCodes = []string{
	// §1 auth
	"OTP_INVALID",
	"OTP_EXPIRED",
	"PHONE_ALREADY_REGISTERED",
	"USER_NOT_FOUND",
	// §1 MyID KYC (contract v1.1)
	"PASSPORT_NOT_FOUND",
	"FACE_MISMATCH",
	"MYID_SERVICE_UNAVAILABLE",
	"MYID_TOKEN_EXPIRED_OR_INVALID",
	// plan §5 / §10
	"LICENSE_CATEGORY_MISMATCH",
	"PAYMENT_DECLINED",
	// §3 orders
	"ORDER_NOT_FOUND",
	"ORDER_NOT_PUBLISHED",
	"LEG_ALREADY_TAKEN",
}

// declared reads the constant VALUES out of errors.go rather than listing them
// in Go, so the test cannot drift by being updated alongside the thing it
// checks.
func declared(t *testing.T) map[string]string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(".", "errors.go"))
	if err != nil {
		t.Fatalf("read errors.go: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*(Code\w+)\s*=\s*"([A-Z_]+)"`)
	out := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(string(body), -1) {
		out[m[2]] = m[1]
	}
	if len(out) == 0 {
		t.Fatal("parsed no error codes out of errors.go")
	}
	return out
}

func TestContractCodesAreDeclared(t *testing.T) {
	got := declared(t)
	for _, code := range contractCodes {
		if _, ok := got[code]; !ok {
			t.Errorf("contract promises %q but no constant declares it", code)
		}
	}
}

// Two constants sharing a value means one of them is a copy-paste mistake and
// a handler is returning the wrong code.
func TestNoDuplicateCodeValues(t *testing.T) {
	body, err := os.ReadFile(filepath.Join(".", "errors.go"))
	if err != nil {
		t.Fatalf("read errors.go: %v", err)
	}
	re := regexp.MustCompile(`(?m)^\s*(Code\w+)\s*=\s*"([A-Z_]+)"`)
	seen := map[string]string{}
	for _, m := range re.FindAllStringSubmatch(string(body), -1) {
		if prev, ok := seen[m[2]]; ok {
			t.Errorf("%s and %s both map to %q", prev, m[1], m[2])
		}
		seen[m[2]] = m[1]
	}
}

// Codes must be SCREAMING_SNAKE_CASE: the app compares them literally.
func TestCodeFormat(t *testing.T) {
	valid := regexp.MustCompile(`^[A-Z][A-Z0-9_]*[A-Z0-9]$`)
	for code, name := range declared(t) {
		if !valid.MatchString(code) {
			t.Errorf("%s = %q is not SCREAMING_SNAKE_CASE", name, code)
		}
		if strings.Contains(code, "__") {
			t.Errorf("%s = %q has a doubled underscore", name, code)
		}
	}
}
