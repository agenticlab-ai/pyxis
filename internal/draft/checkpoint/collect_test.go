package checkpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/events"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func checkpointGit(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func checkpointFile(t *testing.T, root, path string, body []byte) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func checkpointFixture(t *testing.T, count int, omitPlan string) (string, lifecycle.RunCommand) {
	t.Helper()
	parent := t.TempDir()
	remote, root := filepath.Join(parent, "remote.git"), filepath.Join(parent, "work")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	checkpointGit(t, parent, "init", "--bare", remote)
	checkpointGit(t, root, "init", "-b", "main")
	checkpointGit(t, root, "config", "user.name", "Checkpoint Fixture")
	checkpointGit(t, root, "config", "user.email", "fixture@example.test")
	checkpointGit(t, root, "remote", "add", "origin", remote)
	checkpointFile(t, root, "README.md", []byte("base\n"))
	checkpointGit(t, root, "add", "--", "README.md")
	checkpointGit(t, root, "commit", "-m", "base")
	checkpointGit(t, root, "push", "-u", "origin", "main")
	plan, err := os.ReadFile("../lifecycle/testdata/reopen/001-plan.md")
	if err != nil {
		t.Fatal(err)
	}
	for i, feature := range []string{"alpha", "beta", "gamma"}[:count] {
		opened := events.Event{Schema: events.Schema, ID: fmt.Sprintf("%024x", 100+i), Name: events.DraftOpened,
			Feature: feature, OccurredAt: "2026-09-25T12:00:00Z", Origin: "new"}
		body, _, err := events.Append(nil, opened)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 9, 25, 12, 0, i, 0, time.UTC)
		var entropy [10]byte
		entropy[9] = byte(i + 1)
		r := &record.Record{Schema: record.Schema, ID: record.NewID(record.Document, at, entropy), Kind: record.Document,
			Subject: "feature/" + feature, Family: "events", Statement: "Retained draft events",
			Issuer: "agent:fixture", CreatedAt: at.Format(time.RFC3339), LiteralBody: true}
		if _, err := store.WriteDocument(filepath.Join(root, store.Dir), r, body); err != nil {
			t.Fatal(err)
		}
		files := []string{filepath.ToSlash(filepath.Join(store.Dir, r.Path())), filepath.ToSlash(filepath.Join(store.Dir, r.BodyPath()))}
		if feature != omitPlan {
			planPath := "docs/plans/" + feature + "/001-plan.md"
			checkpointFile(t, root, planPath, []byte(strings.ReplaceAll(string(plan), "widget", feature)))
			files = append(files, planPath)
		}
		for _, path := range []string{"docs/prds/alpha/context.md", "docs/tech-design/beta.md", "docs/brainstorms/gamma-tech-design.md", "docs/reviews/alpha-001.md"} {
			if strings.Contains(path, "/"+feature+"/") || strings.Contains(path, "/"+feature+".") || strings.Contains(path, "/"+feature+"-") {
				checkpointFile(t, root, path, []byte("# corpus file\n"))
				files = append(files, path)
			}
		}
		checkpointGit(t, root, append([]string{"add", "--"}, files...)...)
		checkpointGit(t, root, "commit", "-m", "Lock "+feature)
		merge := checkpointGit(t, root, "rev-parse", "HEAD")
		tag := lock.LockRecord{Schema: "draft-lock/v1", Feature: feature, Version: 1, MergeCommit: merge,
			PullRequest: i + 1, ApprovedHead: merge, ReadinessDigest: "sha256:" + strings.Repeat("a", 64),
			ProductApprover: "operator", EngineeringApprover: "operator", Lineless: true,
			ReviewID: int64(i + 1), ProposalsSinceOpened: 1}
		encoded, err := json.Marshal(tag)
		if err != nil {
			t.Fatal(err)
		}
		checkpointGit(t, root, "tag", "-a", "lock/"+feature+"/v1", "-m", string(encoded))
		checkpointGit(t, root, "push", "origin", "main", "refs/tags/lock/"+feature+"/v1")
	}
	if count == 3 {
		feature := "beta"
		reopened := checkpointEvent(200, feature, "dependency-change")
		body, _, err := events.Append(nil, reopened)
		if err != nil {
			t.Fatal(err)
		}
		at := time.Date(2026, 9, 25, 12, 1, 0, 0, time.UTC)
		var entropy [10]byte
		entropy[9] = 9
		r := &record.Record{Schema: record.Schema, ID: record.NewID(record.Document, at, entropy), Kind: record.Document,
			Subject: "feature/" + feature, Family: "events-v2", Statement: "Retained reopen events",
			Issuer: "agent:fixture", CreatedAt: at.Format(time.RFC3339), LiteralBody: true}
		if _, err := store.WriteDocument(filepath.Join(root, store.Dir), r, body); err != nil {
			t.Fatal(err)
		}
		checkpointGit(t, root, "add", "--", filepath.ToSlash(filepath.Join(store.Dir, r.Path())), filepath.ToSlash(filepath.Join(store.Dir, r.BodyPath())))
		checkpointGit(t, root, "commit", "-m", "retain reopen event")
		checkpointGit(t, root, "push", "origin", "main")
	}
	return root, checkpointRunner(t, "")
}

