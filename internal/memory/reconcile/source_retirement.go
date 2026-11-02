package reconcile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"maps"
	"path"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

const SourceRetirementSchema = "memory-source-retirement/v1"

type SourceRetirementReceipt struct {
	Schema         string `json:"schema_version"`
	Path           string `json:"path"`
	Mode           string `json:"mode"`
	SHA256         string `json:"sha256"`
	SourceCommit   string `json:"source_commit"`
	Introduction   string `json:"introduction_commit"`
	DocumentID     string `json:"document_id"`
	DocumentSHA256 string `json:"document_sha256"`
	Reason         string `json:"reason"`
}

type ActiveSourceRetirement struct {
	RecordID string
	Receipt  SourceRetirementReceipt
}

// SourceRetirementEvidence is supplied only by the pinned Git adapter. The
// receipt cannot select the trusted base, introduce itself before a deletion,
// or provide its own historical source bytes.
type SourceRetirementEvidence struct {
	Issued              Snapshot
	IssuedSet           *store.Set
	Trusted             Snapshot
	TrustedSet          *store.Set
	BeforeDeletion      *File
	Introduction        string
	TrustedIntroduction string
	CurrentIntroduction string
	// IssuedProjection and TrustedProjection are the projections of IssuedSet
	// and TrustedSet when the adapter already holds them. A projection of any
	// other set is ignored and computed here instead.
	IssuedProjection  *resolve.Projection
	TrustedProjection *resolve.Projection
}

// projectionOf returns supplied only when it was computed from exactly this
// set, so an adapter can save the work but never substitute another tree.
func projectionOf(set *store.Set, supplied *resolve.Projection) *resolve.Projection {
	if supplied != nil && supplied.Set == set {
		return supplied
	}
	return resolve.New(set)
}

type SourceRetirementProof struct {
	Evidence SourceRetirementEvidence
	Error    string
}

func StandaloneSourcePath(name string) bool {
	if name == "" || name != path.Clean(name) || path.IsAbs(name) {
		return false
	}
	p := strings.Split(name, "/")
	for _, part := range p {
		if part == "" || part == "." || part == ".." {
			return false
		}
	}
	if len(p) == 4 && p[0] == ".pyxis" && validFeature(p[2]) &&
		(p[1] == "skills" && p[3] == "SKILL.md" || p[1] == "vendor" && p[3] == "README.md") {
		return true
	}
	if len(p) == 3 && p[0] == ".pyxis" && strings.HasSuffix(p[2], ".md") &&
		(p[1] == "authoring" || p[1] == "templates") {
		return true
	}
	if len(p) >= 4 && p[0] == ".pyxis" && p[1] == "skills" && validFeature(p[2]) && strings.HasSuffix(p[len(p)-1], ".md") {
		return true
	}
	return len(p) == 3 && p[0] == "docs" && p[1] == "wiki" && strings.HasSuffix(p[2], ".md")
}

func SourceRetirementSubject(name string) string {
	return "source-retirement/" + sourceDigest([]byte(name))
}

func ParseSourceRetirement(body []byte) (SourceRetirementReceipt, error) {
	var receipt SourceRetirementReceipt
	decoder := json.NewDecoder(bytes.NewReader(body))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&receipt); err != nil {
		return receipt, fmt.Errorf("invalid source retirement: %w", err)
	}
	want, err := MarshalSourceRetirement(receipt)
	if err != nil {
		return receipt, err
	}
	if !bytes.Equal(body, want) {
		return receipt, fmt.Errorf("source retirement must use canonical encoding with no trailing value")
	}
	return receipt, nil
}

