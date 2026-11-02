package e2e

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	forgejocodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/report"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec/taskspectest"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	"github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

// This opt-in scenario reuses an existing Forgejo repository. Installation,
// task projection, Git, confinement, verification and publication are real;
// only the implementation/review model processes are deterministic helpers.
// Local CI keeps this loop fast and independent of an Actions runner queue.
func TestLiveForgejoInstalledFeatureDevelopment(t *testing.T) {
	remote := newForgejoRemote(t) // skips before release work unless opted in
	started := time.Now()
	release := buildForgejoFixtureRelease(t)
	repo := t.TempDir()
	env := forgejoInstalledEnv(t, repo)
	env, calls, harnessConfig := newForgejoHarness(t, env, repo, 1)
	machineBin := filepath.Join(t.TempDir(), "bin")
	for _, pair := range env {
		if path, ok := strings.CutPrefix(pair, "PATH="); ok {
			env = forgejoSetEnv(env, "PATH", machineBin+string(os.PathListSeparator)+path)
			break
		}
	}
	env = forgejoSetEnv(env, "PYXIS_INSTALL_DIR", machineBin)
	env = forgejoSetEnv(env, "PYXIS_VERSION", release.version)
	env = forgejoSetEnv(env, "PYXIS_ARTIFACT_SOURCE", release.source())
	env = forgejoSetEnv(env, "PYXIS_SHA256", hashFile(t, builtBins["pyxis"]))
	forgejoRun(t, repo, env, "sh", filepath.Join(repoRoot(), "install.sh"))
	installed := filepath.Join(machineBin, "pyxis")
	if hashFile(t, installed) != hashFile(t, builtBins["pyxis"]) {
		t.Fatal("machine install did not preserve the checksum-verified source binary")
	}
	forgejoRun(t, repo, env, "git", "init", "--initial-branch", remote.BaseBranch)
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion,
		Connections: map[string]integrations.Connection{"e2e": {Provider: integrations.Forgejo, BaseURL: remote.Binding.BaseURL, TokenEnv: "PYXIS_FORGEJO_TOKEN"}},
		Bindings: integrations.Bindings{
			Tasks:    integrations.Binding{Provider: integrations.Carina, Root: "."},
			CodeHost: integrations.Binding{Connection: "e2e", Repository: remote.Binding.Repository}},
		CI: integrations.CI{Mode: "local"}}
	profileBytes, err := profile.Render()
	if err != nil {
		t.Fatal(err)
	}
	profilePath := filepath.Join(t.TempDir(), "integrations.yaml")
	writeFile(t, profilePath, profileBytes, 0600)
	forgejoRun(t, repo, env, installed, initArgs(release, "--integrations", profilePath)...)
	installedHead := strings.TrimSpace(forgejoRun(t, repo, env, "git", "rev-parse", "HEAD"))

	// Initialize with local tasks, then exercise the supported binding switch.
	// Native-task cold init intentionally onboards a whole repository; this
	// scenario must not alter shared onboarding labels or welcome issues.
	profile.Bindings.Tasks = profile.Bindings.CodeHost
	profileBytes, err = profile.Render()
	if err != nil {
		t.Fatal(err)
	}
	configPath := filepath.Join(repo, ".pyxis/config.yaml")
	config, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	baseConfig, _, found := strings.Cut(string(config), "\nintegrations:\n")
	if !found {
		t.Fatal("installed configuration lacks explicit integration bindings")
	}
	evidenceRef := "refs/pyxis/" + remote.RunID + "/review"
	remote.ownRef(t, evidenceRef)
	remote.ownRef(t, evidenceRef+"-process")
	remote.ownRef(t, evidenceRef+"-publication")
	config = []byte(baseConfig + "\n" + string(profileBytes) +
		"\nexecutor:\n  review:\n    backend: codex\n    budget_minutes: 1\n    evidence_ref: " + evidenceRef +
		"\n    process_evidence_ref: " + evidenceRef + "-process\n    publication_evidence_ref: " + evidenceRef + "-publication" +
		"\ncode-gate:\n  unit-test:\n    command: go test -count=1 ./...\n    class: required\n    timeout-s: 60\n")
	writeFile(t, configPath, config, 0644)
	forgejoRun(t, repo, env, installed, "converge")
	writeFile(t, filepath.Join(repo, forgejoHarnessConfigPath), harnessConfig, 0644)
	writeFile(t, filepath.Join(repo, "go.mod"), []byte("module example.com/pyxis-forgejo-feature\n\ngo 1.24\n"), 0644)
	writeFile(t, filepath.Join(repo, "sum_test.go"), []byte(`package feature

import "testing"

func TestAdd(t *testing.T) {
	for _, c := range []struct { a, b, want int }{{2, 3, 5}, {-2, 2, 0}, {0, 0, 0}, {-4, -7, -11}} {
		if got := Add(c.a, c.b); got != c.want { t.Fatalf("Add(%d, %d) = %d, want %d", c.a, c.b, got, c.want) }
	}
}
`), 0644)
	baseline := exec.CommandContext(t.Context(), "go", "test", "-count=1", "./...")
	baseline.Dir, baseline.Env = repo, env
	if output, err := baseline.CombinedOutput(); err == nil || !strings.Contains(string(output), "undefined: Add") {
		t.Fatalf("seeded test must fail for the missing feature: %v\n%s", err, output)
	}
	forgejoRun(t, repo, env, "git", "add", ".pyxis/config.yaml", forgejoHarnessConfigPath, "go.mod", "sum_test.go")
	forgejoRun(t, repo, env, "git", "commit", "-m", "test: seed temporary Forgejo feature fixture")
	baseSHA := strings.TrimSpace(forgejoRun(t, repo, env, "git", "rev-parse", "HEAD"))
	remote.configureGit(t, repo, installed, env)
	remote.pushBase(t, repo, installedHead, baseSHA, env)
	spec, err := taskspectest.Spec("T1")
	if err != nil {
		t.Fatal(err)
	}
	spec.Goal = "Implement integer addition"
	spec.Acceptance = []string{"go test -count=1 ./... passes all signed and zero cases"}
	capsule, err := taskspec.RenderNative(spec)
	if err != nil {
		t.Fatal(err)
	}
	task := remote.createTask(t, forgejoHarnessTaskMarker+" add integers",
		"Implement Add(a, b int) int in sum.go. Preserve the supplied tests. "+forgejoHarnessTaskMarker,
		capsule)
	journal := filepath.Join(repo, ".pyxis/logs/executor/forgejo-feature.jsonl")
	args := []string{"exec", "implement-now", "--", "--repo", repo, "--remote", "origin", "--base", remote.BaseBranch,
		"--label", remote.Label, "--plan-ref", remote.PlanRef, "--ambient-backend", "codex", "--worker-budget", "90s", "--unattended", "--journal", journal}
	flowStarted := time.Now()
	output := forgejoRun(t, repo, env, installed, args...)
	for _, want := range []string{"succeeded: 1", "failed:    0", "pre-publish-reviewed: 1"} {
		if !strings.Contains(output, want) {
			t.Fatalf("feature sweep lacks %q:\n%s\njournal:\n%s", want, output, forgejoDiagnostic(journal))
		}
	}
	host := forgejocodehost.New(remote.Client, remote.Binding.BaseURL, remote.Binding.Repository, remote.BaseBranch)
	pull := forgejoFeaturePull(t, remote, host)
	remote.registerProposal(t, port.Proposal{Number: pull.Number, Branch: pull.Head.Ref})
	forgejoAssertFeature(t, remote, host, pull, task, repo, env, baseSHA, journal, calls())
	firstPublication := forgejoPublicationSnapshot(t, remote, pull, task)
	firstCalls := len(calls())
	output = forgejoRun(t, repo, env, installed, args...)
	if !strings.Contains(output, "awaiting-proposal: 1") || len(calls()) != firstCalls {
		t.Fatalf("repeat sweep relaunched work or lost the existing proposal:\n%s\ncalls before=%d after=%d", output, firstCalls, len(calls()))
	}
	if again := forgejoFeaturePull(t, remote, host); again.Number != pull.Number || again.Head.SHA != pull.Head.SHA {
		t.Fatal("repeat sweep created or changed a proposal")
	}
	if after := forgejoPublicationSnapshot(t, remote, pull, task); after != firstPublication {
		t.Fatalf("repeat sweep changed native comments or statuses: before=%+v after=%+v", firstPublication, after)
	}
	t.Logf("installed feature + repeat sweep %s; install/build + flow %s; task #%d, draft PR #%d, head %s", time.Since(flowStarted).Round(time.Millisecond), time.Since(started).Round(time.Millisecond), task.Number, pull.Number, pull.Head.SHA)
}

