package cli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/packet"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

const evidenceReportSchema = "memory-evidence-report/v1"

type evidenceEntry struct {
	RecordID       string               `json:"record_id,omitempty"`
	RecordPath     string               `json:"record_path,omitempty"`
	RecordSHA256   string               `json:"record_sha256,omitempty"`
	Locator        string               `json:"locator,omitempty"`
	Snapshot       string               `json:"snapshot,omitempty"`
	Classification string               `json:"classification"`
	Code           string               `json:"code,omitempty"`
	SourcePath     string               `json:"source_path,omitempty"`
	Expected       *record.EvidenceFile `json:"expected,omitempty"`
	Observed       *record.EvidenceFile `json:"observed,omitempty"`
	ExpectedUnit   *record.EvidenceUnit `json:"expected_unit,omitempty"`
	ObservedUnit   *record.EvidenceUnit `json:"observed_unit,omitempty"`
	Message        string               `json:"message,omitempty"`
	Recovery       string               `json:"recovery,omitempty"`
	SuccessorID    string               `json:"verified_successor,omitempty"`
}

type evidenceReport struct {
	Schema    string          `json:"schema_version"`
	Base      string          `json:"base"`
	Candidate string          `json:"candidate"`
	Entries   []evidenceEntry `json:"entries"`
}

func (r evidenceReport) blocking() bool {
	for _, e := range r.Entries {
		if e.Classification == "invalid" || e.Classification == "history-unavailable" {
			return true
		}
	}
	return false
}

func (r evidenceReport) canonical() ([]byte, error) {
	b, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode evidence report: %w", err)
	}
	return append(b, '\n'), nil
}

// evidenceSession owns all pins and caches for one operation. Neither ref
// resolution nor complete-history traversal runs once per record.
type evidenceSession struct {
	m            *memory
	base         string
	baseRef      string
	head         string
	baseSet      *store.Set
	changes      map[string][]gitsrc.TreeChange
	identities   map[string][]string
	declared     []string
	snapshots    map[string]reconcile.EvidenceSnapshot
	retained     map[string]bool
	readErrors   map[string]error
	replaceDraft func(string, *record.Record, []byte) error
}

func newEvidenceSession(ctx context.Context, m *memory, baseRef string, extra ...*record.Record) (*evidenceSession, error) {
	head, err := m.repo.Head(ctx)
	if err != nil {
		return nil, err
	}
	if baseRef == "" {
		baseRef = m.repo.DefaultBase(ctx, m.remote, m.branch)
	}
	if baseRef == "" {
		return nil, fmt.Errorf("evidence-history-unavailable: supply --base or fetch the configured mainline")
	}
	base, err := m.repo.ResolveCommit(ctx, baseRef)
	if err != nil {
		return nil, err
	}
	s := &evidenceSession{m: m, base: base, baseRef: baseRef, head: head}
	if err := m.repo.FullHistory(ctx); err != nil {
		return s, err
	}
	ancestor, err := m.repo.Ancestor(ctx, base, head)
	if err != nil {
		return s, err
	}
	if !ancestor {
		return s, fmt.Errorf("evidence-history-unavailable: trusted base is not an ancestor of candidate; refresh and revalidate the integration tree")
	}
	baseSet, err := m.baseSet(ctx, base)
	if err != nil {
		return s, err
	}
	s.baseSet = baseSet
	s.changes = map[string][]gitsrc.TreeChange{}
	s.identities = map[string][]string{}
	s.snapshots = map[string]reconcile.EvidenceSnapshot{}
	s.retained = map[string]bool{base: true}
	s.readErrors = map[string]error{}
	s.replaceDraft = store.ReplaceDraftEvidence
	changes, err := m.repo.FirstParentChanges(ctx, base, store.Dir)
	if err != nil {
		return s, fmt.Errorf("evidence-history-unavailable: %w", err)
	}
	for _, change := range changes {
		s.changes[change.Path] = append(s.changes[change.Path], change)
		id := strings.TrimSuffix(path.Base(change.Path), ".json")
		s.identities[id] = append(s.identities[id], change.Path)
	}
	records := append([]*record.Record(nil), extra...)
	for _, id := range m.set.IDs() {
		records = append(records, m.set.Get(id))
	}
	for _, r := range records {
		files, _, err := reconcile.EvidenceTargets(r, m.set)
		if err == nil {
			s.declared = append(s.declared, files...)
		}
		if e := r.Provenance.Evidence; e != nil {
			s.declared = append(s.declared, store.Dir+"/"+r.Path())
			for _, f := range e.Files {
				s.declared = append(s.declared, f.Path)
			}
		}
	}
	sort.Strings(s.declared)
	s.snapshots = m.pinned.evidenceSnapshots(s.declared)
	return s, nil
}

