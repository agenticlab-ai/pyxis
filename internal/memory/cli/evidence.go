package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func (s *evidenceSession) bind(ctx context.Context, r *record.Record, source string) (*record.Record, error) {
	copy := *r
	if s.baseSet.Get(r.ID) != nil {
		return nil, fmt.Errorf("%s is frozen at the trusted base; issue a successor", r.ID)
	}
	if len(s.identities[r.ID]) > 0 {
		return nil, fmt.Errorf("%s was previously present on mainline", r.ID)
	}
	if source == "" {
		source = s.head
	}
	var err error
	source, err = s.m.repo.ResolveCommit(ctx, source)
	if err != nil {
		return nil, err
	}
	snapshot, err := s.snapshot(ctx, source)
	if err != nil {
		return nil, err
	}
	locator := record.EvidenceIntroduction
	retained, err := s.isRetained(ctx, source)
	if err != nil {
		return nil, err
	}
	historical := retained && source != s.head
	if historical {
		locator = record.EvidenceMainline
	}
	witness, err := reconcile.CaptureEvidence(r, s.m.set, snapshot, locator)
	if err != nil {
		return nil, err
	}
	if len(witness.Files) == 0 && retained {
		historical = true
		witness.Locator = record.EvidenceMainline
	}
	if r.Provenance.Commit != "" {
		original, err := s.m.repo.ResolveCommit(ctx, r.Provenance.Commit)
		if err != nil {
			return nil, fmt.Errorf("original evidence is unavailable: %w", err)
		}
		originalSnapshot, err := s.snapshot(ctx, original)
		if err != nil {
			return nil, err
		}
		old, err := reconcile.CaptureEvidence(r, s.m.set, originalSnapshot, locator)
		if err != nil {
			return nil, err
		}
		if len(old.Files) == 0 && original != source {
			return nil, fmt.Errorf("commit-only evidence cannot be rebound to a different commit identity")
		}
		if r.Provenance.Evidence != nil && len(reconcile.CompareEvidence(r.Provenance.Evidence, old)) > 0 {
			return nil, fmt.Errorf("existing witness does not match its original declared commit")
		}
		if len(reconcile.CompareEvidence(old, witness)) > 0 {
			return nil, fmt.Errorf("original evidence differs from selected source; no draft was rebound")
		}
	} else if r.Provenance.Evidence != nil && len(reconcile.CompareEvidence(r.Provenance.Evidence, witness)) > 0 {
		return nil, fmt.Errorf("existing witness differs from selected source; no draft was rebound")
	}
	if !historical {
		candidate, err := s.snapshot(ctx, s.head)
		if err != nil {
			return nil, err
		}
		landing, err := reconcile.CaptureEvidence(r, s.m.set, candidate, locator)
		if err != nil {
			return nil, err
		}
		if len(reconcile.CompareEvidence(witness, landing)) > 0 {
			return nil, fmt.Errorf("selected evidence differs from the proposed landing tree")
		}
	}
	if len(witness.Files) == 0 && !retained {
		return nil, fmt.Errorf("commit-only evidence must name a retained mainline commit")
	}
	copy.Provenance.Evidence = witness
	copy.Provenance.Commit = ""
	if historical {
		copy.Provenance.Commit = source
	}
	if err := copy.Validate(); err != nil {
		return nil, err
	}
	return &copy, nil
}

// captureEvidence is shared by every existing record writer. It captures
// only declared repository evidence; plain narrative records need no binding.
func (m *memory) captureEvidence(ctx context.Context, r *record.Record) error {
	files, scopes, err := reconcile.EvidenceTargets(r, m.set)
	if err != nil {
		return err
	}
	if len(files) == 0 && len(scopes) == 0 && r.Provenance.Commit == "" {
		return nil
	}
	s := m.evidence
	if s == nil {
		s, err = newEvidenceSession(ctx, m, "", r)
		if err != nil {
			return err
		}
		m.evidence = s
	} else {
		// A reconciliation batch shares its history and production inventory.
		// An additional explicit source invalidates only snapshot blob caches.
		for _, p := range files {
			if !slices.Contains(s.declared, p) {
				s.declared = append(s.declared, p)
				s.snapshots = map[string]reconcile.EvidenceSnapshot{}
			}
		}
	}
	source := s.head
	if record.ValidCommitID(r.Provenance.Commit) {
		if retained, err := s.isRetained(ctx, r.Provenance.Commit); retained && err == nil {
			source = r.Provenance.Commit
		}
	}
	bound, err := s.bind(ctx, r, source)
	if err != nil {
		return err
	}
	if err := s.cleanSources(ctx, bound); err != nil {
		return err
	}
	if err := s.unchangedPins(ctx); err != nil {
		return err
	}
	r.Provenance = bound.Provenance
	return nil
}

