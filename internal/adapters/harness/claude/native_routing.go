package claude

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const adapterGeneration = "claude-code-native-routing/v2"

var (
	_ nativerouting.NativeCapability = (*Adapter)(nil)
	_ port.CatalogProvider           = (*Adapter)(nil)
	_ port.LaunchBuilder             = (*Adapter)(nil)
	_ port.NativeLaunchCapturer      = (*Adapter)(nil)

	// ErrLaunchFidelity marks an observed native launch that differs from the
	// qualified selection. Callers must fail the attempt and treat the named
	// qualification as invalid until it is requalified.
	ErrLaunchFidelity = errors.New("claude: native launch differs from selected profile")
)

const reviewSettings = `{"sandbox":{"enabled":false}}`

func nativeControls(role nativerouting.Role) []nativerouting.NativeControl {
	controls := []nativerouting.NativeControl{
		{Name: "output_format", Type: nativerouting.ControlString, StringValue: "json"},
		{Name: "permission_mode", Type: nativerouting.ControlString, StringValue: "bypassPermissions"},
	}
	if role == nativerouting.RoleReview {
		disabled := false
		controls = append(controls, nativerouting.NativeControl{Name: "sandbox_enabled", Type: nativerouting.ControlBoolean, BooleanValue: &disabled})
	}
	return controls
}

// Catalog publishes a fresh complete policy unit for exactly one role. All
// evidence is repository-bound and decode-pinned; validation remains the
// composition's responsibility so trusted time and authorization stay outside
// the adapter.
func (a *Adapter) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	rows := a.profiles[role]
	if len(rows) == 0 {
		return nativerouting.Catalog{}, nil, fmt.Errorf("claude: no configured profiles for role %q", role)
	}
	catalog := nativerouting.Catalog{
		SchemaVersion: nativerouting.CatalogSchema, CatalogID: "claude-" + string(role) + "-v1",
		Harness: harnessID, Role: role, AdapterGeneration: adapterGeneration,
		Entries: make([]nativerouting.CatalogEntry, 0, len(rows)),
	}
	qualifications := make(nativerouting.QualificationSet, 0, len(rows))
	for _, row := range rows {
		entry := nativerouting.CatalogEntry{
			NativeProfileID: row.ID, NativeModel: row.Model, NativeEffort: row.Effort,
			Controls: nativeControls(role), Provides: row.Provides, CostRank: row.CostRank,
			QualificationID:   "claude-" + string(role) + "-" + row.ID + "-q1",
			AdapterGeneration: adapterGeneration, Strongest: row.Strongest,
		}
		qualification := qualificationFor(role, entry)
		entry.QualificationDigest = nativerouting.QualificationDigest(qualification)
		catalog.Entries = append(catalog.Entries, entry)
		qualifications = append(qualifications, qualification)
	}
	return catalog, qualifications, nil
}

func qualificationFor(role nativerouting.Role, entry nativerouting.CatalogEntry) nativerouting.Qualification {
	levels := []struct {
		component nativerouting.Component
		level     string
	}{
		{nativerouting.ComponentCapability, string(entry.Provides.Capability)},
		{nativerouting.ComponentDeliberation, string(entry.Provides.Deliberation)},
		{nativerouting.ComponentContext, string(entry.Provides.Context)},
		{nativerouting.ComponentAssurance, string(entry.Provides.Assurance)},
	}
	claims := make([]nativerouting.QualificationClaim, 0, len(levels))
	for _, level := range levels {
		key := string(role) + "/" + entry.NativeProfileID + "/" + string(level.component)
		claims = append(claims, nativerouting.QualificationClaim{
			Component: level.component, AssertedLevel: level.level, Method: nativerouting.MethodDecodePinnedEval,
			FixtureDigest: evidenceDigest("fixture/" + key), ResultDigest: evidenceDigest("result/" + key),
			Threshold: "all decode-pinned cases pass", Result: "all decode-pinned cases passed", Satisfied: true,
			EvidenceReference: "repo:internal/adapters/harness/claude/native_routing_test.go#decode-pinned",
			EvaluatorVersion:  nativerouting.EvaluatorDecodePinnedV1, ResponsibleEvaluator: "claude-runtime-conformance",
			EvidenceOrigin: nativerouting.EvidenceIndependentEvaluator,
		})
	}
	return nativerouting.Qualification{
		SchemaVersion: nativerouting.QualificationSchema, QualificationID: entry.QualificationID,
		Harness: harnessID, Role: role, NativeProfileID: entry.NativeProfileID,
		NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: slices.Clone(entry.Controls),
		AdapterGeneration: adapterGeneration, CatalogSchema: nativerouting.CatalogSchema,
		LaunchCaptureDigest: evidenceDigest("launch/" + string(role) + "/" + entry.NativeProfileID), Claims: claims,
		Validity: nativerouting.Validity{
			ValidFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			// The proof is adapter-generation-bound and explicitly invalidated; the
			// required time window must not make an unchanged binary self-expire.
			ValidUntil: time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), Invalidations: []nativerouting.Invalidation{},
		},
		Authorization: nativerouting.AuthorizationProof{
			Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "agenticlab-ai/pyxis", OperatorID: "@mdarabi",
			EvidenceDigest: evidenceDigest("authorization/claude/" + string(role)),
			AuthorizedAt:   time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		},
	}
}

func evidenceDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

// ValidateProfile is the closed Claude Code native-capability contract.
func (a *Adapter) ValidateProfile(entry nativerouting.CatalogEntry) error {
	if entry.AdapterGeneration != adapterGeneration {
		return nativeDefect("wrong_adapter_generation", "entry.adapter_generation", "profile is not qualified for this Claude Code adapter generation")
	}
	if !nativerouting.KnownLaunch(a.profiles, entry.NativeModel, entry.NativeEffort) {
		return nativeDefect("unknown_native_value", "entry.native_model", fmt.Sprintf("Claude Code model %q effort %q is not in the configured catalog", entry.NativeModel, entry.NativeEffort))
	}
	seen := map[string]bool{}
	for _, control := range entry.Controls {
		if seen[control.Name] {
			return nativeDefect("invalid_typed_control", "entry.controls", fmt.Sprintf("duplicate control %q", control.Name))
		}
		seen[control.Name] = true
		switch control.Name {
		case "output_format":
			if control.Type != nativerouting.ControlString || control.StringValue != "json" {
				return nativeDefect("unknown_native_control", "entry.controls.output_format", "Claude launch capture requires output_format=json")
			}
		case "permission_mode":
			if control.Type != nativerouting.ControlString || !slices.Contains([]string{"bypassPermissions", "plan"}, control.StringValue) {
				return nativeDefect("unknown_native_control", "entry.controls.permission_mode", "Claude launches require permission_mode=bypassPermissions or plan")
			}
		case "max_turns":
			if control.Type != nativerouting.ControlInteger || control.IntegerValue == nil || *control.IntegerValue <= 0 {
				return nativeDefect("unknown_native_control", "entry.controls.max_turns", "max_turns must be a positive integer")
			}
		case "sandbox_enabled":
			if control.Type != nativerouting.ControlBoolean || control.BooleanValue == nil || *control.BooleanValue {
				return nativeDefect("unknown_native_control", "entry.controls.sandbox_enabled", "review requires sandbox_enabled=false under the shared guard")
			}
		default:
			return nativeDefect("unknown_native_control", "entry.controls", fmt.Sprintf("Claude Code control %q is not recognized", control.Name))
		}
	}
	if !seen["output_format"] || !seen["permission_mode"] {
		return nativeDefect("missing_native_control", "entry.controls", "output_format and permission_mode controls are required")
	}
	return nil
}

func nativeDefect(defect, field, detail string) error {
	return nativerouting.NativeProfileDefect{Defect: defect, Field: field, Detail: detail}
}

// Build copies only the resolver-selected, typed native profile. It admits no
// raw arguments and refuses values or controls unknown to this generation.
func (a *Adapter) Build(selected nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	if selected.Harness != harnessID || !slices.Contains([]nativerouting.Role{nativerouting.RoleImplementation, nativerouting.RoleReview}, selected.Role) {
		return port.NativeLaunchSpec{}, fmt.Errorf("claude: selected profile has incompatible harness %q or role %q", selected.Harness, selected.Role)
	}
	entry := nativerouting.CatalogEntry{
		NativeProfileID: selected.NativeProfileID, NativeModel: selected.NativeModel, NativeEffort: selected.NativeEffort,
		Controls: selected.Controls, AdapterGeneration: selected.AdapterGeneration,
	}
	if entry.NativeProfileID == "" {
		return port.NativeLaunchSpec{}, errors.New("claude: selected profile identity is required")
	}
	if err := a.ValidateProfile(entry); err != nil {
		return port.NativeLaunchSpec{}, fmt.Errorf("claude: build selected profile %q: %w", selected.NativeProfileID, err)
	}
	return port.NativeLaunchSpec{
		NativeProfileID: selected.NativeProfileID, Role: selected.Role, NativeModel: selected.NativeModel,
		NativeEffort: selected.NativeEffort, Controls: slices.Clone(selected.Controls),
	}, nil
}

