package lock

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestLoadContractUsesLockedLiteralText(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, ReviewID: 27}}
	if _, err := RecordMerge(t.Context(), work, request, reader, host.run); err != nil {
		t.Fatal(err)
	}
	got, err := LoadContract(t.Context(), work, "origin", "widget", 1, host.run)
	if err != nil || got.Version != 1 || got.Digest != request.ReadinessDigest || !bytes.Equal(got.PRD, readyPRD(t)) || string(got.TechSpec) != "# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n" {
		t.Fatalf("locked text=%+v error=%v", got, err)
	}
}

type recordApproval struct {
	result approval.Approval
	err    error
	calls  int
}

func (r *recordApproval) Approval(_ context.Context, _ int, _ string) (approval.Approval, error) {
	r.calls++
	return r.result, r.err
}

func preparedMerge(t *testing.T) (string, RecordRequest, *prepareHost) {
	t.Helper()
	work, cfg, host := prepareFixture(t, true)
	prepared, err := Prepare(t.Context(), work, cfg, "widget", host.run)
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "switch", "main")
	gitFixture(t, work, "merge", "--squash", "draft/widget")
	gitFixture(t, work, "commit", "-m", "Lock widget (#17)")
	gitFixture(t, work, "push", "origin", "main")
	merge := gitFixture(t, work, "rev-parse", "HEAD")
	return work, RecordRequest{Feature: "widget", Remote: "origin", ExpectedVersion: 1, MergeCommit: merge, PullRequest: 17, ApprovedHead: prepared.Head, ReadinessDigest: prepared.Verdict.DraftDigest, ProposalsSinceOpened: 1}, host
}

func TestRecordMergeCreatesOneTagAndFreshCloneReadsContract(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, CaptureLine: "9 min; FND-1 outside §0", ReviewID: 27}}
	first, err := RecordMerge(t.Context(), work, request, reader, host.run)
	if err != nil || first.Lock == nil || !first.Created || first.Lock.Version != 1 || first.Lock.ProductApprover != "mdarabi" || first.Lock.EngineeringApprover != "mdarabi" || first.Lock.CaptureLine != reader.result.CaptureLine || first.Lock.ProposalsSinceOpened != 1 {
		t.Fatalf("first lock=%+v error=%v", first, err)
	}
	again, err := RecordMerge(t.Context(), work, request, reader, host.run)
	if err != nil || again.Lock == nil || again.Created || *again.Lock != *first.Lock || reader.calls != 2 {
		t.Fatalf("repeated lock=%+v calls=%d error=%v", again, reader.calls, err)
	}
	bare := filepath.Join(filepath.Dir(work), "remote.git")
	gitFixture(t, bare, "symbolic-ref", "HEAD", "refs/heads/main")
	fresh := filepath.Join(t.TempDir(), "fresh")
	gitFixture(t, filepath.Dir(fresh), "clone", "--no-local", bare, fresh)
	read, err := ReadLock(t.Context(), fresh, "widget", 1, host.run)
	if err != nil || *read != *first.Lock {
		t.Fatalf("fresh lock=%+v error=%v", read, err)
	}
	repo, err := gitsrc.Open(t.Context(), fresh)
	if err != nil {
		t.Fatal(err)
	}
	set, err := store.Load(t.Context(), gitsrc.TreeSource{Repo: repo, Ref: "refs/tags/lock/widget/v1", Prefix: store.Dir})
	if err != nil {
		t.Fatal(err)
	}
	prd := resolve.New(set).Head("prd/widget", "prd")
	if prd == nil || !strings.Contains(string(set.Bodies[prd.ID]), "## 0. Read to rule") {
		t.Fatalf("fresh clone could not render locked PRD: %+v", prd)
	}
}

func TestRecordMergeSkipsOrdinaryCommitAndRefusesMissingApproval(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &recordApproval{err: approval.ErrNoApproval}
	if _, err := RecordMerge(t.Context(), work, request, reader, host.run); !errors.Is(err, approval.ErrNoApproval) {
		t.Fatalf("unapproved lock: %v", err)
	}
	if tag := gitFixture(t, work, "tag", "--list", "lock/widget/v*"); tag != "" {
		t.Fatalf("unapproved tag %q", tag)
	}
	fixtureFile(t, work, "README.md", []byte("ordinary change\n"))
	gitFixture(t, work, "add", "--", "README.md")
	gitFixture(t, work, "commit", "-m", "ordinary change")
	request.MergeCommit = gitFixture(t, work, "rev-parse", "HEAD")
	out, err := RecordMerge(t.Context(), work, request, nil, host.run)
	if err != nil || out.Lock != nil || out.Created || reader.calls != 1 {
		t.Fatalf("ordinary merge=%+v calls=%d error=%v", out, reader.calls, err)
	}
}

