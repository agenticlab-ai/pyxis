package lock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/approval"
)

func TestRecordRangeRejectsBadRangeAndFailedEnumeration(t *testing.T) {
	if _, err := RecordRange(t.Context(), "", config.Defaults(), "example/repo", "bad", strings.Repeat("a", 40), nil, nil); err == nil {
		t.Fatal("bad range accepted")
	}
	run := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("git failed")
	}
	if got, err := RecordRange(t.Context(), "", config.Defaults(), "example/repo", strings.Repeat("0", 40), strings.Repeat("a", 40), nil, run); err != nil || len(got) != 0 {
		t.Fatalf("first push=%+v error=%v", got, err)
	}
	if _, err := RecordRange(t.Context(), "", config.Defaults(), "example/repo", strings.Repeat("a", 40), strings.Repeat("b", 40), nil, run); err == nil || !strings.Contains(err.Error(), "enumerate") {
		t.Fatalf("enumeration error=%v", err)
	}
}

func TestRecordPushIgnoresNonDraftAndRejectsUnverifiablePR(t *testing.T) {
	work, request, host := preparedMerge(t)
	cfg := config.Defaults()
	reader := &recordApproval{result: approval.Approval{Operator: "mdarabi", Head: request.ApprovedHead, ReviewID: 27}}
	api := pushAPI(t, work, request, host)
	for _, tc := range []struct {
		name     string
		endpoint string
		body     string
		failure  bool
		want     string
	}{
		{"non-draft", "/pulls?per_page=100", `[]`, false, ""},
		{"malformed association", "/pulls?per_page=100", `{`, false, "decode"},
		{"association API failure", "/pulls?per_page=100", "", true, "find pull request"},
		{"malformed head commit", "/commits/" + request.ApprovedHead, `{`, false, "decode prepared"},
		{"head API failure", "/commits/" + request.ApprovedHead, "", true, "read prepared"},
		{"missing digest", "/commits/" + request.ApprovedHead, `{"commit":{"message":"ordinary commit"}}`, false, "not a preparation"},
		{"bad proposals", "/pulls?head=", `{`, false, "decode draft proposals"},
		{"proposal API failure", "/pulls?head=", "", true, "count draft proposals"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "gh" && strings.Contains(args[len(args)-1], tc.endpoint) {
					if tc.failure {
						return nil, errors.New("API unavailable")
					}
					return []byte(tc.body), nil
				}
				return api(ctx, dir, env, name, args...)
			}
			out, err := RecordPush(t.Context(), work, cfg, "example/repo", request.MergeCommit, reader, run)
			if tc.want == "" {
				if err != nil || out.Lock != nil {
					t.Fatalf("non-draft=%+v error=%v", out, err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	if reader.calls != 0 {
		t.Fatalf("approval read on rejected merges: %d", reader.calls)
	}
	if _, err := RecordPush(t.Context(), work, cfg, "bad repo", request.MergeCommit, reader, api); err == nil {
		t.Fatal("invalid repository accepted")
	}
}

func TestFeatureVersionUsesLastVersionSuffixAndPriorTag(t *testing.T) {
	cfg := config.Defaults()
	queries := 0
	run := func(_ context.Context, _ string, _ []string, name string, args ...string) ([]byte, error) {
		if name != "git" || args[0] != "ls-remote" {
			return nil, fmt.Errorf("unexpected command %s %v", name, args)
		}
		queries++
		if strings.HasSuffix(args[len(args)-1], "/v2") {
			return []byte(strings.Repeat("a", 40) + "\t" + args[len(args)-1]), nil
		}
		return nil, nil
	}
	for _, tc := range []struct {
		branch  string
		feature string
		version int
	}{
		{"draft/widget", "widget", 1},
		{"draft/widget-v2", "widget-v2", 1},
		{"draft/widget-v2-v3", "widget-v2", 3},
	} {
		got, version, err := featureVersion(t.Context(), "", cfg, tc.branch, run)
		if err != nil || got != tc.feature || version != tc.version {
			t.Fatalf("%s => %s v%d: %v", tc.branch, got, version, err)
		}
	}
	if queries != 2 {
		t.Fatalf("prior-tag queries=%d", queries)
	}
	if _, _, err := featureVersion(t.Context(), "", cfg, "other/widget", run); err == nil {
		t.Fatal("non-draft branch accepted")
	}
	failing := func(context.Context, string, []string, string, ...string) ([]byte, error) {
		return nil, errors.New("remote unavailable")
	}
	if _, _, err := featureVersion(t.Context(), "", cfg, "draft/widget-v2", failing); err == nil || !strings.Contains(err.Error(), "prior") {
		t.Fatalf("tag lookup error=%v", err)
	}
}

func TestPreparationDigestRequiresOneValidFooter(t *testing.T) {
	valid := "docs(draft): prepare lock widget\n\nDraft-Digest: sha256:" + strings.Repeat("a", 64)
	if digest, err := preparationDigest("widget", valid); err != nil || digest != "sha256:"+strings.Repeat("a", 64) {
		t.Fatalf("digest=%q error=%v", digest, err)
	}
	for _, body := range []string{"", "docs(draft): prepare lock widget", valid + "\nDraft-Digest: sha256:" + strings.Repeat("b", 64), "docs(draft): prepare lock widget\nDraft-Digest: bad"} {
		if _, err := preparationDigest("widget", body); err == nil {
			t.Fatalf("invalid preparation %q accepted", body)
		}
	}
}

func TestProposalCountRequiresSelectedPRAcrossPages(t *testing.T) {
	p := mergedPull{Number: 17}
	p.Head.Ref, p.Head.Repo.FullName = "draft/widget", "example/repo"
	items := make([]mergedPull, 100)
	for i := range items {
		items[i] = p
		items[i].Number = i + 100
	}
	first, err := json.Marshal(items)
	if err != nil {
		t.Fatal(err)
	}
	second, err := json.Marshal([]mergedPull{p})
	if err != nil {
		t.Fatal(err)
	}
	run := func(_ context.Context, _ string, _ []string, _ string, args ...string) ([]byte, error) {
		if strings.HasSuffix(args[len(args)-1], "page=1") {
			return first, nil
		}
		return second, nil
	}
	if count, err := proposalCount(t.Context(), "", "example/repo", "example", "draft/widget", 17, run); err != nil || count != 101 {
		t.Fatalf("count=%d error=%v", count, err)
	}
	missing := func(context.Context, string, []string, string, ...string) ([]byte, error) { return []byte(`[]`), nil }
	if _, err := proposalCount(t.Context(), "", "example/repo", "example", "draft/widget", 17, missing); err == nil || !strings.Contains(err.Error(), "absent") {
		t.Fatalf("missing selected PR error=%v", err)
	}
}