func MarshalSourceRetirement(r SourceRetirementReceipt) ([]byte, error) {
	if r.Schema != SourceRetirementSchema || !StandaloneSourcePath(r.Path) || (r.Mode != "100644" && r.Mode != "100755") ||
		!hexDigest(r.SHA256, 64) || !hexDigest(r.DocumentSHA256, 64) || !hexDigest(r.SourceCommit, 40) || !hexDigest(r.Introduction, 40) ||
		record.KindOfID(r.DocumentID) != record.Document || strings.TrimSpace(r.Reason) == "" || strings.TrimSpace(r.Reason) != r.Reason {
		return nil, fmt.Errorf("invalid source retirement identity for %q", r.Path)
	}
	body, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode source retirement: %w", err)
	}
	return append(body, '\n'), nil
}

func hexDigest(s string, n int) bool {
	if len(s) != n {
		return false
	}
	for _, c := range s {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func ActiveSourceRetirements(set *store.Set, proj *resolve.Projection) ([]ActiveSourceRetirement, error) {
	if set == nil || proj == nil {
		return nil, fmt.Errorf("source retirement memory is unavailable")
	}
	needed := false
	for _, subject := range set.Subjects() {
		if strings.HasPrefix(subject, "source-retirement/") {
			needed = true
			break
		}
	}
	if !needed {
		return nil, nil
	}
	if err := trustworthyMemory(set, proj); err != nil {
		return nil, err
	}
	var active []ActiveSourceRetirement
	for _, id := range set.IDs() {
		r := set.Get(id)
		if !strings.HasPrefix(r.Subject, "source-retirement/") || proj.Status(id) != resolve.Active {
			continue
		}
		if r.Kind == record.Revocation {
			continue
		}
		if r.Kind != record.Document || r.Family != "seal" || !r.LiteralBody || !r.HasTag("reconcile:source-retirement") {
			return nil, fmt.Errorf("source retirement %s is not a literal seal Document", id)
		}
		receipt, err := ParseSourceRetirement(set.Bodies[id])
		if err != nil {
			return nil, fmt.Errorf("source retirement %s: %w", id, err)
		}
		if r.Subject != SourceRetirementSubject(receipt.Path) || r.Provenance.Source != "source-retirement:"+receipt.Path ||
			r.Provenance.Commit != receipt.SourceCommit || r.Provenance.Note != SourceRetirementSchema || r.BodySHA256 != sourceDigest(set.Bodies[id]) {
			return nil, fmt.Errorf("source retirement %s envelope does not bind its receipt", id)
		}
		active = append(active, ActiveSourceRetirement{id, receipt})
	}
	sort.Slice(active, func(i, j int) bool { return active[i].Receipt.Path < active[j].Receipt.Path })
	return active, nil
}

func standaloneCapture(s Snapshot, name string, set *store.Set, proj *resolve.Projection) (*record.Record, error) {
	if !StandaloneSourcePath(name) {
		return nil, fmt.Errorf("unsupported standalone source %q", name)
	}
	// Repository configuration can place governed documents inside the system
	// namespace. A matching filename must not turn that document into a skill
	// or vendor note eligible for standalone retirement.
	opts := optionDefaults(s.SourceOptions)
	for _, protected := range []string{opts.ProductVision, opts.ArchitecturePrinciples, opts.Incidents} {
		if name == path.Clean(protected) {
			return nil, fmt.Errorf("governed source %s is ineligible for standalone retirement", name)
		}
	}
	for _, root := range []string{opts.PRDs, opts.TechDesigns, opts.Plans, opts.Brainstorms, opts.Problems, opts.Reviews, opts.Feedback} {
		if belowRoot(name, root) {
			return nil, fmt.Errorf("governed source %s belongs to the configured %s corpus", name, root)
		}
	}
	owners, err := ContextSourceOwners(s)
	if err != nil {
		return nil, fmt.Errorf("source ownership: %w", err)
	}
	if len(owners[name]) != 0 {
		return nil, fmt.Errorf("source %s has feature owners %v", name, owners[name])
	}
	f, ok := s.Files[name]
	if !ok || (f.Mode != "100644" && f.Mode != "100755") {
		return nil, fmt.Errorf("source %s is not a regular captured file", name)
	}
	var exact *record.Record
	for _, id := range set.IDs() {
		r := set.Get(id)
		if r.Kind != record.Document || r.Provenance.Source != name || proj.Status(id) != resolve.Active {
			continue
		}
		if exact != nil || !documentMatchesSource(r, set.Bodies[id], f) {
			return nil, fmt.Errorf("source %s has conflicting or inconsistent captures", name)
		}
		exact = r
	}
	if exact == nil {
		return nil, fmt.Errorf("source %s has no active exact literal capture", name)
	}
	return exact, nil
}

// BuildSourceRetirement never deletes a source. Its inputs must already be
// pinned to clean configured mainline by the writer.
func BuildSourceRetirement(name, reason, introduction string, s Snapshot, set *store.Set, proj *resolve.Projection) (Draft, SourceRetirementReceipt, error) {
	active, err := ActiveSourceRetirements(set, proj)
	if err != nil {
		return Draft{}, SourceRetirementReceipt{}, err
	}
	capture, err := standaloneCapture(s, name, set, proj)
	if err != nil {
		return Draft{}, SourceRetirementReceipt{}, err
	}
	f := s.Files[name]
	r := SourceRetirementReceipt{SourceRetirementSchema, name, f.Mode, f.SHA256, s.Commit, introduction, capture.ID, digestJSON(capture), reason}
	body, err := MarshalSourceRetirement(r)
	if err != nil {
		return Draft{}, r, err
	}
	var relations []record.Relation
	for _, old := range active {
		if old.Receipt.Path != name {
			continue
		}
		prior := old.Receipt
		prior.SourceCommit = r.SourceCommit
		if prior == r {
			return Draft{}, old.Receipt, nil
		}
		relations = append(relations, record.Relation{Type: record.Supersedes, Target: old.RecordID})
	}
	return Draft{
		Kind: record.Document, Subject: SourceRetirementSubject(name), Family: "seal",
		Statement: "Retire the exact standalone source " + name + ".",
		Intent:    "Preserve exact source evidence before its later deletion.",
		Rationale: reason, FitCriterion: "The retained mainline, source introduction, bytes, mode and active capture all match this seal.",
		Tags: []string{"reconcile:source-retirement"}, Relations: relations,
		Provenance:  record.Provenance{Source: "source-retirement:" + name, Commit: s.Commit, Note: SourceRetirementSchema},
		LiteralBody: true, Body: body,
	}, r, nil
}

// ValidateSourceRetirement is the common identity predicate for the writer,
// reconciliation and offline verifier. Git ancestry is established by the
// adapter before it supplies Evidence. A nil trusted set never grants absence.
func ValidateSourceRetirement(active ActiveSourceRetirement, e SourceRetirementEvidence, current Snapshot, set *store.Set, proj *resolve.Projection, requireTrusted bool) error {
	r := active.Receipt
	if _, err := MarshalSourceRetirement(r); err != nil {
		return err
	}
	if e.Issued.Commit != r.SourceCommit || e.Introduction != r.Introduction || e.CurrentIntroduction != r.Introduction {
		return fmt.Errorf("source %s has changed introduction or unavailable issuance history", r.Path)
	}
	if e.IssuedSet == nil {
		return fmt.Errorf("source %s issuance memory is unavailable", r.Path)
	}
	issued := projectionOf(e.IssuedSet, e.IssuedProjection)
	if err := trustworthyMemory(e.IssuedSet, issued); err != nil {
		return err
	}
	capture, err := standaloneCapture(e.Issued, r.Path, e.IssuedSet, issued)
	if err != nil {
		return err
	}
	file := e.Issued.Files[r.Path]
	if capture.ID != r.DocumentID || digestJSON(capture) != r.DocumentSHA256 || file.Mode != r.Mode || file.SHA256 != r.SHA256 {
		return fmt.Errorf("source %s issuance identity differs from its seal", r.Path)
	}
	// For an absent path, validate its retained literal capture using the exact
	// historical file; no manufactured file enters the reconciliation snapshot.
	currentSource := current
	currentSource.Files = make(map[string]File, len(current.Files)+1)
	maps.Copy(currentSource.Files, current.Files)
	if f, present := current.Files[r.Path]; present {
		if f.Mode != r.Mode || f.SHA256 != r.SHA256 {
			return fmt.Errorf("source %s changed after sealing", r.Path)
		}
	} else {
		currentSource.Files[r.Path] = file
		if e.BeforeDeletion == nil || e.BeforeDeletion.Mode != r.Mode || e.BeforeDeletion.SHA256 != r.SHA256 {
			return fmt.Errorf("source %s changed before deletion or its deletion history is unavailable", r.Path)
		}
	}
	bound, err := standaloneCapture(currentSource, r.Path, set, proj)
	if err != nil {
		return err
	}
	if bound.ID != r.DocumentID || digestJSON(bound) != r.DocumentSHA256 {
		return fmt.Errorf("source %s bound capture is revoked, superseded or changed", r.Path)
	}
	if !requireTrusted {
		return nil
	}
	if e.TrustedSet == nil || e.TrustedIntroduction != r.Introduction {
		return fmt.Errorf("source %s has no trusted receipt before deletion", r.Path)
	}
	trusted := projectionOf(e.TrustedSet, e.TrustedProjection)
	seals, err := ActiveSourceRetirements(e.TrustedSet, trusted)
	if err != nil {
		return err
	}
	found := false
	for _, seal := range seals {
		if seal.RecordID == active.RecordID && seal.Receipt == r {
			found = true
		}
	}
	if !found || digestJSON(e.TrustedSet.Get(active.RecordID)) != digestJSON(set.Get(active.RecordID)) {
		return fmt.Errorf("source %s receipt was not active at the trusted base before deletion", r.Path)
	}
	prior, err := standaloneCapture(e.Trusted, r.Path, e.TrustedSet, trusted)
	if err != nil {
		return err
	}
	f := e.Trusted.Files[r.Path]
	if prior.ID != r.DocumentID || digestJSON(prior) != r.DocumentSHA256 || f.Mode != r.Mode || f.SHA256 != r.SHA256 {
		return fmt.Errorf("source %s differs at the trusted base", r.Path)
	}
	return nil
}

func sourceRetirementBindings(s Snapshot, set *store.Set, proj *resolve.Projection) (map[string]map[string]bool, []Finding) {
	sealed := map[string]map[string]bool{}
	active, err := ActiveSourceRetirements(set, proj)
	var findings []Finding
	add := func(code, source string, err error) {
		findings = append(findings, Finding{Code: code, Scope: source, Detail: err.Error(), Remedy: "restore exact source evidence or issue a valid replacement seal from mainline before deletion", Blocking: true})
	}
	if err != nil {
		add("invalid-source-retirement", "source-retirement/", err)
		return sealed, findings
	}
	for _, a := range active {
		proof, ok := s.SourceRetirements[a.RecordID]
		if !ok || proof.Error != "" {
			add("source-retirement-history-unavailable", a.Receipt.Path, fmt.Errorf("retirement %s: %s", a.RecordID, proof.Error))
			continue
		}
		_, present := s.Files[a.Receipt.Path]
		if err := ValidateSourceRetirement(a, proof.Evidence, s, set, proj, !present); err != nil {
			add("invalid-source-retirement", a.Receipt.Path, err)
			continue
		}
		sealed[a.Receipt.Path] = map[string]bool{a.Receipt.DocumentID: true}
	}
	return sealed, findings
}

// TrustedStandaloneRetirements authorizes only absences whose active receipt
// was proved at the actual trusted predecessor. The same predicate produces
// ordinary reconciliation findings when evidence is missing or invalid.
func TrustedStandaloneRetirements(s Snapshot, set *store.Set, proj *resolve.Projection) map[string]bool {
	bindings, _ := sourceRetirementBindings(s, set, proj) // Findings remain in the candidate report.
	out := map[string]bool{}
	for name := range bindings {
		if _, present := s.Files[name]; !present {
			out[name] = true
		}
	}
	return out
}
