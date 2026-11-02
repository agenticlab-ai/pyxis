package reviewgate

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

func TestRetainedCodeGateWireBytesAreFrozenAndDetachedFromRuntime(t *testing.T) {
	config := codegatemirror.Config{Checks: []codegatemirror.Check{{Name: "test", Command: "go test ./...", Group: "tests", Phase: 2, FailOn: codegatemirror.FailOnOutput, Class: codegatemirror.ClassRequired, TimeoutS: 60, Env: []string{"CI"}}}}
	report := codegatemirror.Report{Passed: true, Outcomes: []codegatemirror.Outcome{{Name: "test", Class: codegatemirror.ClassRequired, Passed: true, Duration: time.Second, Detail: "complete", Output: "whole output"}}}
	policy, result := FreezeCodeGatePolicy(config), FreezeCodeGateReport(report)
	wantPolicy := `{"schema_version":"local-task-code-gate-policy/v1","checks":[{"name":"test","command":"go test ./...","group":"tests","phase":2,"fail_on":"output","class":"required","timeout_s":60,"env":["CI"]}]}`
	wantReport := `{"schema_version":"local-task-code-gate-report/v1","outcomes":[{"name":"test","class":"required","passed":true,"duration_ns":1000000000,"detail":"complete","output":"whole output"}],"passed":true}`
	for _, tc := range []struct {
		value any
		want  string
	}{{policy, wantPolicy}, {result, wantReport}} {
		body, err := json.Marshal(tc.value)
		if err != nil || string(body) != tc.want {
			t.Fatalf("v1 canonical gate bytes changed: %s %v", body, err)
		}
	}
	config.Checks[0].Env[0] = "changed"
	config.Checks[0].Command = "changed"
	report.Outcomes[0].Output = "changed"
	if policy.Checks[0].Env[0] != "CI" || policy.Checks[0].Command != "go test ./..." || result.Outcomes[0].Output != "whole output" {
		t.Fatal("runtime mutation changed retained proof")
	}
	// A runtime's future optional fields are deliberately outside the v1 DTO.
	upgraded := struct {
		Config           codegatemirror.Config
		NewOptionalField bool
	}{Config: codegatemirror.Config{Checks: []codegatemirror.Check{{Name: "test", Command: "go test ./...", Group: "tests", Phase: 2, FailOn: codegatemirror.FailOnOutput, Class: codegatemirror.ClassRequired, TimeoutS: 60, Env: []string{"CI"}}}}}
	body, err := json.Marshal(FreezeCodeGatePolicy(upgraded.Config))
	if err != nil || string(body) != wantPolicy {
		t.Fatal("new runtime field changed retained v1 bytes")
	}
}

func TestRetainedCodeGateRefusesUnsupportedNestedVersionsAndValues(t *testing.T) {
	for _, fault := range []string{"policy version", "report version", "phase", "timeout", "failure semantics", "duration"} {
		t.Run(fault, func(t *testing.T) {
			b, s, p := completeFixture(t)
			switch fault {
			case "policy version":
				p.CodeGate.SchemaVersion = "v2"
			case "report version":
				s.CodeGate.SchemaVersion = "v2"
			case "phase":
				p.CodeGate.Checks[0].Phase = -1
			case "timeout":
				p.CodeGate.Checks[0].TimeoutS = -1
			case "failure semantics":
				p.CodeGate.Checks[0].FailOn = "unknown"
			case "duration":
				s.CodeGate.Outcomes[0].DurationNS = -1
			}
			s.Policy = p
			if Validate(b, s, p) == nil {
				t.Fatal("unsupported retained gate protocol accepted")
			}
		})
	}
}