func (s *evidenceSession) snapshot(ctx context.Context, pin string) (reconcile.EvidenceSnapshot, error) {
	if snap, ok := s.snapshots[pin]; ok {
		return snap, nil
	}
	snap, err := reconcile.BuildEvidenceSnapshot(ctx, s.m.repo, pin, s.declared)
	if err != nil {
		return snap, err
	}
	s.snapshots[pin] = snap
	return snap, nil
}

func (s *evidenceSession) isRetained(ctx context.Context, pin string) (bool, error) {
	if !record.ValidCommitID(pin) {
		return false, nil
	}
	if answer, ok := s.retained[pin]; ok {
		return answer, s.readErrors[pin]
	}
	ok, err := s.m.repo.Ancestor(ctx, pin, s.base)
	s.retained[pin] = ok
	s.readErrors[pin] = err
	return ok, err
}

func (s *evidenceSession) verify(ctx context.Context) evidenceReport {
	report := evidenceReport{Schema: evidenceReportSchema, Base: s.base, Candidate: s.head, Entries: []evidenceEntry{}}
	for _, problem := range s.m.set.Problems {
		report.Entries = append(report.Entries, evidenceEntry{RecordPath: store.Dir + "/" + problem.Path, Classification: "invalid", Code: "invalid-record-schema", Message: problem.Err.Error()})
	}
	for _, id := range s.m.set.IDs() {
		report.Entries = append(report.Entries, s.verifyRecord(ctx, s.m.set.Get(id))...)
	}
	s.classifyRecoveredLearnings(ctx, &report)
	var retired map[string]bool
	retirementChecked := false
	for _, id := range s.baseSet.IDs() {
		if s.m.set.Get(id) == nil && s.baseSet.Get(id).Provenance.Evidence != nil {
			if !retirementChecked {
				retired = packet.BookkeepingRetirements(s.baseSet, s.baseSet, s.m.set, s.m.set)
				retirementChecked = true
			}
			if retired[s.baseSet.Paths[id]] {
				continue
			}
			report.Entries = append(report.Entries, evidenceEntry{RecordID: id, RecordPath: store.Dir + "/" + s.baseSet.Paths[id], Classification: "invalid", Code: "evidence-record-rewritten", Message: "an inherited immutable identity was removed"})
		}
	}
	if err := s.unchangedPins(ctx); err != nil {
		report.Entries = append(report.Entries, unavailableEvidence(s, err).Entries...)
	}
	sort.Slice(report.Entries, func(i, j int) bool {
		a, b := report.Entries[i], report.Entries[j]
		return a.RecordPath+"\x00"+a.SourcePath+"\x00"+a.Code < b.RecordPath+"\x00"+b.SourcePath+"\x00"+b.Code
	})
	return report
}

