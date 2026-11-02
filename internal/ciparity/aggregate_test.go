package ciparity

import (
	"os/exec"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

// Both jobs select shared groups; adding or regrouping a check requires no
// workflow edits. The default group includes newly configured checks.
const partitionedWorkflow = `name: docs-gate
on: [pull_request, push]
jobs:
  quality:
    runs-on: ubuntu-latest
    env:
      PYXIS_REQUIRE_PROCESS_CONFINEMENT: "1"
      PYXIS_CI_EVENT: "${{ github.event_name }}"
      PYXIS_CI_BASE: "${{ github.event.pull_request.base.sha || github.event.before }}"
    steps:
      - run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --group default
      - run: .pyxis/scripts/docgate-resolve.sh gate --merge-base "$PYXIS_CI_BASE" --defects-json -
  tests:
    runs-on: ubuntu-latest
    env:
      PYXIS_REQUIRE_PROCESS_CONFINEMENT: "1"
      PYXIS_CI_EVENT: "${{ github.event_name }}"
      PYXIS_CI_BASE: "${{ github.event.pull_request.base.sha || github.event.before }}"
    steps:
      - run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --group tests
  docs-gate:
    needs: [quality, tests]
    if: always()
    runs-on: ubuntu-latest
    env:
      PYXIS_CI_RESULTS: "${{ join(needs.*.result, ' ') }}"
      PYXIS_CI_EXPECTED: success success
    steps:
      - name: require both check jobs to pass
        run: test "$PYXIS_CI_RESULTS" = "$PYXIS_CI_EXPECTED"
`

func TestPartitionedWorkflowCoversNewSharedChecks(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".github/workflows/checks.yml", partitionedWorkflow)
	for _, config := range []string{
		testConfig,
		testConfig + "  new-check:\n    command: new-required-command\n",
		testConfig + "  new-test:\n    command: new-test-command\n    group: tests\n",
		strings.Replace(testConfig, "  memory:\n", "  memory:\n    group: tests\n", 1),
	} {
		writeFile(t, root, ".pyxis/config.yaml", config)
		if err := Check(t.Context(), root); err != nil {
			t.Fatalf("complete partition refused: %v", err)
		}
	}
}

