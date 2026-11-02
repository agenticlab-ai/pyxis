package installer

import (
	"bytes"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/gitcredential"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

func TestPrepareIntegrationBindingsAndConfigSurviveEpoch(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"local": {Provider: "forgejo", BaseURL: "http://127.0.0.1:3000", TokenEnv: "TEST_INSTALL_TOKEN"}}, CI: integrations.CI{Mode: "both", RunnerLabel: "pyxis-linux"}}
	profile.Bindings.Tasks = integrations.Binding{Provider: "carina", Root: "work-items"}
	profile.Bindings.CodeHost = integrations.Binding{Connection: "local", Repository: "o/code"}
	opts := Options{Integrations: profile}
	if err := prepareIntegrations(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.Answers.Tracker != "carina" || opts.Answers.Coordinate != "o/code" || opts.Answers.CIMode != "both" || !opts.AnswersProvided {
		t.Fatalf("%#v", opts.Answers)
	}
	t.Setenv("TEST_INSTALL_TOKEN", "never-serialize-this")
	for _, ep := range []*config.Epoch{nil, {Commit: "abc", SystemVersion: "v1.0.0", ManifestSHA256: strings.Repeat("a", 64)}} {
		b, err := appendIntegrations(renderConfig(Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("a", 64)}, "", config.Knobs{Tracker: "carina", CIMode: "both", Wiring: config.Wiring{Repo: "use", Protection: "required-checks"}}, "main", ep), profile)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(b), "never-serialize-this") {
			t.Fatal("credential serialized")
		}
		c, err := integrations.Parse(b)
		if err != nil {
			t.Fatal(err)
		}
		host, err := c.CodeHost("")
		if err != nil || host.Repository != "o/code" {
			t.Fatalf("epoch discarded binding: %#v %v", host, err)
		}
		if _, d := config.Parse(b); len(d) != 0 {
			t.Fatalf("foreign config rejected: %v", d)
		}
	}
}

func TestIntegrationRemoteKeepsSelectedOriginAndRefusesConflicts(t *testing.T) {
	run := tailBoundaryRun(t)
	run.repo.Remote = "origin"
	run.opts.Integrations = integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"host": {Provider: integrations.Forgejo, BaseURL: "https://forge.example"}}}
	run.opts.Integrations.Bindings.CodeHost = integrations.Binding{Connection: "host", Repository: "o/code"}
	if err := prepareIntegrations(&run.opts); err != nil || run.opts.Answers.CIMode != "both" {
		t.Fatalf("explicit code host did not receive its CI default: %v", err)
	}
	if err := stageIntegrationRemote(t.Context(), run); err == nil {
		t.Fatal("configured a remote outside a Git repository")
	}
	if out, err := exec.Command("git", "-C", run.opts.Target, "init", "--quiet").CombinedOutput(); err != nil {
		t.Fatalf("git init %v %s", err, out)
	}
	if err := stageIntegrationRemote(t.Context(), run); err != nil {
		t.Fatal(err)
	}
	if err := stageIntegrationRemote(t.Context(), run); err != nil {
		t.Fatalf("matching remote refused: %v", err)
	}
	run.opts.Integrations.Bindings.CodeHost.Repository = "o/other"
	if err := stageIntegrationRemote(t.Context(), run); err == nil || !strings.Contains(err.Error(), "differs") {
		t.Fatalf("conflicting remote accepted: %v", err)
	}
	run.opts.Integrations.Bindings.CodeHost.Connection = "absent"
	if err := stageIntegrationRemote(t.Context(), run); err == nil {
		t.Fatal("missing selected connection accepted")
	}
	run.opts.Integrations.Bindings.CodeHost = integrations.Binding{}
	run.knobs.Tracker, run.knobs.Wiring.Protection = integrations.Carina, "none"
	if err := stageIntegrationRemote(t.Context(), run); err != nil {
		t.Fatal("local-only init unexpectedly required a hosted remote")
	}
}

