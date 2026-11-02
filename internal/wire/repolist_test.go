package wire

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// listRunner scripts one gh answer and records the argv it was given.
type listRunner struct {
	stdout, stderr string
	exit           int
	err            error
	calls          [][]string
}

func (r *listRunner) RunGH(_ context.Context, args ...string) (string, string, int, error) {
	r.calls = append(r.calls, append([]string(nil), args...))
	return r.stdout, r.stderr, r.exit, r.err
}

func nodesJSON(rows string) string {
	return pagedJSON(rows, false)
}

func pagedJSON(rows string, hasNext bool) string {
	next := "false"
	if hasNext {
		next = "true"
	}
	return `{"data":{"viewer":{"repositories":{"pageInfo":{"hasNextPage":` + next + `},"nodes":[` + rows + `]}}}}`
}

func node(slug string, empty, archived bool, perm string) string {
	b := func(v bool) string {
		if v {
			return "true"
		}
		return "false"
	}
	return `{"nameWithOwner":"` + slug + `","isEmpty":` + b(empty) + `,"isArchived":` + b(archived) + `,"viewerPermission":"` + perm + `"}`
}

// Only empty, unarchived, writable repositories are offerable: everything
// else would be a choice the run cannot honor.
func TestListEmptyRepositoriesFilters(t *testing.T) {
	r := &listRunner{stdout: nodesJSON(strings.Join([]string{
		node("octo/empty-admin", true, false, "ADMIN"),
		node("octo/empty-write", true, false, "WRITE"),
		node("octo/empty-maintain", true, false, "MAINTAIN"),
		node("octo/has-content", false, false, "ADMIN"), // not empty
		node("octo/archived", true, true, "ADMIN"),      // archived
		node("octo/read-only", true, false, "READ"),     // cannot wire it
		node("octo/triage", true, false, "TRIAGE"),      // cannot wire it
		node("not-a-slug", true, false, "ADMIN"),        // unusable coordinate
	}, ","))}

	got, err := ListEmptyRepositories(context.Background(), r)
	if err != nil {
		t.Fatalf("ListEmptyRepositories: %v", err)
	}
	want := []string{"octo/empty-admin", "octo/empty-write", "octo/empty-maintain"}
	if len(got.Candidates) != len(want) {
		t.Fatalf("got %d candidates %v, want %v", len(got.Candidates), got.Candidates, want)
	}
	for i, w := range want {
		if got.Candidates[i].Slug != w {
			t.Errorf("candidate %d = %q, want %q (order must follow the query's recency)", i, got.Candidates[i].Slug, w)
		}
	}
}

// ownerAffiliations is load-bearing: its default drops every org-owned
// repository, which is the common case for a team. Assert it rides the argv.
func TestListEmptyRepositoriesQueryCarriesOwnerAffiliations(t *testing.T) {
	r := &listRunner{stdout: nodesJSON("")}
	if _, err := ListEmptyRepositories(context.Background(), r); err != nil {
		t.Fatalf("ListEmptyRepositories: %v", err)
	}
	if len(r.calls) != 1 {
		t.Fatalf("gh calls = %d, want 1", len(r.calls))
	}
	joined := strings.Join(r.calls[0], " ")
	for _, want := range []string{"api", "graphql", "isEmpty", "viewerPermission", "pageInfo"} {
		if !strings.Contains(joined, want) {
			t.Errorf("query lacks %q:\n%s", want, joined)
		}
	}
	// Read the ownerAffiliations CLAUSE itself. Asserting that
	// ORGANIZATION_MEMBER appears ANYWHERE passes on a broken query, because
	// it also appears in the affiliations: clause next door — the two are
	// different filters and only this one admits org-OWNED repositories.
	clause, ok := ownerAffiliationsClause(joined)
	if !ok {
		t.Fatalf("query carries no ownerAffiliations clause — every org-owned repository would be silently dropped:\n%s", joined)
	}
	if !strings.Contains(clause, "ORGANIZATION_MEMBER") {
		t.Errorf("ownerAffiliations = %q, want ORGANIZATION_MEMBER among them (its default is [OWNER, COLLABORATOR], which drops org-owned repos)", clause)
	}
}

// ownerAffiliationsClause extracts the bracketed value of the
// ownerAffiliations argument, so a test can assert on that filter alone.
func ownerAffiliationsClause(query string) (string, bool) {
	const key = "ownerAffiliations:"
	i := strings.Index(query, key)
	if i < 0 {
		return "", false
	}
	rest := query[i+len(key):]
	open := strings.Index(rest, "[")
	close := strings.Index(rest, "]")
	if open < 0 || close < open {
		return "", false
	}
	return rest[open : close+1], true
}