func TestRecordMergeRejectsIncompleteAndConflictingLocks(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, ReviewID: 27}}
	for _, tc := range []struct {
		name string
		edit func(*RecordRequest, *recordApproval)
		want string
	}{
		{"missing approval", func(_ *RecordRequest, r *recordApproval) { r.result.ReviewID = 0 }, "incomplete"},
		{"wrong head", func(_ *RecordRequest, r *recordApproval) { r.result.Head = strings.Repeat("f", 40) }, "mismatched"},
		{"bad digest", func(q *RecordRequest, _ *recordApproval) { q.ReadinessDigest = "bad" }, "digest"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			q, r := request, *reader
			tc.edit(&q, &r)
			if _, err := RecordMerge(t.Context(), work, q, &r, host.run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	if _, err := RecordMerge(t.Context(), work, request, reader, host.run); err != nil {
		t.Fatal(err)
	}
	changed := request
	changed.ProposalsSinceOpened = 2
	if _, err := RecordMerge(t.Context(), work, changed, reader, host.run); err == nil || !strings.Contains(err.Error(), "differs from this approval") {
		t.Fatalf("conflicting rerun: %v", err)
	}
}

func TestReadLockRejectsMalformedTag(t *testing.T) {
	work, request, host := preparedMerge(t)
	gitFixture(t, work, "tag", "-a", "lock/widget/v1", request.MergeCommit, "-m", "not json")
	if _, err := ReadLock(t.Context(), work, "widget", 1, host.run); err == nil || !strings.Contains(err.Error(), "decode lock tag") {
		t.Fatalf("malformed tag: %v", err)
	}
	if _, err := ReadLock(t.Context(), work, "widget", 0, host.run); err == nil {
		t.Fatal("zero version accepted")
	}
	if _, err := os.Stat(filepath.Join(work, "drafts/widget/prd.md")); !os.IsNotExist(err) {
		t.Fatalf("prepared working file remained: %v", err)
	}
}

func pushAPI(t *testing.T, work string, request RecordRequest, host *prepareHost) lifecycle.RunCommand {
	t.Helper()
	p := mergedPull{Number: request.PullRequest, MergedAt: "2026-09-25T00:00:00Z", MergeCommitSHA: request.MergeCommit}
	p.Head.Ref = "draft/widget"
	p.Head.SHA = request.ApprovedHead
	p.Head.Repo.FullName = "example/repo"
	p.Base.Ref = "main"
	associated, err := json.Marshal([]mergedPull{p})
	if err != nil {
		t.Fatal(err)
	}
	message := gitFixture(t, work, "show", "-s", "--format=%B", request.ApprovedHead)
	commit, err := json.Marshal(struct {
		Commit struct {
			Message string `json:"message"`
		} `json:"commit"`
	}{Commit: struct {
		Message string `json:"message"`
	}{Message: message}})
	if err != nil {
		t.Fatal(err)
	}
	return func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "gh" {
			path := args[len(args)-1]
			switch {
			case strings.Contains(path, "/commits/"+request.MergeCommit+"/pulls?"):
				return associated, nil
			case strings.HasSuffix(path, "/commits/"+request.ApprovedHead):
				return commit, nil
			case strings.Contains(path, "/pulls?head=example:draft/widget"):
				return associated, nil
			default:
				return nil, fmt.Errorf("unexpected gh endpoint %q", path)
			}
		}
		return host.run(ctx, dir, env, name, args...)
	}
}

func TestRecordRangeFindsLockInMultiCommitPush(t *testing.T) {
	work, request, host := preparedMerge(t)
	before := gitFixture(t, work, "rev-parse", request.MergeCommit+"^")
	fixtureFile(t, work, "README.md", []byte("ordinary change\n"))
	gitFixture(t, work, "add", "--", "README.md")
	gitFixture(t, work, "commit", "-m", "ordinary change")
	after := gitFixture(t, work, "rev-parse", "HEAD")
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, ReviewID: 27}}
	records, err := RecordRange(t.Context(), work, config.Defaults(), "example/repo", before, after, reader, pushAPI(t, work, request, host))
	if err != nil || len(records) != 1 || records[0].MergeCommit != request.MergeCommit || records[0].ProposalsSinceOpened != 1 || reader.calls != 1 {
		t.Fatalf("records=%+v approval calls=%d error=%v", records, reader.calls, err)
	}
	again, err := RecordRange(t.Context(), work, config.Defaults(), "example/repo", before, after, reader, pushAPI(t, work, request, host))
	if err != nil || len(again) != 1 || again[0] != records[0] {
		t.Fatalf("rerun=%+v error=%v", again, err)
	}
}

