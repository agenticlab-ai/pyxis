package reconcile

import (
	"context"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// EvidenceTargets is the single mechanical selector shared by capture and
// verification. Narrative learned_from values remain attribution. A literal
// repository path (optionally followed by a #fragment) declares file evidence.
func EvidenceTargets(r *record.Record, lookup association.Lookup) (files, scopes []string, err error) {
	addFile := func(value string) error {
		p, _, _ := strings.Cut(value, "#")
		if !record.ValidEvidencePath(p) {
			return fmt.Errorf("invalid declared repository path %q", value)
		}
		if p == "memory/"+r.Path() {
			return fmt.Errorf("evidence cannot name its own record envelope")
		}
		files = append(files, p)
		return nil
	}
	source := r.Provenance.Source
	if strings.ContainsAny(source, "\x00\r\n") {
		return nil, nil, fmt.Errorf("invalid declared source %q", source)
	}
	if after, ok := strings.CutPrefix(source, "code:"); ok {
		scopes = append(scopes, after)
	} else if repositoryReference(source) || (strings.Contains(source, "/") && !strings.Contains(source, ":")) {
		if err := addFile(source); err != nil {
			return nil, nil, err
		}
	}
	for _, value := range r.LearnedFrom {
		if repositoryReference(value) {
			if err := addFile(value); err != nil {
				return nil, nil, err
			}
		}
	}
	scopes = append(scopes, association.Scopes(r, lookup)...)
	if slices.Contains(r.Tags, "code:") {
		return nil, nil, fmt.Errorf("empty declared code scope")
	}
	for _, scope := range scopes {
		if !record.ValidEvidencePath(scope) {
			return nil, nil, fmt.Errorf("invalid code scope %q", scope)
		}
	}
	return uniqueEvidencePaths(files), uniqueEvidencePaths(scopes), nil
}

func repositoryReference(value string) bool {
	if value == "" {
		return false
	}
	// Typed citations and URLs are attribution, not paths. A colon in an
	// apparent local path remains a malformed declaration, never a waiver.
	colon := strings.IndexByte(value, ':')
	slash := strings.IndexByte(value, '/')
	if colon >= 0 && (slash < 0 || colon < slash) && !strings.HasPrefix(value, "/") {
		return false
	}
	if strings.ContainsAny(value, " \t\r\n") {
		return false
	}
	return slash >= 0 || strings.Contains(value, ".")
}

func uniqueEvidencePaths(in []string) []string {
	sort.Strings(in)
	out := in[:0]
	for _, p := range in {
		if len(out) == 0 || out[len(out)-1] != p {
			out = append(out, p)
		}
	}
	return out
}

// EvidenceSnapshot is an immutable tree input. Blob reads are batched once
// per source snapshot, including all explicitly declared files and all
// production candidates needed to prove exact unit membership.
type EvidenceSnapshot struct {
	Commit  string
	Entries map[string]gitsrc.TreeEntry
	Files   map[string][]byte
	Units   []codeinventory.Unit
}

func BuildEvidenceSnapshot(ctx context.Context, repo *gitsrc.Repo, commit string, declared []string) (EvidenceSnapshot, error) {
	entries, err := repo.EvidenceTree(ctx, commit)
	if err != nil {
		return EvidenceSnapshot{}, err
	}
	s := EvidenceSnapshot{Commit: commit, Entries: map[string]gitsrc.TreeEntry{}}
	wanted := map[string]bool{}
	for _, p := range declared {
		if !record.ValidEvidencePath(p) {
			return s, fmt.Errorf("invalid evidence path %q", p)
		}
		wanted[p] = true
	}
	var inventory []codeinventory.Entry
	for _, e := range entries {
		s.Entries[e.Path] = e
		if codeinventory.CandidatePath(e.Path) {
			inventory = append(inventory, codeinventory.Entry{Path: e.Path, Mode: e.Mode, Type: e.Type})
			wanted[e.Path] = true
		}
	}
	var paths []string
	for p := range wanted {
		if s.Entries[p].Regular() {
			paths = append(paths, p)
		}
	}
	sort.Strings(paths)
	s.Files, err = repo.Files(ctx, commit, paths)
	if err != nil {
		return s, err
	}
	for _, p := range paths {
		if _, ok := s.Files[p]; !ok {
			return s, fmt.Errorf("evidence-history-unavailable: missing blob %s at %s", p, commit)
		}
	}
	inv, err := codeinventory.Build(inventory, s.Files)
	if err != nil {
		return s, err
	}
	s.Units = inv.Units
	return s, nil
}

// CaptureEvidence is pure: identity, prose, timestamps and entropy play no
// part in its result. Only declarations and exact pinned bytes are selected.
func CaptureEvidence(r *record.Record, lookup association.Lookup, s EvidenceSnapshot, locator string) (*record.Evidence, error) {
	paths, scopes, err := EvidenceTargets(r, lookup)
	if err != nil {
		return nil, err
	}
	e := &record.Evidence{Schema: record.EvidenceSchema, Locator: locator}
	for _, scope := range scopes {
		matched := false
		for _, u := range s.Units {
			if !scopeMatchesUnit(scope, u) {
				continue
			}
			matched = true
			paths = append(paths, u.Files...)
			e.Units = append(e.Units, record.EvidenceUnit{Inventory: record.EvidenceInventory, Domain: record.EvidenceUnitDomain, Kind: string(u.Kind), Path: u.Path, SHA256: u.SHA256, Files: append([]string(nil), u.Files...)})
		}
		if !matched {
			return nil, fmt.Errorf("evidence-membership-mismatch: code scope %s selects no production unit at %s", scope, s.Commit)
		}
	}
	sort.Slice(e.Units, func(i, j int) bool {
		return e.Units[i].Path+"\x00"+e.Units[i].Kind < e.Units[j].Path+"\x00"+e.Units[j].Kind
	})
	units := e.Units[:0]
	for _, u := range e.Units {
		if len(units) == 0 || units[len(units)-1].Path != u.Path || units[len(units)-1].Kind != u.Kind {
			units = append(units, u)
		}
	}
	e.Units = units
	for _, p := range uniqueEvidencePaths(paths) {
		entry, ok := s.Entries[p]
		if !ok || !entry.Regular() {
			return nil, fmt.Errorf("evidence-content-mismatch: %s is missing or not a regular file at %s", p, s.Commit)
		}
		body, ok := s.Files[p]
		if !ok {
			return nil, fmt.Errorf("evidence-history-unavailable: blob %s was not read at %s", p, s.Commit)
		}
		e.Files = append(e.Files, record.EvidenceFile{Path: p, Mode: entry.Mode, SHA256: sourceDigest(body)})
	}
	return e, nil
}

// EvidenceDifference is the exact expected/observed witness mismatch.
type EvidenceDifference struct {
	Code         string               `json:"code"`
	Path         string               `json:"path,omitempty"`
	Expected     *record.EvidenceFile `json:"expected,omitempty"`
	Observed     *record.EvidenceFile `json:"observed,omitempty"`
	ExpectedUnit *record.EvidenceUnit `json:"expected_unit,omitempty"`
	ObservedUnit *record.EvidenceUnit `json:"observed_unit,omitempty"`
}

func CompareEvidence(expected, observed *record.Evidence) []EvidenceDifference {
	var out []EvidenceDifference
	want, got := map[string]record.EvidenceFile{}, map[string]record.EvidenceFile{}
	var paths []string
	for _, f := range expected.Files {
		want[f.Path] = f
		paths = append(paths, f.Path)
	}
	for _, f := range observed.Files {
		got[f.Path] = f
		paths = append(paths, f.Path)
	}
	for _, p := range uniqueEvidencePaths(paths) {
		w, wok := want[p]
		g, gok := got[p]
		if wok && gok && w == g {
			continue
		}
		d := EvidenceDifference{Code: "evidence-content-mismatch", Path: p}
		if wok {
			d.Expected = &w
		}
		if gok {
			d.Observed = &g
		}
		if !wok || !gok {
			d.Code = "evidence-membership-mismatch"
		}
		out = append(out, d)
	}
	wantUnits, gotUnits := map[string]record.EvidenceUnit{}, map[string]record.EvidenceUnit{}
	var unitKeys []string
	for _, u := range expected.Units {
		key := u.Path + "\x00" + u.Kind
		wantUnits[key] = u
		unitKeys = append(unitKeys, key)
	}
	for _, u := range observed.Units {
		key := u.Path + "\x00" + u.Kind
		gotUnits[key] = u
		unitKeys = append(unitKeys, key)
	}
	for _, key := range uniqueEvidencePaths(unitKeys) {
		w, wok := wantUnits[key]
		g, gok := gotUnits[key]
		if wok && gok && reflect.DeepEqual(w, g) {
			continue
		}
		p, _, _ := strings.Cut(key, "\x00")
		d := EvidenceDifference{Code: "evidence-membership-mismatch", Path: p}
		if wok {
			d.ExpectedUnit = &w
		}
		if gok {
			d.ObservedUnit = &g
		}
		out = append(out, d)
	}
	return out
}

// CurrentUnitEvidence is the transition predicate: a historical association
// cannot acknowledge newer production bytes with an older witness.
func CurrentUnitEvidence(r *record.Record, u codeinventory.Unit) bool {
	if r.Provenance.Evidence == nil {
		return false
	}
	for _, w := range r.Provenance.Evidence.Units {
		if w.Kind == string(u.Kind) && w.Path == u.Path && w.SHA256 == u.SHA256 && reflect.DeepEqual(w.Files, u.Files) {
			return true
		}
	}
	return false
}

func hasSourceLocator(r *record.Record) bool {
	return validGitObjectID(r.Provenance.Commit) || (r.Provenance.Evidence != nil && r.Provenance.Evidence.Locator == record.EvidenceIntroduction && r.ValidateEvidence() == nil)
}
