package reconcile

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"path"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

var (
	// ErrFeatureIncomplete means a feature cannot yet be sealed without
	// discarding source information or claiming an association memory has not
	// made explicitly.
	ErrFeatureIncomplete = errors.New("feature is not complete in memory")
	// ErrInvalidCloseout identifies a malformed, non-canonical, or internally
	// inconsistent closeout receipt.
	ErrInvalidCloseout = errors.New("invalid memory closeout receipt")
)

// CloseoutSourceDocument binds one lossless source capture. RecordSHA256 is
// the exact immutable JSON envelope; BodySHA256 is the exact authored body.
type CloseoutSourceDocument struct {
	Path         string `json:"path"`
	DocumentID   string `json:"document_id"`
	RecordSHA256 string `json:"record_sha256"`
	BodySHA256   string `json:"body_sha256"`
}

// CloseoutNativeRecord binds one active, semantic memory record selected for
// the feature. Literal source captures and reconciliation scaffolding live in
// their own classes and are deliberately absent here.
type CloseoutNativeRecord struct {
	ID      string `json:"id"`
	Kind    string `json:"kind"`
	Subject string `json:"subject"`
	Family  string `json:"family,omitempty"`
	SHA256  string `json:"sha256"`
}

// CloseoutCodeUnit binds one production code identity from the pinned tree.
type CloseoutCodeUnit struct {
	Kind          codeinventory.UnitKind `json:"kind"`
	Path          string                 `json:"path"`
	Files         []string               `json:"files"`
	SHA256        string                 `json:"sha256"`
	MemoryRecords []string               `json:"memory_records"`
}

// CloseoutPRD binds the exact result of replaying the feature's PRD ledger.
type CloseoutPRD struct {
	Feature         string `json:"feature"`
	Source          string `json:"source"`
	AsOf            string `json:"as_of"`
	Fingerprint     string `json:"content_fingerprint"`
	SourceMapSHA256 string `json:"source_map_sha256"`
}

// CloseoutDesign binds one live technical design selected only through an
// exact Serves declaration or an active Decision's serves edge.
type CloseoutDesign struct {
	Slug   string `json:"slug"`
	Source string `json:"source"`
	SHA256 string `json:"sha256"`
}

// CloseoutReceipt is the canonical, memory-native proof that a feature's
// authored sources may be retired. ClosureSHA256 authenticates every other
// field. Its arrays are always present and lexically sorted.
type CloseoutReceipt struct {
	Schema              string                   `json:"schema_version"`
	Feature             string                   `json:"feature"`
	SourceCommit        string                   `json:"source_commit"`
	SourceClosureSHA256 string                   `json:"source_closure_sha256"`
	CodeInventorySHA256 string                   `json:"code_inventory_sha256"`
	DesignIndexSHA256   string                   `json:"design_index_sha256"`
	SourceDocuments     []CloseoutSourceDocument `json:"source_documents"`
	NativeRecords       []CloseoutNativeRecord   `json:"native_records"`
	CodeUnits           []CloseoutCodeUnit       `json:"code_units"`
	PRD                 CloseoutPRD              `json:"prd"`
	Designs             []CloseoutDesign         `json:"designs"`
	ClosureSHA256       string                   `json:"closure_sha256"`
}

// ActiveCloseout couples a parsed active receipt with its immutable envelope.
type ActiveCloseout struct {
	RecordID string
	Receipt  CloseoutReceipt
}

