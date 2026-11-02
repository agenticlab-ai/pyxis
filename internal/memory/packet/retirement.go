package packet

import (
	"bytes"
	"context"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"io"
	"maps"
	"regexp"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// bookkeepingRetirement pins the exact records selected in the operator's
// September 29 cleanup. It grants no authority to delete another record, edit
// a retained record, or leave references or historical effects behind.
//
//go:embed bookkeeping-retirement.json
var bookkeepingRetirement []byte

type retirementManifest struct {
	Schema      string            `json:"schema_version"`
	Source      string            `json:"source_commit"`
	Authority   string            `json:"authority"`
	Instruction string            `json:"operator_instruction"`
	Records     []retirementEntry `json:"records"`
}

type retirementEntry struct {
	Path   string `json:"path"`
	SHA256 string `json:"sha256"`
	Reason string `json:"reason"`
}

// BookkeepingRetirements returns exact approved deleted paths shared by the
// packet and durable-evidence gates. The caller supplies the trusted base and
// proposed graph; the exception preserves all retained references and status.
func BookkeepingRetirements(ancestor, base, head, union *store.Set) map[string]bool {
	var manifest retirementManifest
	dec := json.NewDecoder(bytes.NewReader(bookkeepingRetirement))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&manifest); err != nil {
		return nil
	}
	var extra json.RawMessage
	if dec.Decode(&extra) != io.EOF || manifest.Schema != "memory-bookkeeping-retirement/v1" ||
		!record.ValidCommitID(manifest.Source) || manifest.Authority == "" || manifest.Instruction == "" {
		return nil
	}
	approved := map[string]string{}
	for _, entry := range manifest.Records {
		if entry.Reason == "" || approved[entry.Path] != "" {
			return nil
		}
		approved[entry.Path] = entry.SHA256
	}
	deleted := map[string]bool{}
	for _, id := range ancestor.IDs() {
		r := ancestor.Get(id)
		switch r.Kind {
		case record.Need, record.Learning, record.Decision, record.Revocation:
		default:
			continue // This cleanup cannot remove contracts, Documents, or policy.
		}
		_, onHead := head.Bytes[r.Path()]
		_, onUnion := union.Bytes[r.Path()]
		if !onHead && !onUnion && matchesRetirement(approved[r.Path()], ancestor.RawBytes[r.Path()]) &&
			matchesRetirement(approved[r.Path()], base.RawBytes[r.Path()]) && sameRecordBytes(id, ancestor, base) {
			deleted[id] = true
		}
	}
	if len(deleted) == 0 {
		return nil
	}
	dependencies := retirementDependencies(ancestor, base, head, union)
	for changed := true; changed; {
		changed = false
		for id := range deleted {
			for other := range dependencies[id] {
				if !deleted[other] {
					delete(deleted, id)
					changed = true
					break
				}
			}
		}
	}
	// Verify the effect of deletion alone. A separate authorized addition may
	// change a retained claim, but the cleanup itself must not reactivate one.
	remaining := store.MapSource{}
	maps.Copy(remaining, base.Bytes)
	paths := map[string]bool{}
	for id := range deleted {
		path := base.Paths[id]
		delete(remaining, path)
		paths[path] = true
	}
	set, err := store.Load(context.Background(), remaining)
	if err != nil || len(set.Problems) != 0 || len(set.Dangling) != 0 {
		return nil
	}
	before, after := resolve.New(base), resolve.New(set)
	if len(after.Violations) != 0 {
		return nil
	}
	for _, id := range set.IDs() {
		if before.Status(id) != after.Status(id) {
			return nil
		}
	}
	return paths
}

func matchesRetirement(expected string, raw []byte) bool {
	if expected == "" || len(raw) == 0 {
		return false
	}
	sum := sha256.Sum256(raw)
	return expected == hex.EncodeToString(sum[:])
}

var retirementID = regexp.MustCompile(`(?:Need|Promise|Expectation|Decision|Assumption|Policy|Learning|Plan|Question|Answer|Revocation|Document)-[0-9A-HJKMNP-TV-Z]{26}`)

// A target can disappear only with all its references. A superseding or
// revoking carrier can disappear only with its target as well. This closes
// historical chains in both directions and protects textual citations and
// Document embeds in addition to typed relations.
func retirementDependencies(sets ...*store.Set) map[string]map[string]bool {
	deps := map[string]map[string]bool{}
	bind := func(id, other string) {
		if id == other {
			return
		}
		if deps[id] == nil {
			deps[id] = map[string]bool{}
		}
		deps[id][other] = true
	}
	families := map[string]map[string]bool{}
	for _, set := range sets {
		for _, r := range set.Records {
			ref := r.Subject + "#" + r.Family
			if families[ref] == nil {
				families[ref] = map[string]bool{}
			}
			families[ref][r.ID] = true
		}
	}
	for _, set := range sets {
		for _, r := range set.Records {
			for _, rel := range r.Relations {
				bind(rel.Target, r.ID)
				if rel.Type == record.Supersedes || rel.Type == record.Revokes {
					bind(r.ID, rel.Target)
				}
			}
			for _, body := range [][]byte{set.Bytes[r.Path()], set.Bodies[r.ID]} {
				for _, target := range retirementID.FindAll(body, -1) {
					bind(string(target), r.ID)
				}
			}
			for _, embed := range r.Embeds {
				for target := range families[embed] {
					bind(target, r.ID)
				}
			}
		}
	}
	return deps
}
