package near

import "testing"

func TestClosest(t *testing.T) {
	names := []string{"stale_days", "home_owners", "refresh", "updated", "drafts_last", "inbox", "search"}
	tests := []struct {
		s, want string
		ok      bool
	}{
		{"stal_days", "stale_days", true},
		{"refersh", "refresh", true},
		{"update", "updated", true},
		{"inbx", "inbox", true},
		{"serach", "search", true},
		{"newest", "", false},
		{"x", "", false},
		{"inbox", "", false},
	}
	for _, tt := range tests {
		got, ok := Closest(tt.s, names)
		if got != tt.want || ok != tt.ok {
			t.Errorf("Closest(%q) = %q, %v; want %q, %v", tt.s, got, ok, tt.want, tt.ok)
		}
	}
}