func TestInvalidIntegrationSetupRefusesBeforeWriting(t *testing.T) {
	invalid := integrations.Config{SchemaVersion: "unsupported"}
	if _, err := Init(t.Context(), Options{}); err == nil {
		t.Fatal("init accepted missing target and pin")
	}
	if _, err := Init(t.Context(), Options{Target: t.TempDir(), Pin: Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("a", 64)}, Integrations: invalid}); err == nil {
		t.Fatal("init accepted invalid integration profile")
	}
	if err := prepareIntegrations(&Options{Integrations: invalid}); err == nil {
		t.Fatal("invalid profile reached setup")
	}
	if _, err := appendIntegrations(nil, invalid); err == nil {
		t.Fatal("invalid profile rendered into operand config")
	}
	run := tailBoundaryRun(t)
	run.opts.Stdout = io.Discard
	run.opts.Integrations = invalid
	run.opts.Integrations.Bindings.Tasks = integrations.Binding{Connection: "absent", Repository: "o/tasks"}
	if err := stageWiring(t.Context(), run); err == nil {
		t.Fatal("invalid task connection reached wiring transport")
	}
	if err := stageEpoch(t.Context(), run); err == nil {
		t.Fatal("invalid profile reached epoch config")
	}
	run.rep.Commits = []Commit{{SHA: strings.Repeat("a", 40)}}
	if err := stageEpoch(t.Context(), run); err == nil {
		t.Fatal("invalid profile reached config after tree commit")
	}
	if _, err := os.Stat(filepath.Join(run.opts.Target, ".pyxis", "config.yaml")); !os.IsNotExist(err) {
		t.Fatal("invalid profile wrote operand configuration")
	}
}

func TestNativeRemoteCleanupAndCredentialRestoreFailureStayVisible(t *testing.T) {
	run := tailBoundaryRun(t)
	bound := integrations.ResolvedRepository{Provider: integrations.Forgejo, BaseURL: "https://forge.example", Repository: "o/code", TokenEnv: "TEST_NATIVE_AUTH"}
	ops := []wire.Op{{Name: wire.OpCreateRepo, Transport: wire.TransportForgejo, Connection: bound, Slug: bound.Repository}}
	audit := wire.Audit{Operations: []wire.OpResult{{Name: wire.OpCreateRepo, Transport: wire.TransportForgejo, Connection: bound, Outcome: wire.OutcomeApplied}, {Name: wire.OpPushWorkflow, Transport: wire.TransportConverge, Connection: bound, Outcome: wire.OutcomeApplied}}}
	recordRemoteCreations(run.j, ops, audit)
	run.j.recordCredentials([]gitcredential.Change{{Key: "credential.helper", Before: []string{"prior"}}})
	rb := run.unwind(t.Context(), "later-stage")
	if rb.Restored || len(rb.Errors) == 0 || len(rb.RemoteCleanup) != 1 || !strings.Contains(rb.RemoteCleanup[0], "${TEST_NATIVE_AUTH}") {
		t.Fatalf("cleanup evidence lost: %#v", rb)
	}
	var out bytes.Buffer
	printWiringChecklist(&out, bound.Repository, audit)
	if !strings.Contains(out.String(), "your code host") || strings.Contains(out.String(), "GitHub") {
		t.Fatalf("native workflow checklist points to wrong provider: %s", out.String())
	}
	steps := welcomeNextSteps(config.Knobs{Tracker: integrations.Forgejo}, config.Defaults(), "", nil)
	if !strings.Contains(strings.Join(steps, "\n"), "Forgejo tracker") {
		t.Fatal("native welcome fallback lost its selected tracker")
	}
}

func TestUnconfiguredForgejoRefusesBeforeInit(t *testing.T) {
	if err := prepareIntegrations(&Options{Answers: Answers{Tracker: "forgejo"}}); err == nil || !strings.Contains(err.Error(), "--integrations") {
		t.Fatalf("%v", err)
	}
}

func TestArtifactProfileRetainsLocalTaskDefaults(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion, Connections: map[string]integrations.Connection{"local": {Provider: "forgejo", BaseURL: "http://127.0.0.1:3000"}}}
	profile.Bindings.Artifacts = integrations.Binding{Connection: "local", Repository: "o/releases"}
	opts := Options{Integrations: profile, Answers: Answers{Tracker: "carina", CIMode: "local"}}
	if err := prepareIntegrations(&opts); err != nil {
		t.Fatal(err)
	}
	if opts.Answers.Tracker != "carina" || opts.Answers.CIMode != "local" || opts.Answers.Coordinate != "" {
		t.Fatalf("artifact binding changed collaboration: %#v", opts.Answers)
	}
}

func TestInitCarinaBindingOwnsOnlyItsTargetOperand(t *testing.T) {
	profile := integrations.Config{SchemaVersion: integrations.SchemaVersion}
	profile.Bindings.Tasks = integrations.Binding{Provider: "carina", Root: "../other-operand"}
	opts := Options{Target: t.TempDir(), Integrations: profile}
	if err := prepareIntegrations(&opts); err == nil || !strings.Contains(err.Error(), "tasks.root") {
		t.Fatalf("external operand admitted during init: %v", err)
	}
	profile.Bindings.Tasks.Root = "."
	opts.Integrations = profile
	if err := prepareIntegrations(&opts); err != nil {
		t.Fatalf("same operand rejected: %v", err)
	}
}
