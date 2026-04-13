package varstore

// Context: This file belongs to the MetricsNode application layer around varstore_test.

import "testing"

func TestValidVarName(t *testing.T) {
	cases := []struct {
		name  string
		valid bool
	}{
		{"sys_battery_percent", true},
		{"A_1", true},
		{"", false},
		{"a.b", false},
		{"a-b", false},
		{"a b", false},
	}
	for _, tc := range cases {
		if got := ValidVarName(tc.name); got != tc.valid {
			t.Fatalf("ValidVarName(%q)=%v want %v", tc.name, got, tc.valid)
		}
	}
}