// BuildCloseout proves feature completeness from one pinned snapshot and the
// current memory projection, then returns the literal Document to append. If
// an identical receipt is already the active seal, Draft is the zero value.
func BuildCloseout(feature string, snap Snapshot, set *store.Set, proj *resolve.Projection) (Draft, CloseoutReceipt, error) {
	if !validFeature(feature) {
		return Draft{}, CloseoutReceipt{}, incompletef("feature %q is not a canonical feature slug", feature)
	}
	if err := trustworthyMemory(set, proj); err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}
	var completion Report
	classifyContractCompletion(&completion, snap, set, proj)
	var outstanding []string
	for _, finding := range completion.Completion.Findings {
		if finding.Feature == feature {
			outstanding = append(outstanding, finding.State+" "+finding.Scope)
		}
	}
	if len(outstanding) > 0 {
		return Draft{}, CloseoutReceipt{}, incompletef("feature %s has outstanding contract completion: %s", feature, strings.Join(outstanding, "; "))
	}
	if !validGitObjectID(snap.Commit) || !validDigest(snap.SourceClosureSHA256) || !validDigest(snap.CodeInventorySHA256) {
		return Draft{}, CloseoutReceipt{}, incompletef("the pinned snapshot identity is incomplete")
	}
	for _, finding := range snap.CompilerFindings {
		if finding.Blocking && finding.Feature == feature {
			return Draft{}, CloseoutReceipt{}, incompletef("the compiled feature contract is inconsistent at %s: %s", finding.Scope, finding.Detail)
		}
	}

	prd, err := exactFeaturePRD(feature, snap.PRDs)
	if err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}
	liveHeads, dispositions, err := alignedPRDHeads(prd, proj)
	if err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}

	designs, decisions, err := servingDesigns(feature, snap, set, proj, liveHeads, designRequiredHeads(prd, liveHeads))
	if err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}

	subjects := map[string]bool{"prd/" + feature: true}
	for _, design := range designs {
		subjects["design/"+design.Slug] = true
	}
	if open := proj.OpenQuestions(sortedKeys(subjects)); len(open) > 0 {
		return Draft{}, CloseoutReceipt{}, incompletef("selected subject %s has active unanswered Question %s", open[0].Subject, open[0].ID)
	}
	sources, err := bindCurrentSources(snap, prd, subjects, set, proj)
	if err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}
	native, nativeRecords, err := bindNativeRecords(subjects, set, proj)
	if err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}
	// The PRD heads and serving Decisions are the minimum native truth. Keep
	// these checks separate from the broad binding so a similarly named record
	// can never make an absent required record look present.
	for _, family := range sortedMapKeys(liveHeads) {
		head := liveHeads[family]
		if !recordInList(head.ID, nativeRecords) {
			return Draft{}, CloseoutReceipt{}, incompletef("active PRD head %s is not a native semantic record", head.ID)
		}
	}
	for _, decision := range decisions {
		if !recordInList(decision.ID, nativeRecords) {
			return Draft{}, CloseoutReceipt{}, incompletef("serving Decision %s is not a native semantic record", decision.ID)
		}
	}
	for _, disposition := range dispositions {
		if !recordInList(disposition.ID, nativeRecords) {
			return Draft{}, CloseoutReceipt{}, incompletef("PRD disposition %s is not a native semantic record", disposition.ID)
		}
	}

	code, err := bindCodeUnits(snap.Code, subjects, set, proj, nativeRecords)
	if err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}
	if len(code) == 0 {
		return Draft{}, CloseoutReceipt{}, incompletef("feature %s has no production code unit selected by active semantic memory", feature)
	}

	receipt := CloseoutReceipt{
		Schema: CloseoutSchema, Feature: feature, SourceCommit: snap.Commit,
		SourceClosureSHA256: snap.SourceClosureSHA256, CodeInventorySHA256: snap.CodeInventorySHA256,
		DesignIndexSHA256: snap.DesignIndexSHA256,
		SourceDocuments:   sources, NativeRecords: native, CodeUnits: code,
		PRD:     CloseoutPRD{Feature: prd.Feature, Source: prd.Source, AsOf: prd.AsOf, Fingerprint: prd.Fingerprint, SourceMapSHA256: prd.SourceMapSHA256},
		Designs: make([]CloseoutDesign, 0, len(designs)),
	}
	for _, design := range designs {
		receipt.Designs = append(receipt.Designs, CloseoutDesign{Slug: design.Slug, Source: design.Source, SHA256: design.SHA256})
	}
	receipt.ClosureSHA256 = closeoutDigest(receipt)
	body, err := MarshalCloseout(receipt)
	if err != nil {
		return Draft{}, CloseoutReceipt{}, err
	}

	active, err := activeCloseouts(set, proj, false)
	if err != nil {
		return Draft{}, CloseoutReceipt{}, incompletef("read active closeout receipts: %v", err)
	}
	var prior string
	for _, existing := range active {
		if existing.Receipt.Feature != feature {
			continue
		}
		if prior != "" {
			return Draft{}, CloseoutReceipt{}, incompletef("feature %s has more than one active closeout receipt", feature)
		}
		prior = existing.RecordID
		existingBody, marshalErr := MarshalCloseout(existing.Receipt)
		if marshalErr != nil {
			return Draft{}, CloseoutReceipt{}, incompletef("marshal active closeout %s: %v", prior, marshalErr)
		}
		if bytes.Equal(existingBody, body) {
			return Draft{}, receipt, nil
		}
		if err := validateReplacementEvidence(existing.Receipt, receipt); err != nil {
			return Draft{}, CloseoutReceipt{}, incompletef("replacement closeout requires reconciled native memory: %v", err)
		}
	}

	relations := make([]record.Relation, 0, 1)
	if prior != "" {
		relations = append(relations, record.Relation{Type: record.Supersedes, Target: prior})
	}
	draft := Draft{
		Kind: record.Document, Subject: "closeout/" + feature, Family: "seal",
		Statement:    "Feature " + feature + " is complete in native memory at " + snap.Commit + ".",
		Intent:       "Bind the exact source, memory, design, and code identities that authorize retiring the selected authored sources.",
		Rationale:    "The offline closeout verifier proved every live mapped PRD row aligned, every live Must requirement served, every selected source exact, and every selected code unit covered by semantic memory.",
		FitCriterion: "The receipt remains parseable, canonical, internally valid, and bound to its immutable records.",
		Tags:         []string{"reconcile:closeout", "sha256:" + receipt.ClosureSHA256},
		Relations:    relations,
		Provenance:   record.Provenance{Source: "memory-closeout:" + feature, Commit: snap.Commit, Note: CloseoutSchema},
		LiteralBody:  true,
		Body:         body,
	}
	return draft, receipt, nil
}

// MarshalCloseout emits the only accepted receipt encoding.
func MarshalCloseout(receipt CloseoutReceipt) ([]byte, error) {
	if err := ValidateCloseout(receipt); err != nil {
		return nil, err
	}
	body, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("%w: marshal: %v", ErrInvalidCloseout, err)
	}
	return append(body, '\n'), nil
}

// ParseCloseout accepts only the closed schema and its canonical byte form.
func ParseCloseout(body []byte) (CloseoutReceipt, error) {
	canonicalInput := body
	dec := json.NewDecoder(bytes.NewReader(canonicalInput))
	dec.DisallowUnknownFields()
	var receipt CloseoutReceipt
	if err := dec.Decode(&receipt); err != nil {
		return CloseoutReceipt{}, fmt.Errorf("%w: decode: %v", ErrInvalidCloseout, err)
	}
	var trailing any
	if err := dec.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			err = errors.New("trailing JSON value")
		}
		return CloseoutReceipt{}, fmt.Errorf("%w: %v", ErrInvalidCloseout, err)
	}
	if err := ValidateCloseout(receipt); err != nil {
		return CloseoutReceipt{}, err
	}
	want, err := MarshalCloseout(receipt)
	if err != nil {
		return CloseoutReceipt{}, err
	}
	if !bytes.Equal(canonicalInput, want) {
		return CloseoutReceipt{}, fmt.Errorf("%w: body is not canonical two-space JSON with one trailing newline", ErrInvalidCloseout)
	}
	return receipt, nil
}

