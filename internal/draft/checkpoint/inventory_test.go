package checkpoint

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func checkpointEvent(id int, feature, cause string) events.Event {
	return events.Event{Schema: events.Schema, ID: fmt.Sprintf("%024x", id), Name: events.DraftReopened,
		Feature: feature, OccurredAt: "2026-09-25T12:00:00Z", FromVersion: 1, Cause: cause, CauseRef: "issue:42"}
}

func TestEvaluateCheckpointTriggerAndMeasurements(t *testing.T) {
	cfg := config.Defaults()
	features := []Feature{
		{Name: "alpha", HasPlan: true, Events: []events.Event{checkpointEvent(1, "alpha", "new-need"), checkpointEvent(2, "alpha", "dependency-change")}},
		{Name: "beta", HasPlan: true, Events: []events.Event{checkpointEvent(3, "beta", "gap-lock-should-have-caught")}},
		{Name: "gamma", HasPlan: true, Events: []events.Event{checkpointEvent(4, "gamma", "other"), checkpointEvent(4, "gamma", "other")}},
		{Name: "later", HasPlan: true, Events: []events.Event{checkpointEvent(5, "later", "new-need")}},
	}
	paths := []string{"docs/prds/alpha/000-base.md", "docs/tech-design/beta.md", "docs/brainstorms/gamma-tech-design.md", "docs/reviews/beta-001.md", "docs/reviews/later-001.md", "docs/wiki/readme.md"}
	classify := func(file, feature string) bool { return ownsCorpusFile(file, feature, cfg) }
	mainline := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name string
		list []Feature
		want string
	}{
		{"two locks", features[:2], "2 of the first 3"},
		{"no plan", append([]Feature{features[0], {Name: "beta"}}, features[2]), "beta: no merged plan"},
		{"unfinished", append([]Feature{features[0], {Name: "beta", HasPlan: true, Unfinished: []string{"T2 (issue #8 open)"}}}, features[2]), "beta: T2"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Evaluate(mainline, tc.list, paths, classify)
			if err != nil || got.Due || !strings.Contains(strings.Join(got.Missing, "; "), tc.want) {
				t.Fatalf("premature checkpoint: %+v %v", got, err)
			}
		})
	}
	got, err := Evaluate(mainline, features, paths, classify)
	if err != nil || !got.Due || got.Inventory.SchemaVersion != schema || !reflect.DeepEqual(got.Inventory.Features, []string{"alpha", "beta", "gamma"}) || got.Inventory.NewCorpusFiles != 4 || !reflect.DeepEqual(got.Inventory.ReopensByCause, map[string]int{"new-need": 1, "dependency-change": 1, "gap-lock-should-have-caught": 1, "other": 1}) {
		t.Fatalf("due inventory: %+v %v", got, err)
	}
	features[1].Events[0].Feature = "other"
	if _, err := Evaluate(mainline, features, paths, classify); err == nil || !strings.Contains(err.Error(), "belongs to other") {
		t.Fatalf("foreign retained event accepted: %v", err)
	}
}

