package ciparity

import "testing"

func TestSetupIsClosed(t *testing.T) {
	for _, tt := range []struct {
		name string
		s    step
		want bool
	}{
		{"linux packages", step{Run: linuxSetup}, true},
		{"linux extra command", step{Run: linuxSetup + "\ngo test ./..."}, false},
		{"linux condition", step{Run: linuxSetup, If: "false"}, false},
		{"linux optional apparmor", step{Run: linuxSetupOptionalAppArmor}, true},
		{"linux optional apparmor extra command", step{Run: linuxSetupOptionalAppArmor + "\ngo test ./..."}, false},
		{"linux optional apparmor condition", step{Run: linuxSetupOptionalAppArmor, If: "false"}, false},
		{"coverage disk", step{Run: linuxCoverageDiskSetup}, true},
		{"coverage disk extra command", step{Run: linuxCoverageDiskSetup + "\ngo test ./..."}, false},
		{"coverage disk path override", step{Run: "sudo rm -rf -- /usr/local/lib"}, false},
		{"coverage disk condition", step{Run: linuxCoverageDiskSetup, If: "false"}, false},
		{"coverage disk environment", step{Run: linuxCoverageDiskSetup, Env: map[string]string{"PATH": "/tmp"}}, false},
		{"run environment", step{Run: linuxSetup, Env: map[string]string{"PATH": "/tmp"}}, false},
		{"run action inputs", step{Run: linuxSetup, With: map[string]string{"x": "y"}}, false},
		{"historical fetch", step{Run: replayFetch, If: replayCondition}, true},
		{"unconditional replay", step{Run: replayFetch}, false},
		{"checkout", step{Uses: "actions/checkout@v4", With: map[string]string{"fetch-depth": "0"}}, true},
		{"shallow checkout", step{Uses: "actions/checkout@v4", With: map[string]string{"fetch-depth": "1"}}, false},
		{"blacksmith checkout", step{Uses: "useblacksmith/checkout@v1", With: map[string]string{"fetch-depth": "0"}}, true},
		{"blacksmith checkout extra input", step{Uses: "useblacksmith/checkout@v1", With: map[string]string{"fetch-depth": "0", "ref": "x"}}, false},
		{"go", step{Uses: "actions/setup-go@v5", With: map[string]string{"go-version-file": "go.mod", "cache": "false"}}, true},
		{"go override", step{Uses: "actions/setup-go@v5", With: map[string]string{"go-version-file": "go.mod", "go-version": "old"}}, false},
		{"python", step{Uses: "actions/setup-python@v5", With: map[string]string{"python-version": "3.x"}}, true},
		{"node", step{Uses: "actions/setup-node@v4", With: map[string]string{"node-version": "20"}}, true},
		{"cache restore", step{Uses: "actions/cache/restore@v4", With: map[string]string{"path": "cache", "key": "key", "restore-keys": "fallback"}}, true},
		{"cache save", step{Uses: "actions/cache/save@v4", With: map[string]string{"path": "cache", "key": "key"}}, true},
		{"action env", step{Uses: "actions/setup-node@v4", Env: map[string]string{"PATH": "/tmp"}}, false},
		{"unknown action", step{Uses: "vendor/check@v1"}, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := setupStep(tt.s, "github"); got != tt.want {
				t.Fatalf("setupStep = %v, want %v", got, tt.want)
			}
		})
	}
}
