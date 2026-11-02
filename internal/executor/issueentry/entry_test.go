package issueentry

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
	"github.com/agenticlab-ai/pyxis/internal/taskrequirements/requirementstest"
)

func intake(t *testing.T) (Input, port.IssueSnapshot) {
	t.Helper()
	s := port.IssueSnapshot{Number: 17, Title: "Fix output", Body: "Return the expected output", Open: true, Executable: true, Comments: []port.IssueComment{{ID: "3", Body: "Preserve compatibility"}}, Dependencies: []port.IssueDependency{{Number: 9, Closed: true}}}
	i, err := Inspect("github", "github:https://github.com:o/r", s)
	if err != nil {
		t.Fatal(err)
	}
	req, err := taskrequirements.Parse([]byte(requirementstest.Input))
	if err != nil {
		t.Fatal(err)
	}
	i.Source.Citations = i.Comments
	return Input{Schema: InputSchema, Source: i.Source, NativeInput: taskspec.NativeInput{Title: s.Title, Goal: "Return expected output", Scope: "Only output handling", Acceptance: []string{"Output matches the compatibility fixture"}, Requirements: req}}, s
}

func TestExactAdmissionAndRestartIdentity(t *testing.T) {
	in, s := intake(t)
	c, task, err := Prepare(in, "github", "github:https://github.com:o/r", 17, s)
	if err != nil {
		t.Fatal(err)
	}
	retry, again, err := Prepare(c.Input, "github", "github:https://github.com:o/r", 17, s)
	if err != nil || retry.ID != c.ID || again.DemandID != task.DemandID || again.Brief != task.Brief {
		t.Fatalf("restart changed contract: %+v %v", retry, err)
	}
	spec, err := taskspec.ParseTask(task.Brief, nil)
	if err != nil {
		t.Fatal(err)
	}
	if spec.Plan != "" || !strings.HasPrefix(spec.PlanTaskID, "adhoc/issue-") || spec.PlanFingerprint != c.ID || task.SourceKey != "github:https://github.com:o/r" {
		t.Fatalf("invented plan or lost identity: %+v %+v", spec, task)
	}
	for _, change := range []struct {
		name   string
		mutate func(*Input, *port.IssueSnapshot)
	}{
		{"wrong binding", func(i *Input, s *port.IssueSnapshot) { i.Source.BindingDigest = BindingDigest("other") }},
		{"wrong provider", func(i *Input, s *port.IssueSnapshot) { i.Source.Provider = "forgejo" }},
		{"wrong number", func(i *Input, s *port.IssueSnapshot) { s.Number++ }},
		{"closed", func(i *Input, s *port.IssueSnapshot) { s.Open = false }},
		{"rollup", func(i *Input, s *port.IssueSnapshot) { s.Executable = false }},
		{"outside open blocker", func(i *Input, s *port.IssueSnapshot) { s.Dependencies[0].Closed = false }},
		{"new dependency", func(i *Input, s *port.IssueSnapshot) {
			s.Dependencies = append(s.Dependencies, port.IssueDependency{Number: 22, Closed: true})
		}},
		{"edited title", func(i *Input, s *port.IssueSnapshot) { s.Title = "New scope" }},
		{"edited body", func(i *Input, s *port.IssueSnapshot) { s.Body = "New scope" }},
		{"edited citation", func(i *Input, s *port.IssueSnapshot) { s.Comments[0].Body = "New requirement" }},
		{"deleted citation", func(i *Input, s *port.IssueSnapshot) { s.Comments = nil }},
		{"conflicting capsule", func(i *Input, s *port.IssueSnapshot) {
			s.Comments = append(s.Comments, port.IssueComment{ID: "5", Body: "<!-- fs-spec:v2 {} -->"})
		}},
	} {
		t.Run(change.name, func(t *testing.T) {
			i, s := intake(t)
			change.mutate(&i, &s)
			if _, _, err := Prepare(i, "github", "github:https://github.com:o/r", 17, s); err == nil {
				t.Fatal("admitted unsafe source")
			}
		})
	}
	s.Comments = append(s.Comments, port.IssueComment{ID: "4", Body: "Generated progress"})
	if _, _, err := Prepare(in, "github", "github:https://github.com:o/r", 17, s); err != nil {
		t.Fatalf("uncited progress changed contract: %v", err)
	}
}

