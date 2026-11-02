package schema

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"time"
)

// Verdict is the immutable, atomic result of a verification run.
// It carries bindings, per-rung and per-check results, findings, and state classification.
type Verdict struct {
	RunID     string           `json:"run_id"`
	Workspace string           `json:"workspace"`
	Mode      string           `json:"mode"`
	State     string           `json:"state"`
	StartTime time.Time        `json:"start_time"`
	EndTime   time.Time        `json:"end_time"`
	Rungs     map[string]*Rung `json:"rungs"`
	Findings  []Finding        `json:"findings"`
	Bindings  Bindings         `json:"bindings"`
	// CarriedFrom is the prior verdict's RunID whenever this run qualified as
	// T8's incremental state (design codegate-v1 §3.8) — i.e. Mode ==
	// "incremental". Empty on a full run, including a requested-but-not-
	// qualifying --incremental invocation, which silently runs full instead
	// (DINV-8). Per-rung provenance lives on the individual Rung.Carried.
	CarriedFrom string `json:"carried_from,omitempty"`
}

// Bindings are the six revision-bound fields that identify what was evaluated.
type Bindings struct {
	HeadSHA         string `json:"head_sha"`
	DiffBase        string `json:"diff_base"`
	ManifestHash    string `json:"manifest_hash"`
	BaselineHash    string `json:"baseline_hash"`
	CodegateVersion string `json:"codegate_version"`
	GitVersion      string `json:"git_version"`
}

// DurationMS is a duration that (un)marshals as an integer number of
// MILLISECONDS. A raw time.Duration marshals as its int64 nanosecond count, so a
// `duration_ms` JSON field carrying a time.Duration would report 1,000,000× the
// advertised unit; this type makes the wire value match the field name.
type DurationMS time.Duration

// MarshalJSON emits the duration as whole milliseconds.
func (d DurationMS) MarshalJSON() ([]byte, error) {
	return json.Marshal(time.Duration(d).Milliseconds())
}

// UnmarshalJSON reads whole milliseconds back into the duration.
func (d *DurationMS) UnmarshalJSON(b []byte) error {
	var ms int64
	if err := json.Unmarshal(b, &ms); err != nil {
		return err
	}
	*d = DurationMS(time.Duration(ms) * time.Millisecond)
	return nil
}

// Rung is one level of the ladder with its checks and aggregate state.
type Rung struct {
	Name       string                  `json:"name"`
	Order      int                     `json:"order"`
	Checks     map[string]*CheckResult `json:"checks"`
	State      string                  `json:"state"`
	Duration   DurationMS              `json:"duration_ms"`
	Skipped    bool                    `json:"skipped"`
	SkipReason string                  `json:"skip_reason,omitempty"`
	Shrinkage  int                     `json:"shrinkage,omitempty"`
	// Carried is set only when this rung's result was NOT executed this run —
	// it was spliced forward from a prior verdict by T8's incremental
	// planning (design codegate-v1 §3.8). nil means the rung actually ran
	// this invocation.
	Carried *CarriedPass `json:"carried,omitempty"`
}

// CheckResult is the per-check evaluation outcome.
type CheckResult struct {
	CheckID    string     `json:"check_id"`
	Class      string     `json:"class,omitempty"`
	State      string     `json:"state"`
	Duration   DurationMS `json:"duration_ms"`
	Message    string     `json:"message,omitempty"`
	SkipReason string     `json:"skip_reason,omitempty"`
}

// Finding is a defect or notable condition.
type Finding struct {
	CheckID string `json:"check_id"`
	Class   string `json:"class"`
	Rung    string `json:"rung"`
	Path    string `json:"path,omitempty"`
	Line    int    `json:"line,omitempty"`
	Message string `json:"message"`
	LogRef  string `json:"log_ref,omitempty"`
}

// Manifest is the closed-schema repo binding configuration.
type Manifest struct {
	Version            string                    `json:"version"`
	Stack              string                    `json:"stack"`
	Verbs              map[string]*Verb          `json:"verbs"`
	TestGlobs          []string                  `json:"test_globs,omitempty"`
	BaselinePath       string                    `json:"baseline_path,omitempty"`
	StateDirs          []string                  `json:"state_dirs,omitempty"`
	SecretScan         *SecretScanConfig         `json:"secret_scan,omitempty"`
	ForbiddenArtifacts *ForbiddenArtifactsConfig `json:"forbidden_artifacts,omitempty"`
}

// SecretScanConfig is rung 0's manifest extension point for the secret-scan
// check (design codegate-v1 §3.7/§3.10): repo-specific patterns added to the
// compiled ecosystem table. A nil value means no repo-specific patterns —
// the ecosystem defaults apply unmodified.
type SecretScanConfig struct {
	ExtraPatterns []string `json:"extra_patterns,omitempty"`
}