// ValidateCloseout checks the closed receipt independently of a store.
func ValidateCloseout(receipt CloseoutReceipt) error {
	bad := func(format string, args ...any) error {
		return fmt.Errorf("%w: %s", ErrInvalidCloseout, fmt.Sprintf(format, args...))
	}
	if receipt.Schema != CloseoutSchema {
		return bad("schema_version is %q, want %q", receipt.Schema, CloseoutSchema)
	}
	if !validFeature(receipt.Feature) || receipt.PRD.Feature != receipt.Feature {
		return bad("feature and PRD feature must be the same canonical slug")
	}
	if !validGitObjectID(receipt.SourceCommit) {
		return bad("source_commit is not a full lowercase Git object id")
	}
	for _, field := range []struct{ name, digest string }{
		{"source_closure_sha256", receipt.SourceClosureSHA256},
		{"code_inventory_sha256", receipt.CodeInventorySHA256},
		{"design_index_sha256", receipt.DesignIndexSHA256},
		{"prd.source_map_sha256", receipt.PRD.SourceMapSHA256},
		{"closure_sha256", receipt.ClosureSHA256},
	} {
		if !validDigest(field.digest) {
			return bad("%s is not a lowercase SHA-256", field.name)
		}
	}
	if !fileProvenance(receipt.PRD.Source) || receipt.PRD.AsOf == "" || !validFingerprint(receipt.PRD.Fingerprint) {
		return bad("PRD binding is incomplete")
	}
	if len(receipt.SourceDocuments) == 0 || len(receipt.NativeRecords) == 0 || len(receipt.CodeUnits) == 0 || len(receipt.Designs) == 0 {
		return bad("source_documents, native_records, code_units, and designs must all be present and non-empty")
	}

	seenSourcePath := map[string]bool{}
	seenSourceID := map[string]bool{}
	for i, binding := range receipt.SourceDocuments {
		if !fileProvenance(binding.Path) || record.KindOfID(binding.DocumentID) != record.Document || !validDigest(binding.RecordSHA256) || !validDigest(binding.BodySHA256) {
			return bad("source_documents[%d] is invalid", i)
		}
		if seenSourcePath[binding.Path] || seenSourceID[binding.DocumentID] {
			return bad("source_documents repeat a path or Document id")
		}
		seenSourcePath[binding.Path], seenSourceID[binding.DocumentID] = true, true
		if i > 0 && !lessSource(receipt.SourceDocuments[i-1], binding) {
			return bad("source_documents are not strictly sorted and unique")
		}
	}
	seenNativeID := map[string]bool{}
	for i, binding := range receipt.NativeRecords {
		if record.KindOfID(binding.ID) != binding.Kind || !record.ValidSubject(binding.Subject) || !record.IsKind(binding.Kind) || !validDigest(binding.SHA256) {
			return bad("native_records[%d] is invalid", i)
		}
		if seenNativeID[binding.ID] {
			return bad("native_records repeat id %s", binding.ID)
		}
		seenNativeID[binding.ID] = true
		if record.IsClaim(binding.Kind) && binding.Family == "" {
			return bad("native_records[%d] omits a claim family", i)
		}
		if binding.Family != "" && !record.ValidRef(binding.Subject+"#"+binding.Family) {
			return bad("native_records[%d] has an invalid family", i)
		}
		if i > 0 && !lessNative(receipt.NativeRecords[i-1], binding) {
			return bad("native_records are not strictly sorted and unique")
		}
	}
	seenCodePath := map[string]bool{}
	for i, binding := range receipt.CodeUnits {
		if (binding.Kind != codeinventory.GoPackage && binding.Kind != codeinventory.ExecutableScript) || !validCodeScope(binding.Path) || !validDigest(binding.SHA256) {
			return bad("code_units[%d] is invalid", i)
		}
		if len(binding.Files) == 0 {
			return bad("code_units[%d] has no source files", i)
		}
		for j, file := range binding.Files {
			if !validCodeScope(file) || (j > 0 && binding.Files[j-1] >= file) {
				return bad("code_units[%d].files are invalid or not strictly sorted and unique", i)
			}
			switch binding.Kind {
			case codeinventory.GoPackage:
				if path.Dir(file) != binding.Path || !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") {
					return bad("code_units[%d].files[%d] is not production Go source in its package", i, j)
				}
			case codeinventory.ExecutableScript:
				if len(binding.Files) != 1 || file != binding.Path || !codeinventory.IsScriptPath(file) {
					return bad("code_units[%d].files do not identify the executable script", i)
				}
			}
		}
		if len(binding.MemoryRecords) == 0 {
			return bad("code_units[%d] has no semantic memory_records", i)
		}
		for j, id := range binding.MemoryRecords {
			if !seenNativeID[id] {
				return bad("code_units[%d].memory_records[%d] is not a bound native record", i, j)
			}
			if j > 0 && binding.MemoryRecords[j-1] >= id {
				return bad("code_units[%d].memory_records are not strictly sorted and unique", i)
			}
		}
		if seenCodePath[binding.Path] {
			return bad("code_units repeat path %s", binding.Path)
		}
		seenCodePath[binding.Path] = true
		if i > 0 && !lessCode(receipt.CodeUnits[i-1], binding) {
			return bad("code_units are not strictly sorted and unique")
		}
	}
	seenDesignSource := map[string]bool{}
	for i, binding := range receipt.Designs {
		if !validFeature(binding.Slug) || !fileProvenance(binding.Source) || !validDigest(binding.SHA256) {
			return bad("designs[%d] is invalid", i)
		}
		if seenDesignSource[binding.Source] {
			return bad("designs repeat source %s", binding.Source)
		}
		seenDesignSource[binding.Source] = true
		if i > 0 && receipt.Designs[i-1].Slug >= binding.Slug {
			return bad("designs are not strictly sorted and unique")
		}
	}
	if want := closeoutDigest(receipt); receipt.ClosureSHA256 != want {
		return bad("closure_sha256 is %s, want %s", receipt.ClosureSHA256, want)
	}
	return nil
}

// ActiveCloseouts returns every valid active seal in feature order. One bad
// active seal invalidates the result rather than silently granting a partial
// source-retirement allowance.
func ActiveCloseouts(set *store.Set, proj *resolve.Projection) ([]ActiveCloseout, error) {
	return activeCloseouts(set, proj, true)
}

// ValidateCloseoutAtSnapshot replays the closeout compiler against the exact
// source commit named by an active receipt. This is stronger than validating
// the receipt's hashes against the surviving memory tree: it proves that the
// receipt is the deterministic output the closeout command would have
// produced from that historical repository state.
func ValidateCloseoutAtSnapshot(active ActiveCloseout, snap Snapshot, set *store.Set, proj *resolve.Projection) error {
	if snap.Commit != active.Receipt.SourceCommit {
		return fmt.Errorf("%w: source snapshot %s does not match receipt commit %s", ErrInvalidCloseout, snap.Commit, active.Receipt.SourceCommit)
	}
	draft, rebuilt, err := BuildCloseout(active.Receipt.Feature, snap, set, proj)
	if err != nil {
		return fmt.Errorf("%w: replay closeout %s: %v", ErrInvalidCloseout, active.RecordID, err)
	}
	if draft.Kind != "" || rebuilt.ClosureSHA256 != active.Receipt.ClosureSHA256 {
		return fmt.Errorf("%w: replayed closeout %s is not the active receipt's deterministic fixed point", ErrInvalidCloseout, active.RecordID)
	}
	return nil
}