func (s *evidenceSession) cleanSources(ctx context.Context, r *record.Record) error {
	files, scopes, err := reconcile.EvidenceTargets(r, s.m.set)
	if err != nil {
		return err
	}
	dirty, err := s.m.repo.DirtyPaths(ctx, append(files, scopes...))
	if err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("declared evidence sources have uncommitted changes; commit the selected bytes before capture")
	}
	return nil
}

func (s *evidenceSession) unchangedPins(ctx context.Context) error {
	head, err := s.m.repo.Head(ctx)
	if err != nil {
		return err
	}
	if head != s.head {
		return fmt.Errorf("candidate HEAD changed since evidence preflight")
	}
	base, err := s.m.repo.ResolveCommit(ctx, s.baseRef)
	if err != nil {
		return err
	}
	if base != s.base {
		return fmt.Errorf("trusted base changed since evidence preflight")
	}
	return nil
}

type evidenceRepairOutput struct {
	Schema   string           `json:"schema_version"`
	Report   evidenceReport   `json:"report"`
	Proposed []*record.Record `json:"proposed,omitempty"`
	Applied  []string         `json:"applied"`
	Pending  []string         `json:"pending"`
	Failure  string           `json:"failure,omitempty"`
}

func runEvidence(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("evidence", env)
	base := fs.String("base", "", "trusted mainline ref")
	source := fs.String("source", "", "pinned repair source (defaults to HEAD)")
	check := fs.Bool("check", false, "verify without changing records")
	write := fs.Bool("write", false, "bind only the explicitly selected draft records")
	// Older callers may still pass --retain; checks now only emit their report.
	fs.Bool("retain", false, "deprecated compatibility flag; reports are emitted without saving files")
	asJSON := fs.Bool("json", false, "canonical machine output")
	var selected multi
	fs.Var(&selected, "record", "draft record ID or repository-relative path (repeatable)")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) > 0 || (*check && (*write || len(selected) > 0 || *source != "")) || (!*check && len(selected) == 0) {
		return exitUsage, usage("evidence --check [--base REF] [--json], or evidence --record ID [--record ID...] [--base REF] [--source REF] [--write] [--json]")
	}
	var m *memory
	if *write {
		var release func()
		m, release, err = openLocked(ctx, env)
		if err == nil {
			defer release()
		}
	} else {
		m, err = open(ctx, env)
	}
	if err != nil {
		return exitRefused, err
	}
	s, err := newEvidenceSession(ctx, m, *base)
	if err != nil {
		return emitEvidence(env, unavailableEvidence(s, err), *asJSON)
	}
	if *check {
		return emitEvidence(env, s.verify(ctx), *asJSON)
	}
	return repairEvidence(ctx, s, selected, *source, *write, *asJSON, env)
}

