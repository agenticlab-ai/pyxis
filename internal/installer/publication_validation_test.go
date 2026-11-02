package installer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

func publicationGitFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, args := range [][]string{{"init", "-b", "main"}, {"config", "user.name", "Test"}, {"config", "user.email", "test@example.invalid"}} {
		if _, err := gitOutput(t.Context(), root, args...); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestPendingPublicationRejectsMalformedInstallerState(t *testing.T) {
	for name, body := range map[string]string{
		"malformed JSON":    "{",
		"unknown field":     `{"schema_version":"pyxis-init-publication/v1","unknown":true}`,
		"trailing JSON":     `{} {}`,
		"incomplete record": `{"schema_version":"pyxis-init-publication/v1"}`,
		"wrong schema":      `{"schema_version":"future/v99"}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := publicationGitFixture(t)
			if _, err := gitOutput(t.Context(), root, "config", "--local", publicationKey, body); err != nil {
				t.Fatal(err)
			}
			if p, err := readPendingPublication(t.Context(), root); err == nil || p != nil {
				t.Fatalf("unsafe state accepted: %+v %v", p, err)
			}
			if _, err := Init(t.Context(), Options{Target: root, Pin: Pin{Version: "v1.0.0", ManifestSHA256: strings.Repeat("a", 64)}}); err == nil || !strings.Contains(err.Error(), "pending publication") {
				t.Fatalf("init did not refuse malformed pending state before admission: %v", err)
			}
			if _, err := os.Stat(filepath.Join(root, ".pyxis")); !os.IsNotExist(err) {
				t.Fatalf("malformed pending state caused installation writes: %v", err)
			}
		})
	}
}

func TestInitialPublicationRejectsAmbiguousRemoteConfiguration(t *testing.T) {
	for name, setup := range map[string][][]string{
		"missing remote":     nil,
		"different push URL": {{"remote", "add", "origin", "/local/fetch.git"}, {"config", "remote.origin.pushurl", "/local/push.git"}},
		"multiple push URLs": {{"remote", "add", "origin", "/local/fetch.git"}, {"config", "--add", "remote.origin.pushurl", "/local/fetch.git"}, {"config", "--add", "remote.origin.pushurl", "/local/other.git"}},
	} {
		t.Run(name, func(t *testing.T) {
			root := publicationGitFixture(t)
			for _, args := range setup {
				if _, err := gitOutput(t.Context(), root, args...); err != nil {
					t.Fatal(err)
				}
			}
			if url, err := publicationDestination(t.Context(), root, "origin"); err == nil || url != "" {
				t.Fatalf("ambiguous destination accepted: %s %v", url, err)
			}
		})
	}
}

func TestPublicationRequiresVerifiedCommittedInstallation(t *testing.T) {
	root := publicationGitFixture(t)
	run := &initRun{opts: Options{Target: root}, rep: &Report{}, repo: config.Defaults()}
	if err := stageFirstPush(t.Context(), run); err == nil {
		t.Fatal("publication without installation proof was accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := stageFirstPush(ctx, run); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled publication: %v", err)
	}
	if err := validatePublication(t.Context(), run, pendingPublication{}); err == nil {
		t.Fatal("unborn HEAD accepted")
	}
	if _, err := gitOutput(t.Context(), root, "commit", "--allow-empty", "-m", "forged init root"); err != nil {
		t.Fatal(err)
	}
	head, err := gitOutput(t.Context(), root, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	p := pendingPublication{Head: strings.TrimSpace(head), Branch: "main", Remote: "origin", Tree: strings.TrimSpace(head)}
	if err := validatePublication(t.Context(), run, p); err == nil {
		t.Fatal("a public commit subject authenticated a forged installation")
	}
	if _, err := gitOutput(t.Context(), root, "checkout", "--detach"); err != nil {
		t.Fatal(err)
	}
	if err := validatePublication(t.Context(), run, p); err == nil {
		t.Fatal("detached installation head accepted")
	}
	if !publicationSHA(strings.Repeat("a", 40)) || publicationSHA("HEAD") || publicationSHA(strings.Repeat("g", 40)) {
		t.Fatal("installation SHA validation failed")
	}
	cause := errors.New("cancelled transport")
	if !errors.Is(&publicationUncertainError{cause}, cause) {
		t.Fatal("uncertain publication hid its transport cause")
	}
}

func TestPublicationDestinationAnswersRemainClosed(t *testing.T) {
	ans, err := ParseAnswers([]byte("remote_url: \"/local/destination.git\"\n"))
	if err != nil || ans.RemoteURL != "/local/destination.git" {
		t.Fatalf("remote_url: %+v %v", ans, err)
	}
	for _, url := range []string{"-unsafe", "unsafe\nremote", "unsafe\x00remote"} {
		opts := Options{Target: t.TempDir(), Answers: Answers{RemoteURL: url}}
		if err := resolvePublicationDestination(t.Context(), &opts, config.Knobs{}, nil); err == nil {
			t.Fatalf("unsafe destination %q accepted", url)
		}
	}
	root := publicationGitFixture(t)
	if _, err := gitOutput(t.Context(), root, "remote", "add", "origin", "/chosen.git"); err != nil {
		t.Fatal(err)
	}
	opts := Options{Target: root, Answers: Answers{RemoteURL: "/different.git"}}
	if err := resolvePublicationDestination(t.Context(), &opts, config.Knobs{}, nil); err == nil {
		t.Fatal("conflicting explicit destination silently replaced configured remote")
	}
	// A nested target cannot inherit pending publication from an enclosing repo.
	nested := filepath.Join(root, "nested")
	if err := os.Mkdir(nested, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := gitOutput(t.Context(), root, "config", publicationKey, "bad parent state"); err != nil {
		t.Fatal(err)
	}
	if p, err := readPendingPublication(t.Context(), nested); err != nil || p != nil {
		t.Fatalf("parent installation state leaked: %+v %v", p, err)
	}
}

func TestPublicationDestinationMatchesSelectedCodeHost(t *testing.T) {
	knobs := config.Knobs{Tracker: "github"}
	opts := Options{Target: t.TempDir(), Answers: Answers{Coordinate: "owner/repository", RemoteURL: "/different/local.git"}}
	if err := resolvePublicationDestination(t.Context(), &opts, knobs, nil); err == nil {
		t.Fatal("explicit URL redirected the chosen GitHub destination")
	}
	opts.Answers.RemoteURL = wire.RemoteURL(opts.Answers.Coordinate)
	if err := resolvePublicationDestination(t.Context(), &opts, knobs, nil); err != nil {
		t.Fatalf("matching GitHub destination refused: %v", err)
	}
	opts.Integrations = integrations.Config{SchemaVersion: integrations.SchemaVersion,
		Connections: map[string]integrations.Connection{"host": {Provider: integrations.Forgejo, BaseURL: "https://forge.example"}},
		Bindings:    integrations.Bindings{CodeHost: integrations.Binding{Connection: "host", Repository: "owner/repository"}}}
	if err := resolvePublicationDestination(t.Context(), &opts, knobs, nil); err == nil {
		t.Fatal("GitHub URL silently replaced the Forgejo binding")
	}
	opts.Answers.RemoteURL = "https://forge.example/owner/repository.git"
	if err := resolvePublicationDestination(t.Context(), &opts, knobs, nil); err != nil {
		t.Fatalf("matching Forgejo destination refused: %v", err)
	}
	delete(opts.Integrations.Connections, "host")
	if err := resolvePublicationDestination(t.Context(), &opts, knobs, nil); err == nil {
		t.Fatal("missing code-host connection accepted")
	}
	opts.Integrations = integrations.Config{}
	opts.Answers.RemoteURL = ""
	if err := resolvePublicationDestination(t.Context(), &opts, config.Knobs{}, newPrompter(t.Context(), strings.NewReader(""), nil)); err == nil {
		t.Fatal("EOF selected a local publication destination")
	}
}