func TestPartitionMutationsCannotCertifyMissingOrFailedWork(t *testing.T) {
	cfg, err := codegatemirror.Parse([]byte(testConfig))
	if err != nil {
		t.Fatal(err)
	}
	for _, tt := range []struct{ name, from, to, want string }{
		{"missing group", "--group default", "--group tests", "does not cover"},
		{"unknown group", "--group tests", "--group missing", "unknown group"},
		{"empty group", "--group tests", "--group ", "unregistered blocking step"},
		{"shell suffix", "--group tests", "--group tests; true", "not registered"},
		{"mixed selection", "--group tests", "--group tests --check process-boundary", "not registered"},
		{"diagnostic cannot partition", "--group tests", "--check process-boundary", "exactly one shared group"},
		{"independent check list", "--group tests", "--check process-boundary,memory", "not registered"},
		{"independent exclusion list", "--group default", "--exclude-check process-boundary", "not registered"},
		{"full gate in partition", " --group default", "", "repeats"},
		{"sequential groups", "--group default\n", "--group default\n      - run: " + gateRun + " --group tests\n", "sequential invocations"},
		{"conditional worker", "  tests:\n", "  tests:\n    if: false\n", "conditional blocking jobs"},
		{"advisory worker", "  tests:\n", "  tests:\n    continue-on-error: true\n", "every independent"},
		{"advisory check", "      - run: " + gateRun + " --group tests", "      - continue-on-error: true\n        run: " + gateRun + " --group tests", "no unconditional shared gate"},
		{"missing documents", "      - run: " + docRun + "\n", "", "does not cover"},
		{"repeated documents", "      - run: " + docRun + "\n", "      - run: " + docRun + "\n      - run: " + docRun + "\n", "repeats the document gate"},
		{"missing aggregate", "  docs-gate:\n", "  unrelated:\n", "required context"},
		{"renamed context", "  docs-gate:\n", "  docs-gate:\n    name: other\n", "required context"},
		{"colliding context", "  tests:\n", "  tests:\n    name: docs-gate\n", "duplicates the docs-gate"},
		{"padded context", "  tests:\n", "  tests:\n    name: ' docs-gate '\n", "duplicates the docs-gate"},
		{"computed context", "  tests:\n", "  tests:\n    name: \"${{ 'docs-gate' }}\"\n", "computed names"},
		{"missing needs", "    needs: [quality, tests]\n", "", "conditional blocking jobs"},
		{"omitted dependency", "[quality, tests]", "[quality]", "every independent"},
		{"unknown dependency", "[quality, tests]", "[quality, missing]", "dependency"},
		{"self dependency", "[quality, tests]", "[quality, docs-gate]", "dependency"},
		{"duplicate dependency", "[quality, tests]", "[quality, quality]", "duplicated"},
		{"dependent worker", "  tests:\n", "  tests:\n    needs: [quality]\n", "independent"},
		{"implicit success condition", "    if: always()\n", "", "always()"},
		{"conditional aggregate", "if: always()", "if: success()", "always()"},
		{"advisory aggregate", "  docs-gate:\n", "  docs-gate:\n    continue-on-error: true\n", "one required result assertion"},
		{"no aggregate runner", "    if: always()\n    runs-on: ubuntu-latest\n", "    if: always()\n", "on a runner"},
		{"skip assertion", "      - name: require both", "      - if: false\n        name: require both", "unconditional result assertion"},
		{"advisory assertion", "      - name: require both", "      - continue-on-error: true\n        name: require both", "unconditional result assertion"},
		{"assertion bypass", aggregateRun, "true", "unconditional result assertion"},
		{"assertion shell", "      - name: require both", "      - shell: 'sh {0} || true'\n        name: require both", "without overrides"},
		{"extra aggregate command", "      - name: require both", "      - run: true\n      - name: require both", "one required result assertion"},
		{"literal successful results", aggregateResults, "success success", "compare all needs results"},
		{"one result", aggregateResults, "${{ needs.quality.result }}", "compare all needs results"},
		{"incorrect success count", "PYXIS_CI_EXPECTED: success success", "PYXIS_CI_EXPECTED: success", "exactly 2 successes"},
		{"accept skips", "PYXIS_CI_EXPECTED: success success", "PYXIS_CI_EXPECTED: success skipped", "exactly 2 successes"},
		{"step overrides results", "      - name: require both", "      - env: {PYXIS_CI_RESULTS: 'success success'}\n        name: require both", "compare all needs results"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(partitionedWorkflow, tt.from) {
				t.Fatal("mutation did not match fixture")
			}
			for _, provider := range []string{"github", "forgejo"} {
				body := strings.Replace(partitionedWorkflow, tt.from, tt.to, 1)
				body = strings.ReplaceAll(body, "github.", provider+".")
				if _, err := checkWorkflow([]byte(body), cfg, provider); err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("%s error = %v, want %q", provider, err, tt.want)
				}
			}
		})
	}
	// The independent jobs alone must never satisfy the complete-gate rule.
	withoutAggregate, _, _ := strings.Cut(partitionedWorkflow, "  docs-gate:\n")
	if complete, err := checkWorkflow([]byte(withoutAggregate), cfg, "github"); err != nil || complete {
		t.Fatalf("partition without aggregate = %v, %v", complete, err)
	}
}