// ForbiddenArtifactsConfig is rung 0's manifest extension point for the
// forbidden-artifact check: a repo-specific override of the ecosystem size
// ceiling and/or additional baked-absolute-path patterns.
type ForbiddenArtifactsConfig struct {
	MaxFileBytes              int64    `json:"max_file_bytes,omitempty"`
	ExtraAbsolutePathPatterns []string `json:"extra_absolute_path_patterns,omitempty"`
}

// EnvMode is how one per-verb environment entry is resolved.
type EnvMode string

const (
	// EnvPass passes the AMBIENT value of the named variable through to the verb.
	EnvPass EnvMode = "pass"
	// EnvPin pins the variable to an EXACT value, independent of the ambient env.
	EnvPin EnvMode = "pin"
)

// EnvEntry is one per-verb environment binding — a pass-through name or a pinned
// value. The distinction is load-bearing for the strip-by-default runner
// (DINV-5): a pass-through name inherits whatever the ambient environment holds,
// while a pinned value is fixed; a bare string cannot express which is meant.
type EnvEntry struct {
	Mode  EnvMode `json:"mode"`
	Value string  `json:"value,omitempty"` // set only when Mode == EnvPin
}

// Verb is a repo-supplied executable with its environment and bounds.
type Verb struct {
	Command  string              `json:"command"`
	Env      map[string]EnvEntry `json:"env,omitempty"`
	TimeoutS int                 `json:"timeout_s,omitempty"`
	GraceS   int                 `json:"grace_s,omitempty"`
}

// Check is the registry entry for one check.
type Check struct {
	ID                string
	Rung              string
	VerbDeps          []string
	AuthorIndependent bool
	InputSet          []string
}

// MarshalManifestIndent serializes a manifest in the human-owned template
// shape — two-space indent, trailing newline — the shape `codegate seed`
// writes and operators edit. Kept beside UnmarshalManifest so the wire
// format's two directions live in one package.
func MarshalManifestIndent(m *Manifest) ([]byte, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("manifest marshal: %w", err)
	}
	return append(data, '\n'), nil
}

// UnmarshalManifest parses a manifest from JSON, failing closed on unknown keys.
func UnmarshalManifest(data []byte) (*Manifest, error) {
	var m Manifest
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(&m); err != nil {
		return nil, fmt.Errorf("manifest parse: %w", err)
	}

	if m.Version == "" {
		return nil, fmt.Errorf("manifest parse: missing version")
	}
	if m.Stack == "" {
		return nil, fmt.Errorf("manifest parse: missing stack")
	}

	// A declared verb must carry a command and well-formed env entries. The
	// closed parse rejects unknown keys; these checks reject the incomplete or
	// ambiguous bindings a well-formed-but-wrong manifest can still express.
	for name, verb := range m.Verbs {
		if verb == nil || verb.Command == "" {
			return nil, fmt.Errorf("manifest parse: verb %q has no command", name)
		}
		for envName, entry := range verb.Env {
			switch entry.Mode {
			case EnvPin:
				// A pinned value may be any string, including empty.
			case EnvPass:
				if entry.Value != "" {
					return nil, fmt.Errorf("manifest parse: verb %q env %q is pass-through but also pins a value", name, envName)
				}
			default:
				return nil, fmt.Errorf("manifest parse: verb %q env %q has invalid mode %q (want %q or %q)", name, envName, entry.Mode, EnvPass, EnvPin)
			}
		}
	}

	// The rung-0 extension points must compile as regexes at parse time — a
	// pattern that fails to compile is a malformed manifest (UB-3), caught
	// here rather than discovered lazily mid-run as a rung error.
	if m.SecretScan != nil {
		for _, p := range m.SecretScan.ExtraPatterns {
			if _, err := regexp.Compile(p); err != nil {
				return nil, fmt.Errorf("manifest parse: secret_scan.extra_patterns %q: %w", p, err)
			}
		}
	}
	if m.ForbiddenArtifacts != nil {
		if m.ForbiddenArtifacts.MaxFileBytes < 0 {
			return nil, fmt.Errorf("manifest parse: forbidden_artifacts.max_file_bytes must be non-negative, got %d", m.ForbiddenArtifacts.MaxFileBytes)
		}
		for _, p := range m.ForbiddenArtifacts.ExtraAbsolutePathPatterns {
			if _, err := regexp.Compile(p); err != nil {
				return nil, fmt.Errorf("manifest parse: forbidden_artifacts.extra_absolute_path_patterns %q: %w", p, err)
			}
		}
	}

	return &m, nil
}

// Marshal serializes the verdict to JSON.
func (v *Verdict) Marshal() ([]byte, error) {
	return json.MarshalIndent(v, "", "  ")
}

// WriteAtomicToFile writes the verdict atomically to a file using temp+rename.
// This satisfies DINV-2: an interrupted write leaves no artifact.
func (v *Verdict) WriteAtomicToFile(path string) error {
	return v.writeAtomicToFileInternal(path, true)
}

