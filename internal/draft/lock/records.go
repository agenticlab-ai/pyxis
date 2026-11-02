package lock

import (
	"crypto/rand"
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// writeRecords issues local dependencies before their consumers, resolves
// every relation to an immutable record ID, and writes literal draft documents.
func writeRecords(root, issuer string, result compile.Result, sources readiness.Sources) (map[string]string, []string, error) {
	return writeRecordsWithBinding(root, issuer, result, sources, nil, 1)
}

func writeRecordsWithBinding(root, issuer string, result compile.Result, sources readiness.Sources, binding *resolve.Projection, version int) (map[string]string, []string, error) {
	documents, paths, err := issueRecords(root, issuer, result, sources, binding, version)
	if err != nil {
		return nil, nil, err
	}
	if documents["prd"] == "" || documents["tech-spec"] == "" {
		return nil, nil, fmt.Errorf("compiled PRD and tech spec Documents are required")
	}
	return documents, paths, nil
}

func issueRecords(root, issuer string, result compile.Result, sources readiness.Sources, binding *resolve.Projection, version int) (map[string]string, []string, error) {
	if binding != nil {
		result = versionHistory(result, version)
	}
	entries := result.Records
	local := map[string]int{}
	for i, entry := range entries {
		if entry.Draft.Family == "" {
			continue
		}
		key := entry.Draft.Subject + "#" + entry.Draft.Family
		if _, exists := local[key]; exists {
			return nil, nil, fmt.Errorf("duplicate compiled address %s", key)
		}
		local[key] = i
	}
	var order []int
	state := make([]uint8, len(entries))
	var visit func(int) error
	visit = func(i int) error {
		switch state[i] {
		case 1:
			return fmt.Errorf("compiled relation cycle at %s#%s", entries[i].Draft.Subject, entries[i].Draft.Family)
		case 2:
			return nil
		}
		state[i] = 1
		for _, link := range entries[i].Links {
			if target, ok := local[link.Target]; ok {
				if err := visit(target); err != nil {
					return err
				}
			}
		}
		state[i] = 2
		order = append(order, i)
		return nil
	}
	for i := range entries {
		if err := visit(i); err != nil {
			return nil, nil, err
		}
	}
	changes, revocations, err := compareRecords(entries, result.Feature, binding, sources.Mainline, order)
	if err != nil {
		return nil, nil, err
	}
	var predecessors []string
	for _, projection := range []*resolve.Projection{binding, sources.Mainline} {
		if projection != nil {
			for id := range projection.Set.Records {
				predecessors = append(predecessors, id)
			}
		}
	}
	count := uint64(len(revocations))
	for i, entry := range entries {
		if changes[i].Verdict != Unchanged {
			count++
		}
		// Native captures carry literal ID relations even without a binding
		// projection; compiled links otherwise resolve into the source or batch.
		for _, rel := range entry.Draft.Relations {
			predecessors = append(predecessors, rel.Target)
		}
	}
	started, err := record.BatchTime(time.Now().UTC(), count, predecessors...)
	if err != nil {
		return nil, nil, fmt.Errorf("mint compiled record chronology: %w", err)
	}
	next := started.UnixMilli()
	memoryRoot := filepath.Join(root, store.Dir)
	issued := map[string]string{}
	documents := map[string]string{}
	var paths []string
	for _, i := range order {
		entry := entries[i]
		change := changes[i]
		if change.Verdict == Unchanged {
			if entry.Draft.Family != "" {
				issued[entry.Draft.Subject+"#"+entry.Draft.Family] = change.Previous.ID
			}
			if entry.Draft.Kind == record.Document {
				setDocumentPath(documents, result.Feature, entry.Draft.Subject, entry.Draft.Family, filepath.ToSlash(filepath.Join(store.Dir, change.Previous.BodyPath())))
			}
			continue
		}
		var entropy [10]byte
		if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
			return nil, nil, fmt.Errorf("mint record identity: %w", err)
		}
		at := time.UnixMilli(next).UTC()
		next++
		d := entry.Draft
		r := &record.Record{
			Schema: record.Schema, ID: record.NewID(d.Kind, at, entropy),
			Kind: d.Kind, Subject: d.Subject, Family: d.Family,
			Statement: d.Statement, Intent: d.Intent, Rationale: d.Rationale,
			FitCriterion: d.FitCriterion, Tags: append([]string(nil), d.Tags...),
			Consequence: d.Consequence, LiteralBody: d.LiteralBody,
			Issuer: issuer, CreatedAt: at.Format(time.RFC3339),
			Provenance: d.Provenance,
			Relations:  append([]record.Relation(nil), d.Relations...),
		}
		for _, link := range entry.Links {
			target := issued[link.Target]
			if target == "" && sources.Mainline != nil {
				subject, family, ok := strings.Cut(link.Target, "#")
				if ok {
					if head := sources.Mainline.Head(subject, family); head != nil {
						target = head.ID
					}
				}
			}
			if target == "" {
				return nil, nil, fmt.Errorf("resolve compiled %s relation to %s: target absent", link.Type, link.Target)
			}
			r.Relations = append(r.Relations, record.Relation{Type: link.Type, Target: target})
		}
		if change.Verdict == Superseding {
			r.Relations = append(r.Relations, record.Relation{Type: record.Supersedes, Target: change.Previous.ID})
		}
		sort.Strings(r.Tags)
		sort.Slice(r.Relations, func(i, j int) bool {
			if r.Relations[i].Type != r.Relations[j].Type {
				return r.Relations[i].Type < r.Relations[j].Type
			}
			return r.Relations[i].Target < r.Relations[j].Target
		})
		if d.Kind == record.Document {
			if _, err := store.WriteDocument(memoryRoot, r, d.Body); err != nil {
				return nil, nil, fmt.Errorf("write compiled Document %s#%s: %w", d.Subject, d.Family, err)
			}
			paths = append(paths, filepath.ToSlash(filepath.Join(store.Dir, r.BodyPath())))
			setDocumentPath(documents, result.Feature, d.Subject, d.Family, filepath.ToSlash(filepath.Join(store.Dir, r.BodyPath())))
		} else {
			if _, err := store.Write(memoryRoot, r); err != nil {
				return nil, nil, fmt.Errorf("write compiled %s %s#%s: %w", d.Kind, d.Subject, d.Family, err)
			}
		}
		paths = append(paths, filepath.ToSlash(filepath.Join(store.Dir, r.Path())))
		if d.Family != "" {
			issued[d.Subject+"#"+d.Family] = r.ID
		}
	}
	for _, old := range revocations {
		var entropy [10]byte
		if _, err := io.ReadFull(rand.Reader, entropy[:]); err != nil {
			return nil, nil, fmt.Errorf("mint revocation identity: %w", err)
		}
		at := time.UnixMilli(next).UTC()
		next++
		label := old.Family
		if label == "" {
			label = old.ID
		}
		r := &record.Record{
			Schema: record.Schema, ID: record.NewID(record.Revocation, at, entropy),
			Kind: record.Revocation, Subject: old.Subject,
			Statement: "Withdraw " + label + " from draft contract", Issuer: issuer,
			CreatedAt: at.Format(time.RFC3339),
			Relations: []record.Relation{{Type: record.Revokes, Target: old.ID}},
		}
		if _, err := store.Write(memoryRoot, r); err != nil {
			return nil, nil, fmt.Errorf("revoke %s: %w", old.ID, err)
		}
		paths = append(paths, filepath.ToSlash(filepath.Join(store.Dir, r.Path())))
	}
	return documents, paths, nil
}

func setDocumentPath(documents map[string]string, feature, subject, family, path string) {
	switch {
	case subject == "prd/"+feature && family == "prd":
		documents["prd"] = path
	case subject == "prd/"+feature && strings.HasPrefix(family, "problems"):
		documents["problems"] = path
	case subject == "design/"+feature && family == "design":
		documents["tech-spec"] = path
	}
}
