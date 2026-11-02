package precheck

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
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/lock"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v: %s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func write(t *testing.T, root, name string, body []byte) {
	t.Helper()
	p := filepath.Join(root, name)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, body, 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	parent := t.TempDir()
	remote, root := filepath.Join(parent, "remote.git"), filepath.Join(parent, "work")
	if err := os.Mkdir(root, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, parent, "init", "--bare", remote)
	git(t, root, "init", "-b", "main")
	git(t, root, "config", "user.name", "Fixture Author")
	git(t, root, "config", "user.email", "fixture@example.test")
	git(t, root, "remote", "add", "origin", remote)
	write(t, root, "README.md", []byte("base\n"))
	write(t, root, "docs/prds/other/effective.source.json", []byte(`{"units":{"FR-9":{"status":"live"}}}`))
	git(t, root, "add", "--", "README.md", "docs/prds/other/effective.source.json")
	git(t, root, "commit", "-m", "base")
	git(t, root, "push", "-u", "origin", "main")
	return root
}

func lockedFixture(t *testing.T) string {
	return lockedFixtureWith(t, false, false)
}

func lockedFixtureWith(t *testing.T, malformedPRD, emptyReview bool) string {
	t.Helper()
	root := fixture(t)
	prd, err := os.ReadFile("../../draft/compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	body := string(prd)
	body = strings.Replace(body, "| Q-1 | Who owns retries? | PM | lock | open | |", "| Q-1 | Who owns retries? | PM | lock | resolved | The PM owns retries. |", 1)
	body = strings.Replace(body, "## 5. System requirements", "## 1. Summary\n\n## 2. Problem space\n\n## 3. Goals, metrics, and guardrails\n\n## 4. Solution space\n\n## 5. System requirements", 1)
	body = strings.Replace(body, "| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", "### 5.8 Definition of Done\n\n| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", 1)
	body = strings.Replace(body, "## 7. Learning", "## 6. Delivery\n\n## 7. Learning", 1)
	prd = []byte(body + "\n## 8. Contract readiness\n")
	if malformedPRD {
		prd = []byte("not a PRD\n")
	}
	tech := []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n\nThe design follows prd:other/FR-9.\n")
	digest := compile.ContractDigest(prd, tech)
	docs := []struct {
		subject, family string
		body            []byte
	}{
		{"prd/widget", "prd", prd},
		{"design/widget", "design", tech},
		{"review/widget", "report-001", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** " + digest + "\n")},
		{"review/widget", "report-002", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** " + digest + "\n")},
	}
	if emptyReview {
		docs[3].body = []byte{}
	}
	for n, doc := range docs {
		at := time.Date(2026, 9, 25, 10, 0, n, 0, time.UTC)
		var entropy [10]byte
		entropy[9] = byte(n + 1)
		r := &record.Record{Schema: record.Schema, ID: record.NewID(record.Document, at, entropy), Kind: record.Document,
			Subject: doc.subject, Family: doc.family, Statement: "Locked " + doc.family,
			Issuer: "fixture@example.test", CreatedAt: at.Format(time.RFC3339), LiteralBody: true}
		if _, err := store.WriteDocument(filepath.Join(root, store.Dir), r, doc.body); err != nil {
			t.Fatal(err)
		}
	}
	git(t, root, "add", "--", "memory")
	git(t, root, "commit", "-m", "lock widget")
	git(t, root, "push", "origin", "main")
	merge := git(t, root, "rev-parse", "HEAD")
	tag := lock.LockRecord{Schema: "draft-lock/v1", Feature: "widget", Version: 1, MergeCommit: merge,
		PullRequest: 17, ApprovedHead: merge, ReadinessDigest: digest, ProductApprover: "operator",
		EngineeringApprover: "operator", Lineless: true, ReviewID: 27, ProposalsSinceOpened: 1}
	encoded, err := json.Marshal(tag)
	if err != nil {
		t.Fatal(err)
	}
	git(t, root, "tag", "-a", "lock/widget/v1", "-m", string(encoded))
	git(t, root, "push", "origin", "refs/tags/lock/widget/v1")
	return root
}

func TestDocumentPipelineUsesContractGateWithoutGit(t *testing.T) {
	root := filepath.Join("..", "plan", "testdata", "precheck-clean")
	never := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		t.Fatal("document-pipeline precheck called Git")
		return nil, nil
	}
	basis, defects := Run(t.Context(), root, "demo", never)
	if basis != "" || len(defects) != 0 {
		t.Fatalf("basis=%q defects=%v", basis, defects)
	}
}

func TestDraftBornNeedsBindingLock(t *testing.T) {
	root := fixture(t)
	basis, defects := Run(t.Context(), root, "widget", draftshell.Run)
	if basis != "" || len(defects) != 1 || !strings.Contains(defects[0].Reason, "no binding locked version") {
		t.Fatalf("basis=%q defects=%v", basis, defects)
	}
}

func TestLockedVersionPlansDuringReopenAndRetiredCitationRefuses(t *testing.T) {
	root := lockedFixture(t)
	check := func(wantBasis, wantDefect string) {
		t.Helper()
		basis, defects := Run(t.Context(), root, "widget", draftshell.Run)
		if basis != wantBasis || len(defects) == 0 && wantDefect != "" || len(defects) != 0 && wantDefect == "" {
			t.Fatalf("basis=%q defects=%v; want basis=%q defect=%q", basis, defects, wantBasis, wantDefect)
		}
		if wantDefect != "" {
			joined := fmt.Sprint(defects)
			if !strings.Contains(joined, wantDefect) {
				t.Fatalf("defects %s do not name %q", joined, wantDefect)
			}
		}
	}
	check("lock/widget/v1", "")
	git(t, root, "switch", "-c", "draft/widget-v2")
	write(t, root, "drafts/widget/prd.md", []byte("open and unfinished\n"))
	git(t, root, "add", "--", "drafts/widget/prd.md")
	git(t, root, "commit", "-m", "reopen widget")
	check("lock/widget/v1", "")
	git(t, root, "switch", "main")
	write(t, root, "docs/prds/other/effective.source.json", []byte(`{"units":{"FR-9":{"status":"retired"}}}`))
	git(t, root, "add", "--", "docs/prds/other/effective.source.json")
	git(t, root, "commit", "-m", "retire cited requirement")
	git(t, root, "push", "origin", "main")
	check("", "FR-38")
}

func TestLatestVersionValidatesRemoteHistory(t *testing.T) {
	for _, tc := range []struct {
		name, lines, failure, want string
		version                    int
	}{
		{"no lock", "", "", "", 0},
		{"peeled annotated tag", "a\trefs/tags/lock/widget/v1\na\trefs/tags/lock/widget/v1^{}\n", "", "", 1},
		{"gap", "a\trefs/tags/lock/widget/v2\n", "", "skip version 1", 0},
		{"bad version", "a\trefs/tags/lock/widget/vzero\n", "", "invalid lock version", 0},
		{"unexpected ref", "a\trefs/tags/lock/other/v1\n", "", "unexpected lock ref", 0},
		{"malformed", "bad line here\n", "", "malformed remote lock line", 0},
		{"remote down", "", "remote down", "list binding locks", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				if name != "git" || len(args) < 1 || args[0] != "ls-remote" {
					t.Fatalf("unexpected command %s %v", name, args)
				}
				if tc.failure != "" {
					return nil, fmt.Errorf("%s", tc.failure)
				}
				return []byte(tc.lines), nil
			}
			version, err := latestVersion(t.Context(), ".", "origin", "widget", run)
			if version != tc.version || tc.want != "" && (err == nil || !strings.Contains(err.Error(), tc.want)) || tc.want == "" && err != nil {
				t.Fatalf("version=%d error=%v; want version=%d error=%q", version, err, tc.version, tc.want)
			}
		})
	}
	if _, err := latestVersion(t.Context(), ".", "", "widget", draftshell.Run); err == nil {
		t.Fatal("missing remote accepted")
	}
	if _, err := latestVersion(t.Context(), ".", "origin", "widget", nil); err == nil {
		t.Fatal("missing process adapter accepted")
	}
}

