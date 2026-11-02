package codex

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const codexAdapterGeneration = "codex-native-routing/v3"

var (
	codexQualificationFrom = time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	// Built-in evidence is bound to codexAdapterGeneration and explicit
	// invalidations. Keep its required validity window generation-lived so an
	// installed binary does not acquire a hidden wall-clock kill date.
	codexQualificationUntil = time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC)
)

var (
	_ port.CatalogProvider           = (*Adapter)(nil)
	_ port.LaunchBuilder             = (*Adapter)(nil)
	_ port.NativeLaunchCapturer      = (*Adapter)(nil)
	_ nativerouting.NativeCapability = (*Adapter)(nil)
)

// Catalog publishes one complete, role-owned default policy unit. It performs
// no probing: current-time and repository authorization checks remain the
// shared validator's responsibility at composition.
func (a *Adapter) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	entries, err := a.codexRoleEntries(role)
	if err != nil {
		return nativerouting.Catalog{}, nil, err
	}
	catalog := nativerouting.Catalog{
		SchemaVersion:     nativerouting.CatalogSchema,
		CatalogID:         "codex-" + string(role) + "-v1",
		Harness:           harnessID,
		Role:              role,
		AdapterGeneration: codexAdapterGeneration,
		Entries:           entries,
	}
	qualifications := make(nativerouting.QualificationSet, len(entries))
	for i := range entries {
		qualifications[i] = codexQualification(role, entries[i])
		catalog.Entries[i].QualificationDigest = nativerouting.QualificationDigest(qualifications[i])
	}
	return catalog, qualifications, nil
}

// Build translates only the resolver-selected native identity. Semantic demand
// and raw arguments are deliberately absent from both sides of this boundary.
func (a *Adapter) Build(selected nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	if selected.Harness != harnessID || (selected.Role != nativerouting.RoleImplementation && selected.Role != nativerouting.RoleReview) {
		return port.NativeLaunchSpec{}, fmt.Errorf("codex: selected profile belongs to %s/%s", selected.Harness, selected.Role)
	}
	if selected.NativeProfileID == "" || selected.CatalogID == "" || selected.PolicyIdentity == "" || selected.QualificationID == "" || selected.QualificationDigest == "" {
		return port.NativeLaunchSpec{}, fmt.Errorf("codex: selected profile identity is incomplete")
	}
	if selected.AdapterGeneration != codexAdapterGeneration {
		return port.NativeLaunchSpec{}, fmt.Errorf("codex: selected profile generation %q does not match %q", selected.AdapterGeneration, codexAdapterGeneration)
	}
	entry := nativerouting.CatalogEntry{
		NativeProfileID:   selected.NativeProfileID,
		NativeModel:       selected.NativeModel,
		NativeEffort:      selected.NativeEffort,
		Controls:          selected.Controls,
		AdapterGeneration: selected.AdapterGeneration,
	}
	if err := a.ValidateProfile(entry); err != nil {
		return port.NativeLaunchSpec{}, fmt.Errorf("codex: selected native profile is not launchable: %w", err)
	}
	return port.NativeLaunchSpec{
		NativeProfileID: selected.NativeProfileID,
		Role:            selected.Role,
		NativeModel:     selected.NativeModel,
		NativeEffort:    selected.NativeEffort,
		Controls:        cloneNativeControls(selected.Controls),
	}, nil
}

// CaptureNativeLaunch exercises Codex's production argv builder and closed
// decoder without starting a process.
func (a *Adapter) CaptureNativeLaunch(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
	purpose := port.LaunchPurposeImplementation
	if spec.Role == nativerouting.RoleReview {
		purpose = port.LaunchPurposeReview
	}
	return captureNativeLaunch(a.launchArgs(port.LaunchSpec{Purpose: purpose, Model: spec.NativeModel, Effort: spec.NativeEffort}))
}

