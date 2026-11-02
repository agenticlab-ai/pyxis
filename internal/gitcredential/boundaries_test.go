package gitcredential

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

type credentialTransport func(*http.Request) (*http.Response, error)

func (f credentialTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type brokenCredentialIO struct{}

func (brokenCredentialIO) Read([]byte) (int, error)  { return 0, errors.New("input unavailable") }
func (brokenCredentialIO) Write([]byte) (int, error) { return 0, errors.New("output unavailable") }

func TestServeUsesSelectedNativeActorAndFailsClosed(t *testing.T) {
	root := credentialRoot(t)
	t.Setenv("TEST_GIT_CREDENTIAL", "selected-token")
	prior := http.DefaultTransport
	t.Cleanup(func() { http.DefaultTransport = prior })
	calls := 0
	http.DefaultTransport = credentialTransport(func(r *http.Request) (*http.Response, error) {
		calls++
		if r.URL.String() != "https://forge.example:8443/base/api/v1/user" || r.Header.Get("Authorization") != "token selected-token" {
			t.Fatal("actor request escaped selected origin or credential")
		}
		return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(`{"id":42,"login":"assistant"}`)), Header: make(http.Header)}, nil
	})
	input := "protocol=https\nhost=forge.example:8443\npath=base/owner/code.git\npassword=ignored\n\n"
	var out bytes.Buffer
	if err := Serve(t.Context(), root, "get", strings.NewReader(input), &out); err != nil || out.String() != "username=assistant\npassword=selected-token\n\n" || calls != 1 {
		t.Fatalf("credential actor flow failed: calls=%d err=%v", calls, err)
	}
	if err := Serve(t.Context(), root, "get", strings.NewReader(input), brokenCredentialIO{}); err == nil {
		t.Fatal("ignored credential output failure")
	}
	if err := Serve(t.Context(), root, "get", brokenCredentialIO{}, io.Discard); err == nil {
		t.Fatal("ignored credential input failure")
	}
	if _, err := readRequest(strings.NewReader("host=host\n")); err != nil {
		t.Fatal(err)
	}
	if matches(request{protocol: "https", host: "host", path: "%zz"}, integrations.ResolvedRepository{BaseURL: "https://host", Repository: "o/r"}) {
		t.Fatal("malformed encoded path matched")
	}
	if _, err := configuredActor(t.Context(), integrations.ResolvedRepository{BaseURL: ":bad"}); err == nil {
		t.Fatal("invalid actor origin accepted")
	}
	if err := Serve(t.Context(), t.TempDir(), "get", strings.NewReader(input), io.Discard); err != nil {
		t.Fatal("legacy repository must not emit a selected credential")
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("integrations:\n  bad: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := Serve(t.Context(), root, "get", strings.NewReader(input), io.Discard); err == nil {
		t.Fatal("invalid integration configuration accepted")
	}
}

func TestCredentialExecutableAndGitHubActor(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("PATH", dir)
	root := credentialRoot(t)
	if _, err := Executable(); err == nil {
		t.Fatal("transient test executable accepted without installed pyxis")
	}
	if _, err := Plan(t.Context(), root, ""); err == nil {
		t.Fatal("credential plan retained a transient executable")
	}
	exe := filepath.Join(dir, "pyxis")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\nexit 0\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if got, err := Executable(); err != nil || got != exe {
		t.Fatalf("durable executable %q %v", got, err)
	}
	if _, err := Plan(t.Context(), root, ""); err == nil || !strings.Contains(err.Error(), "inspect Git repository root") {
		t.Fatalf("missing Git executable was not diagnosed: %v", err)
	}
	if transientExecutable(exe) || !transientExecutable("/tmp/go-build123/exe/pyxis") || !transientExecutable("/tmp/pyxis.test") {
		t.Fatal("transient executable policy failed")
	}
	t.Setenv("TEST_GIT_CREDENTIAL", "selected")
	t.Setenv("GH_TOKEN", "ambient")
	t.Setenv("GH_HOST", "other.example")
	if err := os.WriteFile(filepath.Join(dir, "gh"), []byte("#!/bin/sh\n[ \"$GH_HOST\" = github.com ] && [ \"$GH_TOKEN\" = selected ] || exit 2\nprintf 'assistant\\n'\n"), 0755); err != nil {
		t.Fatal(err)
	}
	got, err := configuredActor(t.Context(), integrations.ResolvedRepository{Provider: integrations.GitHub, TokenEnv: "TEST_GIT_CREDENTIAL"})
	if err != nil || got != "assistant" {
		t.Fatalf("selected GitHub actor %q %v", got, err)
	}
}