func TestLockedPartsMapOnlyFeatureDocuments(t *testing.T) {
	for _, tc := range []struct{ subject, family, want string }{
		{"prd/widget", "prd", "prd.md"},
		{"prd/widget", "problems", "problems.md"},
		{"design/widget", "design", ""},
		{"feature/widget", "events", "events.md"},
		{"brainstorm/widget", "record-001", "brainstorm/001.md"},
		{"review/widget", "report-002", "reviews/002.md"},
		{"review/widget", "other", ""},
		{"review/other", "report-001", ""},
	} {
		if got := lockedPart(tc.subject, "widget", tc.family); got != tc.want {
			t.Errorf("lockedPart(%s, %s)=%q want %q", tc.subject, tc.family, got, tc.want)
		}
	}
}

func TestPrecheckFailsClosedAtRemoteAndInvalidFeature(t *testing.T) {
	root := fixture(t)
	if _, defects := Run(t.Context(), root, "../widget", draftshell.Run); len(defects) != 1 || !strings.Contains(defects[0].Reason, "invalid feature") {
		t.Fatalf("invalid feature defects=%v", defects)
	}
	if _, defects := Run(t.Context(), root, "widget", nil); len(defects) != 1 || !strings.Contains(defects[0].Reason, "no Git process adapter") {
		t.Fatalf("missing adapter defects=%v", defects)
	}
	for _, tc := range []struct{ fail, option, want string }{
		{"fetch", "--no-tags", "fetch current mainline"},
		{"ls-remote", "--refs", "resolve current mainline"},
		{"ls-remote", "--tags", "list binding locks"},
	} {
		runner := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
			if name == "git" && len(args) > 1 && args[0] == tc.fail && args[1] == tc.option {
				return nil, fmt.Errorf("offline")
			}
			return draftshell.Run(ctx, dir, env, name, args...)
		}
		if _, defects := Run(t.Context(), root, "widget", runner); len(defects) != 1 || !strings.Contains(defects[0].Reason, tc.want) {
			t.Errorf("%s defects=%v", tc.fail, defects)
		}
	}
}