func checkpointRunner(t *testing.T, openTask string) lifecycle.RunCommand {
	t.Helper()
	type issue struct {
		Number int    `json:"number"`
		Body   string `json:"body"`
		State  string `json:"state"`
	}
	var issues []issue
	for _, feature := range []string{"alpha", "beta", "gamma"} {
		for n := 1; n <= 2; n++ {
			state := "closed"
			if openTask == fmt.Sprintf("%s/T%d", feature, n) {
				state = "open"
			}
			issues = append(issues, issue{Number: len(issues) + 1, Body: fmt.Sprintf("<!-- pyxis:plan-task %s/001/T%d -->", feature, n), State: state})
		}
	}
	data, err := json.Marshal(issues)
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			if len(args) > 0 && args[0] == "repo" {
				return []byte("example/repo\n"), nil
			}
			if len(args) > 0 && args[0] == "api" {
				return data, nil
			}
			return nil, fmt.Errorf("unexpected code-host command %v", args)
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
}

func TestRunCheckpointFromPinnedLocksPlansAndEvents(t *testing.T) {
	root, _ := checkpointFixture(t, 3, "")
	cfg := config.Defaults()
	before, err := Run(t.Context(), root, cfg, checkpointRunner(t, "gamma/T2"))
	if err != nil || before.Due || !strings.Contains(strings.Join(before.Missing, "; "), "gamma: T2") {
		t.Fatalf("unfinished checkpoint: %+v %v", before, err)
	}
	if _, err := os.Stat(filepath.Join(root, store.Dir, subject)); !os.IsNotExist(err) {
		t.Fatalf("premature checkpoint wrote memory: %v", err)
	}
	first, err := Run(t.Context(), root, cfg, checkpointRunner(t, ""))
	if err != nil || !first.Due || first.AlreadyRecorded || first.Inventory.NewCorpusFiles != 4 || first.Inventory.ReopensByCause["dependency-change"] != 1 {
		t.Fatalf("due checkpoint: %+v %v", first, err)
	}
	second, err := Run(t.Context(), root, cfg, checkpointRunner(t, ""))
	if err != nil || !second.AlreadyRecorded || second.RecordPath != first.RecordPath {
		t.Fatalf("repeat checkpoint: %+v %v", second, err)
	}
}

func TestRunCheckpointNamesMissingLocksAndPlan(t *testing.T) {
	root, run := checkpointFixture(t, 2, "")
	result, err := Run(t.Context(), root, config.Defaults(), run)
	if err != nil || result.Due || !strings.Contains(strings.Join(result.Missing, "; "), "2 of the first 3") {
		t.Fatalf("two-lock checkpoint: %+v %v", result, err)
	}
	root, run = checkpointFixture(t, 3, "gamma")
	result, err = Run(t.Context(), root, config.Defaults(), run)
	if err != nil || result.Due || !strings.Contains(strings.Join(result.Missing, "; "), "gamma: no merged plan") {
		t.Fatalf("missing-plan checkpoint: %+v %v", result, err)
	}
}

