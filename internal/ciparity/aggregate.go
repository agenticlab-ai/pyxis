package ciparity

import (
	"fmt"
	"maps"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

// GitHub/Forgejo run steps require a shell entry point. This sole assertion
// compares platform-supplied results; it runs no repository code or checks.
const aggregateRun = `test "$PYXIS_CI_RESULTS" = "$PYXIS_CI_EXPECTED"`
const aggregateResults = "${{ join(needs.*.result, ' ') }}"

type gateCoverage struct {
	checks      map[string]int
	groups      map[string]int
	groupPhases map[string]int
	selected    codegatemirror.Config
	runs        int
	diagnostic  bool
	documents   int
}

type hostedWorker struct {
	coverage gateCoverage
	needs    []string
}

func groupPhaseKey(check codegatemirror.Check) string {
	return check.ExecutionGroup() + "@" + strconv.Itoa(check.ExecutionPhase())
}

func samePhase(cfg codegatemirror.Config) bool {
	if len(cfg.Checks) == 0 {
		return false
	}
	phase := cfg.Checks[0].ExecutionPhase()
	for _, check := range cfg.Checks[1:] {
		if check.ExecutionPhase() != phase {
			return false
		}
	}
	return true
}

func validPartition(c gateCoverage) bool {
	return len(c.groups) == 1 || samePhase(c.selected)
}

func (c *gateCoverage) addChecks(cfg codegatemirror.Config) {
	if c.checks == nil {
		c.checks = map[string]int{}
	}
	if c.groupPhases == nil {
		c.groupPhases = map[string]int{}
	}
	seenPhase := map[string]bool{}
	for _, check := range cfg.Checks {
		if check.Class == codegatemirror.ClassRequired {
			c.checks[check.Name]++
		}
		key := groupPhaseKey(check)
		if !seenPhase[key] {
			c.groupPhases[key]++
			seenPhase[key] = true
		}
		c.selected.Checks = append(c.selected.Checks, check)
	}
	if len(cfg.Checks) != 0 {
		c.runs++
		if c.groups == nil {
			c.groups = map[string]int{}
		}
		for _, group := range cfg.Groups() {
			c.groups[group]++
		}
	}
}

func (c *gateCoverage) merge(other gateCoverage) {
	if c.checks == nil {
		c.checks = map[string]int{}
	}
	for name, count := range other.checks {
		c.checks[name] += count
	}
	if c.groups == nil {
		c.groups = map[string]int{}
	}
	for name, count := range other.groups {
		c.groups[name] += count
	}
	if c.groupPhases == nil {
		c.groupPhases = map[string]int{}
	}
	for name, count := range other.groupPhases {
		c.groupPhases[name] += count
	}
	c.selected.Checks = append(c.selected.Checks, other.selected.Checks...)
	c.runs += other.runs
	c.diagnostic = c.diagnostic || other.diagnostic
	c.documents += other.documents
}

func (c gateCoverage) complete(cfg codegatemirror.Config) bool {
	if c.documents == 0 || c.diagnostic {
		return false
	}
	for _, check := range cfg.Checks {
		if check.Class == codegatemirror.ClassRequired && c.checks[check.Name] == 0 {
			return false
		}
	}
	for _, group := range cfg.Groups() {
		if c.groups[group] == 0 {
			return false
		}
	}
	return true
}

// Only this closed aggregate may depend on verification jobs. Its direct needs
// must cover all unconditional workers, each required check exactly once, and
// the document gate. Missing, skipped, failed and cancelled results differ from
// the exact all-success string, even though the platform treats skips as green.
func checkAggregate(name string, j job, workflowEnv map[string]string, workers map[string]hostedWorker, cfg codegatemirror.Config) error {
	if name != "docs-gate" || (j.Name != "" && j.Name != "docs-gate") {
		return fmt.Errorf("aggregate must preserve the docs-gate required context")
	}
	if j.If != "always()" && j.If != "${{ always() }}" {
		return fmt.Errorf("aggregate must run with always() after failed or skipped jobs")
	}
	if j.ContinueOnError || len(j.Steps) != 1 || j.RunsOn == "" {
		return fmt.Errorf("aggregate must have one required result assertion on a runner")
	}
	if len(j.Needs) < 2 || len(j.Needs) != len(workers) {
		return fmt.Errorf("aggregate needs must include every independent verification job")
	}
	seen := map[string]bool{}
	var coverage gateCoverage
	for _, dependency := range j.Needs {
		worker, ok := workers[dependency]
		if !ok || seen[dependency] {
			return fmt.Errorf("aggregate dependency %q is missing, advisory, dependent, or duplicated", dependency)
		}
		seen[dependency] = true
		if worker.coverage.runs != 1 || worker.coverage.diagnostic || !validPartition(worker.coverage) {
			return fmt.Errorf("aggregate worker %q must execute exactly one shared group", dependency)
		}
		coverage.merge(worker.coverage)
	}
	if !coverage.complete(cfg) {
		return fmt.Errorf("aggregate does not cover the complete shared code gate and document gate")
	}
	for _, check := range slices.Sorted(maps.Keys(coverage.checks)) {
		if coverage.checks[check] != 1 {
			return fmt.Errorf("aggregate repeats required check %q across its jobs", check)
		}
	}
	if coverage.documents != 1 {
		return fmt.Errorf("aggregate repeats the document gate")
	}
	for _, key := range slices.Sorted(maps.Keys(coverage.groupPhases)) {
		if coverage.groupPhases[key] != 1 {
			group, _, _ := strings.Cut(key, "@")
			return fmt.Errorf("aggregate repeats shared group %q across its jobs", group)
		}
	}
	s := j.Steps[0]
	if strings.TrimSpace(s.Run) != aggregateRun || s.Uses != "" || s.If != "" || s.ContinueOnError || s.Shell != "" || s.Directory != "" || len(s.With) != 0 {
		return fmt.Errorf("aggregate must use the unconditional result assertion without overrides")
	}
	expected := strings.TrimSpace(strings.Repeat("success ", len(j.Needs)))
	wantEnv := map[string]string{"PYXIS_CI_RESULTS": aggregateResults, "PYXIS_CI_EXPECTED": expected}
	if !reflect.DeepEqual(mergedEnv(workflowEnv, j.Env, s.Env), wantEnv) {
		return fmt.Errorf("aggregate must compare all needs results with exactly %d successes", len(j.Needs))
	}
	return nil
}
