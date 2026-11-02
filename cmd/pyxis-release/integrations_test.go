package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/release"
	"go.yaml.in/yaml/v3"
)

const releaseProfile = `schema_version: pyxis-config/v2
integrations:
  schema_version: pyxis-integrations/v1
  connections:
    local:
      provider: forgejo
      base_url: http://127.0.0.1:3000
      token_env: SOURCE_TOKEN
    public:
      provider: github
      token_env: DISTRIBUTION_TOKEN
  bindings:
    code_host:
      connection: local
      repository: o/source
    releases:
      source:
        connection: local
        repository: o/source
      distribution:
        connection: public
        repository: o/distribution
  ci:
    mode: actions
`

func releaseProfileRoot(t *testing.T, profile string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.Mkdir(filepath.Join(root, ".pyxis"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(profile), 0600); err != nil {
		t.Fatal(err)
	}
	return root
}

func TestReleaseCommandsKeepIndependentTargetsAndCLIOverrides(t *testing.T) {
	root := releaseProfileRoot(t, releaseProfile)
	oldPublish, oldPins, oldMirror, oldCut := releasePublish, releasePins, releaseMirror, releaseCut
	t.Cleanup(func() {
		releasePublish, releasePins, releaseMirror, releaseCut = oldPublish, oldPins, oldMirror, oldCut
	})
	var target *integrations.ResolvedRepository
	var mirror bool
	var source, dist *integrations.ResolvedRepository
	releasePublish = func(_ context.Context, o release.PublishOptions) (release.PublishResult, error) {
		target = o.Target
		mirror = o.Mirror
		return release.PublishResult{}, nil
	}
	releasePins = func(_ context.Context, o release.PinsOptions) (release.PinsResult, error) {
		target = o.Target
		return release.PinsResult{}, nil
	}
	releaseMirror = func(_ context.Context, o release.MirrorOptions) (release.MirrorResult, error) {
		source, dist = o.SourceTarget, o.DistTarget
		return release.MirrorResult{}, nil
	}
	releaseCut = func(_ context.Context, o release.CutOptions) (release.CutResult, error) {
		source, dist = o.SourceTarget, o.DistTarget
		return release.CutResult{}, nil
	}
	for _, tc := range []struct {
		args           []string
		provider, repo string
	}{
		{[]string{"publish"}, integrations.Forgejo, "o/source"},
		{[]string{"publish", "--repo", "o/source-override"}, integrations.Forgejo, "o/source-override"},
		{[]string{"publish", "--target", "distribution"}, integrations.GitHub, "o/distribution"},
		{[]string{"publish", "--target", "distribution", "--repo", "o/override"}, integrations.GitHub, "o/override"},
		{[]string{"pins", "--target", "distribution"}, integrations.GitHub, "o/distribution"},
		{[]string{"pins", "--source-repo", "o/override"}, integrations.Forgejo, "o/override"},
	} {
		var stdout, stderr bytes.Buffer
		if code := run(append(tc.args, "--repo-root", root, "--require-bindings"), &stdout, &stderr); code != 0 {
			t.Fatalf("%v: %d %s", tc.args, code, &stderr)
		}
		if target == nil || target.Provider != tc.provider || target.Repository != tc.repo {
			t.Fatalf("%v selected %+v", tc.args, target)
		}
		if tc.args[0] == "publish" && mirror != (tc.provider == integrations.GitHub) {
			t.Fatalf("%v changed canonical tag verification: mirror=%v", tc.args, mirror)
		}
	}
	for _, verb := range []string{"mirror", "cut"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{verb, "--repo-root", root, "--require-bindings", "--source-repo", "o/source-override", "--dist-repo", "o/dist-override"}, &stdout, &stderr); code != 0 {
			t.Fatal(code, &stderr)
		}
		if source == nil || dist == nil || source.Provider != integrations.Forgejo || dist.Provider != integrations.GitHub || source.Repository != "o/source-override" || dist.Repository != "o/dist-override" || source.TokenEnv == dist.TokenEnv {
			t.Fatalf("%s lost target identity: %+v %+v", verb, source, dist)
		}
	}
}

