package ciparity

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const testConfig = `code-gate:
  ci-parity:
    command: go run ./cmd/ci-parity
  memory:
    command: go run ./cmd/memory-gate
  process-boundary:
    group: tests
    command: go test ./internal/executor/runtime
    env: PYXIS_REQUIRE_PROCESS_CONFINEMENT
  optional:
    command: false
    class: advisory
`

const testWorkflow = `name: checks
on: [pull_request, push]
jobs:
  checks:
    runs-on: ubuntu-latest
    env:
      PYXIS_REQUIRE_PROCESS_CONFINEMENT: "1"
      PYXIS_CI_EVENT: "${{ github.event_name }}"
      PYXIS_CI_BASE: "${{ github.event.pull_request.base.sha || github.event.before }}"
    steps:
      - uses: actions/checkout@v4
        with: {fetch-depth: 0}
      - name: all checks
        run: .pyxis/scripts/code-gate-resolve.sh run --no-cache
      - name: documents
        run: .pyxis/scripts/docgate-resolve.sh gate --merge-base "$PYXIS_CI_BASE" --defects-json -
`

const testHook = `#!/bin/sh
# source repository delegates
set -e
ROOT="$(git rev-parse --show-toplevel)"
"$ROOT/.pyxis/scripts/docgate-resolve.sh" pre-push
exec "$ROOT/.pyxis/scripts/code-gate-resolve.sh" run
`