func TestCurrentLockValidatesAdvertisedMainline(t *testing.T) {
	sha1, sha256 := strings.Repeat("a", 40), strings.Repeat("b", 64)
	for _, tc := range []struct{ name, advertised, want string }{
		{"SHA-1", sha1 + "\trefs/heads/main\n", sha1},
		{"SHA-256", sha256 + "\trefs/heads/main\n", sha256},
		{"absent mainline", "", ""},
		{"other branch", sha1 + "\trefs/heads/other\n", ""},
		{"short OID", "abc123\trefs/heads/main\n", ""},
		{"nonhex OID", strings.Repeat("g", 40) + "\trefs/heads/main\n", ""},
		{"multiple refs", sha1 + "\trefs/heads/main\n" + sha1 + "\trefs/heads/main\n", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fetches := 0
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) >= 2 && args[0] == "ls-remote" {
					if args[1] == "--refs" {
						return []byte(tc.advertised), nil
					}
					return []byte(sha1 + "\trefs/tags/lock/widget/v1\n"), nil
				}
				if name != "git" || len(args) < 2 || args[0] != "fetch" || args[len(args)-1] != tc.want || !strings.Contains(strings.Join(args, " "), "--no-write-fetch-head") {
					t.Fatalf("unadvertised or shared fetch read: %s %v", name, args)
				}
				fetches++
				return nil, nil
			}
			mainline, version, err := currentLock(t.Context(), ".", "widget", config.Defaults(), run)
			if tc.want == "" {
				if err == nil || mainline != "" || version != 0 || fetches != 0 {
					t.Fatalf("invalid mainline accepted: %q v%d, fetches=%d err=%v", mainline, version, fetches, err)
				}
			} else if err != nil || mainline != tc.want || version != 1 || fetches != 1 {
				t.Fatalf("advertised mainline not pinned: %q v%d, fetches=%d err=%v", mainline, version, fetches, err)
			}
		})
	}
}