// A verified successor repairs use of an inherited Learning, not its failed
// historical witness. Keep the original defect, snapshot and observed bytes
// visible. New invalid records and failures of identity/history remain blocking.
func (s *evidenceSession) classifyRecoveredLearnings(ctx context.Context, report *evidenceReport) {
	if len(s.m.set.Problems) != 0 {
		return
	}
	entries := map[string][]evidenceEntry{}
	for _, e := range report.Entries {
		entries[e.RecordID] = append(entries[e.RecordID], e)
	}
	p := s.m.projection(s.m.set)
	if len(p.Violations) != 0 || len(s.m.set.Dangling) != 0 {
		return
	}
	baseMemory := *s.m
	baseMemory.set = s.baseSet
	baseSession := *s
	baseSession.m, baseSession.head = &baseMemory, s.base
	replacements := map[string]string{}
	for _, id := range s.m.set.IDs() {
		r := s.m.set.Get(id)
		verdict := entries[id]
		if r.Kind != record.Learning || r.Provenance.Evidence == nil || p.Status(id) == resolve.Revoked || len(verdict) != 1 || verdict[0].Classification != "verified" {
			continue
		}
		for _, target := range p.Targets(r, record.Supersedes) {
			old := s.baseSet.Get(target)
			if old == nil || old.Kind != record.Learning || old.Provenance.Evidence == nil || replacements[target] != "" || p.Status(target) != resolve.Superseded || !bytes.Equal(rawRecord(s.baseSet, target), rawRecord(s.m.set, target)) {
				continue
			}
			if witnessMismatch(entries[target]) && witnessMismatch(baseSession.verifyRecord(ctx, old)) {
				replacements[target] = id
			}
		}
	}
	for i := range report.Entries {
		e := &report.Entries[i]
		if successor := replacements[e.RecordID]; successor != "" {
			e.Classification = "historical-unresolved"
			e.SuccessorID = successor
			e.Recovery = "Retained failed witness; use verified successor " + successor + ". The predecessor's historical evidence remains unresolved."
		}
	}
}

func witnessMismatch(entries []evidenceEntry) bool {
	if len(entries) == 0 {
		return false
	}
	for _, e := range entries {
		if e.Classification != "invalid" || e.Snapshot == "" || (e.Code != "evidence-content-mismatch" && e.Code != "evidence-membership-mismatch") {
			return false
		}
	}
	return true
}

func rawRecord(set *store.Set, id string) []byte {
	p := set.Paths[id]
	if b, ok := set.RawBytes[p]; ok {
		return b
	}
	return set.Bytes[p] // In-memory fixtures predating raw-byte retention.
}