// WriteAtomicToFileSkipBindings writes the verdict atomically but skips only the
// bindings-present check — for a configuration-error verdict emitted before the
// revision bindings can be resolved. The verdict's own state invariants
// (DINV-3/4) are still enforced.
func (v *Verdict) WriteAtomicToFileSkipBindings(path string) error {
	return v.writeAtomicToFileInternal(path, false)
}

// writeAtomicToFileInternal is the shared implementation that optionally
// skips binding validation for configuration-error verdicts.
func (v *Verdict) writeAtomicToFileInternal(path string, validateBindings bool) error {
	// Fail closed: never emit a verdict that violates its own state invariants or
	// is missing a revision binding. Completeness against the configured ladder is
	// a separate check the caller runs with the expected set (ValidateCompleteness),
	// since the ladder lives outside this package.
	if err := v.ValidateVerdictInvariants(); err != nil {
		return fmt.Errorf("refusing to write invalid verdict: %w", err)
	}
	if validateBindings {
		if err := v.Bindings.BindingsPresent(); err != nil {
			return fmt.Errorf("refusing to write verdict with incomplete bindings: %w", err)
		}
	}

	return v.writeAtomicPayload(path)
}

// writeAtomicPayload writes v's marshaled JSON atomically to path.
func (v *Verdict) writeAtomicPayload(path string) error {
	data, err := v.Marshal()
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	// Create the temp file in the DESTINATION directory (not $TMPDIR): os.Rename
	// is only atomic — and only succeeds — when source and destination are on the
	// same filesystem. A system-temp source fails with EXDEV whenever the target
	// lives on a different mount (routine on CI/containers), defeating the DINV-2
	// no-partial-verdict guarantee this function promises.
	f, err := os.CreateTemp(filepath.Dir(path), ".codegate-verdict-")
	if err != nil {
		return fmt.Errorf("temp file: %w", err)
	}
	tempPath := f.Name()
	defer os.Remove(tempPath)

	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write temp: %w", err)
	}

	if err := f.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("rename: %w", err)
	}

	return nil
}

// ValidateVerdictInvariants checks DINV-3 and DINV-4 constraints.
func (v *Verdict) ValidateVerdictInvariants() error {
	if len(v.Rungs) == 0 {
		return fmt.Errorf("verdict invariant: no rungs recorded")
	}

	passStates := map[string]bool{"pass": true, "green-degraded": true, "green": true}
	isPass := passStates[v.State]

	if isPass {
		for _, rung := range v.Rungs {
			if rung.SkipReason == "not-reached" {
				return fmt.Errorf("verdict invariant DINV-3: pass-class state contains skip:not-reached rung %s", rung.Name)
			}
			for _, check := range rung.Checks {
				if check.State == "fail" || check.State == "error" {
					return fmt.Errorf("verdict invariant DINV-3: pass-class state contains failed/errored check %s in rung %s", check.CheckID, rung.Name)
				}
			}
		}
	}

	for rungName, rung := range v.Rungs {
		if len(rung.Checks) == 0 && !rung.Skipped {
			return fmt.Errorf("verdict invariant DINV-4: rung %s has no checks and not skipped", rungName)
		}
	}

	return nil
}

// ValidateCompleteness enforces DINV-4 completeness against the CONFIGURED
// ladder: every expected rung must be present in the verdict (a not-reached rung
// is present as a recorded skip, never absent), and — for a rung that actually
// ran — every expected check must be recorded. "Silent absence is
// unrepresentable": a green verdict cannot omit a registered check by leaving it
// out. The expected set (rung name → its configured check IDs) is supplied by
// the caller from the derived ladder, because the check registry lives outside
// this package (schema is imported by registry, never the reverse).
func (v *Verdict) ValidateCompleteness(expected map[string][]string) error {
	for rungName, checkIDs := range expected {
		rung, ok := v.Rungs[rungName]
		if !ok {
			return fmt.Errorf("verdict invariant DINV-4: configured rung %s absent from the verdict", rungName)
		}
		if rung.Skipped {
			continue // a skipped rung (not-reached / verb-absent) records no checks by design
		}
		for _, id := range checkIDs {
			if _, ok := rung.Checks[id]; !ok {
				return fmt.Errorf("verdict invariant DINV-4: registered check %s absent from rung %s", id, rungName)
			}
		}
	}
	return nil
}

// BindingsPresent checks DINV-1.
func (b *Bindings) BindingsPresent() error {
	if b.HeadSHA == "" {
		return fmt.Errorf("binding invariant: missing head_sha")
	}
	if b.DiffBase == "" {
		return fmt.Errorf("binding invariant: missing diff_base")
	}
	if b.ManifestHash == "" {
		return fmt.Errorf("binding invariant: missing manifest_hash")
	}
	if b.BaselineHash == "" {
		return fmt.Errorf("binding invariant: missing baseline_hash")
	}
	if b.CodegateVersion == "" {
		return fmt.Errorf("binding invariant: missing codegate_version")
	}
	if b.GitVersion == "" {
		return fmt.Errorf("binding invariant: missing git_version")
	}
	return nil
}
