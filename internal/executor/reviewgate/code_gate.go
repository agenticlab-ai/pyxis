package reviewgate

import (
	"slices"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

const (
	GatePolicySchema = "local-task-code-gate-policy/v1"
	GateReportSchema = "local-task-code-gate-report/v1"
)

// These v1 wire fields are frozen independently of runtime gate structs.
// New gate semantics require a new wire version and an explicit conversion.
type GateCheck struct {
	Name     string   `json:"name"`
	Command  string   `json:"command"`
	Group    string   `json:"group"`
	Phase    int      `json:"phase"`
	FailOn   string   `json:"fail_on"`
	Class    string   `json:"class"`
	TimeoutS int      `json:"timeout_s"`
	Env      []string `json:"env"`
}

type GatePolicy struct {
	SchemaVersion string      `json:"schema_version"`
	Checks        []GateCheck `json:"checks"`
}

type GateOutcome struct {
	Name       string `json:"name"`
	Class      string `json:"class"`
	Passed     bool   `json:"passed"`
	DurationNS int64  `json:"duration_ns"`
	Detail     string `json:"detail"`
	Output     string `json:"output"`
}

type GateReport struct {
	SchemaVersion string        `json:"schema_version"`
	Outcomes      []GateOutcome `json:"outcomes"`
	Passed        bool          `json:"passed"`
}

func FreezeCodeGatePolicy(config codegatemirror.Config) GatePolicy {
	p := GatePolicy{SchemaVersion: GatePolicySchema}
	for _, c := range config.Checks {
		p.Checks = append(p.Checks, GateCheck{Name: c.Name, Command: c.Command, Group: c.Group, Phase: c.Phase, FailOn: string(c.FailOn), Class: string(c.Class), TimeoutS: c.TimeoutS, Env: slices.Clone(c.Env)})
	}
	return p
}

func FreezeCodeGateReport(report codegatemirror.Report) GateReport {
	r := GateReport{SchemaVersion: GateReportSchema, Passed: report.Passed}
	for _, o := range report.Outcomes {
		r.Outcomes = append(r.Outcomes, GateOutcome{Name: o.Name, Class: string(o.Class), Passed: o.Passed, DurationNS: int64(o.Duration), Detail: o.Detail, Output: o.Output})
	}
	return r
}
