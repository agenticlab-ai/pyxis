package converge

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"maps"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
	"github.com/agenticlab-ai/pyxis/internal/relmanifest"
)

const bootstrapTestVersion = "v1.2.3"

// bootstrapRoot is an operand tree carrying the real shipped template.
func bootstrapRoot(t *testing.T) string {
	t.Helper()
	tmpl, err := os.ReadFile(filepath.Join("..", "..", filepath.FromSlash(payload.InstalledWorkflow)))
	if err != nil {
		t.Fatalf("read shipped template: %v", err)
	}
	root := t.TempDir()
	dst := filepath.Join(root, filepath.FromSlash(payload.InstalledWorkflow))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(dst, tmpl, 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// bootstrapStepEnv is what GitHub Actions hands the workflow's bootstrap step.
func bootstrapStepEnv(t *testing.T, rendered []byte) map[string]string {
	t.Helper()
	var wf struct {
		Jobs map[string]struct {
			Env   map[string]string `yaml:"env"`
			Steps []struct {
				Name string            `yaml:"name"`
				Env  map[string]string `yaml:"env"`
				Run  string            `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(rendered, &wf); err != nil {
		t.Fatalf("rendered workflow is not YAML: %v\n%s", err, rendered)
	}
	job := wf.Jobs["docs-gate"]
	for _, step := range job.Steps {
		if strings.Contains(step.Run, ".pyxis/scripts/pyxis-bootstrap.sh") {
			env := map[string]string{}
			maps.Copy(env, job.Env)
			maps.Copy(env, step.Env)
			return env
		}
	}
	t.Fatalf("rendered workflow has no bootstrap step\n%s", rendered)
	return nil
}

// runBootstrapShim runs the real shim in a fresh directory with only env set,
// as a Linux runner of the given machine type: uname is the one stub, so the test
// exercises the workflow's Linux pins on any host.
func runBootstrapShim(t *testing.T, env map[string]string, machine string) (dir, output string, err error) {
	t.Helper()
	shim, err := filepath.Abs(filepath.Join("..", "..", ".pyxis", "scripts", "pyxis-bootstrap.sh"))
	if err != nil {
		t.Fatal(err)
	}
	stubs := t.TempDir()
	uname := "#!/bin/sh\nif [ \"$1\" = -m ]; then echo " + machine + "; else echo Linux; fi\n"
	if err := os.WriteFile(filepath.Join(stubs, "uname"), []byte(uname), 0o755); err != nil {
		t.Fatal(err)
	}
	dir = t.TempDir()
	cmd := exec.CommandContext(t.Context(), "bash", shim)
	cmd.Dir = dir
	cmd.Env = []string{"PATH=" + stubs + string(os.PathListSeparator) + os.Getenv("PATH")}
	for k, v := range env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	out, err := cmd.CombinedOutput()
	return dir, string(out), err
}

func hexSHA256(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// The rendered workflow's own bootstrap env drives the real shim to a verified
// binary on both Linux architectures, and the shim refuses bytes that do not
// match the manifest's pin. Without the rendered pins the shim exits 2 on "no
// version pin" and every installed repository's docs-gate is red.
func TestInstalledWorkflowEnvSatisfiesBootstrapShim(t *testing.T) {
	for _, tc := range []struct{ machine, platform string }{
		{"x86_64", "linux-amd64"},
		{"aarch64", "linux-arm64"},
	} {
		t.Run(tc.platform, func(t *testing.T) {
			binary := []byte("fixture pyxis " + tc.platform + "\n")
			src := t.TempDir()
			artifact := filepath.Join(src, "pyxis-"+bootstrapTestVersion+"-"+tc.platform)
			if err := os.WriteFile(artifact, binary, 0o644); err != nil {
				t.Fatal(err)
			}
			pins := map[string]string{"linux-amd64": strings.Repeat("a", 64), "linux-arm64": strings.Repeat("b", 64)}
			pins[tc.platform] = hexSHA256(binary)
			m := relmanifest.Manifest{SystemVersion: bootstrapTestVersion, Binaries: map[string]map[string]string{"pyxis": pins}}
			repo := config.Defaults()
			repo.System.ArtifactSource = "file://" + src

			rendered, reason := desiredWorkflow(bootstrapRoot(t), repo, m, "")
			if reason != "" {
				t.Fatal(reason)
			}
			env := bootstrapStepEnv(t, rendered)

			dir, out, err := runBootstrapShim(t, env, tc.machine)
			if err != nil {
				t.Fatalf("the shim refused the rendered workflow's env %v: %v\n%s", env, err, out)
			}
			got, err := os.ReadFile(filepath.Join(dir, ".pyxis", "bin", "pyxis"))
			if err != nil || !bytes.Equal(got, binary) {
				t.Fatalf("installed pyxis = %q, %v; want the pinned bytes", got, err)
			}

			// A release serving other bytes than the manifest pins is refused.
			if err := os.WriteFile(artifact, []byte("tampered\n"), 0o644); err != nil {
				t.Fatal(err)
			}
			dir, out, err = runBootstrapShim(t, env, tc.machine)
			var exit *exec.ExitError
			if !errors.As(err, &exit) || !strings.Contains(out, "sha256 mismatch") {
				t.Fatalf("the shim accepted tampered bytes: %v\n%s", err, out)
			}
			if _, err := os.Stat(filepath.Join(dir, ".pyxis", "bin", "pyxis")); !errors.Is(err, os.ErrNotExist) {
				t.Fatalf("a tampered binary was installed: %v", err)
			}
		})
	}
}

// On the default configuration the workflow names the public release with no
// credential, and its bytes do not depend on the machine that rendered them:
// doctor on another clone must read the committed copy as converged.
func TestInstalledWorkflowDefaultSourceIsPublicAndMachineIndependent(t *testing.T) {
	root := bootstrapRoot(t)
	sum := strings.Repeat("a", 64)
	m := relmanifest.Manifest{SystemVersion: bootstrapTestVersion, Binaries: map[string]map[string]string{"pyxis": {"linux-amd64": sum, "linux-arm64": sum}}}
	repo := config.Defaults()

	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	first, reason := desiredWorkflow(root, repo, m, "")
	if reason != "" {
		t.Fatal(reason)
	}
	env := bootstrapStepEnv(t, first)
	if got, want := env["PYXIS_ARTIFACT_SOURCE"], "https://github.com/agenticlab-ai/pyxis/releases/download/"+bootstrapTestVersion; got != want {
		t.Errorf("artifact source = %q, want %q", got, want)
	}
	if bytes.Contains(first, []byte("secrets.")) || bytes.Contains(first, []byte("TOKEN")) {
		t.Errorf("the public-release workflow reaches for a credential:\n%s", first)
	}

	home := t.TempDir()
	if err := os.MkdirAll(filepath.Join(home, "pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	record := "PYXIS_SYSTEM_VERSION=v1.2.3\nPYXIS_MANIFEST_SHA256=abc\nPYXIS_CHANNEL=https://channel.example.com/pyxis\n"
	if err := os.WriteFile(filepath.Join(home, "pyxis", "release.env"), []byte(record), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", home)
	second, reason := desiredWorkflow(root, repo, m, "")
	if reason != "" {
		t.Fatal(reason)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("a machine's release record changed the committed workflow\n--- without ---\n%s\n--- with ---\n%s", first, second)
	}
}

// A release that does not pin what the workflow bootstraps is a named refusal,
// never a rendered workflow the shim would refuse on every run; so is a source
// that would leave its quoted YAML scalar.
func TestInstalledWorkflowRefusesUnrenderablePins(t *testing.T) {
	root := bootstrapRoot(t)
	sum := strings.Repeat("a", 64)
	tests := []struct {
		name   string
		pins   map[string]string
		source string
		want   string
	}{
		{"no pyxis binary", nil, "", "linux_amd_checksum"},
		{"no amd64 binary", map[string]string{"linux-arm64": sum}, "", "linux_amd_checksum"},
		{"no arm64 binary", map[string]string{"linux-amd64": sum}, "", "linux_arm_checksum"},
		{"quote in source", map[string]string{"linux-amd64": sum, "linux-arm64": sum}, "https://x/'; touch /tmp/escaped", "unsafe value for artifact_source"},
		{"newline in source", map[string]string{"linux-amd64": sum, "linux-arm64": sum}, "https://x/\nrun: id", "unsafe value for artifact_source"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			repo := config.Defaults()
			repo.System.ArtifactSource = tc.source
			m := relmanifest.Manifest{SystemVersion: bootstrapTestVersion, Binaries: map[string]map[string]string{"pyxis": tc.pins}}
			out, reason := desiredWorkflow(root, repo, m, "")
			if out != nil || !strings.Contains(reason, tc.want) {
				t.Fatalf("desiredWorkflow = %q, reason %q; want a refusal naming %q", out, reason, tc.want)
			}
		})
	}
}