func activeCloseouts(set *store.Set, proj *resolve.Projection, requireBindingsActive bool) ([]ActiveCloseout, error) {
	if set == nil || proj == nil || proj.Set != set {
		return nil, fmt.Errorf("%w: memory set and projection are unavailable or unrelated", ErrInvalidCloseout)
	}
	conflicts := proj.Conflicts()
	answerConflicts := proj.AnswerConflicts()
	revocationConflicts := proj.RevocationConflicts()
	if len(set.Problems) > 0 || len(set.Dangling) > 0 || len(proj.Violations) > 0 || len(conflicts) > 0 || len(answerConflicts) > 0 || len(revocationConflicts) > 0 {
		return nil, fmt.Errorf("%w: memory integrity is unresolved (%d invalid files, %d dangling relations, %d projection violations, %d family conflicts, %d answer conflicts, %d revocation conflicts)", ErrInvalidCloseout, len(set.Problems), len(set.Dangling), len(proj.Violations), len(conflicts), len(answerConflicts), len(revocationConflicts))
	}
	var out []ActiveCloseout
	for _, subject := range set.Subjects() {
		if !strings.HasPrefix(subject, "closeout/") {
			continue
		}
		for _, r := range proj.Active(subject) {
			if r.Family != "seal" {
				continue
			}
			if r.Kind != record.Document || !r.LiteralBody || !r.HasTag("reconcile:closeout") {
				return nil, fmt.Errorf("%w: active seal %s is not a literal reconciliation Document", ErrInvalidCloseout, r.ID)
			}
			receipt, err := ParseCloseout(set.Bodies[r.ID])
			if err != nil {
				return nil, fmt.Errorf("active seal %s: %w", r.ID, err)
			}
			feature := strings.TrimPrefix(subject, "closeout/")
			if receipt.Feature != feature || r.Provenance.Commit != receipt.SourceCommit || !r.HasTag("sha256:"+receipt.ClosureSHA256) {
				return nil, fmt.Errorf("%w: active seal %s envelope does not bind its receipt", ErrInvalidCloseout, r.ID)
			}
			if err := validateReceiptRecords(receipt, r.ID, set); err != nil {
				return nil, fmt.Errorf("active seal %s: %w", r.ID, err)
			}
			if requireBindingsActive {
				for _, binding := range receipt.SourceDocuments {
					if proj.Status(binding.DocumentID) != resolve.Active {
						return nil, fmt.Errorf("%w: active seal %s binds non-active source Document %s", ErrInvalidCloseout, r.ID, binding.DocumentID)
					}
				}
				for _, binding := range receipt.NativeRecords {
					if proj.Status(binding.ID) != resolve.Active {
						return nil, fmt.Errorf("%w: active seal %s binds non-active native record %s", ErrInvalidCloseout, r.ID, binding.ID)
					}
				}
			}
			out = append(out, ActiveCloseout{RecordID: r.ID, Receipt: receipt})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Receipt.Feature != out[j].Receipt.Feature {
			return out[i].Receipt.Feature < out[j].Receipt.Feature
		}
		return out[i].RecordID < out[j].RecordID
	})
	return out, nil
}

// sealedSourcePaths returns the exact source paths authorized for absence by
// valid active receipts. Any bad active receipt fails closed to no allowance.
func sealedSourcePaths(set *store.Set, proj *resolve.Projection) map[string]bool {
	sealed := map[string]bool{}
	receipts, err := ActiveCloseouts(set, proj)
	if err != nil {
		return sealed
	}
	for _, active := range receipts {
		for _, source := range active.Receipt.SourceDocuments {
			sealed[source.Path] = true
		}
	}
	return sealed
}

func exactFeaturePRD(feature string, prds []PRD) (PRD, error) {
	var matches []PRD
	for _, prd := range prds {
		if prd.Feature == feature {
			matches = append(matches, prd)
		}
	}
	if len(matches) != 1 {
		return PRD{}, incompletef("feature %s has %d compiled PRD ledgers, want exactly one", feature, len(matches))
	}
	prd := matches[0]
	if !fileProvenance(prd.Source) || prd.AsOf == "" || !validFingerprint(prd.Fingerprint) || !validDigest(prd.SourceMapSHA256) {
		return PRD{}, incompletef("feature %s has an incomplete PRD compiler identity", feature)
	}
	return prd, nil
}

func alignedPRDHeads(prd PRD, proj *resolve.Projection) (map[string]*record.Record, []*record.Record, error) {
	heads := map[string]*record.Record{}
	var dispositions []*record.Record
	seen := map[string]bool{}
	for _, unit := range prd.Units {
		if seen[unit.ID] || unit.ID == "" {
			return nil, nil, incompletef("PRD %s has a duplicate or empty unit id %q", prd.Feature, unit.ID)
		}
		seen[unit.ID] = true
		head := proj.Head("prd/"+prd.Feature, unit.ID)
		if unit.Status != "live" {
			if head != nil {
				return nil, nil, incompletef("non-live PRD unit %s still has active memory head %s", unit.ID, head.ID)
			}
			if unit.Prefix == "Q" || unit.Prefix == "R" {
				disposition, err := explicitPRDDisposition(prd, unit, proj)
				if err != nil {
					return nil, nil, incompletef("non-live PRD unit %s has an invalid native disposition: %v", unit.ID, err)
				}
				if disposition.Record != nil {
					return nil, nil, incompletef("non-live PRD unit %s still has active native disposition %s", unit.ID, disposition.Record.ID)
				}
			}
			continue
		}
		if !unit.Mapped {
			if unit.Prefix == "NG" || unit.Prefix == "WR" {
				if head != nil {
					return nil, nil, incompletef("live %s context unit %s unexpectedly has active memory head %s", unit.Prefix, unit.ID, head.ID)
				}
				continue
			}
			disposition, err := explicitPRDDisposition(prd, unit, proj)
			if err != nil {
				return nil, nil, incompletef("live PRD unit %s has an invalid native disposition: %v", unit.ID, err)
			}
			if disposition.Record == nil {
				return nil, nil, incompletef("live PRD unit %s has no lossless memory mapping or exact native disposition", unit.ID)
			}
			if unit.Prefix == "Q" && disposition.Answer == nil {
				return nil, nil, incompletef("live PRD question %s has no active Answer", unit.ID)
			}
			dispositions = append(dispositions, disposition.Record)
			if disposition.Answer != nil {
				dispositions = append(dispositions, disposition.Answer)
			}
			continue
		}
		if !validDigest(unit.SourceSHA256) {
			return nil, nil, incompletef("live mapped PRD unit %s has no exact source-unit digest", unit.ID)
		}
		if head == nil {
			return nil, nil, incompletef("live PRD unit %s has no active memory head", unit.ID)
		}
		if !samePRDSemantics(head, unit) || head.Consequence != unit.Consequence {
			return nil, nil, incompletef("active PRD head %s differs from compiled unit %s", head.ID, unit.ID)
		}
		heads[unit.ID] = head
	}
	if len(heads) == 0 {
		return nil, nil, incompletef("feature %s has no live mapped PRD units", prd.Feature)
	}
	return heads, dispositions, nil
}

