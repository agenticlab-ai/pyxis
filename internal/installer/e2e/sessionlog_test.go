package e2e

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestSessionLogsAreOptionalAfterInstall(t *testing.T) {
	r := buildFixtureRelease(t)
	repo, env := scratchRepo(t, r)
	agents, err := os.ReadFile(at(repo, "AGENTS.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(agents), "A session with nothing durable to add creates no documentation.") || strings.Contains(string(agents), "open a session log") {
		t.Fatalf("installed manual requires session bookkeeping:\n%s", agents)
	}
	if _, err := os.Stat(at(repo, "logs")); !os.IsNotExist(err) {
		t.Fatalf("installation created session history: %v", err)
	}
	if err := os.Mkdir(at(repo, "logs"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(at(repo, "logs/optional.md"), []byte("optional local diagnostic\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "check-ignore", "logs/optional.md")
	cmd.Dir, cmd.Env = repo, env
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("optional log enters Git: %v\n%s", err, out)
	}
	res := pyxis(t, repo, env, "exec", "docgate", "--", "gate")
	if res.code != 0 {
		t.Fatalf("optional log blocks installed gate: %d\n%s\n%s", res.code, res.stdout, res.stderr)
	}
}