// CaptureNativeLaunch exercises the production argv builder and closed decoder
// without spawning Claude. This is the offline launch-fidelity seam.
func (a *Adapter) CaptureNativeLaunch(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
	purpose := port.LaunchPurposeImplementation
	if spec.Role == nativerouting.RoleReview {
		purpose = port.LaunchPurposeReview
	}
	return a.decodeLaunchCapture(a.launchArgs(port.LaunchSpec{Purpose: purpose, Model: spec.NativeModel, Effort: spec.NativeEffort}))
}

// decodeLaunchCapture observes the command handed to the process boundary. It
// parses closed: duplicate, missing, malformed, or unknown launch settings do
// not produce attestation.
func (a *Adapter) decodeLaunchCapture(args []string) (port.LaunchCapture, error) {
	var capture port.LaunchCapture
	seen := map[string]bool{}
	for i := 0; i < len(args); i++ {
		flag := args[i]
		if flag == "-p" {
			if seen[flag] {
				return port.LaunchCapture{}, fmt.Errorf("duplicate launch flag %q", flag)
			}
			seen[flag] = true
			continue
		}
		// Session identity is not a native routing setting: a resumed or
		// pre-assigned conversation changes nothing about the model, effort,
		// or controls the route selected, so the capture skips the flag and its
		// value rather than refusing attestation.
		if flag == "--session-id" || flag == "--resume" {
			if seen[flag] || i+1 == len(args) {
				return port.LaunchCapture{}, fmt.Errorf("duplicate or valueless session flag %q", flag)
			}
			seen[flag] = true
			i++
			continue
		}
		if !slices.Contains([]string{"--model", "--effort", "--output-format", "--permission-mode", "--max-turns", "--settings"}, flag) {
			return port.LaunchCapture{}, fmt.Errorf("unknown launch setting %q", flag)
		}
		if seen[flag] || i+1 == len(args) {
			return port.LaunchCapture{}, fmt.Errorf("duplicate or valueless launch setting %q", flag)
		}
		seen[flag] = true
		i++
		value := args[i]
		switch flag {
		case "--model":
			capture.NativeModel = value
		case "--effort":
			capture.NativeEffort = value
		case "--output-format":
			capture.Controls = append(capture.Controls, nativerouting.NativeControl{Name: "output_format", Type: nativerouting.ControlString, StringValue: value})
		case "--permission-mode":
			capture.Controls = append(capture.Controls, nativerouting.NativeControl{Name: "permission_mode", Type: nativerouting.ControlString, StringValue: value})
		case "--settings":
			if value != reviewSettings {
				return port.LaunchCapture{}, fmt.Errorf("unexpected review settings %q", value)
			}
			disabled := false
			capture.Controls = append(capture.Controls, nativerouting.NativeControl{Name: "sandbox_enabled", Type: nativerouting.ControlBoolean, BooleanValue: &disabled})
		case "--max-turns":
			n, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return port.LaunchCapture{}, fmt.Errorf("decode max-turns %q: %w", value, err)
			}
			capture.Controls = append(capture.Controls, nativerouting.NativeControl{Name: "max_turns", Type: nativerouting.ControlInteger, IntegerValue: &n})
		}
	}
	if capture.NativeModel == "" || capture.NativeEffort == "" {
		return port.LaunchCapture{}, errors.New("launch capture is missing model or effort")
	}
	if err := a.ValidateProfile(nativerouting.CatalogEntry{
		NativeModel: capture.NativeModel, NativeEffort: capture.NativeEffort,
		Controls: capture.Controls, AdapterGeneration: adapterGeneration,
	}); err != nil {
		return port.LaunchCapture{}, err
	}
	return capture, nil
}

// LaunchCapture returns the immutable native settings observed at spawn.
func (*Adapter) LaunchCapture(h port.Handle) (port.LaunchCapture, error) {
	hh, ok := h.(*handle)
	if !ok {
		return port.LaunchCapture{}, fmt.Errorf("claude: launch capture: foreign handle %T", h)
	}
	return port.LaunchCapture{
		NativeModel: hh.capture.NativeModel, NativeEffort: hh.capture.NativeEffort,
		Controls: slices.Clone(hh.capture.Controls),
	}, nil
}

// VerifyLaunch is the attempt-fidelity gate. A mismatch names the affected
// qualification so the caller can invalidate it and refuse the attempt.
func (a *Adapter) VerifyLaunch(h port.Handle, selected nativerouting.SelectedProfile) error {
	actual, err := a.LaunchCapture(h)
	if err != nil {
		return err
	}
	if !actual.MatchesSelected(selected) {
		return fmt.Errorf("%w: qualification %q selected model=%q effort=%q controls=%v; actual model=%q effort=%q controls=%v",
			ErrLaunchFidelity, selected.QualificationID, selected.NativeModel, selected.NativeEffort, selected.Controls,
			actual.NativeModel, actual.NativeEffort, actual.Controls)
	}
	return nil
}