func servingDesigns(feature string, snap Snapshot, set *store.Set, proj *resolve.Projection, heads, required map[string]*record.Record) ([]Design, []*record.Record, error) {
	bySlug := map[string]Design{}
	selected := map[string]bool{}
	for _, design := range snap.Designs {
		if design.Slug == "" || bySlug[design.Slug].Slug != "" {
			return nil, nil, incompletef("the design index contains a duplicate or empty slug %q", design.Slug)
		}
		bySlug[design.Slug] = design
		if design.Status != "live" && design.Status != "superseded" {
			return nil, nil, incompletef("design %s has unsupported status %q", design.Slug, design.Status)
		}
		for _, ref := range design.Serves {
			subject, _, ok := memoryPRDRef(ref)
			if ok && subject == "prd/"+feature {
				if design.Status == "live" {
					selected[design.Slug] = true
				}
			}
		}
	}

	headIDs := map[string]string{}
	for _, family := range sortedMapKeys(heads) {
		head := heads[family]
		headIDs[head.ID] = family
	}
	authorizedHeads := authorizedDesignPRDHeads(snap.Designs, proj)
	for _, id := range set.IDs() {
		r := set.Get(id)
		if proj.Status(id) != resolve.Active || r.Kind != record.Decision {
			continue
		}
		targetsFeature := false
		for _, targetID := range r.Rel(record.Serves) {
			_, targetsFeature = headIDs[targetID]
			if targetsFeature {
				break
			}
		}
		if record.Namespace(r.Subject) != "design" {
			if targetsFeature {
				return nil, nil, incompletef("Decision %s serves feature %s outside a design subject", r.ID, feature)
			}
			continue
		}
		slug := strings.TrimPrefix(r.Subject, "design/")
		if !selected[slug] {
			if targetsFeature {
				design, ok := bySlug[slug]
				if !ok || design.Status != "live" {
					return nil, nil, incompletef("Decision %s serves feature %s but has no matching live design metadata", r.ID, feature)
				}
				return nil, nil, incompletef("Decision %s serves feature %s but design %s does not declare that feature in Serves metadata", r.ID, feature, slug)
			}
			continue
		}
		design, ok := bySlug[slug]
		if !ok || design.Status != "live" {
			return nil, nil, incompletef("Decision %s serves feature %s but has no matching live design metadata", r.ID, feature)
		}
		for _, targetID := range r.Rel(record.Serves) {
			target := set.Get(targetID)
			if target == nil || proj.Status(targetID) != resolve.Active {
				return nil, nil, incompletef("Decision %s on live design %s serves non-active record %s", r.ID, slug, targetID)
			}
			if record.Namespace(target.Subject) != "prd" {
				continue
			}
			if !exactPRDTarget(snap, target.Subject, target.Family, target) {
				return nil, nil, incompletef("Decision %s on live design %s serves %s, which is not an exact active PRD source head", r.ID, slug, targetID)
			}
			if !authorizedHeads[r.Subject][targetID] {
				return nil, nil, incompletef("Decision %s serves current PRD family %s but live design %s does not declare exact reference prd:%s/%s", r.ID, target.Family, slug, strings.TrimPrefix(target.Subject, "prd/"), target.Family)
			}
		}
	}

	if len(selected) == 0 {
		return nil, nil, incompletef("feature %s has no explicitly serving live design", feature)
	}
	slugs := make([]string, 0, len(selected))
	for slug := range selected {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	designs := make([]Design, 0, len(slugs))
	var decisions []*record.Record
	served := map[string]bool{}
	for _, slug := range slugs {
		design := bySlug[slug]
		if err := validateDesignSource(snap, design); err != nil {
			return nil, nil, err
		}
		if err := validateDistilledDesign(snap, design, set, proj); err != nil {
			return nil, nil, err
		}
		active := activeDecisions(proj, "design/"+slug)
		if len(active) == 0 {
			return nil, nil, incompletef("live serving design %s has no active Decision", slug)
		}
		for _, decision := range active {
			for _, target := range decision.Rel(record.Serves) {
				if proj.Status(target) != resolve.Active {
					return nil, nil, incompletef("active Decision %s serves non-active record %s", decision.ID, target)
				}
			}
		}
		for _, ref := range design.Serves {
			subject, family, ok := memoryPRDRef(ref)
			if !ok || subject != "prd/"+feature {
				continue
			}
			if nonDurablePRDContextRef(snap, subject, family) {
				continue
			}
			head := heads[family]
			if head == nil {
				return nil, nil, incompletef("design %s serves %s but that exact live mapped PRD head is absent", slug, ref)
			}
			if !anyServes(active, head.ID) {
				return nil, nil, incompletef("design %s has no active Decision carrying serves -> %s", slug, head.ID)
			}
		}
		for _, decision := range active {
			for _, target := range decision.Rel(record.Serves) {
				if _, ok := headIDs[target]; ok {
					served[target] = true
				}
			}
		}
		designs = append(designs, design)
		decisions = append(decisions, active...)
	}
	for _, family := range sortedMapKeys(required) {
		head := required[family]
		if !served[head.ID] {
			return nil, nil, incompletef("active Must PRD head %s is served by no active Decision on a selected live design", head.ID)
		}
	}
	sort.Slice(decisions, func(i, j int) bool { return decisions[i].ID < decisions[j].ID })
	return designs, decisions, nil
}

func validateDistilledDesign(snap Snapshot, design Design, set *store.Set, proj *resolve.Projection) error {
	document := exactDesignDocument(snap, design, set, proj)
	if document == nil {
		return incompletef("live serving design %s has no exact active source Document", design.Slug)
	}
	file := snap.Files[design.Source]
	blocks, err := ExtractLiveDesignDecisions(design, file)
	if err != nil {
		return incompletef("live serving design %s has invalid explicit Decision source: %v", design.Slug, err)
	}
	for _, block := range blocks {
		head := proj.Head(block.Subject, block.Family)
		if head == nil {
			return incompletef("live serving design %s is missing exact source Decision %s", design.Slug, block.Family)
		}
		if err := validateDesignDecisionMemory(head, block, document, snap.Code); err != nil {
			return incompletef("live serving design %s has drifted source Decision %s: %v", design.Slug, block.Family, err)
		}
	}
	expectedFamilies := make(map[string]bool, len(blocks))
	for _, block := range blocks {
		expectedFamilies[block.Family] = true
	}
	for _, head := range proj.Active("design/" + design.Slug) {
		if head.Kind == record.Decision && head.HasTag(designDecisionMemoryTag) && !expectedFamilies[head.Family] {
			return incompletef("live serving design %s retains source-projected Decision %s with no current explicit block", design.Slug, head.ID)
		}
	}
	if coverage := proj.Head("design/"+design.Slug, designCoverageFamily); coverage != nil {
		expected, ok := expectedDesignCoverage(snap, design, set, proj)
		if !ok {
			return incompletef("live serving design %s coverage head cannot be reconstructed from exact current sources", design.Slug)
		}
		if err := validateDesignCoverageMemory(coverage, expected); err != nil {
			return incompletef("live serving design %s has drifted coverage memory: %v", design.Slug, err)
		}
	}
	return nil
}

func designRequiredHeads(prd PRD, heads map[string]*record.Record) map[string]*record.Record {
	required := map[string]*record.Record{}
	for _, unit := range prd.Units {
		if !requiresDesignServe(unit) {
			continue
		}
		if head := heads[unit.ID]; head != nil {
			required[unit.ID] = head
		}
	}
	return required
}

func validateDesignSource(snap Snapshot, design Design) error {
	if !fileProvenance(design.Source) || !validDigest(design.SHA256) {
		return incompletef("live serving design %s has no exact source fingerprint", design.Slug)
	}
	file, ok := snap.Files[design.Source]
	if !ok || file.Path != design.Source || file.SHA256 != design.SHA256 || sourceDigest(file.Body) != design.SHA256 {
		return incompletef("live serving design %s does not match its pinned source %s", design.Slug, design.Source)
	}
	if snap.DesignIndexSHA256 == "" || !validDigest(snap.DesignIndexSHA256) {
		return incompletef("the design index identity is unavailable for serving design %s", design.Slug)
	}
	return nil
}

func bindCurrentSources(snap Snapshot, prd PRD, subjects map[string]bool, set *store.Set, proj *resolve.Projection) ([]CloseoutSourceDocument, error) {
	expected := map[string]string{}
	contextSources, err := FeatureContextSources(snap, prd.Feature)
	if err != nil {
		return nil, err
	}
	for _, source := range contextSources {
		expected[source.Path] = source.Subject
	}

	byPath := map[string][]*record.Record{}
	for _, id := range set.IDs() {
		r := set.Get(id)
		if proj.Status(id) != resolve.Active || r.Kind != record.Document || !r.LiteralBody || !fileProvenance(r.Provenance.Source) {
			continue
		}
		byPath[r.Provenance.Source] = append(byPath[r.Provenance.Source], r)
	}
	for _, source := range sortedMapKeys(expected) {
		subject := expected[source]
		file, ok := snap.Files[source]
		if !ok || file.Path != source || sourceDigest(file.Body) != file.SHA256 {
			return nil, incompletef("selected authored source %s is absent or internally inconsistent", source)
		}
		var exact int
		for _, r := range byPath[source] {
			if r.Subject == subject && documentMatchesSource(r, set.Bodies[r.ID], file) {
				exact++
			}
		}
		if exact != 1 || len(byPath[source]) != 1 {
			return nil, incompletef("selected authored source %s has %d exact and %d total active Document bindings", source, exact, len(byPath[source]))
		}
	}
	for _, source := range sortedMapKeys(byPath) {
		if _, selected := expected[source]; selected {
			continue
		}
		for _, document := range byPath[source] {
			if subjects[document.Subject] {
				return nil, incompletef("active source Document at %s is not selected feature context", source)
			}
		}
	}

	var out []CloseoutSourceDocument
	for _, source := range sortedMapKeys(expected) {
		docs := byPath[source]
		if len(docs) != 1 {
			return nil, incompletef("source %s has %d active Document bindings on selected subjects", source, len(docs))
		}
		r := docs[0]
		file, ok := snap.Files[source]
		if !ok || !documentMatchesSource(r, set.Bodies[r.ID], file) {
			return nil, incompletef("active source Document %s is not exact at %s", r.ID, source)
		}
		recordHash, err := storedRecordDigest(set, r.ID)
		if err != nil {
			return nil, incompletef("bind source Document %s: %v", r.ID, err)
		}
		out = append(out, CloseoutSourceDocument{Path: source, DocumentID: r.ID, RecordSHA256: recordHash, BodySHA256: r.BodySHA256})
	}
	sort.Slice(out, func(i, j int) bool { return lessSource(out[i], out[j]) })
	return out, nil
}

func bindNativeRecords(subjects map[string]bool, set *store.Set, proj *resolve.Projection) ([]CloseoutNativeRecord, []*record.Record, error) {
	var bindings []CloseoutNativeRecord
	var records []*record.Record
	for _, subject := range sortedKeys(subjects) {
		for _, r := range proj.Active(subject) {
			if !nativeForCloseout(r) {
				continue
			}
			hash, err := storedRecordDigest(set, r.ID)
			if err != nil {
				return nil, nil, incompletef("bind native record %s: %v", r.ID, err)
			}
			bindings = append(bindings, CloseoutNativeRecord{ID: r.ID, Kind: r.Kind, Subject: r.Subject, Family: r.Family, SHA256: hash})
			records = append(records, r)
		}
	}
	sort.Slice(bindings, func(i, j int) bool { return lessNative(bindings[i], bindings[j]) })
	sort.Slice(records, func(i, j int) bool { return records[i].ID < records[j].ID })
	return bindings, records, nil
}

func bindCodeUnits(units []codeinventory.Unit, subjects map[string]bool, set *store.Set, proj *resolve.Projection, native []*record.Record) ([]CloseoutCodeUnit, error) {
	selectedScopes := map[string]bool{}
	lookup := activeLookup{set: set, proj: proj}
	for _, subject := range sortedKeys(subjects) {
		for _, r := range proj.Active(subject) {
			for _, scope := range association.Scopes(r, lookup) {
				if !validCodeScope(scope) {
					return nil, incompletef("selected record %s has invalid code scope %q", r.ID, scope)
				}
				selectedScopes[scope] = true
			}
		}
	}
	ordered := append([]codeinventory.Unit(nil), units...)
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].Path != ordered[j].Path {
			return ordered[i].Path < ordered[j].Path
		}
		return ordered[i].Kind < ordered[j].Kind
	})
	seen := map[string]bool{}
	matchedScope := map[string]bool{}
	var out []CloseoutCodeUnit
	for _, unit := range ordered {
		key := string(unit.Kind) + "\x00" + unit.Path
		if seen[key] || (unit.Kind != codeinventory.GoPackage && unit.Kind != codeinventory.ExecutableScript) || !validCodeScope(unit.Path) || !validDigest(unit.SHA256) {
			return nil, incompletef("the code inventory contains an invalid or duplicate unit %s", unit.Path)
		}
		seen[key] = true
		selected := false
		for _, scope := range sortedKeys(selectedScopes) {
			if scopeMatchesUnit(scope, unit) {
				selected = true
				matchedScope[scope] = true
			}
		}
		if !selected {
			continue
		}
		var memoryRecords []string
		for _, r := range native {
			if !semanticForCodeCloseout(r) {
				continue
			}
			for _, scope := range association.Scopes(r, lookup) {
				if scopeMatchesUnit(scope, unit) {
					memoryRecords = append(memoryRecords, r.ID)
					break
				}
			}
		}
		if len(memoryRecords) == 0 {
			return nil, incompletef("selected code unit %s has no active semantic feature memory", unit.Path)
		}
		sort.Strings(memoryRecords)
		out = append(out, CloseoutCodeUnit{Kind: unit.Kind, Path: unit.Path, Files: append([]string(nil), unit.Files...), SHA256: unit.SHA256, MemoryRecords: memoryRecords})
	}
	for _, scope := range sortedKeys(selectedScopes) {
		if !matchedScope[scope] {
			return nil, incompletef("selected code scope %s matches no production code unit", scope)
		}
	}
	return out, nil
}

