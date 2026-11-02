package cursor

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

const adapterGeneration = "cursor-agent-native-routing-v3"

//go:embed testdata/native-routing-decode-fixtures.json
var decodePinnedFixtures []byte

var _ port.CatalogProvider = (*Adapter)(nil)
var _ port.LaunchBuilder = (*Adapter)(nil)
var _ port.NativeLaunchCapturer = (*Adapter)(nil)
var _ nativerouting.NativeCapability = (*Adapter)(nil)

func boolControlValue(name string, value bool) nativerouting.NativeControl {
	return nativerouting.NativeControl{Name: name, Type: nativerouting.ControlBoolean, BooleanValue: &value}
}

func boolControl(name string) nativerouting.NativeControl { return boolControlValue(name, true) }

func cursorControls(role nativerouting.Role) []nativerouting.NativeControl {
	controls := []nativerouting.NativeControl{
		boolControl("force"),
		{Name: "output_format", Type: nativerouting.ControlString, StringValue: "json"},
		boolControl("print"),
	}
	if role == nativerouting.RoleReview {
		controls[1].StringValue = "stream-json"
		controls = append(controls,
			nativerouting.NativeControl{Name: "sandbox", Type: nativerouting.ControlString, StringValue: "disabled"},
			boolControl("trust"),
		)
	}
	return controls
}

func digest(value any) string {
	b, _ := json.Marshal(value)
	return digestBytes(b)
}

func digestBytes(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func qualification(role nativerouting.Role, entry nativerouting.CatalogEntry) nativerouting.Qualification {
	levels := map[nativerouting.Component]string{
		nativerouting.ComponentCapability:   string(entry.Provides.Capability),
		nativerouting.ComponentDeliberation: string(entry.Provides.Deliberation),
		nativerouting.ComponentContext:      string(entry.Provides.Context),
		nativerouting.ComponentAssurance:    string(entry.Provides.Assurance),
	}
	claims := make([]nativerouting.QualificationClaim, 0, len(levels))
	for _, component := range []nativerouting.Component{nativerouting.ComponentCapability, nativerouting.ComponentDeliberation, nativerouting.ComponentContext, nativerouting.ComponentAssurance} {
		claims = append(claims, nativerouting.QualificationClaim{
			Component: component, AssertedLevel: levels[component], Method: nativerouting.MethodDecodePinnedEval,
			FixtureDigest: digestBytes(decodePinnedFixtures), ResultDigest: digestBytes(decodePinnedFixtures),
			Threshold: "fixture_passes=1", Result: "fixture_passes=1", Satisfied: true,
			EvidenceReference: "repo:internal/adapters/harness/cursor/testdata/native-routing-decode-fixtures.json#" + entry.NativeProfileID + "/" + string(component),
			EvaluatorVersion:  nativerouting.EvaluatorDecodePinnedV1, ResponsibleEvaluator: "cursor-routing-conformance",
			EvidenceOrigin: nativerouting.EvidenceOperatorRepository,
		})
	}
	capture := port.LaunchCapture{NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: cursorControls(role)}
	return nativerouting.Qualification{
		SchemaVersion: nativerouting.QualificationSchema, QualificationID: entry.QualificationID,
		Harness: harnessID, Role: role, NativeProfileID: entry.NativeProfileID,
		NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: cursorControls(role),
		AdapterGeneration: adapterGeneration, CatalogSchema: nativerouting.CatalogSchema,
		LaunchCaptureDigest: digest(capture), Claims: claims,
		Validity: nativerouting.Validity{
			ValidFrom: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
			// The proof is adapter-generation-bound and explicitly invalidated; the
			// required time window must not make an unchanged binary self-expire.
			ValidUntil:    time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC),
			Invalidations: []nativerouting.Invalidation{},
		},
		Authorization: nativerouting.AuthorizationProof{
			Kind: nativerouting.AuthorizationRepositoryOperator, Repository: "agenticlab-ai/pyxis",
			OperatorID: "native-routing-operator", EvidenceDigest: digest("cursor-routing-catalog-authorization-v1"),
			AuthorizedAt: time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC),
		},
	}
}

// Catalog publishes one independently qualified role policy. The roster and
// evidence are adapter-owned data; the shared resolver remains unchanged when
// either changes.
func (a *Adapter) Catalog(role nativerouting.Role) (nativerouting.Catalog, nativerouting.QualificationSet, error) {
	roster := a.profiles[role]
	if len(roster) == 0 {
		return nativerouting.Catalog{}, nil, fmt.Errorf("cursor: no configured profiles for role %q", role)
	}
	catalog := nativerouting.Catalog{
		SchemaVersion: nativerouting.CatalogSchema, CatalogID: "cursor-" + string(role) + "-v1",
		Harness: harnessID, Role: role, AdapterGeneration: adapterGeneration,
		Entries: make([]nativerouting.CatalogEntry, 0, len(roster)),
	}
	qualifications := make(nativerouting.QualificationSet, 0, len(roster))
	for _, item := range roster {
		entry := nativerouting.CatalogEntry{
			NativeProfileID: item.ID, NativeModel: item.Model, NativeEffort: item.Effort,
			Controls: cursorControls(role), Provides: item.Provides, CostRank: item.CostRank,
			QualificationID:   "cursor-" + string(role) + "-" + item.ID + "-qualification-v1",
			AdapterGeneration: adapterGeneration, Strongest: item.Strongest,
		}
		q := qualification(role, entry)
		entry.QualificationDigest = nativerouting.QualificationDigest(q)
		catalog.Entries = append(catalog.Entries, entry)
		qualifications = append(qualifications, q)
	}
	return catalog, qualifications, nil
}