func TestCredentialPlanRejectsInvalidInputsAndLocalGitFailures(t *testing.T) {
	root := credentialRoot(t)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	profile, err := integrations.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, candidate := range []string{filepath.Join(root, "missing"), root} {
		if _, err := Plan(t.Context(), root, candidate); err == nil {
			t.Fatal("untrusted executable accepted")
		}
	}
	if changes, err := ConfigureForProfile(t.Context(), root, exe, integrations.Config{}); err != nil || len(changes) != 0 {
		t.Fatal("legacy configuration changed")
	}
	github := integrations.Config{Connections: map[string]integrations.Connection{"gh": {Provider: integrations.GitHub}}}
	github.Bindings.CodeHost = integrations.Binding{Connection: "gh", Repository: "o/r"}
	if changes, err := PlanForProfile(t.Context(), root, exe, github); err != nil || len(changes) != 0 {
		t.Fatal("unscoped GitHub credential setup changed")
	}
	github.Bindings.CodeHost.Connection = "absent"
	if _, err := PlanForProfile(t.Context(), root, exe, github); err == nil {
		t.Fatal("invalid connection accepted")
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := PlanForProfile(ctx, root, exe, profile); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancelled plan %v", err)
	}
	gitTest(t, root, "init", "--quiet")
	if err := os.WriteFile(filepath.Join(root, ".git", "config"), []byte("[broken"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := readValues(t.Context(), root, "credential.helper"); err == nil {
		t.Fatal("corrupt local configuration accepted")
	}
	if err := writeValues(t.Context(), root, "credential.helper", []string{"value"}); err == nil {
		t.Fatal("corrupt local configuration overwritten")
	}
	if err := Restore(t.Context(), root, []Change{{Key: "credential.helper"}}); err == nil {
		t.Fatal("failed restoration ignored")
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("integrations:\n  bad: true\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Plan(t.Context(), root, exe); err == nil {
		t.Fatal("invalid profile accepted by Plan")
	}
	if _, err := Configure(t.Context(), root, exe); err == nil {
		t.Fatal("invalid profile accepted by Configure")
	}
}

func TestCredentialConfigureRestoresAfterPartialWriteFailure(t *testing.T) {
	for _, restoreFails := range []bool{false, true} {
		t.Run(map[bool]string{false: "restored", true: "restore-failed"}[restoreFails], func(t *testing.T) {
			root := credentialRoot(t)
			gitTest(t, root, "init", "--quiet")
			key := "credential.https://forge.example:8443/base/owner/code.helper"
			gitTest(t, root, "config", "--add", key, "original")
			realGit, err := exec.LookPath("git")
			if err != nil {
				t.Fatal(err)
			}
			dir := t.TempDir()
			marker := filepath.Join(dir, "failed")
			script := "#!/bin/sh\nif [ \"$5\" = --add ] && [ \"$6\" = " + quote(key) + " ] && [ ! -f " + quote(marker) + " ]; then : > " + quote(marker) + "; exit 9; fi\n"
			if restoreFails {
				script += "[ ! -f " + quote(marker) + " ] || exit 9\n"
			}
			script += "exec " + quote(realGit) + " \"$@\"\n"
			if err := os.WriteFile(filepath.Join(dir, "git"), []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			exe, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			changes, err := Configure(t.Context(), root, exe)
			if err == nil || strings.Contains(err.Error(), "restore credential") != restoreFails || (len(changes) > 0) != restoreFails {
				t.Fatalf("partial failure records=%d err=%v", len(changes), err)
			}
			if !restoreFails && gitTest(t, root, "config", "--get-all", key) != "original\n" {
				t.Fatal("partial write did not restore original helper")
			}
		})
	}
}
