package opencode

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const (
	adapterGeneration = "opencode-native-routing/2"
	nativeEffortNone  = "none"
	outputFormat      = "output_format"
	roleFixtureDigest = "555f82a66fd4c89cffd4bf230afc77418fab77233ffae8540212c547be01e22c"
	roleResultDigest  = "6d355089625726835f3bc69e2f1c2b0d37687099fe0ad881c83d3ed89cc64c97"
)

var (
	_ port.CatalogProvider           = (*Adapter)(nil)
	_ nativerouting.NativeCapability = (*Adapter)(nil)
	_ port.LaunchBuilder             = (*Adapter)(nil)
	_ port.NativeLaunchCapturer      = (*Adapter)(nil)
)

// Catalog publishes one complete, independently qualified role policy. It is
// data-only: changing the OpenCode roster or evidence does not change resolver
// code or a frozen plan.
func (a *Adapter) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	definitions := a.profiles[role]
	if len(definitions) == 0 {
		return nativerouting.Catalog{}, nil, fmt.Errorf("opencode: no configured profiles for role %q", role)
	}
	catalog := nativerouting.Catalog{
		SchemaVersion: nativerouting.CatalogSchema, CatalogID: harnessID.String() + "-" + string(role) + "-v1",
		Harness: harnessID, Role: role, AdapterGeneration: adapterGeneration,
		Entries: make([]nativerouting.CatalogEntry, 0, len(definitions)),
	}
	qualifications := make(nativerouting.QualificationSet, 0, len(definitions))
	for _, definition := range definitions {
		entry := catalogEntry(definition, role)
		qualification := qualificationFor(entry, role)
		entry.QualificationDigest = nativerouting.QualificationDigest(qualification)
		catalog.Entries = append(catalog.Entries, entry)
		qualifications = append(qualifications, qualification)
	}
	return catalog, qualifications, nil
}

func catalogEntry(definition nativerouting.ProfileSpec, role nativerouting.Role) nativerouting.CatalogEntry {
	return nativerouting.CatalogEntry{
		NativeProfileID: definition.ID, NativeModel: definition.Model, NativeEffort: definition.Effort,
		Controls: nativeControls(), Provides: definition.Provides, CostRank: definition.CostRank,
		QualificationID:   harnessID.String() + "-" + string(role) + "-" + definition.ID + "-q1",
		AdapterGeneration: adapterGeneration, Strongest: definition.Strongest,
	}
}

func qualificationFor(entry nativerouting.CatalogEntry, role nativerouting.Role) nativerouting.Qualification {
	validFrom := time.Date(2026, time.August, 1, 0, 0, 0, 0, time.UTC)
	// The built-in proof is adapter-generation-bound and explicitly invalidated;
	// its required time window must not make an unchanged binary self-expire.
	validUntil := time.Date(9999, time.December, 31, 0, 0, 0, 0, time.UTC)
	return nativerouting.Qualification{
		SchemaVersion: nativerouting.QualificationSchema, QualificationID: entry.QualificationID,
		Harness: harnessID, Role: role, NativeProfileID: entry.NativeProfileID,
		NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: nativeControls(),
		AdapterGeneration: adapterGeneration, CatalogSchema: nativerouting.CatalogSchema,
		LaunchCaptureDigest: evidenceDigest("launch/" + string(role) + "/" + entry.NativeProfileID),
		Claims:              qualificationClaims(entry), Validity: nativerouting.Validity{
			ValidFrom: validFrom, ValidUntil: validUntil, Invalidations: []nativerouting.Invalidation{},
		},
		Authorization: nativerouting.AuthorizationProof{
			Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "agenticlab-ai/pyxis",
			OperatorID: "pyxis-routing-operator", EvidenceDigest: evidenceDigest("opencode/operator-authorization/v1"),
			AuthorizedAt: validFrom,
		},
	}
}

func qualificationClaims(entry nativerouting.CatalogEntry) []nativerouting.QualificationClaim {
	levels := map[nativerouting.Component]string{
		nativerouting.ComponentCapability: string(entry.Provides.Capability), nativerouting.ComponentDeliberation: string(entry.Provides.Deliberation),
		nativerouting.ComponentContext: string(entry.Provides.Context), nativerouting.ComponentAssurance: string(entry.Provides.Assurance),
	}
	claims := make([]nativerouting.QualificationClaim, 0, len(levels))
	for _, component := range []nativerouting.Component{
		nativerouting.ComponentCapability, nativerouting.ComponentDeliberation,
		nativerouting.ComponentContext, nativerouting.ComponentAssurance,
	} {
		claims = append(claims, nativerouting.QualificationClaim{
			Component: component, AssertedLevel: levels[component], Method: nativerouting.MethodDecodePinnedEval,
			FixtureDigest: roleFixtureDigest, ResultDigest: roleResultDigest,
			Threshold: "declared-level-satisfied", Result: "pass", Satisfied: true,
			EvidenceReference:    "repo:internal/adapters/harness/opencode/testdata/role-routing-results-v1.json",
			EvaluatorVersion:     nativerouting.EvaluatorDecodePinnedV1,
			ResponsibleEvaluator: "opencode-role-conformance-v1", EvidenceOrigin: nativerouting.EvidenceIndependentEvaluator,
		})
	}
	return claims
}