// validateReplacementEvidence requires explicit evolution of changed contracts
// and retired associations. A replacement receipt binds changed code bytes to
// the still-applicable knowledge without inventing a fresh semantic record.
// The old receipt remains stale until a new receipt is explicitly issued.
func validateReplacementEvidence(before, after CloseoutReceipt) error {
	beforeNative := make(map[string]bool, len(before.NativeRecords))
	afterNative := make(map[string]bool, len(after.NativeRecords))
	for _, binding := range before.NativeRecords {
		beforeNative[binding.ID] = true
	}
	for _, binding := range after.NativeRecords {
		afterNative[binding.ID] = true
	}
	beforeCode := make(map[string]CloseoutCodeUnit, len(before.CodeUnits))
	afterCode := make(map[string]CloseoutCodeUnit, len(after.CodeUnits))
	for _, binding := range before.CodeUnits {
		beforeCode[codeUnitKey(binding.Kind, binding.Path)] = binding
	}
	for _, binding := range after.CodeUnits {
		key := codeUnitKey(binding.Kind, binding.Path)
		afterCode[key] = binding
	}
	for key, binding := range beforeCode {
		if _, exists := afterCode[key]; exists {
			continue
		}
		associationRetired := false
		for _, id := range binding.MemoryRecords {
			if !afterNative[id] {
				associationRetired = true
				break
			}
		}
		if !associationRetired {
			return fmt.Errorf("removed code unit %s retains every prior semantic association", binding.Path)
		}
	}
	if before.PRD.Source != after.PRD.Source || before.PRD.AsOf != after.PRD.AsOf || before.PRD.Fingerprint != after.PRD.Fingerprint || before.PRD.SourceMapSHA256 != after.PRD.SourceMapSHA256 {
		fresh := false
		for _, native := range after.NativeRecords {
			if native.Subject == "prd/"+after.Feature && !beforeNative[native.ID] {
				fresh = true
				break
			}
		}
		if !fresh {
			return fmt.Errorf("changed PRD %s has no new native memory bound by the replacement", after.Feature)
		}
	}

	beforeDesign := make(map[string]CloseoutDesign, len(before.Designs))
	for _, binding := range before.Designs {
		beforeDesign[binding.Slug] = binding
	}
	for _, binding := range after.Designs {
		previous, existed := beforeDesign[binding.Slug]
		if existed && previous.Source == binding.Source && previous.SHA256 == binding.SHA256 {
			continue
		}
		fresh := false
		for _, native := range after.NativeRecords {
			if native.Subject == "design/"+binding.Slug && native.Kind == record.Decision && !beforeNative[native.ID] {
				fresh = true
				break
			}
		}
		if !fresh {
			return fmt.Errorf("changed design %s has no new Decision bound by the replacement", binding.Slug)
		}
	}
	return nil
}