func TestFirstLockHistoryRejectsMalformedAndUnavailableTags(t *testing.T) {
	sha := strings.Repeat("a", 40)
	for _, tc := range []struct {
		name, listing, fail, want string
	}{
		{"list unavailable", "", "ls-remote", "list first lock tags"},
		{"history unavailable", "", "rev-list", "read mainline lock order"},
		{"malformed line", "bad line", "", "malformed first lock tag line"},
		{"invalid object", strings.Repeat("z", 40) + "\trefs/tags/lock/alpha/v1", "", "invalid first lock tag object"},
		{"unexpected ref", sha + "\trefs/tags/other/alpha/v1", "", "unexpected first lock ref"},
		{"invalid feature", sha + "\trefs/tags/lock/Bad/v1", "", "invalid or duplicate first lock"},
		{"tag unavailable", sha + "\trefs/tags/lock/alpha/v1", "fetch", "fetch first lock"},
		{"tag unreadable", sha + "\trefs/tags/lock/alpha/v1", "cat-file", "read first lock"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				if name != "git" || len(args) == 0 {
					t.Fatalf("unexpected command: %s %v", name, args)
				}
				if args[0] == tc.fail {
					return nil, fmt.Errorf("injected %s failure", tc.fail)
				}
				switch args[0] {
				case "ls-remote":
					return []byte(tc.listing), nil
				case "rev-list":
					return []byte(sha), nil
				case "fetch":
					return nil, nil
				default:
					return nil, fmt.Errorf("unexpected command %v", args)
				}
			}
			if _, err := firstLocks(t.Context(), ".", "origin", sha, run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("invalid lock history error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestRunCheckpointFailsClosedOnInvalidInputs(t *testing.T) {
	cfg := config.Defaults()
	if _, err := Run(t.Context(), t.TempDir(), cfg, nil); err == nil || !strings.Contains(err.Error(), "process adapter") {
		t.Fatalf("nil process adapter error=%v", err)
	}
	if _, err := Run(t.Context(), t.TempDir(), cfg, func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, fmt.Errorf("remote down")
	}); err == nil || !strings.Contains(err.Error(), "fetch checkpoint mainline") {
		t.Fatalf("fetch failure error=%v", err)
	}
	if _, err := Run(t.Context(), t.TempDir(), cfg, func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, nil
	}); err == nil || !strings.Contains(err.Error(), "open checkpoint repository") {
		t.Fatalf("non-repository error=%v", err)
	}
	root, ordinary := checkpointFixture(t, 3, "")
	badTags := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "ls-remote" {
			return []byte("bad tag line"), nil
		}
		return ordinary(ctx, dir, env, name, args...)
	}
	if _, err := Run(t.Context(), root, cfg, badTags); err == nil || !strings.Contains(err.Error(), "malformed first lock tag") {
		t.Fatalf("malformed tag error=%v", err)
	}
	wrongHistory := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "rev-list" {
			return []byte(strings.Repeat("b", 40)), nil
		}
		return ordinary(ctx, dir, env, name, args...)
	}
	if _, err := Run(t.Context(), root, cfg, wrongHistory); err == nil || !strings.Contains(err.Error(), "not on current first-parent mainline") {
		t.Fatalf("orphaned lock error=%v", err)
	}
	badTracker := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			return nil, fmt.Errorf("tracker unavailable")
		}
		return ordinary(ctx, dir, env, name, args...)
	}
	if _, err := Run(t.Context(), root, cfg, badTracker); err == nil || !strings.Contains(err.Error(), "inspect alpha implementation") {
		t.Fatalf("tracker failure error=%v", err)
	}
	checkpointFile(t, root, "docs/prds/alpha/000-base.md", []byte("# collision\n"))
	checkpointGit(t, root, "add", "--", "docs/prds/alpha/000-base.md")
	checkpointGit(t, root, "commit", "-m", "invalid ledger and lock collision")
	checkpointGit(t, root, "push", "origin", "main")
	if _, err := Run(t.Context(), root, cfg, ordinary); err == nil || !strings.Contains(err.Error(), "also has a PRD ledger") {
		t.Fatalf("ledger and lock collision error=%v", err)
	}
}

func TestRetainedEventsAndCorpusFamilyBoundaries(t *testing.T) {
	set, err := store.Load(t.Context(), store.MapSource{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := retainedEvents(set, resolve.New(set), "alpha"); err == nil || !strings.Contains(err.Error(), "no retained event log") {
		t.Fatalf("missing first event log error=%v", err)
	}
	for _, tc := range []struct {
		family string
		want   bool
	}{
		{"events", true}, {"events-v2", true}, {"events-v02", false}, {"events-v1", false}, {"events-vbad", false}, {"event", false},
	} {
		if got := eventFamily(tc.family); got != tc.want {
			t.Errorf("eventFamily(%q)=%v", tc.family, got)
		}
	}
	cfg := config.Defaults()
	for _, tc := range []struct {
		path string
		want bool
	}{
		{"docs/prds/alpha/context.md", true},
		{"docs/tech-design/alpha.md", true},
		{"docs/brainstorms/alpha-tech-design.md", true},
		{"docs/reviews/alpha-001.md", true},
		{"docs/reviews/beta-001.md", false},
		{"docs/wiki/alpha.md", false},
	} {
		if got := ownsCorpusFile(tc.path, "alpha", cfg); got != tc.want {
			t.Errorf("ownsCorpusFile(%q)=%v", tc.path, got)
		}
	}
}

func TestRunCheckpointRefusesMissingRetainedEvents(t *testing.T) {
	root, run := checkpointFixture(t, 3, "")
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil || len(set.BySubject["feature/gamma"]) != 1 {
		t.Fatalf("fixture events: %v %+v", err, set.BySubject["feature/gamma"])
	}
	r := set.BySubject["feature/gamma"][0]
	checkpointGit(t, root, "rm", "--", filepath.ToSlash(filepath.Join(store.Dir, r.Path())), filepath.ToSlash(filepath.Join(store.Dir, r.BodyPath())))
	checkpointGit(t, root, "commit", "-m", "remove retained events in fixture")
	checkpointGit(t, root, "push", "origin", "main")
	if _, err := Run(t.Context(), root, config.Defaults(), run); err == nil || !strings.Contains(err.Error(), "no retained event log") {
		t.Fatalf("missing retained events accepted: %v", err)
	}
}