func forgejoInstalledEnv(t *testing.T, repo string) []string {
	t.Helper()
	env := pyxisEnv(t, repo)
	tools := t.TempDir()
	for _, name := range []string{"uname", "tr", "mktemp", "cp", "chmod", "mv", "mkdir", "rm", "sha256sum", "shasum", "cut", "openssl", "awk", "dirname", "cat", "perl", "zsh", "sandbox-exec"} {
		if executable, err := exec.LookPath(name); err == nil {
			if err := os.Symlink(executable, filepath.Join(tools, name)); err != nil {
				t.Fatal(err)
			}
		}
	}
	for _, pair := range env {
		if path, ok := strings.CutPrefix(pair, "PATH="); ok {
			env = forgejoSetEnv(env, "PATH", tools+string(os.PathListSeparator)+path)
			break
		}
	}
	for _, name := range []string{"ZDOTDIR", "XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "CODEX_HOME"} {
		env = forgejoSetEnv(env, name, t.TempDir())
	}
	env = forgejoSetEnv(env, "GIT_TERMINAL_PROMPT", "0")
	return forgejoSetEnv(env, "PYXIS_FORGEJO_TOKEN", os.Getenv("PYXIS_FORGEJO_TOKEN"))
}

func forgejoSetEnv(env []string, name, value string) []string {
	next := make([]string, 0, len(env)+1)
	for _, pair := range env {
		if !strings.HasPrefix(pair, name+"=") {
			next = append(next, pair)
		}
	}
	return append(next, name+"="+value)
}