func validateReceiptRecords(receipt CloseoutReceipt, receiptID string, set *store.Set) error {
	allowedSubjects := map[string]bool{"prd/" + receipt.Feature: true}
	for _, design := range receipt.Designs {
		allowedSubjects["design/"+design.Slug] = true
	}
	sourceSubjects := map[string]bool{}
	for _, binding := range receipt.SourceDocuments {
		r := set.Get(binding.DocumentID)
		if r == nil || r.ID == receiptID || r.Kind != record.Document || !r.LiteralBody || r.Provenance.Source != binding.Path || r.BodySHA256 != binding.BodySHA256 || sourceDigest(set.Bodies[r.ID]) != binding.BodySHA256 {
			return fmt.Errorf("%w: source binding %s no longer resolves exactly", ErrInvalidCloseout, binding.Path)
		}
		sourceSubjects[r.Subject] = true
		hash, err := storedRecordDigest(set, r.ID)
		if err != nil || hash != binding.RecordSHA256 {
			return fmt.Errorf("%w: source binding %s envelope hash differs", ErrInvalidCloseout, binding.Path)
		}
	}
	for _, subject := range sortedKeys(allowedSubjects) {
		if !sourceSubjects[subject] {
			return fmt.Errorf("%w: selected subject %s has no bound source Document", ErrInvalidCloseout, subject)
		}
	}
	for _, binding := range receipt.NativeRecords {
		r := set.Get(binding.ID)
		if r == nil || r.ID == receiptID || !allowedSubjects[binding.Subject] || r.Kind != binding.Kind || r.Subject != binding.Subject || r.Family != binding.Family {
			return fmt.Errorf("%w: native binding %s no longer resolves exactly", ErrInvalidCloseout, binding.ID)
		}
		hash, err := storedRecordDigest(set, r.ID)
		if err != nil || hash != binding.SHA256 {
			return fmt.Errorf("%w: native binding %s envelope hash differs", ErrInvalidCloseout, binding.ID)
		}
	}
	for _, binding := range receipt.CodeUnits {
		unit := codeinventory.Unit{Kind: binding.Kind, Path: binding.Path, Files: binding.Files, SHA256: binding.SHA256}
		for _, id := range binding.MemoryRecords {
			r := set.Get(id)
			if !semanticForCodeCloseout(r) {
				return fmt.Errorf("%w: code binding %s names non-semantic record %s", ErrInvalidCloseout, binding.Path, id)
			}
			associated := false
			for _, scope := range association.Scopes(r, set) {
				if validCodeScope(scope) && scopeMatchesUnit(scope, unit) {
					associated = true
					break
				}
			}
			if !associated {
				return fmt.Errorf("%w: code binding %s is not explicitly selected by record %s", ErrInvalidCloseout, binding.Path, id)
			}
		}
	}
	return nil
}