func TestGeneratedReleaseCommandsRefuseRemovedBindings(t *testing.T) {
	root := t.TempDir()
	for _, verb := range []string{"publish", "pins", "legacy", "cut", "mirror"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{verb, "--repo-root", root, "--require-bindings"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "required release binding is absent") {
			t.Fatalf("%s fell back without its binding: %d %s", verb, code, &stderr)
		}
	}
	profile := strings.Replace(releaseProfile, "      distribution:\n        connection: public\n        repository: o/distribution\n", "", 1)
	root = releaseProfileRoot(t, profile)
	for _, args := range [][]string{{"mirror"}, {"cut", "--mirror"}, {"publish", "--target", "distribution"}} {
		var stdout, stderr bytes.Buffer
		if code := run(append(args, "--repo-root", root, "--require-bindings"), &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "required release binding is absent") {
			t.Fatalf("%v fell back without distribution binding: %d %s", args, code, &stderr)
		}
	}
	root = releaseProfileRoot(t, strings.Replace(releaseProfile, "token_env: SOURCE_TOKEN", "token_env: PYXIS_RELEASE_TAG", 1))
	var stdout, stderr bytes.Buffer
	if code := run([]string{"mirror", "--repo-root", root, "--require-bindings"}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "reserved workflow") {
		t.Fatalf("changed credential reference reached publication: %d %s", code, &stderr)
	}
}

func TestReleaseCommandsRefuseInvalidBindingOverridesBeforeAction(t *testing.T) {
	root := releaseProfileRoot(t, releaseProfile)
	for _, flags := range [][]string{{"--target", "unknown"}, {"--repo", "invalid"}, {"--source-repo", "invalid"}, {"--dist-repo", "invalid"}} {
		var stdout, stderr bytes.Buffer
		args := append([]string{"publish", "--repo-root", root}, flags...)
		if code := run(args, &stdout, &stderr); code != 2 {
			t.Fatalf("%v = %d: %s", flags, code, &stderr)
		}
	}
	root = releaseProfileRoot(t, "integrations:\n  unknown: true\n")
	if code := run([]string{"publish", "--repo-root", root}, &bytes.Buffer{}, &bytes.Buffer{}); code != 2 {
		t.Fatal("invalid profile accepted")
	}
}

func TestReleaseWorkflowCommandIsExplicitAndPreservesExistingFiles(t *testing.T) {
	root := releaseProfileRoot(t, releaseProfile)
	if code := run([]string{"workflows", "--repo-root", root}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Fatal("missing template accepted")
	}
	dir := filepath.Join(root, ".pyxis/dist/forgejo-source-workflows")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "gate.yml"), []byte("name: source\nruns-on: {{runner_label}}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	if code := run([]string{"workflows", "--repo-root", root}, &stdout, &stderr); code != 0 || !strings.Contains(stdout.String(), ".forgejo/workflows/gate.yml") {
		t.Fatal(code, &stdout, &stderr)
	}
}