func TestMissingAndRepeatedGroupsRefuseIncludingAdvisoryGroups(t *testing.T) {
	// Adding a third group in shared config must require a matching hosted
	// worker, even if that group's checks are advisory.
	cfg, err := codegatemirror.Parse([]byte(testConfig + "  extra:\n    command: false\n    class: advisory\n    group: extra\n"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkWorkflow([]byte(partitionedWorkflow), cfg, "github"); err == nil || !strings.Contains(err.Error(), "does not cover") {
		t.Fatalf("missing advisory group escaped: %v", err)
	}
	// A third worker repeats the tests group. Exercise both required and
	// advisory membership so group coverage cannot be inferred from required
	// check coverage alone.
	_, testsAndAggregate, _ := strings.Cut(partitionedWorkflow, "  tests:\n")
	testsBody, _, _ := strings.Cut(testsAndAggregate, "  docs-gate:\n")
	body := strings.Replace(partitionedWorkflow, "  docs-gate:\n", "  repeated:\n"+testsBody+"  docs-gate:\n", 1)
	body = strings.Replace(body, "[quality, tests]", "[quality, tests, repeated]", 1)
	body = strings.Replace(body, "PYXIS_CI_EXPECTED: success success", "PYXIS_CI_EXPECTED: success success success", 1)
	for _, class := range []string{"required", "advisory"} {
		cfg, err := codegatemirror.Parse([]byte(strings.Replace(testConfig, "    group: tests\n", "    group: tests\n    class: "+class+"\n", 1)))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := checkWorkflow([]byte(body), cfg, "github"); err == nil || !strings.Contains(err.Error(), "repeats") {
			t.Fatalf("repeated %s group escaped: %v", class, err)
		}
	}
}

func TestDiagnosticSelectionRemainsLiteralAndCannotClaimCompleteness(t *testing.T) {
	for _, name := range []string{"registered||true", "registered*", "registered${TOKEN}", "registered,another"} {
		cfg := codegatemirror.Config{Checks: []codegatemirror.Check{{Name: name, Class: codegatemirror.ClassRequired}}}
		if _, err := selectHostedChecks(gateRun+" --check "+name, cfg); err == nil || !strings.Contains(err.Error(), "single literal diagnostic check") {
			t.Fatalf("diagnostic accepted registered shell syntax %q: %v", name, err)
		}
	}
	cfg := codegatemirror.Config{Checks: []codegatemirror.Check{{Name: "memory", Class: codegatemirror.ClassRequired}}}
	body := strings.Replace(testWorkflow, gateRun, gateRun+" --check memory", 1)
	if full, err := checkWorkflow([]byte(body), cfg, "github"); err != nil || full {
		t.Fatalf("diagnostic claimed complete gate: %t, %v", full, err)
	}
}

func TestGroupNamesCannotBecomeWorkflowShellSyntax(t *testing.T) {
	for _, name := range []string{"tests||true", "tests*", "tests${TOKEN}", "tests,default", "'tests'", "tests/default"} {
		cfg := codegatemirror.Config{Checks: []codegatemirror.Check{{Name: "test", Group: name}}}
		if _, err := selectHostedChecks(gateRun+" --group "+name, cfg); err == nil || !strings.Contains(err.Error(), "literal group name") {
			t.Fatalf("accepted registered shell syntax %q: %v", name, err)
		}
		body := strings.ReplaceAll(partitionedWorkflow, "--group tests", "--group "+name)
		if _, err := checkWorkflow([]byte(body), cfg, "github"); err == nil {
			t.Fatalf("group shell syntax %q escaped", name)
		}
	}
}

const phasedConfig = testConfig + `  expensive:
    command: false
    phase: 2
  race:
    command: false
    group: tests
    phase: 2
`

const phasedWorkflow = `name: docs-gate
on: [pull_request, push]
jobs:
  cheap:
    runs-on: ubuntu-latest
    env:
      PYXIS_REQUIRE_PROCESS_CONFINEMENT: "1"
      PYXIS_CI_EVENT: "${{ github.event_name }}"
      PYXIS_CI_BASE: "${{ github.event.pull_request.base.sha || github.event.before }}"
    steps:
      - run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --cheap
  quality:
    needs: [cheap]
    runs-on: ubuntu-latest
    env:
      PYXIS_REQUIRE_PROCESS_CONFINEMENT: "1"
      PYXIS_CI_EVENT: "${{ github.event_name }}"
      PYXIS_CI_BASE: "${{ github.event.pull_request.base.sha || github.event.before }}"
    steps:
      - run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --group default --phase 2
      - run: .pyxis/scripts/docgate-resolve.sh gate --merge-base "$PYXIS_CI_BASE" --defects-json -
  tests:
    needs: [cheap]
    runs-on: ubuntu-latest
    env:
      PYXIS_REQUIRE_PROCESS_CONFINEMENT: "1"
      PYXIS_CI_EVENT: "${{ github.event_name }}"
      PYXIS_CI_BASE: "${{ github.event.pull_request.base.sha || github.event.before }}"
    steps:
      - run: .pyxis/scripts/code-gate-resolve.sh run --no-cache --group tests --phase 2
  docs-gate:
    needs: [cheap, quality, tests]
    if: always()
    runs-on: ubuntu-latest
    env:
      PYXIS_CI_RESULTS: "${{ join(needs.*.result, ' ') }}"
      PYXIS_CI_EXPECTED: success success success
    steps:
      - name: require both check jobs to pass
        run: test "$PYXIS_CI_RESULTS" = "$PYXIS_CI_EXPECTED"
`

func TestPhasedWorkflowMirrorsLocalBarrier(t *testing.T) {
	root := fixture(t)
	writeFile(t, root, ".github/workflows/checks.yml", phasedWorkflow)
	for _, config := range []string{
		phasedConfig,
		phasedConfig + "  new-check:\n    command: new-required-command\n",
		phasedConfig + "  new-cover:\n    command: new-cover-command\n    phase: 2\n",
		phasedConfig + "  new-race:\n    command: new-race-command\n    group: tests\n    phase: 2\n",
	} {
		writeFile(t, root, ".pyxis/config.yaml", config)
		if err := Check(t.Context(), root); err != nil {
			t.Fatalf("phased partition refused: %v", err)
		}
	}
}

func TestParallelGroupsRefuseLaterPhaseWithoutBarrier(t *testing.T) {
	cfg, err := codegatemirror.Parse([]byte(phasedConfig))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := checkWorkflow([]byte(partitionedWorkflow), cfg, "github"); err == nil || !strings.Contains(err.Error(), "earlier-phase") {
		t.Fatalf("ungated later phase escaped: %v", err)
	}
	withoutNeeds := strings.Replace(phasedWorkflow, "  tests:\n    needs: [cheap]\n", "  tests:\n", 1)
	if _, err := checkWorkflow([]byte(withoutNeeds), cfg, "github"); err == nil || !strings.Contains(err.Error(), "earlier-phase") {
		t.Fatalf("phase-2 job without cheap escaped: %v", err)
	}
	dependentCheap := strings.Replace(phasedWorkflow, "  cheap:\n", "  cheap:\n    needs: [quality]\n", 1)
	if _, err := checkWorkflow([]byte(dependentCheap), cfg, "github"); err == nil || !strings.Contains(err.Error(), "independent") {
		t.Fatalf("dependent phase-1 job escaped: %v", err)
	}
}

func TestAggregateAssertionAcceptsOnlyTwoSuccessfulJobs(t *testing.T) {
	// Exercise the actual workflow assertion for every platform job result,
	// including missing results; a skipped dependency must not look green.
	for _, first := range []string{"success", "failure", "cancelled", "skipped", ""} {
		for _, second := range []string{"success", "failure", "cancelled", "skipped", ""} {
			cmd := exec.CommandContext(t.Context(), "/bin/sh", "-ec", aggregateRun)
			cmd.Env = []string{"PYXIS_CI_RESULTS=" + first + " " + second, "PYXIS_CI_EXPECTED=success success"}
			err := cmd.Run()
			want := first == "success" && second == "success"
			if (err == nil) != want {
				t.Errorf("results %q %q: error %v, want pass %v", first, second, err, want)
			}
		}
	}
}
