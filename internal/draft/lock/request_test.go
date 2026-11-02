package lock

import (
	"fmt"
	"maps"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

func requestFixture() (compile.Result, readiness.Verdict, map[string]string) {
	result := compile.Result{Feature: "widget", Digest: "sha256:abc", Mainline: "base123", Records: []compile.Record{
		{Draft: reconcile.Draft{Kind: record.Document, Subject: "prd/widget", Family: "prd", Body: []byte("# Widget\n\n## 0. Read to rule\n\nThe operator accepts a queue.\n\n## 1. Summary\n\nThe rest of the PRD.\n")}},
	}, Changes: []compile.RequirementChange{
		{ID: "FR-1", FromCommit: "1111111111111111", ToCommit: "2222222222222222", Ruling: &compile.Ruling{Problem: "P-1", Author: "operator", Reason: "Bound the wait.", Change: "Queue it."}},
		{ID: "FR-2", FromCommit: "2222222222222222", ToCommit: "3333333333333333", Unruled: true},
	}, Reviews: []compile.Review{
		{Path: "reviews/001.md", Part: "prd", Findings: []compile.Finding{{ID: "FND-1", Blocking: true, Disposition: "declined — operator · Risk accepted."}}},
	}}
	verdict := readiness.New("widget", result.Digest, result.Mainline)
	verdict.Sort()
	return result, verdict, map[string]string{"prd": "memory/prd/widget/Document-prd.md", "tech-spec": "memory/design/widget/Document-tech.md", "problems": "memory/prd/widget/Document-problems.md"}
}

func TestRequestShowsExactApprovalInputs(t *testing.T) {
	result, verdict, documents := requestFixture()
	body, err := Request(result, readiness.Sources{}, verdict, "https://github.com/example/repo", "prepared123", documents)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"## 0. Read to rule", "The operator accepts a queue.", "sha256:abc",
		"FR-1", "P-1", "declared author **operator**", "Bound the wait.", "Queue it.",
		"FR-2", "**unruled**", "reviews/001.md#FND-1", "declined — operator · Risk accepted.",
		"https://github.com/example/repo/blob/prepared123/memory/prd/widget/Document-prd.md",
		"https://github.com/example/repo/blob/prepared123/memory/design/widget/Document-tech.md",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("request missing %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "The rest of the PRD") {
		t.Fatal("approval request embedded the full PRD despite linking its literal Document")
	}
}

func TestRequestStaysWithinDescriptionLimit(t *testing.T) {
	result, verdict, documents := requestFixture()
	result.Records[0].Draft.Body = []byte("## 0. Read to rule\n\n" + strings.Repeat("x", 70000) + "\n\n## 1. Summary\n")
	body, err := Request(result, readiness.Sources{}, verdict, "https://github.com/example/repo", "prepared123", documents)
	if err != nil || len(body) > maxDescriptionBytes || !strings.Contains(body, "complete §0 is in the linked PRD") {
		t.Fatalf("large request: bytes=%d error=%v", len(body), err)
	}
}

func TestLocalRequestRetainsCompleteLargeApprovalPacket(t *testing.T) {
	result, verdict, documents := requestFixture()
	section := strings.Repeat("Operator review detail. ", 4000) + "End of operator detail."
	result.Records[0].Draft.Body = []byte("## 0. Read to rule\n\n" + section + "\n\n## 1. Summary\n")
	findings := make([]compile.Finding, 160)
	for i := range findings {
		findings[i] = compile.Finding{ID: fmt.Sprintf("FND-%d", i+1), Blocking: true, Disposition: "declined — operator · " + strings.Repeat("Retained ruling. ", 30)}
	}
	result.Reviews[0].Findings = findings
	body, err := LocalRequest(result, readiness.Sources{}, verdict, "prepared123", documents)
	if err != nil || len(body) <= maxDescriptionBytes || !strings.Contains(body, section) || !strings.Contains(body, "reviews/001.md#FND-160") {
		t.Fatalf("local approval packet lost complete review inputs: bytes=%d error=%v", len(body), err)
	}
	if strings.Contains(body, "complete §0 is in the linked PRD") || !strings.Contains(body, "git show prepared123:memory/prd/widget/Document-prd.md") {
		t.Fatal("local packet inherited hosted compaction or lost its retained Git address")
	}
	if _, err := Request(result, readiness.Sources{}, verdict, "https://github.com/example/repo", "prepared123", documents); err == nil {
		t.Fatal("hosted description accepted a packet exceeding its transport limit after compaction")
	}
}

func TestRequestRefusesMissingInputs(t *testing.T) {
	result, verdict, documents := requestFixture()
	for _, tc := range []struct {
		name, url string
		edit      func(*compile.Result, map[string]string)
		want      string
	}{
		{"credential URL", "https://token@github.com/example/repo", func(*compile.Result, map[string]string) {}, "plain HTTPS"},
		{"missing PRD", "https://github.com/example/repo", func(_ *compile.Result, docs map[string]string) { delete(docs, "prd") }, "needs the committed"},
		{"missing section zero", "https://github.com/example/repo", func(result *compile.Result, _ map[string]string) {
			result.Records[0].Draft.Body = []byte("# Widget\n\n## 1. Summary\n")
		}, "§0"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := result
			r.Records = append([]compile.Record(nil), result.Records...)
			docs := map[string]string{}
			maps.Copy(docs, documents)
			tc.edit(&r, docs)
			if _, err := Request(r, readiness.Sources{}, verdict, tc.url, "head", docs); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
}
