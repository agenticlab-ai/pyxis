// Package checkpoint records the first three completed draft-born features.
package checkpoint

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"path/filepath"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/memory/cli"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

const (
	schema  = "feature-draft-checkpoint/v1"
	subject = "checkpoint/feature-draft"
	family  = "inventory"
)

type Inventory struct {
	SchemaVersion  string         `json:"schema_version"`
	Mainline       string         `json:"mainline"`
	Features       []string       `json:"features"`
	NewCorpusFiles int            `json:"new_corpus_files"`
	ReopensByCause map[string]int `json:"reopens_by_cause"`
}

// Feature is one verified first lock, ordered by its merge on mainline.
type Feature struct {
	Name       string
	HasPlan    bool
	Unfinished []string
	Events     []events.Event
}

type Result struct {
	Due             bool
	Missing         []string
	Inventory       Inventory
	RecordPath      string
	AlreadyRecorded bool
}

// Evaluate uses only pinned inputs. The first three locks remain the cohort
// even if a later feature finishes earlier.
func Evaluate(mainline string, features []Feature, corpusPaths []string, ownsCorpusFile func(string, string) bool) (Result, error) {
	if ownsCorpusFile == nil {
		return Result{}, fmt.Errorf("checkpoint corpus classifier is required")
	}
	out := Result{Missing: []string{}}
	if len(features) < 3 {
		out.Missing = append(out.Missing, fmt.Sprintf("%d of the first 3 draft-born features have locked", len(features)))
	}
	for _, feature := range features[:min(3, len(features))] {
		if !feature.HasPlan {
			out.Missing = append(out.Missing, feature.Name+": no merged plan")
		}
		for _, task := range feature.Unfinished {
			out.Missing = append(out.Missing, feature.Name+": "+task)
		}
	}
	if len(out.Missing) > 0 {
		return out, nil
	}
	out.Inventory = Inventory{SchemaVersion: schema, Mainline: mainline, Features: make([]string, 0, 3), ReopensByCause: map[string]int{}}
	seenEvents := map[string]bool{}
	for _, feature := range features[:3] {
		out.Inventory.Features = append(out.Inventory.Features, feature.Name)
		for _, event := range feature.Events {
			if err := event.Validate(); err != nil {
				return Result{}, fmt.Errorf("invalid retained event for %s: %v", feature.Name, err)
			}
			if event.Feature != feature.Name {
				return Result{}, fmt.Errorf("retained event %s belongs to %s, not %s", event.ID, event.Feature, feature.Name)
			}
			if seenEvents[event.ID] {
				continue
			}
			seenEvents[event.ID] = true
			if event.Name == events.DraftReopened {
				out.Inventory.ReopensByCause[event.Cause]++
			}
		}
	}
	for _, path := range corpusPaths {
		for _, feature := range out.Inventory.Features {
			if ownsCorpusFile(path, feature) {
				out.Inventory.NewCorpusFiles++
				break
			}
		}
	}
	out.Due = true
	return out, nil
}

const bodyPrefix = "# Feature draft checkpoint inventory\n\n```json\n"
const bodySuffix = "\n```\n"

func inventoryBody(in Inventory) ([]byte, error) {
	if err := validateInventory(in); err != nil {
		return nil, err
	}
	encoded, err := json.MarshalIndent(in, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("encode checkpoint inventory: %w", err)
	}
	return append(append([]byte(bodyPrefix), encoded...), bodySuffix...), nil
}

func parseInventory(body []byte) (Inventory, error) {
	raw, ok := bytes.CutPrefix(body, []byte(bodyPrefix))
	if !ok || !bytes.HasSuffix(raw, []byte(bodySuffix)) {
		return Inventory{}, fmt.Errorf("checkpoint inventory Document has invalid body framing")
	}
	raw = bytes.TrimSuffix(raw, []byte(bodySuffix))
	var in Inventory
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&in); err != nil {
		return Inventory{}, fmt.Errorf("decode checkpoint inventory: %w", err)
	}
	var extra any
	if err := dec.Decode(&extra); err != io.EOF {
		return Inventory{}, fmt.Errorf("checkpoint inventory has trailing content")
	}
	if err := validateInventory(in); err != nil {
		return Inventory{}, err
	}
	return in, nil
}