func writeFile(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	writeFile(t, root, ".pyxis/config.yaml", testConfig)
	writeFile(t, root, ".pyxis/hooks/pre-push", testHook)
	if err := os.Chmod(filepath.Join(root, ".pyxis/hooks/pre-push"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(".pyxis/hooks", filepath.Join(root, ".githooks")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, root, ".github/workflows/checks.yml", testWorkflow)
	gitRun(t, root, "init", "-q")
	gitRun(t, root, "add", "--", ".pyxis/hooks/pre-push")
	return root
}

func TestRepositoryCIParity(t *testing.T) {
	if err := Check(t.Context(), filepath.Join("..", "..")); err != nil {
		t.Fatal(err)
	}
}

func TestWorkflowMutationsFailParity(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, from, to, want string
	}{
		{"extra blocking command", "      - name: documents", "      - run: go test ./new-remote-only-package\n      - name: documents", "unregistered blocking step"},
		{"new job", "  checks:", "  surprise:\n    runs-on: ubuntu-latest\n    steps: [{run: 'false'}]\n  checks:", "unregistered blocking step"},
		{"shell suffix", gateRun, gateRun + " && false", "unregistered blocking step"},
		{"selected shell suffix", gateRun, gateRun + " --check memory; false", "not registered"},
		{"unknown selection", gateRun, gateRun + " --check unknown", "not registered"},
		{"advisory selection", gateRun, gateRun + " --check optional", "not registered"},
		{"empty selection", gateRun, gateRun + " --check ", "unregistered blocking step"},
		{"only subset", gateRun, gateRun + " --check memory", "no unconditional"},
		{"missing document gate", "      - name: documents\n        run: " + docRun + "\n", "", "no unconditional"},
		{"conditional check", "      - name: all checks", "      - name: all checks\n        if: false", "unconditionally"},
		{"conditional job", "    runs-on:", "    if: false\n    runs-on:", "conditional blocking jobs"},
		{"advisory complete gate", "      - name: all checks", "      - name: all checks\n        continue-on-error: true", "no unconditional"},
		{"advisory whole job", "    runs-on:", "    continue-on-error: true\n    runs-on:", "no unconditional"},
		{"cached hosted check", gateRun, strings.TrimSuffix(gateRun, " --no-cache"), "unregistered blocking step"},
		{"changed trusted base", baseExpression, "HEAD", "trusted PYXIS_CI_BASE"},
		{"changed event", eventExpression, "schedule", "trusted PYXIS_CI_BASE"},
		{"step overrides base", "      - name: all checks", "      - name: all checks\n        env: {PYXIS_CI_BASE: HEAD}", "trusted PYXIS_CI_BASE"},
		{"overridden PATH", "    env:", "    env:\n      PATH: /tmp/tools", "override PATH"},
		{"empty PATH", "    env:", "    env:\n      PATH: ''", "override PATH"},
		{"empty HOME", "    env:", "    env:\n      HOME: ''", "override PATH"},
		{"disabled confinement", "PYXIS_REQUIRE_PROCESS_CONFINEMENT: \"1\"", "PYXIS_REQUIRE_PROCESS_CONFINEMENT: \"0\"", "requires PYXIS_REQUIRE_PROCESS_CONFINEMENT"},
		{"missing confinement", "      PYXIS_REQUIRE_PROCESS_CONFINEMENT: \"1\"\n", "", "requires PYXIS_REQUIRE_PROCESS_CONFINEMENT"},
		{"computed confinement", "PYXIS_REQUIRE_PROCESS_CONFINEMENT: \"1\"", "PYXIS_REQUIRE_PROCESS_CONFINEMENT: '${{ vars.CONFINEMENT }}'", "requires PYXIS_REQUIRE_PROCESS_CONFINEMENT"},
		{"step disables confinement", "      - name: all checks", "      - name: all checks\n        env: {PYXIS_REQUIRE_PROCESS_CONFINEMENT: '0'}", "requires PYXIS_REQUIRE_PROCESS_CONFINEMENT"},
		{"step clears confinement", "      - name: all checks", "      - name: all checks\n        env: {PYXIS_REQUIRE_PROCESS_CONFINEMENT: ''}", "requires PYXIS_REQUIRE_PROCESS_CONFINEMENT"},
		{"overridden Go flags", "    env:", "    env:\n      GOFLAGS: '-overlay=/tmp/skip.json'", "environment override GOFLAGS"},
		{"overridden hosted identity", "    env:", "    env:\n      GITHUB_ACTIONS: 'false'", "requires GITHUB_ACTIONS"},
		{"custom shell", "      - name: all checks", "      - name: all checks\n        shell: 'bash {0} || true'", "custom shell"},
		{"custom directory", "      - name: all checks", "      - name: all checks\n        working-directory: elsewhere", "working directory"},
		{"mixed action and run", "      - name: all checks", "      - name: all checks\n        uses: actions/checkout@v4", "mixed run/uses"},
		{"gate action inputs", "      - name: all checks", "      - name: all checks\n        with: {skip: yes}", "without action inputs"},
		{"composite action", "actions/checkout@v4", "./.github/actions/hidden-check", "unregistered blocking step"},
		{"checkout wrong revision", "with: {fetch-depth: 0}", "with: {fetch-depth: 0, ref: main}", "unregistered blocking step"},
		{"reusable job", "    runs-on: ubuntu-latest", "    uses: ./other.yml", "unsupported workflow structure"},
		{"job without gate", "      - name: all checks\n        run: " + gateRun + "\n      - name: documents\n        run: " + docRun + "\n", "", "no unconditional shared gate"},
		{"filtered aggregate", "on: [pull_request, push]", "on: {pull_request: {paths: ['cmd/**']}}", "no unconditional"},
		{"unknown field", "    runs-on:", "    unsupported: yes\n    runs-on:", "unsupported workflow structure"},
		{"duplicate field", "    runs-on: ubuntu-latest", "    runs-on: ubuntu-latest\n    runs-on: macos-latest", "duplicate"},
		{"alias", "jobs:", "unused: &job {}\njobs:", "anchors and aliases"},
		{"merged job", "    runs-on:", "    <<: {continue-on-error: true}\n    runs-on:", "merged mapping key"},
		{"computed advisory", "    runs-on:", "    continue-on-error: '${{ true }}'\n    runs-on:", "literal true or false"},
		{"YAML 1.1 advisory", "    runs-on:", "    continue-on-error: yes\n    runs-on:", "literal true or false"},
		{"second document", "name: checks", "---\nname: extra\n---\nname: checks", "exactly one"},
		{"unknown trigger", "[pull_request, push]", "[repository_dispatch]", "unclassified workflow event"},
		{"missing trigger", "on: [pull_request, push]", "", "explicit event"},
		{"event nested sequence", "[pull_request, push]", "[[pull_request]]", "must contain names"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if !strings.Contains(testWorkflow, tt.from) {
				t.Fatal("mutation did not match the fixture")
			}
			root := fixture(t)
			writeFile(t, root, ".github/workflows/checks.yml", strings.Replace(testWorkflow, tt.from, tt.to, 1))
			if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want %q", err, tt.want)
			}
		})
	}
}

