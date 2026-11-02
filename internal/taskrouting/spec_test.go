package taskrouting

import (
	"strings"
	"testing"
)

const valid = `{"schema_version":"task-native-routing/v1","implementation":{"harness":"codex","model":"gpt-5.4","effort":"high"},"review":{"harness":"claude","model":"opus","effort":"high"},"rationale":"Cross-package behavior needs careful implementation and independent review."}`

func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name, input string
		wantError   bool
	}{
		{"valid", valid, false},
		{"missing effort", strings.Replace(valid, `"effort":"high"`, `"effort":""`, 1), true},
		{"unknown field", strings.Replace(valid, `"rationale":`, `"rung":2,"rationale":`, 1), true},
		{"duplicate field", strings.Replace(valid, `"model":"gpt-5.4"`, `"model":"gpt-5.4","model":"other"`, 1), true},
		{"unknown harness", strings.Replace(valid, `"codex"`, `"unknown"`, 1), true},
		{"argument injection", strings.Replace(valid, `"gpt-5.4"`, `"--model=x"`, 1), true},
		{"extra object", valid + `{}`, true},
		{"null", `null`, true},
		{"empty rationale", strings.Replace(valid, "Cross-package behavior needs careful implementation and independent review.", "", 1), true},
		{"empty input", ``, true},
		{"unfinished object", `{"schema_version":`, true},
		{"missing colon", `{"schema_version"}`, true},
		{"invalid key", `{1:2}`, true},
		{"unfinished array", `[{`, true},
		{"array with duplicate key", `[{"x":1,"x":2}]`, true},
		{"array instead of object", `[]`, true},
		{"missing review", `{"schema_version":"task-native-routing/v1","implementation":{"harness":"codex","model":"gpt-5.4","effort":"high"},"rationale":"reason"}`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Parse([]byte(tc.input))
			if (err != nil) != tc.wantError {
				t.Fatalf("got %+v, error %v", s, err)
			}
		})
	}
}