// A window that reported more pages says so: "no candidates" and "no
// candidates among the 100 most recently updated" are different answers, and
// only one of them may be told to an operator as fact.
func TestListEmptyRepositoriesReportsATruncatedWindow(t *testing.T) {
	full := &listRunner{stdout: pagedJSON(node("octo/empty", true, false, "ADMIN"), true)}
	got, err := ListEmptyRepositories(context.Background(), full)
	if err != nil {
		t.Fatalf("ListEmptyRepositories: %v", err)
	}
	if !got.Truncated {
		t.Error("hasNextPage:true did not surface as a truncated window")
	}

	exact := &listRunner{stdout: pagedJSON(node("octo/empty", true, false, "ADMIN"), false)}
	got, err = ListEmptyRepositories(context.Background(), exact)
	if err != nil {
		t.Fatalf("ListEmptyRepositories: %v", err)
	}
	if got.Truncated {
		t.Error("a complete window was reported as truncated")
	}
}

// ActiveAccount names the login that asked, and says nothing rather than
// guessing when it cannot be learned.
func TestActiveAccount(t *testing.T) {
	if got := ActiveAccount(context.Background(), &listRunner{stdout: "octo-bot\n"}); got != "octo-bot" {
		t.Errorf("ActiveAccount = %q, want octo-bot", got)
	}
	for name, r := range map[string]*listRunner{
		"gh failed":     {err: errors.New("boom")},
		"non-zero exit": {exit: 1},
	} {
		if got := ActiveAccount(context.Background(), r); got != "" {
			t.Errorf("%s: ActiveAccount = %q, want empty rather than a guess", name, got)
		}
	}
	if got := ActiveAccount(context.Background(), nil); got != "" {
		t.Errorf("nil runner: ActiveAccount = %q, want empty", got)
	}
}

// Every failure shape is soft: the caller falls back to asking, so none of
// these may look like a reason to stop an install.
func TestListEmptyRepositoriesFailsSoft(t *testing.T) {
	cases := map[string]*listRunner{
		"gh absent":     {err: errors.New("exec: gh: not found")},
		"non-zero exit": {exit: 1, stderr: "gh: not authenticated"},
		"unparseable":   {stdout: "not json at all"},
		"empty output":  {stdout: ""},
		"nil runner":    nil,
	}
	for name, r := range cases {
		t.Run(name, func(t *testing.T) {
			var runner Runner
			if r != nil {
				runner = r
			}
			got, err := ListEmptyRepositories(context.Background(), runner)
			if err == nil {
				t.Fatalf("got %v with no error, want an error the caller reads as 'no list'", got)
			}
			if len(got.Candidates) != 0 {
				t.Errorf("candidates = %v, want none alongside the error", got.Candidates)
			}
		})
	}
}

// A well-formed answer with no qualifying repository is NOT an error: the
// menu still renders with create and local.
func TestListEmptyRepositoriesEmptyIsNotAnError(t *testing.T) {
	got, err := ListEmptyRepositories(context.Background(), &listRunner{stdout: nodesJSON("")})
	if err != nil || len(got.Candidates) != 0 {
		t.Fatalf("got (%v, %v), want an empty list and no error", got, err)
	}
}

// The probe distinguishes the three states the create branch acts on, and
// reads every failure as Unknown — GitHub answers 404 both for absent and
// for private-and-invisible, so "unknown" is the only honest reading.
func TestProbeRepository(t *testing.T) {
	cases := []struct {
		name string
		r    *listRunner
		want RepoState
	}{
		{"empty", &listRunner{stdout: `{"data":{"repository":{"isEmpty":true}}}`}, RepoEmpty},
		{"populated", &listRunner{stdout: `{"data":{"repository":{"isEmpty":false}}}`}, RepoPopulated},
		{"absent or invisible", &listRunner{stdout: `{"data":{"repository":null}}`}, RepoUnknown},
		{"gh failed", &listRunner{err: errors.New("boom")}, RepoUnknown},
		{"non-zero exit", &listRunner{exit: 1}, RepoUnknown},
		{"garbage", &listRunner{stdout: "{{{"}, RepoUnknown},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := ProbeRepository(context.Background(), c.r, "octo/widget"); got != c.want {
				t.Errorf("ProbeRepository = %v, want %v", got, c.want)
			}
		})
	}
	if got := ProbeRepository(context.Background(), &listRunner{}, "no-slash"); got != RepoUnknown {
		t.Errorf("a malformed slug = %v, want RepoUnknown", got)
	}
}