func validateInventory(in Inventory) error {
	if in.SchemaVersion != schema || len(in.Mainline) != 40 || len(in.Features) != 3 || in.NewCorpusFiles < 0 {
		return fmt.Errorf("invalid checkpoint inventory schema, cohort, or corpus count")
	}
	if _, err := hex.DecodeString(in.Mainline); err != nil {
		return fmt.Errorf("invalid checkpoint mainline: %w", err)
	}
	seen := map[string]bool{}
	for _, name := range in.Features {
		if !validFeature(name) || seen[name] {
			return fmt.Errorf("invalid checkpoint feature %q", name)
		}
		seen[name] = true
	}
	for cause, count := range in.ReopensByCause {
		if count < 0 || cause != "new-need" && cause != "dependency-change" && cause != "gap-lock-should-have-caught" && cause != "other" {
			return fmt.Errorf("invalid checkpoint reopen count for %q", cause)
		}
	}
	return nil
}

// Record writes the inventory once under the memory writer lock. A repeated
// invocation reads the original record and never creates a second family head.
func Record(ctx context.Context, root string, result Result) (Result, error) {
	return recordInventory(ctx, root, result, time.Now, rand.Reader)
}

func recordInventory(ctx context.Context, root string, result Result, now func() time.Time, random io.Reader) (Result, error) {
	if !result.Due {
		return result, nil
	}
	body, err := inventoryBody(result.Inventory)
	if err != nil {
		return Result{}, err
	}
	lockPath, err := cli.LockPath(ctx, root)
	if err != nil {
		return Result{}, fmt.Errorf("locate memory writer lock: %w", err)
	}
	release, err := store.Lock(lockPath, 10*time.Second)
	if err != nil {
		return Result{}, fmt.Errorf("acquire memory writer lock: %w", err)
	}
	defer release()
	dir := filepath.Join(root, store.Dir)
	set, err := store.Load(ctx, store.DirSource{Root: dir})
	if err != nil {
		return Result{}, fmt.Errorf("read checkpoint memory: %w", err)
	}
	if len(set.Problems) > 0 || len(set.DocumentIssues) > 0 || len(set.Dangling) > 0 {
		return Result{}, fmt.Errorf("checkpoint memory has unresolved store defects")
	}
	projection := resolve.New(set)
	if len(projection.Violations) > 0 || len(projection.Conflicts()) > 0 {
		return Result{}, fmt.Errorf("checkpoint memory has unresolved conflicts")
	}
	for _, old := range set.BySubject[subject] {
		if old.Family != family {
			continue
		}
		if old.Kind != record.Document || projection.Status(old.ID) != resolve.Active {
			return Result{}, fmt.Errorf("checkpoint inventory history is not one active Document")
		}
		stored, err := parseInventory(set.Bodies[old.ID])
		if err != nil {
			return Result{}, err
		}
		result.Inventory = stored
		result.RecordPath, result.AlreadyRecorded = filepath.ToSlash(filepath.Join(store.Dir, old.Path())), true
		return result, nil
	}
	var entropy [10]byte
	if _, err := io.ReadFull(random, entropy[:]); err != nil {
		return Result{}, fmt.Errorf("mint checkpoint record: %w", err)
	}
	at := now().UTC()
	r := &record.Record{Schema: record.Schema, ID: record.NewID(record.Document, at, entropy), Kind: record.Document,
		Subject: subject, Family: family, Statement: "Feature draft checkpoint inventory",
		Intent:       "Retain the first three completed draft-born features and their checkpoint measurements.",
		Rationale:    "The checkpoint is computed from pinned locks, completed plan tasks, corpus paths, and retained reopen events.",
		FitCriterion: "The cohort and counts are reproducible from the named mainline evidence.",
		LiteralBody:  true, Issuer: "agent:pyxis", CreatedAt: at.Format(time.RFC3339)}
	if _, err := store.WriteDocument(dir, r, body); err != nil {
		return Result{}, fmt.Errorf("write checkpoint inventory: %w", err)
	}
	result.RecordPath = filepath.ToSlash(filepath.Join(store.Dir, r.Path()))
	return result, nil
}