func TestLockedReadinessRejectsUntrustedOrUnreadableLock(t *testing.T) {
	root := lockedFixture(t)
	mainline := git(t, root, "rev-parse", "main")
	base := git(t, root, "rev-parse", "main~1")
	cfg := config.Defaults()
	for _, tc := range []struct {
		name, ref, want string
		run             lifecycle.RunCommand
		version         int
	}{
		{"missing version", mainline, "fetch binding lock", draftshell.Run, 2},
		{"unmerged mainline", base, "not merged into current mainline", draftshell.Run, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := judgeVersion(t.Context(), root, cfg, "widget", tc.version, tc.ref, tc.run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("judgeVersion error=%v, want %q", err, tc.want)
			}
		})
	}
	catFiles := 0
	secondReadFails := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) > 0 && args[0] == "cat-file" {
			catFiles++
			if catFiles == 2 {
				return nil, fmt.Errorf("tag disappeared")
			}
		}
		return draftshell.Run(ctx, dir, env, name, args...)
	}
	if _, err := judgeVersion(t.Context(), root, cfg, "widget", 1, mainline, secondReadFails); err == nil || !strings.Contains(err.Error(), "tag disappeared") {
		t.Fatalf("second lock read error=%v", err)
	}
}

func TestLockedReadinessFailsClosedOnChangingObjects(t *testing.T) {
	for _, tc := range []struct {
		name, want string
		change     func(*testing.T, string)
	}{
		{"repository lost", "open locked repository", func(t *testing.T, root string) {
			if err := os.Rename(filepath.Join(root, ".git"), filepath.Join(root, ".git.gone")); err != nil {
				t.Fatal(err)
			}
		}},
		{"tag lost", "load locked memory", func(t *testing.T, root string) {
			git(t, root, "tag", "-d", "lock/widget/v1")
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := lockedFixture(t)
			mainline := git(t, root, "rev-parse", "main")
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				out, err := draftshell.Run(ctx, dir, env, name, args...)
				if err == nil && name == "git" && len(args) > 0 && args[0] == "merge-base" {
					tc.change(t, dir)
				}
				return out, err
			}
			if _, err := judgeVersion(t.Context(), root, config.Defaults(), "widget", 1, mainline, run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}

func TestLockedReadinessRejectsInvalidStoredDocumentsAndMainline(t *testing.T) {
	for _, tc := range []struct {
		name, want                string
		malformedPRD, emptyReview bool
	}{
		{"empty review", "empty reviews/002.md Document", false, true},
		{"unreadable PRD", "compile locked contract", true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := lockedFixtureWith(t, tc.malformedPRD, tc.emptyReview)
			if _, err := judgeVersion(t.Context(), root, config.Defaults(), "widget", 1, git(t, root, "rev-parse", "main"), draftshell.Run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	root := lockedFixture(t)
	write(t, root, "memory/bad/broken.json", []byte("{"))
	git(t, root, "add", "--", "memory/bad/broken.json")
	git(t, root, "commit", "-m", "broken mainline memory")
	git(t, root, "push", "origin", "main")
	if _, err := judgeVersion(t.Context(), root, config.Defaults(), "widget", 1, git(t, root, "rev-parse", "main"), draftshell.Run); err == nil || !strings.Contains(err.Error(), "mainline memory") {
		t.Fatalf("broken mainline error=%v", err)
	}
}