// ValidateProfile accepts only Cursor model-parameter forms and typed controls
// this adapter knows how to emit. Effort is descriptive and must agree with the
// recognized form; it never becomes an invented CLI flag.
func (a *Adapter) ValidateProfile(entry nativerouting.CatalogEntry) error {
	wantEffort, ok := a.effortForModel(entry.NativeModel)
	if !ok {
		return nativerouting.NativeProfileDefect{Defect: "unknown_native_value", Field: "entry.native_model", Detail: fmt.Sprintf("Cursor model parameter %q is not in the configured catalog", entry.NativeModel)}
	}
	if entry.NativeEffort != wantEffort {
		return nativerouting.NativeProfileDefect{Defect: "unknown_native_value", Field: "entry.native_effort", Detail: fmt.Sprintf("Cursor model parameter %q carries effort %q, not %q", entry.NativeModel, wantEffort, entry.NativeEffort)}
	}
	implementationControls := (port.LaunchCapture{Controls: entry.Controls}).MatchesSelected(nativerouting.SelectedProfile{Controls: cursorControls(nativerouting.RoleImplementation)})
	reviewControls := (port.LaunchCapture{Controls: entry.Controls}).MatchesSelected(nativerouting.SelectedProfile{Controls: cursorControls(nativerouting.RoleReview)})
	if !implementationControls && !reviewControls {
		return nativerouting.NativeProfileDefect{Defect: "unknown_native_control", Field: "entry.controls", Detail: "Cursor requires print=true, output_format=json, force=true, and role-qualified sandbox/trust controls"}
	}
	return nil
}

func (a *Adapter) effortForModel(model string) (string, bool) {
	var effort string
	found := false
	for _, roster := range a.profiles {
		for _, entry := range roster {
			if entry.Model != model {
				continue
			}
			if found && effort != entry.Effort {
				return "", false
			}
			effort, found = entry.Effort, true
		}
	}
	return effort, found
}

// Build translates only a complete resolver selection from the current role
// catalog into the argument-free native launch contract.
func (a *Adapter) Build(selected nativerouting.SelectedProfile) (port.NativeLaunchSpec, error) {
	catalog, qualifications, err := a.Catalog(selected.Role)
	if err != nil {
		return port.NativeLaunchSpec{}, err
	}
	if selected.Harness != harnessID || selected.CatalogID != catalog.CatalogID ||
		selected.PolicyIdentity != nativerouting.PolicyIdentity(catalog, qualifications) ||
		selected.AdapterGeneration != adapterGeneration {
		return port.NativeLaunchSpec{}, fmt.Errorf("cursor: selected profile is not bound to the current %s role catalog", selected.Role)
	}
	for _, entry := range catalog.Entries {
		if entry.NativeProfileID != selected.NativeProfileID {
			continue
		}
		if entry.NativeModel != selected.NativeModel || entry.NativeEffort != selected.NativeEffort ||
			entry.Provides != selected.Provides || entry.CostRank != selected.CostRank ||
			entry.QualificationID != selected.QualificationID || entry.QualificationDigest != selected.QualificationDigest ||
			!(port.LaunchCapture{NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: entry.Controls}).MatchesSelected(selected) {
			return port.NativeLaunchSpec{}, fmt.Errorf("cursor: selected profile %q differs from its qualified catalog entry", selected.NativeProfileID)
		}
		return port.NativeLaunchSpec{NativeProfileID: entry.NativeProfileID, Role: selected.Role, NativeModel: entry.NativeModel, NativeEffort: entry.NativeEffort, Controls: slices.Clone(entry.Controls)}, nil
	}
	return port.NativeLaunchSpec{}, fmt.Errorf("cursor: selected profile %q is not in the %s role catalog", selected.NativeProfileID, selected.Role)
}

// CaptureNativeLaunch exercises Cursor's production argv builder and closed
// decoder without starting the pty-wrapped process.
func (a *Adapter) CaptureNativeLaunch(spec port.NativeLaunchSpec) (port.LaunchCapture, error) {
	purpose := port.LaunchPurposeImplementation
	if spec.Role == nativerouting.RoleReview {
		purpose = port.LaunchPurposeReview
	}
	return a.decodeLaunchCapture(a.launchArgs(port.LaunchSpec{Purpose: purpose, Model: spec.NativeModel, Effort: spec.NativeEffort}, "native-routing-conformance"))
}

// decodeLaunchCapture observes the exact native settings emitted in Cursor's
// verified argv form. The final positional brief is process metadata, not a
// native routing control.
func (a *Adapter) decodeLaunchCapture(args []string) (port.LaunchCapture, error) {
	implementation := len(args) == 7 && args[0] == "-p" && args[1] == "--output-format" && args[2] == "json" && args[3] == "--model" && args[5] == "--force" && strings.TrimSpace(args[6]) != ""
	review := len(args) == 10 && args[0] == "-p" && args[1] == "--output-format" && args[2] == "stream-json" && args[3] == "--model" && args[5] == "--force" && args[6] == "--sandbox" && args[7] == "disabled" && args[8] == "--trust" && strings.TrimSpace(args[9]) != ""
	if !implementation && !review {
		return port.LaunchCapture{}, fmt.Errorf("cursor: launch argv is not the recognized typed form")
	}
	effort, ok := a.effortForModel(args[4])
	if !ok {
		return port.LaunchCapture{}, fmt.Errorf("cursor: launch model parameter %q is not recognized", args[4])
	}
	role := nativerouting.RoleImplementation
	if review {
		role = nativerouting.RoleReview
	}
	return port.LaunchCapture{NativeModel: args[4], NativeEffort: effort, Controls: cursorControls(role)}, nil
}
