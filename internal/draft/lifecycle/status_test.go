package lifecycle

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestStatesDerivesFourContractStates(t *testing.T) {
	got := States(Snapshot{
		OpenDrafts:     map[string]bool{"new": true, "reopened": true},
		LockedVersions: map[string]int{"locked": 2, "reopened": 1},
		PRDLedgers:     map[string]bool{"existing": true},
	})
	want := []State{
		{Feature: "existing", Kind: Documents},
		{Feature: "locked", Kind: Locked, Version: 2},
		{Feature: "new", Kind: DraftingFirst, Version: 1},
		{Feature: "reopened", Kind: DraftingNext, Version: 2},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("states = %+v, want %+v", got, want)
	}
}

func TestInspectReadsLedgersLocksAndOpenPulls(t *testing.T) {
	root, _ := gitFixture(t)
	ledger := filepath.Join(root, "docs/prds/legacy/000-base.md")
	if err := os.MkdirAll(filepath.Dir(ledger), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ledger, []byte("# Legacy PRD\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := draftshell.Run(context.Background(), root, nil, "git", "tag", "-a", "lock/locked/v2", "-m", "lock 2"); err != nil {
		t.Fatal(err)
	}
	if _, err := draftshell.Run(context.Background(), root, nil, "git", "tag", "-a", "lock/reopened/v1", "-m", "lock 1"); err != nil {
		t.Fatal(err)
	}
	if _, err := draftshell.Run(context.Background(), root, nil, "git", "push", "--tags", "origin"); err != nil {
		t.Fatal(err)
	}
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			return []byte("draft/new\ndraft/reopened-v2\nother/ignored\n"), nil
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	snapshot, err := Inspect(context.Background(), root, config.Defaults(), run)
	if err != nil {
		t.Fatal(err)
	}
	got := States(snapshot)
	if len(got) != 4 || got[0].Kind != Documents || got[1].Kind != Locked || got[2].Kind != DraftingFirst || got[3].Kind != DraftingNext {
		t.Fatalf("inspected states = %+v", got)
	}
}

func TestFeatureSlugAndPushURLValidation(t *testing.T) {
	for _, feature := range []string{"", "-bad", "bad/name", "Upper", "a.b"} {
		if validFeature(feature) {
			t.Errorf("accepted feature %q", feature)
		}
	}
	if !validFeature("a-good-2") {
		t.Fatal("refused valid feature")
	}
	for _, raw := range []string{"http://github.com/o/r", "https://user:pass@github.com/o/r", "https://github.com/o/r?q=x"} {
		if _, err := agentPushURL(raw); err == nil {
			t.Errorf("accepted unsafe push URL %q", raw)
		}
	}
	if got, err := agentPushURL("https://github.com/o/r.git"); err != nil || got != "https://github.com/o/r.git" {
		t.Fatalf("push URL %q: %v", got, err)
	}
	for _, tc := range []struct{ branch, want string }{
		{"draft/new", "new"},
		{"draft/reopened-v2", "reopened"},
		{"draft/widget-v2", "widget-v2"},
		{"draft/my-v-feature-v3", "my-v-feature"},
		{"other/no", ""},
	} {
		got, ok := draftFeature(tc.branch, "draft/", map[string]int{"reopened": 1, "my-v-feature": 2})
		if got != tc.want || ok != (tc.want != "") {
			t.Errorf("draftFeature(%q) = %q, %v; want %q", tc.branch, got, ok, tc.want)
		}
	}
}

func TestInspectFailsClosedOnUnreadableStores(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "docs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "docs/prds"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(context.Background(), root, config.Defaults(), draftshell.Run); err == nil || !strings.Contains(err.Error(), "read PRD ledgers") {
		t.Fatalf("unreadable ledger corpus: %v", err)
	}
	if _, err := Inspect(context.Background(), root, config.Defaults(), nil); err == nil || !strings.Contains(err.Error(), "adapter is required") {
		t.Fatalf("missing adapter: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "docs/prds")); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct{ name, want string }{{"git", "list remote lock tags"}, {"gh", "list open draft pull requests"}} {
		_, err := Inspect(context.Background(), root, config.Defaults(), func(_ context.Context, _ string, _ []string, name string, _ ...string) ([]byte, error) {
			if name == tc.name {
				return nil, fmt.Errorf("offline")
			}
			if name == "git" {
				return []byte("abc\trefs/tags/lock/widget/v1\n"), nil
			}
			return []byte("[]"), nil
		})
		if err == nil || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s failure: %v", tc.name, err)
		}
	}
	if _, err := Inspect(context.Background(), root, config.Defaults(), func(_ context.Context, _ string, _ []string, name string, _ ...string) ([]byte, error) {
		if name == "git" {
			return []byte("malformed-ref-line"), nil
		}
		return nil, nil
	}); err == nil || !strings.Contains(err.Error(), "malformed remote lock tag") {
		t.Fatalf("malformed remote tag response: %v", err)
	}
}