func TestRecordCheckpointInventoryIsIdempotent(t *testing.T) {
	root := t.TempDir()
	cmd := exec.Command("git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	in := Result{Due: true, Inventory: Inventory{SchemaVersion: schema, Mainline: strings.Repeat("a", 40), Features: []string{"alpha", "beta", "gamma"}, NewCorpusFiles: 0, ReopensByCause: map[string]int{"new-need": 1}}}
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	mint := func() time.Time { return at }
	first, err := recordInventory(context.Background(), root, in, mint, bytes.NewReader(bytes.Repeat([]byte{1}, 10)))
	if err != nil || !first.Due || first.AlreadyRecorded || first.RecordPath == "" {
		t.Fatalf("first checkpoint write: %+v %v", first, err)
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil || len(set.Problems) > 0 {
		t.Fatalf("read checkpoint memory: %v %+v", err, set.Problems)
	}
	head := resolve.New(set).Head(subject, family)
	if head == nil {
		t.Fatal("checkpoint inventory record absent")
	}
	stored, err := parseInventory(set.Bodies[head.ID])
	if err != nil || !reflect.DeepEqual(stored, in.Inventory) {
		t.Fatalf("stored inventory: %+v %v", stored, err)
	}
	second, err := recordInventory(context.Background(), root, in, mint, bytes.NewReader(nil))
	if err != nil || !second.AlreadyRecorded || second.RecordPath != first.RecordPath {
		t.Fatalf("repeat wrote another inventory: %+v %v", second, err)
	}
	files, err := os.ReadDir(filepath.Join(root, store.Dir, subject))
	if err != nil || len(files) != 2 {
		t.Fatalf("checkpoint record files: %v %+v", err, files)
	}
	if _, err := recordInventory(context.Background(), root, Result{Missing: []string{"third lock"}}, mint, bytes.NewReader(nil)); err != nil {
		t.Fatalf("not-due checkpoint touched memory: %v", err)
	}
}

func TestInventoryRejectsInvalidInputsAndStoredBodies(t *testing.T) {
	in := Inventory{SchemaVersion: schema, Mainline: strings.Repeat("a", 40), Features: []string{"alpha", "beta", "gamma"}, ReopensByCause: map[string]int{}}
	if _, err := Evaluate(in.Mainline, nil, nil, nil); err == nil {
		t.Fatal("nil corpus classifier accepted")
	}
	for _, tc := range []struct {
		name string
		edit func(*Inventory)
	}{
		{"schema", func(i *Inventory) { i.SchemaVersion = "unknown" }},
		{"mainline", func(i *Inventory) { i.Mainline = strings.Repeat("z", 40) }},
		{"duplicate feature", func(i *Inventory) { i.Features[1] = "alpha" }},
		{"invalid feature", func(i *Inventory) { i.Features[1] = "Beta" }},
		{"negative count", func(i *Inventory) { i.NewCorpusFiles = -1 }},
		{"unknown cause", func(i *Inventory) { i.ReopensByCause["guessed"] = 1 }},
		{"negative cause", func(i *Inventory) { i.ReopensByCause["other"] = -1 }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := in
			bad.Features = append([]string(nil), in.Features...)
			bad.ReopensByCause = map[string]int{}
			tc.edit(&bad)
			if _, err := inventoryBody(bad); err == nil {
				t.Fatal("invalid inventory serialized")
			}
		})
	}
	for _, body := range [][]byte{[]byte("not inventory"), []byte(bodyPrefix + "{bad}" + bodySuffix), []byte(bodyPrefix + `{"schema_version":"wrong","mainline":"` + strings.Repeat("a", 40) + `","features":["a","b","c"],"new_corpus_files":0,"reopens_by_cause":{}}` + bodySuffix), []byte(bodyPrefix + `{"schema_version":"feature-draft-checkpoint/v1","mainline":"` + strings.Repeat("a", 40) + `","features":["a","b","c"],"new_corpus_files":0,"reopens_by_cause":{}} {}` + bodySuffix)} {
		if _, err := parseInventory(body); err == nil {
			t.Fatalf("bad stored inventory accepted: %q", body)
		}
	}
	root := t.TempDir()
	valid := Result{Due: true, Inventory: in}
	if _, err := recordInventory(t.Context(), root, valid, time.Now, bytes.NewReader(nil)); err == nil || !strings.Contains(err.Error(), "locate memory writer lock") {
		t.Fatalf("non-repository checkpoint accepted: %v", err)
	}
	cmd := exec.Command("git", "init", "-q", root)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init: %v %s", err, out)
	}
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	if _, err := recordInventory(t.Context(), root, valid, func() time.Time { return at }, bytes.NewReader(nil)); err == nil || !strings.Contains(err.Error(), "mint checkpoint record") {
		t.Fatalf("missing entropy accepted: %v", err)
	}
	old := &record.Record{Schema: record.Schema, ID: record.NewID(record.Document, at, [10]byte{1}), Kind: record.Document,
		Subject: subject, Family: family, Statement: "Corrupt checkpoint", Issuer: "fixture", CreatedAt: at.Format(time.RFC3339), LiteralBody: true}
	if _, err := store.WriteDocument(filepath.Join(root, store.Dir), old, []byte("bad inventory")); err != nil {
		t.Fatal(err)
	}
	if _, err := recordInventory(t.Context(), root, valid, func() time.Time { return at }, bytes.NewReader(nil)); err == nil || !strings.Contains(err.Error(), "body framing") {
		t.Fatalf("corrupt existing checkpoint accepted: %v", err)
	}
}
