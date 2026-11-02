package runner

import "testing"

// landlockEnforced gates the Landlock write-scope leg (confine_linux.go): it is
// opt-in, off by default, pending the Linux-CI enforcement matrix A-6 pins. With
// it off the Linux runner matches the macOS residual (confine returns
// unconfined) so an honest toolchain verb is never reded by a not-yet-verified
// allow-set. This is the one leg behavior a macOS run CAN assert — the neutral
// home of landlockEnforced is exactly what makes that possible.
func TestLandlockEnforced(t *testing.T) {
	tests := []struct {
		name string
		val  string
		want bool
	}{
		{name: "empty (models unset) is off", val: "", want: false},
		{name: "0 is off", val: "0", want: false},
		{name: "arbitrary non-truthy is off", val: "no", want: false},
		{name: "1 turns it on", val: "1", want: true},
		{name: "true turns it on", val: "true", want: true},
		{name: "on turns it on", val: "on", want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// t.Setenv both sets for this test and restores the ambient value
			// on cleanup; the unset case is modeled as empty, which the gate
			// treats identically to an absent var (Getenv returns "" for both).
			t.Setenv("CODEGATE_LANDLOCK", tt.val)
			if got := landlockEnforced(); got != tt.want {
				t.Errorf("landlockEnforced() with CODEGATE_LANDLOCK=%q = %v, want %v", tt.val, got, tt.want)
			}
		})
	}
}
