package auth

import "testing"

// The test-number allowlist is a deliberate hole in authentication, so its
// exact boundaries matter more than most logic here: it must open for the
// listed numbers and for nothing else.
func TestOTPPolicyAccepts(t *testing.T) {
	const realCode = "4271"
	hash := HashOTP(realCode)

	prod := OTPPolicy{
		AllowMaster: false,
		TestPhones:  map[string]bool{"+998900000001": true},
		TestCode:    "0000",
	}

	cases := []struct {
		name       string
		policy     OTPPolicy
		phone      string
		code       string
		wantOK     bool
		wantIsTest bool
	}{
		{"real code always works", prod, "+998901112233", realCode, true, false},
		{"wrong code is rejected", prod, "+998901112233", "1111", false, false},

		{"test number accepts the fixed code", prod, "+998900000001", "0000", true, true},
		{"test number still accepts its real code", prod, "+998900000001", realCode, true, false},
		{"test number rejects other wrong codes", prod, "+998900000001", "1234", false, false},

		// The whole point: the fixed code must be worthless anywhere else.
		{"fixed code does NOT work for a real user", prod, "+998901112233", "0000", false, false},

		{"master code rejected in prod", prod, "+998901112233", OTPMasterCode, false, false},
		{"master code accepted in dev",
			OTPPolicy{AllowMaster: true}, "+998901112233", OTPMasterCode, true, false},

		// An empty allowlist or empty code must not accidentally open anything.
		{"empty allowlist opens nothing",
			OTPPolicy{TestCode: "0000"}, "+998900000001", "0000", false, false},
		{"empty test code opens nothing",
			OTPPolicy{TestPhones: map[string]bool{"+998900000001": true}},
			"+998900000001", "", false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			ok, isTest := c.policy.accepts(c.phone, c.code, hash)
			if ok != c.wantOK {
				t.Errorf("accepted=%v, want %v", ok, c.wantOK)
			}
			if isTest != c.wantIsTest {
				t.Errorf("viaTestNumber=%v, want %v", isTest, c.wantIsTest)
			}
		})
	}
}
