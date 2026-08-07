package config

import "testing"

// The dashboard password is the only credential guarding the back office, so
// the comparison's edges are worth pinning down.
func TestAdminPasswordMatches(t *testing.T) {
	cfg := Config{AdminUsername: "admin", AdminPassword: "s3cret-pass"}

	cases := []struct {
		name string
		user string
		pass string
		want bool
	}{
		{"correct pair", "admin", "s3cret-pass", true},
		{"wrong password", "admin", "s3cret-pas", false},
		{"wrong username", "root", "s3cret-pass", false},
		{"both empty", "", "", false},
		{"empty password", "admin", "", false},
		{"case matters", "Admin", "s3cret-pass", false},
		{"no trailing-space tolerance", "admin", "s3cret-pass ", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cfg.AdminPasswordMatches(c.user, c.pass); got != c.want {
				t.Errorf("got %v, want %v", got, c.want)
			}
		})
	}

	// An unconfigured password must CLOSE the door, not open it. Getting this
	// backwards would leave a fresh deployment wide open.
	empty := Config{AdminUsername: "admin"}
	if empty.AdminPasswordMatches("admin", "") {
		t.Error("no ADMIN_PASSWORD configured must reject sign-in, not allow it")
	}
	if empty.AdminPasswordMatches("admin", "anything") {
		t.Error("no ADMIN_PASSWORD configured must reject any password")
	}
}

func TestTestPhonesAreUniversal(t *testing.T) {
	cases := map[string]bool{
		"*":                          true,
		" * ":                        true,
		"":                           false,
		"+998900000001":              false,
		"+998900000001,+99890000002": false,
		"**":                         false,
	}
	for value, want := range cases {
		if got := (Config{TestPhones: value}).TestPhonesAreUniversal(); got != want {
			t.Errorf("TEST_PHONES=%q: got %v, want %v", value, got, want)
		}
	}
}