func forgejoRun(t *testing.T, dir string, env []string, program string, args ...string) string {
	t.Helper()
	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, program, args...)
	cmd.Dir, cmd.Env = dir, env
	output, err := cmd.CombinedOutput()
	text := string(output)
	if token := os.Getenv("PYXIS_FORGEJO_TOKEN"); token != "" {
		text = strings.ReplaceAll(text, token, "[redacted]")
	}
	if err != nil {
		t.Fatalf("%s %s: %v\n%s", filepath.Base(program), strings.Join(args, " "), err, text)
	}
	return text
}

func forgejoFeaturePull(t *testing.T, remote *forgejoRemote, host *forgejocodehost.Host) forgejocodehost.Pull {
	t.Helper()
	pulls, err := host.Pulls(t.Context(), "open")
	if err != nil {
		t.Fatal(err)
	}
	var owned []forgejocodehost.Pull
	for _, pull := range pulls {
		if pull.Base.Ref == remote.BaseBranch {
			owned = append(owned, pull)
		}
	}
	if len(owned) != 1 || !owned[0].Draft || owned[0].Merged {
		t.Fatalf("want one open unmerged draft proposal for temporary base, got %+v", owned)
	}
	return owned[0]
}

func forgejoAssertFeature(t *testing.T, remote *forgejoRemote, host *forgejocodehost.Host, pull forgejocodehost.Pull, task port.Task, repo string, env []string, baseSHA, journal string, calls []forgejoHarnessCall) {
	t.Helper()
	var implement, review []forgejoHarnessCall
	for _, call := range calls {
		if !call.Accepted {
			t.Errorf("unexpected model invocation: %+v", call)
		}
		if call.Mode != "version" && !call.ForgejoTokenAbsent {
			t.Error("model worker or reviewer inherited the code-host token")
		}
		switch call.Mode {
		case "implement":
			implement = append(implement, call)
		case "review":
			review = append(review, call)
		}
	}
	if len(implement) != 1 || len(review) != 1 || implement[0].Dir == repo || review[0].Dir == repo || implement[0].Dir == review[0].Dir {
		t.Fatalf("wanted one isolated worker and one independent reviewer, got %+v", calls)
	}
	if !strings.Contains(review[0].Prompt, pull.Head.SHA) || !strings.Contains(review[0].Prompt, "do not look for a pull request") {
		t.Fatal("review did not cover the exact local revision before publication")
	}
	if got := strings.TrimSpace(forgejoRun(t, repo, env, "git", "rev-parse", "HEAD")); got != baseSHA {
		t.Fatal("implementation advanced the operand checkout")
	}
	if _, err := os.Stat(filepath.Join(repo, "sum.go")); !os.IsNotExist(err) {
		t.Fatalf("implementation escaped into the operand: %v", err)
	}
	forgejoRun(t, repo, env, "git", "fetch", "origin", pull.Head.Ref)
	if source := forgejoRun(t, repo, env, "git", "show", pull.Head.SHA+":sum.go"); source != forgejoHarnessSource {
		t.Fatal("real remote proposal does not contain the implemented feature")
	}
	for _, path := range []string{"go.mod", "sum_test.go", ".pyxis/config.yaml"} {
		before := forgejoRun(t, repo, env, "git", "show", baseSHA+":"+path)
		after := forgejoRun(t, repo, env, "git", "show", pull.Head.SHA+":"+path)
		if before != after {
			t.Errorf("feature changed its tests or verification configuration: %s", path)
		}
	}
	if state, err := host.ReadStatus(t.Context(), pull.Head.SHA, "pyxis/local-gate"); err != nil || state != port.CheckGreen {
		t.Fatalf("real exact-head local verification status = %s: %v", state, err)
	}
	var comments []struct {
		Body string `json:"body"`
	}
	if err := remote.Client.List(t.Context(), remote.path(fmt.Sprintf("issues/%d/comments", pull.Number)), &comments); err != nil {
		t.Fatal(err)
	}
	artifacts := 0
	for _, comment := range comments {
		payload, err := persist.ParseArtifact(comment.Body)
		if err != nil {
			continue
		}
		sha, ok := verdict.LatestReviewedRevision(payload)
		if !ok || sha != pull.Head.SHA {
			t.Fatal("published review covers a different revision")
		}
		artifacts++
	}
	if artifacts != 1 {
		t.Fatalf("published review artifacts = %d, want 1", artifacts)
	}
	comments = nil
	if err := remote.Client.List(t.Context(), remote.path(fmt.Sprintf("issues/%d/comments", task.Number)), &comments); err != nil {
		t.Fatal(err)
	}
	if len(comments) < 2 {
		t.Fatal("task has no real executor progress comment beyond its capsule")
	}
	refs, err := remote.remoteRefs(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if refs["refs/claim/"+task.ScopeToken()+"/"+fmt.Sprint(task.Number)] != "" {
		t.Fatal("worker retained its remote task claim")
	}
	if refs["refs/heads/main"] != remote.MainSHA {
		t.Fatal("feature execution changed shared main")
	}
	var issue struct {
		State string `json:"state"`
	}
	if err := remote.Client.Request(t.Context(), http.MethodGet, remote.path(fmt.Sprintf("issues/%d", task.Number)), nil, &issue); err != nil {
		t.Fatal(err)
	}
	if issue.State != "open" {
		t.Fatal("unmerged feature task was closed prematurely")
	}
	scanner := bufio.NewScanner(strings.NewReader(forgejoDiagnostic(journal)))
	scanner.Buffer(make([]byte, 4096), 2<<20)
	verified, succeeded, reviewed := false, false, false
	for scanner.Scan() {
		var event report.Event
		if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
			t.Fatal(err)
		}
		switch event.Kind {
		case report.KindTaskVerified:
			verified = event.Verdict == "green"
		case report.KindTaskOutcome:
			succeeded = event.Class == "succeeded"
		case report.KindPrePublishReviewTerminal:
			reviewed = event.Covered && event.Verdict == "approve" && event.Round == 1
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatal(err)
	}
	if !verified || !succeeded || !reviewed {
		t.Fatalf("missing actual verification/review/worker terminals:\n%s", forgejoDiagnostic(journal))
	}
}

func forgejoDiagnostic(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return err.Error()
	}
	return string(b)
}