func TestRecordMergeRetriesTagPushWithoutRewritingRecord(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, ReviewID: 27}}
	failed := false
	run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "git" && len(args) >= 3 && args[0] == "push" && strings.HasPrefix(args[2], "refs/tags/lock/") && !failed {
			failed = true
			return nil, errors.New("temporary push failure")
		}
		return host.run(ctx, dir, env, name, args...)
	}
	if _, err := RecordMerge(t.Context(), work, request, reader, run); err == nil || !strings.Contains(err.Error(), "temporary push failure") {
		t.Fatalf("first push: %v", err)
	}
	first := gitFixture(t, work, "rev-parse", "refs/tags/lock/widget/v1")
	out, err := RecordMerge(t.Context(), work, request, reader, run)
	if err != nil || !out.Created || gitFixture(t, work, "rev-parse", "refs/tags/lock/widget/v1") != first {
		t.Fatalf("retry=%+v error=%v", out, err)
	}
}

func TestRecordMergeRefusesPartialContractAndUntrustedTagHistory(t *testing.T) {
	work, request, host := preparedMerge(t)
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, ReviewID: 27}}
	prdOnly := "A\tmemory/prd/widget/Document-prd.json\n"
	ref := "refs/tags/lock/widget/v1"
	for _, tc := range []struct {
		name, match, output, want string
		fail                      bool
	}{
		{"diff failure", "diff-tree", "", "inspect lock merge", true},
		{"partial contract", "diff-tree", prdOnly, "only one", false},
		{"remote unavailable", "ls-remote", "", "list remote lock tags", true},
		{"invalid remote version", "ls-remote", strings.Repeat("a", 40) + "\trefs/tags/lock/widget/vX", "invalid remote", false},
		{"version gap", "ls-remote", strings.Repeat("a", 40) + "\trefs/tags/lock/widget/v2", "skip version", false},
		{"duplicate version", "ls-remote", strings.Repeat("a", 40) + "\t" + ref + "\n" + strings.Repeat("b", 40) + "\t" + ref, "duplicate remote", false},
		{"fetch failure", "fetch", "", "fetch lock tag", true},
		{"local inspection failure", "tag --list", "", "inspect local lock tag", true},
		{"tag creation failure", "tag -a", "", "create lock tag", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "git" {
					joined := strings.Join(args, " ")
					if tc.match == "fetch" && strings.HasPrefix(joined, "ls-remote") {
						return []byte(strings.Repeat("a", 40) + "\t" + ref), nil
					}
					if strings.Contains(joined, tc.match) {
						if tc.fail {
							return nil, errors.New("simulated git failure")
						}
						return []byte(tc.output), nil
					}
				}
				return host.run(ctx, dir, env, name, args...)
			}
			if _, err := RecordMerge(t.Context(), work, request, reader, run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	invalid := request
	invalid.Remote = ""
	if _, err := RecordMerge(t.Context(), work, invalid, reader, host.run); err == nil {
		t.Fatal("missing remote accepted")
	}
	if _, err := RecordMerge(t.Context(), work, request, nil, host.run); err == nil || !strings.Contains(err.Error(), "approval reader") {
		t.Fatalf("missing approval port error=%v", err)
	}
}

func TestReadLockRejectsNonCommitTrailingAndMismatchedTag(t *testing.T) {
	sha := strings.Repeat("a", 40)
	record := LockRecord{Schema: lockSchema, Feature: "widget", Version: 1, MergeCommit: sha, PullRequest: 17, ApprovedHead: strings.Repeat("b", 40), ReadinessDigest: "sha256:" + strings.Repeat("c", 64), ProductApprover: "mdarabi", EngineeringApprover: "mdarabi", Lineless: true, ReviewID: 27, ProposalsSinceOpened: 1}
	encode := func(value LockRecord) string {
		t.Helper()
		body, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return "object " + sha + "\ntype commit\ntag lock/widget/v1\ntagger test <a@b> 1 +0000\n\n" + string(body)
	}
	for _, tc := range []struct {
		name, contents, want  string
		failRead, failResolve bool
	}{
		{"read failure", "", "read lock tag", true, false},
		{"lightweight", "object " + sha + "\ntype commit\n\n{}", "not an annotated", false, false},
		{"trailing JSON", encode(record) + "\n{}", "trailing content", false, false},
		{"resolve failure", encode(record), "resolve lock tag", false, true},
		{"bad fields", func() string { bad := record; bad.ProductApprover = ""; return encode(bad) }(), "invalid or mismatched", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
				if name != "git" {
					return nil, errors.New("unexpected command")
				}
				switch args[0] {
				case "cat-file":
					if tc.failRead {
						return nil, errors.New("missing tag")
					}
					return []byte(tc.contents), nil
				case "rev-parse":
					if tc.failResolve {
						return nil, errors.New("unresolved")
					}
					return []byte(sha), nil
				default:
					return nil, errors.New("unexpected git command")
				}
			}
			if _, err := ReadLock(t.Context(), "", "widget", 1, run); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}