func TestLegacyCommandUsesSelectedReleaseConnection(t *testing.T) {
	root := releaseProfileRoot(t, releaseProfile)
	old := releasePublish
	t.Cleanup(func() { releasePublish = old })
	if code := run([]string{"legacy", "--repo-root", root, "--tool", "invalid"}, &bytes.Buffer{}, &bytes.Buffer{}); code != 1 {
		t.Fatal("invalid tool accepted")
	}
	if err := os.MkdirAll(filepath.Join(root, "cmd/docgate"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module fixture\n\ngo 1.25\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "cmd/docgate/main.go"), []byte("package main\nfunc main() {}\n"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, fail := range []bool{false, true} {
		releasePublish = func(_ context.Context, o release.PublishOptions) (release.PublishResult, error) {
			if o.LegacyTool != "docgate" || o.Target == nil || o.Target.Provider != integrations.Forgejo {
				t.Fatalf("legacy route %+v", o)
			}
			if fail {
				return release.PublishResult{}, errors.New("fixture publication refusal")
			}
			return release.PublishResult{}, nil
		}
		var stdout, stderr bytes.Buffer
		code := run([]string{"legacy", "--repo-root", root, "--tool", "docgate", "--tag", "docgate/v1.2.3", "--out", t.TempDir()}, &stdout, &stderr)
		want := 0
		if fail {
			want = 1
		}
		if code != want {
			t.Fatalf("legacy: %d %s", code, &stderr)
		}
	}
}

func TestGeneratedSystemMirrorRoutesOnlyToABoundRepository(t *testing.T) {
	const distribution = "      distribution:\n        connection: public\n        repository: o/distribution\n"
	template, err := os.ReadFile("../../.pyxis/dist/forgejo-source-workflows/system-release.yml")
	if err != nil {
		t.Fatal(err)
	}
	oldMirror := releaseMirror
	t.Cleanup(func() { releaseMirror = oldMirror })
	render := func(t *testing.T, profile string) (string, []byte) {
		t.Helper()
		root := releaseProfileRoot(t, profile)
		dir := filepath.Join(root, ".pyxis/dist/forgejo-source-workflows")
		if err := os.MkdirAll(dir, 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "system-release.yml"), template, 0644); err != nil {
			t.Fatal(err)
		}
		paths, err := release.MaterializeSourceWorkflows(root)
		if err != nil || len(paths) != 1 {
			t.Fatalf("rendered paths %v: %v", paths, err)
		}
		data, err := os.ReadFile(paths[0])
		if err != nil {
			t.Fatal(err)
		}
		return root, data
	}
	for _, tc := range []struct {
		name, profile string
		separate      bool
	}{
		{"repository", releaseProfile, true},
		{"shared credential", strings.Replace(releaseProfile, "token_env: DISTRIBUTION_TOKEN", "token_env: SOURCE_TOKEN", 1), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root, data := render(t, tc.profile)
			var workflow struct {
				Jobs map[string]struct {
					If    string            `yaml:"if"`
					Env   map[string]string `yaml:"env"`
					Steps []struct {
						Run string `yaml:"run"`
					} `yaml:"steps"`
				} `yaml:"jobs"`
			}
			if err := yaml.Unmarshal(data, &workflow); err != nil {
				t.Fatal(err)
			}
			mirror := workflow.Jobs["mirror"]
			if mirror.If != "forgejo.event_name == 'workflow_dispatch'" || mirror.Env["SOURCE_TOKEN"] != "${{ secrets.PYXIS_RELEASE_TOKEN }}" {
				t.Fatalf("mirror eligibility or source credential drift: %+v", mirror)
			}
			if _, present := mirror.Env["DISTRIBUTION_TOKEN"]; present != tc.separate {
				t.Fatalf("unexpected distribution credential mapping: %+v", mirror.Env)
			}
			if tc.separate && mirror.Env["DISTRIBUTION_TOKEN"] != "${{ secrets.PYXIS_DISTRIBUTION_RELEASE_TOKEN }}" {
				t.Fatal("distribution secret mapped to the wrong credential")
			}
			if strings.Contains(string(data), "PYXIS_UNUSED_DISTRIBUTION_TOKEN") {
				t.Fatal("rendered an unused secret environment")
			}
			command := mirror.Steps[len(mirror.Steps)-1].Run
			if want := "go run ./cmd/pyxis-release mirror --require-bindings --tag \"$PYXIS_RELEASE_TAG\""; command != want {
				t.Fatalf("command %q, want %q", command, want)
			}
			args := strings.Fields(strings.Replace(command, "\"$PYXIS_RELEASE_TAG\"", "v1.2.3", 1))[3:]
			args = append(args, "--repo-root", root)
			called := 0
			releaseMirror = func(_ context.Context, opts release.MirrorOptions) (release.MirrorResult, error) {
				called++
				if opts.SourceTarget == nil || opts.SourceTarget.Provider != integrations.Forgejo || opts.SourceTarget.TokenEnv != "SOURCE_TOKEN" || opts.DistTarget == nil {
					t.Fatalf("rendered command lost its bindings: %+v", opts)
				}
				wantToken := "SOURCE_TOKEN"
				if tc.separate {
					wantToken = "DISTRIBUTION_TOKEN"
				}
				if opts.DistTarget.Provider != integrations.GitHub || opts.DistTarget.TokenEnv != wantToken {
					t.Fatal("distribution provider or credential changed")
				}
				return release.MirrorResult{}, nil
			}
			var stdout, stderr bytes.Buffer
			if code := run(args, &stdout, &stderr); code != 0 || called != 1 {
				t.Fatalf("rendered command refused: code=%d called=%d %s", code, called, &stderr)
			}
			changed := strings.Replace(tc.profile, distribution, "", 1)
			if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(changed), 0600); err != nil {
				t.Fatal(err)
			}
			if code := run(args, &stdout, &stderr); code != 2 || called != 1 {
				t.Fatalf("removed required binding reached publication: code=%d called=%d %s", code, called, &stderr)
			}
		})
	}
	// Generated without a distribution binding, the mirror job has nowhere to
	// publish, and the rendered command refuses rather than publishing nothing.
	t.Run("no binding", func(t *testing.T) {
		root, data := render(t, strings.Replace(releaseProfile, distribution, "", 1))
		if strings.Contains(string(data), "R2_") {
			t.Fatalf("rendered a retired distribution input:\n%s", data)
		}
		releaseMirror = func(context.Context, release.MirrorOptions) (release.MirrorResult, error) {
			t.Fatal("publication reached without a distribution binding")
			return release.MirrorResult{}, nil
		}
		var stdout, stderr bytes.Buffer
		if code := run([]string{"mirror", "--require-bindings", "--tag", "v1.2.3", "--repo-root", root}, &stdout, &stderr); code != 2 || !strings.Contains(stderr.String(), "required release binding is absent") {
			t.Fatalf("code=%d %s", code, &stderr)
		}
	})
}