func TestIntakeClosedCodec(t *testing.T) {
	in, _ := intake(t)
	b, err := json.Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Decode(b); err != nil {
		t.Fatal(err)
	}
	for _, bad := range [][]byte{append([]byte(`{"unknown":true,`), b[1:]...), append([]byte(`{"schema_version":"duplicate",`), b[1:]...), append(b, []byte("{}")...), []byte(strings.Repeat("x", (1<<20)+1))} {
		if _, err := Decode(bad); err == nil {
			t.Fatal("accepted nonclosed intake")
		}
	}
}

type exactTracker struct {
	snapshot                   port.IssueSnapshot
	reads, frontiers, comments int
}

func (f *exactTracker) ReadIssue(_ context.Context, n int) (port.IssueSnapshot, error) {
	f.reads++
	if n != f.snapshot.Number {
		return port.IssueSnapshot{}, errors.New("unselected read")
	}
	return f.snapshot, nil
}
func (f *exactTracker) Frontier(context.Context) ([]port.Task, error) {
	f.frontiers++
	return nil, errors.New("unrelated malformed task")
}
func (f *exactTracker) Comment(context.Context, int, string) error { f.comments++; return nil }
func TestScopedTrackerNeverReadsOrdinaryFrontier(t *testing.T) {
	in, s := intake(t)
	c, task, err := Prepare(in, "github", "github:https://github.com:o/r", 17, s)
	if err != nil {
		t.Fatal(err)
	}
	f := &exactTracker{snapshot: s}
	scoped := &ScopedTracker{Reader: f, Tracker: f, Contract: c, Task: task, Provider: "github", Key: task.SourceKey}
	tasks, err := scoped.Frontier(t.Context())
	if err != nil || len(tasks) != 1 || f.frontiers != 0 || f.reads != 1 {
		t.Fatalf("selection widened: %+v %v %+v", tasks, err, f)
	}
	if err := scoped.Comment(t.Context(), 18, "bad"); err == nil || f.comments != 0 {
		t.Fatal("unselected write")
	}
	f.snapshot.Body = "changed"
	if _, err := scoped.Frontier(t.Context()); err == nil {
		t.Fatal("source drift at dispatch")
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), "GIT_CONFIG_NOSYSTEM=1", "GIT_CONFIG_GLOBAL=/dev/null", "GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.invalid", "GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.invalid")
	b, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %s %v", args, b, err)
	}
}
func stores(t *testing.T) (*Store, *Store) {
	t.Helper()
	root := t.TempDir()
	remote, left, right := filepath.Join(root, "bare"), filepath.Join(root, "left"), filepath.Join(root, "right")
	git(t, root, "init", "--bare", "--initial-branch=main", remote)
	git(t, root, "init", "--initial-branch=main", left)
	git(t, left, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "--allow-empty", "-m", "seed")
	git(t, left, "remote", "add", "origin", remote)
	git(t, left, "push", "origin", "main")
	git(t, root, "clone", remote, right)
	a, err := NewStore(left, "origin")
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewStore(right, "origin")
	if err != nil {
		t.Fatal(err)
	}
	return a, b
}
func TestGitAdmissionConcurrencyFreshCloneAndConflict(t *testing.T) {
	a, b := stores(t)
	in, s := intake(t)
	c, _, err := Prepare(in, "github", "github:https://github.com:o/r", 17, s)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 2)
	for _, store := range []*Store{a, b} {
		wg.Go(func() { errs <- store.Admit(t.Context(), c) })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Admit(t.Context(), c); err != nil {
		t.Fatal(err)
	}
	changed := in
	changed.Goal = "Different goal"
	different, _, err := Prepare(changed, "github", "github:https://github.com:o/r", 17, s)
	if err != nil {
		t.Fatal(err)
	}
	if err := b.Admit(t.Context(), different); err == nil {
		t.Fatal("overwrote immutable admission")
	}
	p := port.Proposal{Number: 77, Branch: "feat/issue"}
	if err := a.Associate(t.Context(), c, p); err != nil {
		t.Fatal(err)
	}
	sha := strings.Repeat("a", 40)
	if err := a.RecordLocal(t.Context(), c, p, sha); err != nil {
		t.Fatal(err)
	}
	read, ok, err := b.Proposal(t.Context(), c)
	if err != nil || !ok || read != p {
		t.Fatalf("fresh clone association: %+v %t %v", read, ok, err)
	}
	verified, err := b.LocalVerified(t.Context(), c, p, sha)
	if err != nil || !verified {
		t.Fatalf("fresh clone local proof: %t %v", verified, err)
	}
	if ok, err := b.LocalVerified(t.Context(), c, p, strings.Repeat("b", 40)); err != nil || ok {
		t.Fatal("old proof covered a new head")
	}
}