func TestCompleteWorkflowInventory(t *testing.T) {
	for _, ext := range []string{"yml", "yaml"} {
		t.Run(ext, func(t *testing.T) {
			root := fixture(t)
			writeFile(t, root, ".github/workflows/new-check."+ext, "on: pull_request\njobs: {surprise: {runs-on: ubuntu-latest, steps: [{run: false}]}}\n")
			if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "new-check."+ext) {
				t.Fatalf("new remote-only workflow escaped: %v", err)
			}
		})
	}
	root := fixture(t)
	writeFile(t, root, ".github/workflows/README.md", "not a workflow")
	for _, event := range []string{"push", "schedule", "pull_request_target", "merge_group", "workflow_call"} {
		writeFile(t, root, ".github/workflows/extra.yaml", "on: "+event+"\njobs: {new: {steps: [{run: false}]}}\n")
		if err := Check(t.Context(), root); err == nil {
			t.Fatalf("remote-only %s check escaped", event)
		}
	}
}

func TestPermittedAdditionalWorkflows(t *testing.T) {
	for _, body := range []string{
		"on: {push: {tags: ['v*']}}\njobs: {release: {steps: [{run: publish}]}}",
		"on: [workflow_dispatch, release]\njobs: {release: {steps: [{run: publish}]}}",
		"on: pull_request\njobs: {advisory: {continue-on-error: true, steps: [{run: false}]}}",
		strings.Replace(testWorkflow, gateRun, gateRun+" --group default", 1),
		strings.Replace(testWorkflow, "on: [pull_request, push]", "on: {pull_request: {}, push: {branches: [main]}}", 1),
		strings.Replace(testWorkflow, "on: [pull_request, push]", "on: {pull_request: null, push: {branches-ignore: [unused]}}", 1),
	} {
		root := fixture(t)
		writeFile(t, root, ".github/workflows/additional.yml", body)
		if err := Check(t.Context(), root); err != nil {
			t.Fatalf("allowed workflow failed: %v\n%s", err, body)
		}
	}
}

func TestLocalWiringAndConfiguration(t *testing.T) {
	for _, tt := range []struct{ path, body, want string }{
		{".pyxis/config.yaml", "code-gate:\n  no-guard:\n    command: true\n", "ci-parity must remain"},
		{".pyxis/config.yaml", strings.Replace(testConfig, "go run ./cmd/ci-parity", "true", 1), "ci-parity must remain"},
		{".pyxis/config.yaml", strings.Replace(testConfig, "  ci-parity:\n", "  ci-parity:\n    class: advisory\n", 1), "ci-parity must remain"},
		{".pyxis/config.yaml", "code-gate:\n  broken: true\n", "code-gate"},
		{".pyxis/config.local.yaml", "code-gate:\n  memory:\n    command: true\n", "local code-gate overrides"},
		{".pyxis/config.local.yaml", "code-gate:\n  memory:\n    group: tests\n", "local code-gate overrides"},
		{".pyxis/config.local.yaml", "code-gate:\n  memory:\n    unknown: yes\n", "unknown key"},
		{".githooks/pre-push", testHook + "go test ./remote-only\n", "delegate exactly"},
		{".githooks/pre-push", strings.Replace(testHook, "code-gate-resolve.sh\" run", "code-gate-resolve.sh\" run --check memory", 1), "delegate exactly"},
	} {
		root := fixture(t)
		writeFile(t, root, tt.path, tt.body)
		if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), tt.want) {
			t.Fatalf("%s: %v, want %q", tt.path, err, tt.want)
		}
	}
	root := fixture(t)
	writeFile(t, root, ".pyxis/config.local.yaml", "executor:\n  logging:\n    directory: local-logs\n")
	if err := Check(t.Context(), root); err != nil {
		t.Fatal(err)
	}
}

func TestFilesystemAndCancellationFailures(t *testing.T) {
	for _, rel := range []string{".pyxis/config.yaml", ".githooks/pre-push", ".github/workflows"} {
		root := fixture(t)
		if err := os.RemoveAll(filepath.Join(root, rel)); err != nil {
			t.Fatal(err)
		}
		if err := Check(t.Context(), root); err == nil {
			t.Fatalf("missing %s passed", rel)
		}
	}
	root := fixture(t)
	if err := os.Symlink("checks.yml", filepath.Join(root, ".github/workflows/link.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := Check(t.Context(), root); err == nil || !strings.Contains(err.Error(), "regular file") {
		t.Fatalf("symlink escaped: %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := Check(ctx, fixture(t)); err == nil {
		t.Fatal("cancelled parity audit passed")
	}
	if _, err := decodeWorkflow([]byte("[broken")); err == nil {
		t.Fatal("invalid YAML passed")
	}
	if _, err := decodeWorkflow([]byte("on: pull_request\n")); err == nil {
		t.Fatal("empty jobs passed")
	}
}