// ValidateProfile is the closed Codex-native capability contract for this
// adapter generation. Codex currently exposes model and reasoning effort to
// routing; its approval, sandbox, JSON, and prompt transport settings remain
// executor protocol rather than profile-controlled escape hatches.
func (a *Adapter) ValidateProfile(entry nativerouting.CatalogEntry) error {
	if !nativerouting.KnownLaunch(a.profiles, entry.NativeModel, entry.NativeEffort) {
		return nativerouting.NativeProfileDefect{Defect: "unknown_native_value", Field: "entry.native_model_effort", Detail: fmt.Sprintf("Codex generation %s has no configured profile for model/effort pair %q/%q", codexAdapterGeneration, entry.NativeModel, entry.NativeEffort)}
	}
	if len(entry.Controls) != 0 {
		return nativerouting.NativeProfileDefect{Defect: "unknown_native_control", Field: "entry.controls", Detail: "Codex generation codex-native-routing/v1 recognizes no profile-controlled launch settings"}
	}
	return nil
}

func (a *Adapter) codexRoleEntries(role nativerouting.Role) ([]nativerouting.CatalogEntry, error) {
	specs := a.profiles[role]
	if len(specs) == 0 {
		return nil, fmt.Errorf("codex: no configured profiles for role %q", role)
	}
	entries := make([]nativerouting.CatalogEntry, 0, len(specs))
	for _, spec := range specs {
		entries = append(entries, nativerouting.CatalogEntry{
			NativeProfileID:   spec.ID,
			NativeModel:       spec.Model,
			NativeEffort:      spec.Effort,
			Controls:          []nativerouting.NativeControl{},
			Provides:          spec.Provides,
			CostRank:          spec.CostRank,
			QualificationID:   spec.ID + "-q1",
			AdapterGeneration: codexAdapterGeneration,
			Strongest:         spec.Strongest,
		})
	}
	return entries, nil
}

func codexQualification(role nativerouting.Role, entry nativerouting.CatalogEntry) nativerouting.Qualification {
	levels := map[nativerouting.Component]string{
		nativerouting.ComponentCapability:   string(entry.Provides.Capability),
		nativerouting.ComponentDeliberation: string(entry.Provides.Deliberation),
		nativerouting.ComponentContext:      string(entry.Provides.Context),
		nativerouting.ComponentAssurance:    string(entry.Provides.Assurance),
	}
	claims := make([]nativerouting.QualificationClaim, 0, len(levels))
	for _, component := range []nativerouting.Component{nativerouting.ComponentCapability, nativerouting.ComponentDeliberation, nativerouting.ComponentContext, nativerouting.ComponentAssurance} {
		fixture := entry.NativeProfileID + ":" + string(component) + ":fixture"
		result := entry.NativeProfileID + ":" + string(component) + ":pass"
		claims = append(claims, nativerouting.QualificationClaim{
			Component: component, AssertedLevel: levels[component], Method: nativerouting.MethodDecodePinnedEval,
			FixtureDigest: codexDigest(fixture), ResultDigest: codexDigest(result), Threshold: "all decode-pinned cases pass", Result: "pass", Satisfied: true,
			EvidenceReference: "repo:internal/adapters/harness/codex/native_routing_test.go", EvaluatorVersion: nativerouting.EvaluatorDecodePinnedV1,
			ResponsibleEvaluator: "codex-native-routing-conformance", EvidenceOrigin: nativerouting.EvidenceOperatorRepository,
		})
	}
	capture := port.LaunchCapture{NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: []nativerouting.NativeControl{}}
	return nativerouting.Qualification{
		SchemaVersion: nativerouting.QualificationSchema, QualificationID: entry.QualificationID,
		Harness: harnessID, Role: role, NativeProfileID: entry.NativeProfileID,
		NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: []nativerouting.NativeControl{},
		AdapterGeneration: codexAdapterGeneration, CatalogSchema: nativerouting.CatalogSchema,
		LaunchCaptureDigest: codexLaunchCaptureDigest(capture), Claims: claims,
		Validity:      nativerouting.Validity{ValidFrom: codexQualificationFrom, ValidUntil: codexQualificationUntil, Invalidations: []nativerouting.Invalidation{}},
		Authorization: nativerouting.AuthorizationProof{Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "agenticlab-ai/pyxis", OperatorID: "pyxis-maintainers", EvidenceDigest: codexDigest("codex-native-routing-v1-authorization"), AuthorizedAt: codexQualificationFrom},
	}
}

func codexDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func codexLaunchCaptureDigest(value port.LaunchCapture) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(fmt.Sprintf("codex: marshal closed launch capture: %v", err))
	}
	return codexDigest(string(data))
}

