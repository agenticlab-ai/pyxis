package reviewgate

import (
	"crypto/sha256"
	"fmt"
	"reflect"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// Validate compares the retained snapshot with freshly resolved policy and
// origin bindings. It checks the entire required universe, never a subset.
func Validate(binding Binding, snapshot Snapshot, current Policy) error {
	if err := binding.Validate(); err != nil {
		return err
	}
	if snapshot.SchemaVersion != SnapshotSchema || snapshot.Binding != binding || current.SchemaVersion != PolicySchema || !reflect.DeepEqual(snapshot.Policy, current) {
		return fmt.Errorf("%w: snapshot bindings or policy changed", ErrIncomplete)
	}
	if current.NativeModel == "" || current.Prompts == "" || (current.ReviewBand != report.BandStandard && current.ReviewBand != report.BandDeep) {
		return fmt.Errorf("%w: missing review policy", ErrIncomplete)
	}
	table, err := calibration.Parse(strings.NewReader(current.Calibration))
	if err != nil {
		return fmt.Errorf("%w: calibration: %w", ErrIncomplete, err)
	}
	a := snapshot.Review.Aggregate
	if snapshot.Review.Status != implreview.HandoffCompleted || a == nil || a.ProcessID == "" || a.ProposalID != binding.Subject() || a.ReviewedRevision != binding.Head || a.Report == nil {
		return fmt.Errorf("%w: missing current completed aggregate", ErrIncomplete)
	}
	r := a.Report
	if err := report.Validate(r, table); err != nil {
		return fmt.Errorf("%w: report: %w", ErrIncomplete, err)
	}
	if r.ProposalRef != binding.Subject() || r.ReviewID != a.ProcessID || r.Band != current.ReviewBand {
		return fmt.Errorf("%w: report subject, process, or band changed", ErrIncomplete)
	}
	latest := r.Rounds[len(r.Rounds)-1]
	if latest.ReviewedRevision != binding.Head || latest.ContextID == "" || latest.Provenance.Model != current.NativeModel || latest.Provenance.PromptsVersion != current.Prompts || latest.Provenance.CalVersion != calibration.Version {
		return fmt.Errorf("%w: stale review round or provenance", ErrIncomplete)
	}
	payload, err := r.Marshal()
	if err != nil {
		return err
	}
	if snapshot.Review.DiagnosticDigest != fmt.Sprintf("sha256:%x", sha256.Sum256(payload)) {
		return fmt.Errorf("%w: completed aggregate digest changed", ErrIncomplete)
	}
	outcome, err := verdict.Compute(r.Rounds, verdict.Signals{})
	if err != nil {
		return fmt.Errorf("%w: review verdict: %w", ErrIncomplete, err)
	}
	if outcome.Verdict != r.Verdict || outcome.Authority != verdict.AuthorityNone {
		return fmt.Errorf("%w: review requires changes or an operator ruling", ErrIncomplete)
	}
	if err := validateCodeGate(current.CodeGate, snapshot.CodeGate); err != nil {
		return err
	}
	seen := make(map[string]port.CheckState, len(snapshot.Checks))
	for _, check := range snapshot.Checks {
		if check.Name == "" {
			return fmt.Errorf("%w: unnamed check", ErrIncomplete)
		}
		if _, exists := seen[check.Name]; exists {
			return fmt.Errorf("%w: duplicate check %s", ErrIncomplete, check.Name)
		}
		seen[check.Name] = check.State
	}
	required := make(map[string]bool, len(current.Required))
	for _, name := range current.Required {
		if name == "" || required[name] || seen[name] != port.CheckGreen {
			return fmt.Errorf("%w: missing, duplicate, or non-green required check %s", ErrIncomplete, name)
		}
		required[name] = true
	}
	return nil
}

func validateCodeGate(config GatePolicy, gate GateReport) error {
	if config.SchemaVersion != GatePolicySchema || gate.SchemaVersion != GateReportSchema || !gate.Passed || len(config.Checks) == 0 || len(gate.Outcomes) != len(config.Checks) {
		return fmt.Errorf("%w: missing complete code gate", ErrIncomplete)
	}
	seen := make(map[string]GateOutcome, len(gate.Outcomes))
	for _, outcome := range gate.Outcomes {
		if _, exists := seen[outcome.Name]; exists {
			return fmt.Errorf("%w: duplicate code gate outcome", ErrIncomplete)
		}
		seen[outcome.Name] = outcome
	}
	names, required := make(map[string]bool, len(config.Checks)), 0
	for _, check := range config.Checks {
		class := check.Class
		if class == "" {
			class = "required"
		}
		if check.Name == "" || strings.TrimSpace(check.Command) == "" || names[check.Name] || (class != "required" && class != "advisory") || check.Phase < 0 || check.TimeoutS < 0 || (check.FailOn != "" && check.FailOn != "exit" && check.FailOn != "output") {
			return fmt.Errorf("%w: invalid code gate policy", ErrIncomplete)
		}
		names[check.Name] = true
		outcome, exists := seen[check.Name]
		if !exists || outcome.Class != class || outcome.DurationNS < 0 || (class == "required" && !outcome.Passed) {
			return fmt.Errorf("%w: absent or failed required code gate check %s", ErrIncomplete, check.Name)
		}
		if class == "required" {
			required++
		}
	}
	if required == 0 {
		return fmt.Errorf("%w: no required code gate checks", ErrIncomplete)
	}
	return nil
}
