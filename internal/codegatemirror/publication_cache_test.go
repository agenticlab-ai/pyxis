package codegatemirror

import (
	"os"
	"os/exec"
	"testing"
)

func TestPublicationAndReviewRefsDoNotReplayUnchangedSourceChecks(t *testing.T) {
	dir := gitFixture(t)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@e", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@e")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	git("checkout", "-qb", "feat/task")
	writeF(t, dir, ".gitignore", "counter.log\n")
	cfg := Config{Checks: []Check{{Name: "counter", Command: "sh -c 'echo ran >> counter.log'", Class: ClassRequired}}}
	if rep, hit, err := cfg.RunCached(t.Context(), dir); err != nil || hit || !rep.Passed {
		t.Fatalf("initial run: %+v %t %v", rep, hit, err)
	}
	for _, ref := range []string{"refs/remotes/origin/feat/task", "refs/claim/42", "refs/pyxis/implement-now-review-state", "refs/pyxis/impl-review-process-state", "refs/pyxis/impl-review-publication-state", "refs/pyxis/implement-now-issue-state"} {
		git("update-ref", ref, "HEAD")
		if rep, hit, err := cfg.RunCached(t.Context(), dir); err != nil || !hit || !rep.Passed {
			t.Fatalf("publication %s replayed checks: %+v %t %v", ref, rep, hit, err)
		}
	}
	git("update-ref", "refs/remotes/origin/main", "HEAD")
	if _, hit, err := cfg.RunCached(t.Context(), dir); err != nil || hit {
		t.Fatalf("trusted mainline must invalidate: hit=%t err=%v", hit, err)
	}
}
