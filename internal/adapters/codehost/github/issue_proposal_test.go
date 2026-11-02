package ghcodehost

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestDirectIssueProposalPreservesTitleAndClosesSelectedIssue(t *testing.T) {
	fake := &fakeClient{ahead: 1, createNumber: 7}
	host := newHost(fake, "main")
	host.repo = "owner/repo"
	task := port.Task{ID: "issue-long-digest", Number: 42, Title: "fix(parser): retain errors", SourceRef: "https://github.com/owner/repo/issues/42", HeadBranch: "fix/parser"}
	if _, err := host.OpenProposal(t.Context(), task, nil, testDescription); err != nil {
		t.Fatal(err)
	}
	if fake.created.title != task.Title || !strings.Contains(fake.created.body, "\n- Closes #42\n") || !strings.Contains(fake.created.body, task.ID) {
		t.Fatalf("incorrect direct issue proposal: %+v", fake.created)
	}
}

func TestClosingRefRejectsForeignAndAmbiguousIdentity(t *testing.T) {
	for _, tc := range []struct{ ref, repo, want string }{
		{"#42", "owner/repo", "#42"},
		{"https://github.com/owner/repo/issues/42", "owner/repo", "#42"},
		{"https://github.com/other/repo/issues/42", "owner/repo", ""},
		{"https://github.com/owner/other/issues/42", "owner/repo", ""},
		{"https://forge.example/owner/repo/issues/42", "owner/repo", ""},
		{"https://github.com/owner/repo/issues/42", "", ""},
		{"https://github.com/owner/repo/issues/42?x=1", "owner/repo", ""},
		{"https://github.com/owner/repo/issues/42#comment", "owner/repo", ""},
		{"https://github.com/owner/repo/issues/042", "owner/repo", ""},
		{"https://github.com/owner/repo/pull/42", "owner/repo", ""},
		{"https://github.com/owner%2Frepo/issues/42", "owner/repo", ""},
		{"https://user@github.com/owner/repo/issues/42", "owner/repo", ""},
		{"https://github.com:443/owner/repo/issues/42", "owner/repo", ""},
		{"http://github.com/owner/repo/issues/42", "owner/repo", ""},
		{"Carina work-item 42", "owner/repo", ""},
	} {
		t.Run(tc.ref+tc.repo, func(t *testing.T) {
			if got := githubClosingRef(tc.ref, tc.repo); got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
	if NewGH("owner/repo", "main").repo != "owner/repo" {
		t.Fatal("production adapter lost its selected repository")
	}
	if got := proposalTitle(port.Task{ID: "T1"}); got != "chore: implement task T1" {
		t.Fatalf("fallback title: %q", got)
	}
}