func (s *evidenceSession) verifyRecord(ctx context.Context, r *record.Record) []evidenceEntry {
	e := r.Provenance.Evidence
	entry := evidenceEntry{RecordID: r.ID, RecordPath: store.Dir + "/" + r.Path(), RecordSHA256: fmt.Sprintf("%x", sha256.Sum256(rawRecord(s.m.set, r.ID))), Classification: "verified"}
	fail := func(code, message string) []evidenceEntry {
		entry.Classification = "invalid"
		entry.Code = code
		entry.Message = message
		if code == "evidence-history-unavailable" {
			entry.Classification = "history-unavailable"
		}
		entry.Recovery = "memory evidence --base " + s.base + " --source " + s.head + " --record " + r.ID + " (preview; add --write only for a draft)"
		return []evidenceEntry{entry}
	}
	inherited := s.baseSet.Get(r.ID) != nil
	if inherited && !bytes.Equal(rawRecord(s.baseSet, r.ID), rawRecord(s.m.set, r.ID)) {
		return fail("evidence-record-rewritten", "inherited record bytes changed; issue a successor")
	}
	// Identity remains immutable after retirement, even when a replacement
	// drops every repository-evidence field or moves to another subject.
	if !inherited && len(s.identities[r.ID]) > 0 {
		return fail("evidence-record-rewritten", "a removed mainline identity cannot be reintroduced")
	}
	files, scopes, err := reconcile.EvidenceTargets(r, s.m.set)
	required := len(files) > 0 || len(scopes) > 0 || r.Provenance.Commit != "" || err != nil
	if e == nil {
		if !required {
			entry.Classification = "not-applicable"
			return []evidenceEntry{entry}
		}
		if !inherited {
			return fail("missing-durable-evidence", "new repository evidence requires a versioned binding")
		}
		entry.Classification = "legacy-unverified"
		entry.Code = "legacy-unverified"
		// Legacy commit provenance remains reproducible when the commit is
		// retained and its exact declared files can still be selected there.
		if err == nil {
			if ok, readErr := s.isRetained(ctx, r.Provenance.Commit); ok && readErr == nil {
				if snap, readErr := s.snapshot(ctx, r.Provenance.Commit); readErr == nil {
					if _, readErr := reconcile.CaptureEvidence(r, s.m.set, snap, record.EvidenceMainline); readErr == nil {
						entry.Classification = "verified"
						entry.Code = ""
						entry.Locator = record.EvidenceMainline
						entry.Snapshot = r.Provenance.Commit
					}
				}
			}
		}
		return []evidenceEntry{entry}
	}
	entry.Locator = e.Locator
	if err != nil {
		return fail("evidence-membership-mismatch", err.Error())
	}
	if err := r.ValidateEvidence(); err != nil {
		return fail("invalid-record-schema", err.Error())
	}
	changes := s.changes[entry.RecordPath]
	if inherited && (len(changes) != 1 || changes[0].OldMode != "000000" || len(s.identities[r.ID]) != 1) {
		return fail("evidence-record-rewritten", "record introduction is missing, rewritten, or ambiguous")
	}
	pin := r.Provenance.Commit
	if e.Locator == record.EvidenceIntroduction {
		if inherited {
			pin = changes[0].Commit
		} else {
			pin = s.head
		}
	} else {
		entry.Snapshot = pin
		ok, err := s.isRetained(ctx, pin)
		if err != nil {
			return fail("evidence-history-unavailable", "cannot read the declared commit ancestry")
		}
		if !ok {
			return fail("non-durable-evidence-commit", "declared commit is not reachable from the trusted base")
		}
	}
	entry.Snapshot = pin
	snap, err := s.snapshot(ctx, pin)
	if err != nil {
		return fail("evidence-history-unavailable", "cannot read the pinned evidence tree")
	}
	if inherited && e.Locator == record.EvidenceIntroduction && !bytes.Equal(snap.Files[entry.RecordPath], rawRecord(s.m.set, r.ID)) {
		return fail("evidence-record-rewritten", "record differs from its first mainline introduction")
	}
	observed, err := reconcile.CaptureEvidence(r, s.m.set, snap, e.Locator)
	if err != nil {
		return fail("evidence-membership-mismatch", err.Error())
	}
	differences := reconcile.CompareEvidence(e, observed)
	if len(differences) == 0 {
		return []evidenceEntry{entry}
	}
	var entries []evidenceEntry
	for _, d := range differences {
		bad := fail(d.Code, "witness differs from exact selected source bytes or membership")[0]
		bad.SourcePath = d.Path
		bad.Expected = d.Expected
		bad.Observed = d.Observed
		bad.ExpectedUnit = d.ExpectedUnit
		bad.ObservedUnit = d.ObservedUnit
		entries = append(entries, bad)
	}
	return entries
}

func (m *memory) evidenceCheck(ctx context.Context, base string) evidenceReport {
	s, err := newEvidenceSession(ctx, m, base)
	if err != nil {
		return unavailableEvidence(s, err)
	}
	return s.verify(ctx)
}

func unavailableEvidence(s *evidenceSession, causes ...error) evidenceReport {
	r := evidenceReport{Schema: evidenceReportSchema, Entries: []evidenceEntry{{Classification: "history-unavailable", Code: "evidence-history-unavailable", Message: "Cannot verify complete retained history and base ancestry for these pins.", Recovery: "Use a full local clone, refresh the trusted base, and revalidate the candidate integration tree."}}}
	if s != nil {
		r.Base = s.base
		r.Candidate = s.head
	}
	for _, cause := range causes {
		if cause != nil {
			r.Entries[0].Message += " " + cause.Error()
		}
	}
	return r
}

// Evidence is a separate blocking predicate. It never becomes a debt
// allowance, including when a caller asks to regenerate that inventory.
func enforceEvidence(ctx context.Context, m *memory, base string, env Env) (int, error) {
	report := m.evidenceCheck(ctx, base)
	if report.blocking() {
		return emitEvidence(env, report, true)
	}
	return exitOK, nil
}