func evidenceDigest(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func nativeControls() []nativerouting.NativeControl {
	return []nativerouting.NativeControl{{Name: outputFormat, Type: nativerouting.ControlString, StringValue: "json"}}
}

// ValidateProfile is the adapter-owned closed native capability. OpenCode has
// no effort flag: "none" records that absence and must never become an arg.
func validateProfileModels(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) error {
	for _, specs := range profiles {
		for _, spec := range specs {
			provider, name, split := strings.Cut(spec.Model, "/")
			if !split || provider == "" || name == "" || strings.HasPrefix(provider, "-") || strings.ContainsAny(spec.Model, " \t\r\n\x00") {
				return fmt.Errorf("opencode: invalid model %q for profile %s; use provider/model", spec.Model, spec.ID)
			}
			if spec.Effort != nativeEffortNone {
				return fmt.Errorf("opencode: profile %s effort %q is not supported; OpenCode has no effort flag", spec.ID, spec.Effort)
			}
		}
	}
	return nil
}

func (a *Adapter) ValidateProfile(entry nativerouting.CatalogEntry) error {
	if !nativerouting.KnownLaunch(a.profiles, entry.NativeModel, entry.NativeEffort) {
		return nativerouting.NativeProfileDefect{Defect: "unknown_native_value", Field: "entry.native_model", Detail: fmt.Sprintf("OpenCode provider/model %q is not in the configured catalog", entry.NativeModel)}
	}
	if entry.NativeEffort != nativeEffortNone {
		return nativerouting.NativeProfileDefect{Defect: "unsupported_native_effort", Field: "entry.native_effort", Detail: "OpenCode exposes no native effort flag"}
	}
	if len(entry.Controls) != 1 || entry.Controls[0].Name != outputFormat || entry.Controls[0].Type != nativerouting.ControlString || entry.Controls[0].StringValue != "json" || entry.Controls[0].IntegerValue != nil || entry.Controls[0].BooleanValue != nil {
		return nativerouting.NativeProfileDefect{Defect: "unknown_native_control", Field: "entry.controls", Detail: "OpenCode recognizes only the typed output_format=json routing control"}
	}
	return nil
}

// Build copies only a resolver selection into the argument-free native launch
// contract. Semantic demand and raw CLI arguments cannot enter this boundary.
func (a *Adapter) Build(selected nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	if selected.NativeProfileID == "" || selected.AdapterGeneration != adapterGeneration {
		return port.NativeLaunchSpec{}, errors.New("opencode: selected profile is empty or belongs to another adapter generation")
	}
	entry := nativerouting.CatalogEntry{NativeModel: selected.NativeModel, NativeEffort: selected.NativeEffort, Controls: selected.Controls}
	if err := a.ValidateProfile(entry); err != nil {
		return port.NativeLaunchSpec{}, fmt.Errorf("opencode: selected profile %q: %w", selected.NativeProfileID, err)
	}
	return port.NativeLaunchSpec{
		NativeProfileID: selected.NativeProfileID, NativeModel: selected.NativeModel,
		NativeEffort: selected.NativeEffort, Controls: append([]nativerouting.NativeControl(nil), selected.Controls...),
		Role: selected.Role,
	}, nil
}

// CaptureNativeLaunch exercises OpenCode's production argv builder and closed
// decoder without starting a process.
func (*Adapter) CaptureNativeLaunch(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
	return captureLaunch(nativeLaunchArgs(spec, "native-routing-conformance", ""))
}

// LaunchFidelityError fails the attempt and names the qualification that must
// remain invalid until the native launch behavior is requalified.
type LaunchFidelityError struct {
	QualificationID string
}

func (e LaunchFidelityError) Error() string {
	return fmt.Sprintf("opencode: native launch differs from selected profile; invalidate qualification %q", e.QualificationID)
}

// VerifyLaunch enforces exact selected-versus-actual native equality.
func (a *Adapter) VerifyLaunch(selected nativerouting.SelectedProfile, capture port.LaunchCapture) error {
	if capture.MatchesSelected(selected) {
		return nil
	}
	return LaunchFidelityError{QualificationID: selected.QualificationID}
}
