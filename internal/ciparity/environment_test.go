package ciparity

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

func TestEffectiveHostedConfinement(t *testing.T) {
	t.Parallel()
	cfg, err := codegatemirror.Parse([]byte(testConfig))
	if err != nil {
		t.Fatal(err)
	}
	const setting = "      PYXIS_REQUIRE_PROCESS_CONFINEMENT: \"1\"\n"
	for _, tt := range []struct {
		name, body string
		wantError  bool
	}{
		{"workflow value", "env: {PYXIS_REQUIRE_PROCESS_CONFINEMENT: '1'}\n" + strings.Replace(testWorkflow, setting, "", 1), false},
		{"workflow disabled", "env: {PYXIS_REQUIRE_PROCESS_CONFINEMENT: '0'}\n" + strings.Replace(testWorkflow, setting, "", 1), true},
		{"job overrides workflow", "env: {PYXIS_REQUIRE_PROCESS_CONFINEMENT: '0'}\n" + testWorkflow, false},
		{"step enables confinement", strings.Replace(strings.Replace(testWorkflow, setting, "", 1), "      - name: all checks", "      - name: all checks\n        env: {PYXIS_REQUIRE_PROCESS_CONFINEMENT: '1'}", 1), false},
		{"memory subset needs no confinement", strings.Replace(strings.Replace(testWorkflow, setting, "", 1), gateRun, gateRun+" --check memory", 1), false},
		{"process subset requires confinement", strings.Replace(strings.Replace(testWorkflow, setting, "", 1), gateRun, gateRun+" --check process-boundary", 1), true},
		{"hosted identity unchanged", strings.Replace(testWorkflow, "    env:", "    env:\n      GITHUB_ACTIONS: 'true'", 1), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			_, err := checkWorkflow([]byte(tt.body), cfg, "github")
			if (err != nil) != tt.wantError {
				t.Fatalf("error = %v, want error = %t", err, tt.wantError)
			}
		})
	}
}