func repairEvidence(ctx context.Context, s *evidenceSession, selected []string, source string, write, asJSON bool, env Env) (int, error) {
	if source == "" {
		source = s.head
	} else {
		pin, err := s.m.repo.ResolveCommit(ctx, source)
		if err != nil {
			return exitRefused, err
		}
		source = pin
	}
	out := evidenceRepairOutput{Schema: "memory-evidence-repair/v1", Report: s.verify(ctx), Applied: []string{}, Pending: []string{}}
	seen := map[string]bool{}
	for _, value := range selected {
		id := value
		if strings.Contains(value, "/") {
			id = ""
			for candidate, p := range s.m.set.Paths {
				if value == store.Dir+"/"+p {
					id = candidate
					break
				}
			}
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		r := s.m.set.Get(id)
		if r == nil {
			return exitRefused, fmt.Errorf("record %q is absent or invalid", value)
		}
		bound, err := s.bind(ctx, r, source)
		if err != nil {
			return exitRefused, fmt.Errorf("%s: %w", id, err)
		}
		if err := s.cleanSources(ctx, bound); err != nil {
			return exitRefused, err
		}
		body, err := record.Marshal(bound)
		if err != nil {
			return exitRefused, err
		}
		if bytes.Equal(body, rawRecord(s.m.set, id)) {
			continue
		}
		out.Proposed = append(out.Proposed, bound)
	}
	sort.Slice(out.Proposed, func(i, j int) bool { return out.Proposed[i].Path() < out.Proposed[j].Path() })
	for _, r := range out.Proposed {
		out.Pending = append(out.Pending, r.ID)
	}
	if write {
		if err := s.unchangedPins(ctx); err != nil {
			return exitRefused, err
		}
		// Whole-batch preflight precedes the first replacement.
		for _, r := range out.Proposed {
			b, err := os.ReadFile(filepath.Join(s.m.dir, filepath.FromSlash(r.Path())))
			if err != nil {
				return exitRefused, err
			}
			if !bytes.Equal(b, rawRecord(s.m.set, r.ID)) {
				return exitRefused, fmt.Errorf("draft %s changed since preflight", r.ID)
			}
		}
		for _, r := range out.Proposed {
			if err := s.unchangedPins(ctx); err != nil {
				out.Failure = err.Error()
				break
			}
			if err := s.replaceDraft(s.m.dir, r, rawRecord(s.m.set, r.ID)); err != nil {
				out.Failure = err.Error()
				break
			}
			out.Applied = append(out.Applied, r.ID)
			out.Pending = out.Pending[1:]
		}
	}
	if asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(out); err != nil {
			return exitRefused, err
		}
	} else {
		var render strings.Builder
		_, _ = fmt.Fprintf(&render, "evidence repair: %d proposed, %d applied, %d pending\n", len(out.Proposed), len(out.Applied), len(out.Pending)) // strings.Builder writes cannot fail.
		for _, r := range out.Proposed {
			_, _ = fmt.Fprintf(&render, "%s: %s\n", r.ID, r.Provenance.Evidence.Locator) // strings.Builder writes cannot fail.
		}
		if _, err := fmt.Fprint(env.Stdout, render.String()); err != nil {
			return exitRefused, err
		}
	}
	if out.Failure != "" {
		return exitRefused, fmt.Errorf("draft repair stopped: %s (applied: %s; pending: %s)", out.Failure, strings.Join(out.Applied, ", "), strings.Join(out.Pending, ", "))
	}
	return exitOK, nil
}

func emitEvidence(env Env, report evidenceReport, asJSON bool) (int, error) {
	if asJSON {
		body, err := report.canonical()
		if err != nil {
			return exitRefused, err
		}
		if _, err := env.Stdout.Write(body); err != nil {
			return exitRefused, err
		}
	} else {
		counts := map[string]int{}
		var render strings.Builder
		_, _ = fmt.Fprintf(&render, "evidence pins: base %s; candidate %s\n", report.Base, report.Candidate) // strings.Builder writes cannot fail.
		for _, e := range report.Entries {
			counts[e.Classification]++
			if e.Classification == "invalid" || e.Classification == "history-unavailable" || e.Classification == "historical-unresolved" {
				_, _ = fmt.Fprintf(&render, "%s %s %s: %s\n", e.RecordID, e.SourcePath, e.Code, e.Message) // strings.Builder writes cannot fail.
				if e.Snapshot != "" {
					render.WriteString("  snapshot: " + e.Snapshot + "\n")
				}
				if e.Expected != nil {
					_, _ = fmt.Fprintf(&render, "  expected: %s %s\n", e.Expected.Mode, e.Expected.SHA256) // strings.Builder writes cannot fail.
				}
				if e.Observed != nil {
					_, _ = fmt.Fprintf(&render, "  observed: %s %s\n", e.Observed.Mode, e.Observed.SHA256) // strings.Builder writes cannot fail.
				}
				if e.SuccessorID != "" {
					render.WriteString("  verified successor: " + e.SuccessorID + "; historical witness remains unresolved\n")
				}
			}
		}
		_, _ = fmt.Fprintf(&render, "evidence: %d verified, %d legacy-unverified, %d invalid, %d history-unavailable, %d historical-unresolved\n", counts["verified"], counts["legacy-unverified"], counts["invalid"], counts["history-unavailable"], counts["historical-unresolved"]) // strings.Builder writes cannot fail.
		if _, err := fmt.Fprint(env.Stdout, render.String()); err != nil {
			return exitRefused, err
		}
	}
	if report.blocking() {
		return exitRefused, fmt.Errorf("durable evidence failed; resolve the reported defects and rerun the check")
	}
	return exitOK, nil
}