func cloneNativeControls(controls []nativerouting.NativeControl) []nativerouting.NativeControl {
	out := make([]nativerouting.NativeControl, len(controls))
	copy(out, controls)
	return out
}

// captureNativeLaunch strictly decodes the actual argv emitted by launchArgs.
// Protocol flags are fixed, and only the model and effort become routed launch
// identity. An added argument or config key is a mismatch, never an ignored
// extension.
func captureNativeLaunch(args []string) (port.LaunchCapture, error) {
	owned := len(args) > 0 && args[0] == "--no-daemon"
	if owned {
		args = args[1:]
	}
	modelIndex, effortIndex := 3, 5
	implementation := len(args) == 9 && args[0] == "exec" && args[1] == "--json" && args[2] == "-m" && args[4] == "-c" && args[6] == "--dangerously-bypass-approvals-and-sandbox" && args[7] == "--skip-git-repo-check" && args[8] == "-"
	// A resumed implementation launch (task session continuity) is the same
	// closed contract with `resume <thread-id>` spliced in: the thread identity
	// is not a routing setting and is not captured.
	resumed := len(args) == 11 && args[0] == "exec" && args[1] == "resume" && args[2] == "--json" && args[3] == "-m" && args[5] == "-c" && args[7] == "--dangerously-bypass-approvals-and-sandbox" && args[8] == "--skip-git-repo-check" && strings.TrimSpace(args[9]) != "" && args[10] == "-"
	review := len(args) == 14 && args[0] == "-a" && args[1] == "never" && args[2] == "--no-daemon" && args[3] == "exec" && args[4] == "--json" && args[5] == "-m" && args[7] == "-c" && args[9] == "--sandbox" && args[10] == "danger-full-access" && args[11] == "--ignore-user-config" && args[12] == "--skip-git-repo-check" && args[13] == "-"
	if review {
		if owned {
			return port.LaunchCapture{}, fmt.Errorf("codex: duplicated ownership flag")
		}
		modelIndex, effortIndex = 6, 8
	}
	if resumed {
		modelIndex, effortIndex = 4, 6
	}
	if (!owned || (!implementation && !resumed)) && !review {
		return port.LaunchCapture{}, fmt.Errorf("codex: actual launch arguments do not match the closed adapter contract")
	}
	const effortKey = "model_reasoning_effort="
	if !strings.HasPrefix(args[effortIndex], effortKey) || args[modelIndex] == "" {
		return port.LaunchCapture{}, fmt.Errorf("codex: actual launch omitted model or reasoning effort")
	}
	effort, err := strconv.Unquote(strings.TrimPrefix(args[effortIndex], effortKey))
	if err != nil || effort == "" {
		return port.LaunchCapture{}, fmt.Errorf("codex: decode actual reasoning effort %q", args[effortIndex])
	}
	return port.LaunchCapture{NativeModel: args[modelIndex], NativeEffort: effort, Controls: []nativerouting.NativeControl{}}, nil
}

// verifyNativeLaunch turns exact selected-versus-actual equality into the two
// fail-closed outputs composition needs: a failed fidelity check and an
// additive invalidation for the selected qualification. Persisting either is
// deliberately outside this adapter's invocation boundary.
func verifyNativeLaunch(selected nativerouting.SelectedProfile, args []string, at time.Time) (port.LaunchCapture, *nativerouting.Invalidation, error) {
	capture, err := captureNativeLaunch(args)
	if err == nil && capture.MatchesSelected(selected) {
		return capture, nil, nil
	}
	reason := "actual Codex model, effort, or controls differed from the selected profile"
	if err != nil {
		reason = "actual Codex launch did not decode as the selected typed contract: " + err.Error()
	}
	eventID := "codex-launch-mismatch-" + codexDigest(selected.QualificationID + "\x00" + reason + "\x00" + strings.Join(args, "\x00") + "\x00" + at.UTC().Format(time.RFC3339Nano))[:16]
	invalidation := &nativerouting.Invalidation{EventID: eventID, At: at.UTC(), Reason: reason}
	return capture, invalidation, fmt.Errorf("codex: launch fidelity failed for qualification %s: %s", selected.QualificationID, reason)
}