func storedRecordDigest(set *store.Set, id string) (string, error) {
	p := set.Paths[id]
	body, ok := set.Bytes[p]
	if p == "" || !ok || len(body) == 0 {
		return "", fmt.Errorf("record %s has no immutable envelope bytes", id)
	}
	return sourceDigest(body), nil
}

func trustworthyMemory(set *store.Set, proj *resolve.Projection) error {
	if set == nil || proj == nil || proj.Set != set {
		return incompletef("the memory set and its projection are unavailable or unrelated")
	}
	conflicts := proj.Conflicts()
	answerConflicts := proj.AnswerConflicts()
	revocationConflicts := proj.RevocationConflicts()
	if len(set.Problems) > 0 || len(set.Dangling) > 0 || len(proj.Violations) > 0 || len(conflicts) > 0 || len(answerConflicts) > 0 || len(revocationConflicts) > 0 {
		return incompletef("memory has %d invalid files, %d dangling relations, %d projection violations, %d active family conflicts, %d answer conflicts, and %d revocation conflicts", len(set.Problems), len(set.Dangling), len(proj.Violations), len(conflicts), len(answerConflicts), len(revocationConflicts))
	}
	return nil
}

func activeDecisions(proj *resolve.Projection, subject string) []*record.Record {
	var out []*record.Record
	for _, r := range proj.Active(subject) {
		if r.Kind == record.Decision {
			out = append(out, r)
		}
	}
	return out
}

func anyServes(records []*record.Record, target string) bool {
	for _, r := range records {
		if slices.Contains(r.Rel(record.Serves), target) {
			return true
		}
	}
	return false
}

func nativeForCloseout(r *record.Record) bool {
	if r == nil || r.Kind == record.Plan || isCodeAnchor(r) || strings.HasPrefix(r.Subject, "closeout/") {
		return false
	}
	if r.Kind == record.Document && r.LiteralBody && fileProvenance(r.Provenance.Source) {
		return false
	}
	return r.Kind != record.Revocation
}

func semanticForCodeCloseout(r *record.Record) bool {
	if !nativeForCloseout(r) || !isSemanticCodeMemory(r) {
		return false
	}
	return r.Kind != record.Document || !r.LiteralBody
}

func recordInList(id string, records []*record.Record) bool {
	for _, r := range records {
		if r.ID == id {
			return true
		}
	}
	return false
}

func closeoutDigest(receipt CloseoutReceipt) string {
	receipt.ClosureSHA256 = ""
	body, err := json.Marshal(struct {
		Domain  string          `json:"domain"`
		Receipt CloseoutReceipt `json:"receipt"`
	}{Domain: "pyxis-memory-closeout/v1", Receipt: receipt})
	if err != nil {
		panic(fmt.Sprintf("reconcile: marshal closeout digest input: %v", err))
	}
	return sourceDigest(body)
}

func incompletef(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrFeatureIncomplete, fmt.Sprintf(format, args...))
}

func validFeature(feature string) bool {
	return feature != "" && !strings.Contains(feature, "/") && record.ValidSubject("prd/"+feature)
}

func validDigest(digest string) bool {
	if len(digest) != 64 {
		return false
	}
	for _, c := range digest {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func validGitObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for _, c := range id {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func validFingerprint(fingerprint string) bool {
	return strings.HasPrefix(fingerprint, "sha256:") && validDigest(strings.TrimPrefix(fingerprint, "sha256:"))
}

func lessSource(a, b CloseoutSourceDocument) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.DocumentID < b.DocumentID
}

func lessNative(a, b CloseoutNativeRecord) bool {
	if a.Subject != b.Subject {
		return a.Subject < b.Subject
	}
	if a.Family != b.Family {
		return a.Family < b.Family
	}
	return a.ID < b.ID
}

func lessCode(a, b CloseoutCodeUnit) bool {
	if a.Path != b.Path {
		return a.Path < b.Path
	}
	return a.Kind < b.Kind
}

func sortedKeys(values map[string]bool) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sortedMapKeys[V any](values map[string]V) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
