package tracker

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestGitHubAdapterExercisesEveryTrackerOperation(t *testing.T) {
	labelReads := 0
	typeCreates := 0
	g := &GH{Repo: "owner/repo"}
	g.run = func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "repos/owner/repo/labels/needs-review"):
			labelReads++
			return nil, errors.New("HTTP 404")
		case strings.Contains(joined, "repos/owner/repo/labels"):
			return []byte(`{}`), nil
		case strings.Contains(joined, "repos/owner/repo/issues?"):
			return []byte(`[{"number":11,"title":"task","body":"body","state":"OPEN","labels":[{"name":"local-agent"}],"type":{"name":"Task"}},{"number":99,"pull_request":{}}]`), nil
		case strings.Contains(joined, "repos/owner/repo/issues/10/sub_issues?"):
			return []byte(`[{"number":11,"title":"child","state":"OPEN"}]`), nil
		case strings.Contains(joined, "repos/owner/repo/issues/12/dependencies/blocked_by?"):
			return []byte(`[{"number":10}][{"number":11}]`), nil
		case strings.Contains(joined, "repos/owner/repo/issues/12/comments?"):
			return []byte(`[{"body":"first"}][{"body":"second"}]`), nil
		case strings.Contains(joined, "repos/owner/repo/issues/11 --jq .id"):
			return []byte("1011\n"), nil
		case strings.Contains(joined, "repos/owner/repo/issues") && strings.Contains(joined, "title=task"):
			if strings.Contains(joined, "type=Task") {
				typeCreates++
				return nil, errors.New("unsupported issue type")
			}
			return []byte(`{"number":12}`), nil
		default:
			return []byte(`{}`), nil
		}
	}

	if err := g.EnsureLabel("needs-review", "ffffff", "review it"); err != nil || labelReads != 1 {
		t.Fatalf("EnsureLabel: reads=%d err=%v", labelReads, err)
	}
	issue, err := g.CreateIssue("task", "body", []string{"local-agent", "kind:feat"}, "Task")
	if err != nil || issue != 12 || typeCreates != 1 {
		t.Fatalf("CreateIssue: issue=%d type attempts=%d err=%v", issue, typeCreates, err)
	}
	open, err := g.OpenIssues()
	if err != nil || len(open) != 1 || open[0].Number != 11 || open[0].Type != "Task" || !open[0].HasLabel("local-agent") || open[0].HasLabel("absent") {
		t.Fatalf("OpenIssues: %#v, %v", open, err)
	}
	subs, err := g.SubIssues(10)
	if err != nil || len(subs) != 1 || subs[0].Number != 11 {
		t.Fatalf("SubIssues: %#v, %v", subs, err)
	}
	blocked, err := g.BlockedBy(12)
	if err != nil || len(blocked) != 2 || blocked[0] != 10 || blocked[1] != 11 {
		t.Fatalf("BlockedBy: %#v, %v", blocked, err)
	}
	if err := g.LinkSubIssue(10, 11); err != nil {
		t.Fatalf("LinkSubIssue: %v", err)
	}
	if err := g.AddBlockedBy(12, 11); err != nil {
		t.Fatalf("AddBlockedBy: %v", err)
	}
	if err := g.AddLabels(12, []string{"one", "two"}); err != nil {
		t.Fatalf("AddLabels: %v", err)
	}
	comments, err := g.Comments(12)
	if err != nil || !strings.EqualFold(strings.Join(comments, ","), "first,second") {
		t.Fatalf("Comments: %#v, %v", comments, err)
	}
	if err := g.Comment(12, "done"); err != nil {
		t.Fatalf("Comment: %v", err)
	}
}

func TestGitHubAdapterFallbacksAndMalformedResponses(t *testing.T) {
	t.Run("REST server error falls back to GraphQL", func(t *testing.T) {
		g := &GH{Repo: "owner/repo", run: func(args ...string) ([]byte, error) {
			if len(args) > 0 && args[0] == "graphql" {
				return []byte(`{"data":{"repository":{"issues":{"nodes":[{"number":4,"title":"four","body":"b","state":"OPEN","labels":{"nodes":[{"name":"x"}]}}]}}}}`), nil
			}
			return nil, errors.New("gh api: HTTP 503")
		}}
		issues, err := g.OpenIssues()
		if err != nil || len(issues) != 1 || issues[0].Number != 4 {
			t.Fatalf("fallback issues=%#v err=%v", issues, err)
		}
	})

	for name, invoke := range map[string]func(*GH) error{
		"open 4xx":   func(g *GH) error { _, err := g.OpenIssues(); return err },
		"sub issues": func(g *GH) error { _, err := g.SubIssues(1); return err },
		"blocked by": func(g *GH) error { _, err := g.BlockedBy(1); return err },
		"comments":   func(g *GH) error { _, err := g.Comments(1); return err },
		"comment":    func(g *GH) error { return g.Comment(1, "x") },
		"labels":     func(g *GH) error { return g.AddLabels(1, []string{"x"}) },
	} {
		t.Run(name, func(t *testing.T) {
			g := &GH{Repo: "owner/repo", run: func(...string) ([]byte, error) { return nil, errors.New("HTTP 401") }}
			if err := invoke(g); err == nil {
				t.Fatal("adapter swallowed a host failure")
			}
		})
	}

	for name, invoke := range map[string]func(*GH) error{
		"created issue": func(g *GH) error { _, err := g.CreateIssue("x", "y", nil, ""); return err },
		"open issues":   func(g *GH) error { _, err := g.OpenIssues(); return err },
		"sub issues":    func(g *GH) error { _, err := g.SubIssues(1); return err },
		"blocked by":    func(g *GH) error { _, err := g.BlockedBy(1); return err },
		"comments":      func(g *GH) error { _, err := g.Comments(1); return err },
		"internal id":   func(g *GH) error { _, err := g.internalID(1); return err },
	} {
		t.Run("malformed "+name, func(t *testing.T) {
			g := &GH{Repo: "owner/repo", run: func(...string) ([]byte, error) { return []byte(`{broken`), nil }}
			if err := invoke(g); err == nil {
				t.Fatal("adapter accepted malformed host data")
			}
		})
	}

	for name, invoke := range map[string]func(*GH) error{
		"sub issues": func(g *GH) error {
			got, err := g.SubIssues(1)
			if err == nil && got != nil {
				return errors.New("wanted nil")
			}
			return err
		},
		"blocked by": func(g *GH) error {
			got, err := g.BlockedBy(1)
			if err == nil && got != nil {
				return errors.New("wanted nil")
			}
			return err
		},
	} {
		t.Run("404 "+name, func(t *testing.T) {
			g := &GH{Repo: "owner/repo", run: func(...string) ([]byte, error) { return nil, errors.New("HTTP 404") }}
			if err := invoke(g); err != nil {
				t.Fatal(err)
			}
		})
	}

	if _, err := (&GH{Repo: "invalid"}).openIssuesGraphQL(); err == nil {
		t.Fatal("GraphQL accepted an invalid repository slug")
	}
	g := &GH{Repo: "owner/repo", run: func(...string) ([]byte, error) { return nil, errors.New("graphql unavailable") }}
	if _, err := g.openIssuesGraphQL(); err == nil {
		t.Fatal("GraphQL transport failure was swallowed")
	}
}

func TestResolveRepoUsesTheGitHubCLIAndSurfacesStderr(t *testing.T) {
	dir := t.TempDir()
	gh := filepath.Join(dir, "gh")
	if err := os.WriteFile(gh, []byte("#!/bin/sh\nprintf 'owner/repo\\n'\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	if got, err := ResolveRepo(t.TempDir()); err != nil || got != "owner/repo" {
		t.Fatalf("ResolveRepo = %q, %v", got, err)
	}
	if err := os.WriteFile(gh, []byte("#!/bin/sh\nprintf 'not authenticated' >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := ResolveRepo(t.TempDir()); err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("ResolveRepo error = %v", err)
	}
}

func TestGitHubAdapterRewritesOnlyTheNamedSpecComment(t *testing.T) {
	var patched []string
	g := &GH{Repo: "owner/repo"}
	g.run = func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case strings.Contains(joined, "repos/owner/repo/issues/12/comments?"):
			return []byte(`[{"id":7,"body":"note"},{"id":8,"body":"old spec"}][{"id":9,"body":"later"}]`), nil
		case strings.Contains(joined, "repos/owner/repo/issues/13/comments?"):
			return []byte(`[{"id":7,"body":"old spec"},{"id":8,"body":"old spec"}]`), nil
		case strings.HasPrefix(joined, "--method PATCH"):
			patched = append(patched, joined)
			return []byte(`{}`), nil
		}
		return nil, errors.New("unexpected call " + joined)
	}
	if err := g.ReplaceComment(12, "old spec", "new spec"); err != nil {
		t.Fatal(err)
	}
	if err := g.EditIssue(12, "[T1] new title", "new body"); err != nil {
		t.Fatal(err)
	}
	if len(patched) != 2 || patched[0] != "--method PATCH repos/owner/repo/issues/comments/8 -f body=new spec" || patched[1] != "--method PATCH repos/owner/repo/issues/12 -f title=[T1] new title -f body=new body" {
		t.Fatalf("patched = %q", patched)
	}
	if err := g.ReplaceComment(13, "old spec", "new spec"); err == nil || !strings.Contains(err.Error(), "2 comments") {
		t.Fatalf("ambiguous spec comments accepted: %v", err)
	}
	if err := g.ReplaceComment(12, "absent", "new spec"); err == nil || !strings.Contains(err.Error(), "0 comments") {
		t.Fatalf("missing spec comment accepted: %v", err)
	}
	if err := (&Carina{}).ReplaceComment(1, "old", "new"); !errors.Is(err, ErrInPlaceUnsupported) {
		t.Fatalf("Carina claimed in-place comment edits: %v", err)
	}
}

func TestGitHubAdapterWithdrawsALabelAndAnEdge(t *testing.T) {
	var deleted []string
	g := &GH{Repo: "owner/repo"}
	g.run = func(args ...string) ([]byte, error) {
		joined := strings.Join(args, " ")
		switch {
		case joined == "repos/owner/repo/issues/3 --jq .id":
			return []byte("9003\n"), nil
		case strings.HasPrefix(joined, "--method DELETE"):
			deleted = append(deleted, joined)
			return nil, nil
		}
		return nil, errors.New("unexpected call " + joined)
	}
	if err := g.RemoveLabel(12, "kind:feat x"); err != nil {
		t.Fatal(err)
	}
	if err := g.RemoveBlockedBy(12, 3); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"--method DELETE repos/owner/repo/issues/12/labels/kind:feat%20x",
		"--method DELETE -H Accept: application/vnd.github+json repos/owner/repo/issues/12/dependencies/blocked_by/9003",
	}
	if !slices.Equal(deleted, want) {
		t.Fatalf("deleted = %q", deleted)
	}
	if err := g.RemoveBlockedBy(12, 4); err == nil {
		t.Fatal("an unresolvable blocker was removed")
	}
}

func TestIssueLabelsMatchWithoutRegardToCase(t *testing.T) {
	is := Issue{Labels: []string{"Local-Agent", "kind:feat"}}
	if !is.HasLabel("local-agent") || !is.HasLabel("KIND:FEAT") || is.HasLabel("operator") {
		t.Fatalf("labels %v matched wrongly", is.Labels)
	}
}
