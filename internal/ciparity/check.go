package ciparity

import (
	"context"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codegatemirror"
)

const gateRun = ".pyxis/scripts/code-gate-resolve.sh run --no-cache"
const docRun = `.pyxis/scripts/docgate-resolve.sh gate --merge-base "$PYXIS_CI_BASE" --defects-json -`
const baseExpression = "${{ github.event.pull_request.base.sha || github.event.before }}"
const eventExpression = "${{ github.event_name }}"

// Check validates every workflow, not a list of previously troublesome files.
// Each blocking verification step must delegate to the canonical check list or
// the independent document gate. Only explicit, bounded runner setup is exempt.
func Check(ctx context.Context, root string) error {
	global, err := os.ReadFile(filepath.Join(root, ".pyxis/config.yaml"))
	if err != nil {
		return fmt.Errorf("read shared checks: %w", err)
	}
	cfg, err := codegatemirror.Parse(global)
	if err != nil {
		return err
	}
	local, err := codegatemirror.Load(root)
	if err != nil {
		return err
	}
	if !reflect.DeepEqual(cfg, local) {
		return fmt.Errorf("local code-gate overrides diverge from the shared CI check list")
	}
	guard, err := cfg.Select("ci-parity")
	if err != nil || guard.Checks[0].Command != "go run ./cmd/ci-parity" || guard.Checks[0].Class != codegatemirror.ClassRequired {
		return fmt.Errorf("ci-parity must remain a required shared check: go run ./cmd/ci-parity")
	}
	if err := checkHook(ctx, root); err != nil {
		return err
	}
	provider, err := workflowProvider(root)
	if err != nil {
		return err
	}
	if provider == "" {
		return ctx.Err()
	}
	dir := filepath.Join(root, "."+provider, "workflows")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return fmt.Errorf("enumerate workflows: %w", err)
	}
	fullGate := false
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ext := filepath.Ext(entry.Name()); ext != ".yml" && ext != ".yaml" {
			continue
		}
		if !entry.Type().IsRegular() {
			return fmt.Errorf("workflow %s must be a regular file", entry.Name())
		}
		data, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			return fmt.Errorf("read workflow %s: %w", entry.Name(), err)
		}
		full, err := checkWorkflow(data, cfg, provider)
		if err != nil {
			return fmt.Errorf("%s: %w", entry.Name(), err)
		}
		fullGate = fullGate || full
	}
	if !fullGate {
		return fmt.Errorf("no unconditional pull_request workflow runs the complete shared code gate and document gate")
	}
	return nil
}

func checkWorkflow(data []byte, cfg codegatemirror.Config, provider string) (bool, error) {
	wf, err := decodeWorkflow(data)
	if err != nil {
		return false, err
	}
	verify, fullPR, err := verificationEvents(wf.On)
	if err != nil || !verify {
		return false, err
	}
	complete := false
	workers := map[string]hostedWorker{}
	aggregates := map[string]job{}
	for _, name := range slices.Sorted(maps.Keys(wf.Jobs)) {
		j := wf.Jobs[name]
		if strings.Contains(j.Name, "${{") {
			return false, fmt.Errorf("job %s: computed names may shadow the docs-gate required context", name)
		}
		if name != "docs-gate" && strings.TrimSpace(j.Name) == "docs-gate" {
			return false, fmt.Errorf("job %s duplicates the docs-gate required context", name)
		}
	}
	for _, name := range slices.Sorted(maps.Keys(wf.Jobs)) {
		j := wf.Jobs[name]
		if j.ContinueOnError && len(j.Needs) == 0 {
			continue // Explicitly advisory jobs never count toward the mirror.
		}
		env := mergedEnv(wf.Env, j.Env)
		gated := false
		var coverage gateCoverage
		var stepErr error
		for i, s := range j.Steps {
			covered, check, err := checkStep(s, env, cfg, provider)
			if err != nil {
				stepErr = fmt.Errorf("job %s step %d (%s): %w", name, i+1, s.Name, err)
				break
			}
			gated = gated || check
			coverage.merge(covered)
		}
		if len(j.Needs) != 0 && !gated {
			aggregates[name] = j
			continue
		}
		if j.If != "" {
			return false, fmt.Errorf("job %s: conditional blocking jobs are outside the shared gate", name)
		}
		if stepErr != nil {
			return false, stepErr
		}
		if !gated {
			return false, fmt.Errorf("job %s has no unconditional shared gate invocation", name)
		}
		if coverage.runs > 1 {
			return false, fmt.Errorf("job %s must invoke one shared group or the complete gate once; sequential invocations change shared scheduling", name)
		}
		workers[name] = hostedWorker{coverage: coverage, needs: j.Needs}
		complete = complete || coverage.complete(cfg)
	}
	if err := checkPhaseOrder(workers, cfg); err != nil {
		return false, err
	}
	for _, name := range slices.Sorted(maps.Keys(aggregates)) {
		if err := checkAggregate(name, aggregates[name], wf.Env, workers, cfg); err != nil {
			return false, fmt.Errorf("job %s: %w", name, err)
		}
		complete = true
	}
	return complete && fullPR, nil
}

func checkStep(s step, jobEnv map[string]string, cfg codegatemirror.Config, provider string) (gateCoverage, bool, error) {
	if s.ContinueOnError {
		return gateCoverage{}, false, nil
	}
	if s.Shell != "" || s.Directory != "" || (s.Run != "" && s.Uses != "") {
		return gateCoverage{}, false, fmt.Errorf("custom shell, working directory, or mixed run/uses can change gate semantics")
	}
	run := strings.TrimSpace(s.Run)
	if strings.HasPrefix(run, gateRun+" --exclude-check ") {
		return gateCoverage{}, false, fmt.Errorf("workflow check lists are not registered execution groups; use code-gate run --no-cache --group NAME")
	}
	if run == gateRun || hostedSelection(run) || run == docRun {
		if s.If != "" || len(s.With) != 0 {
			return gateCoverage{}, false, fmt.Errorf("shared gate must run unconditionally without action inputs")
		}
		env := mergedEnv(jobEnv, s.Env)
		if env["PYXIS_CI_BASE"] != hostedExpression(baseExpression, provider) || env["PYXIS_CI_EVENT"] != hostedExpression(eventExpression, provider) {
			return gateCoverage{}, false, fmt.Errorf("shared gate must receive trusted PYXIS_CI_BASE and PYXIS_CI_EVENT from %s", provider)
		}
		selected := cfg
		var coverage gateCoverage
		if run == docRun {
			selected = codegatemirror.Config{}
			coverage.documents = 1
		} else if run != gateRun {
			coverage.diagnostic = strings.HasPrefix(run, gateRun+" --check ")
			var err error
			selected, err = selectHostedChecks(run, cfg)
			if err != nil {
				return gateCoverage{}, false, err
			}
		}
		if err := checkGateEnv(env, selected, provider); err != nil {
			return gateCoverage{}, false, err
		}
		coverage.addChecks(selected)
		return coverage, true, nil
	}
	if setupStep(s, provider) {
		return gateCoverage{}, false, nil
	}
	return gateCoverage{}, false, fmt.Errorf("unregistered blocking step; add its check to .pyxis/config.yaml and invoke code-gate run --no-cache --group NAME")
}

func hostedSelection(run string) bool {
	return strings.HasPrefix(run, gateRun+" --group ") ||
		strings.HasPrefix(run, gateRun+" --check ") ||
		strings.HasPrefix(run, gateRun+" --cheap") ||
		strings.HasPrefix(run, gateRun+" --phase ")
}

func selectHostedChecks(run string, cfg codegatemirror.Config) (codegatemirror.Config, error) {
	if name, diagnostic := strings.CutPrefix(run, gateRun+" --check "); diagnostic {
		// Existing standalone diagnostic workflows may rerun ONE required
		// check. They cannot define a partition or satisfy the complete gate.
		if name == "" || strings.ContainsFunc(name, func(r rune) bool {
			return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_./:-", r)
		}) {
			return codegatemirror.Config{}, fmt.Errorf("remote selection %q is not registered as a single literal diagnostic check", name)
		}
		selected, err := cfg.Select(name)
		if err != nil {
			return codegatemirror.Config{}, fmt.Errorf("remote diagnostic %q is not registered: %w", name, err)
		}
		if selected.Checks[0].Class != codegatemirror.ClassRequired {
			return codegatemirror.Config{}, fmt.Errorf("remote diagnostic %q is not registered as required", name)
		}
		return selected, nil
	}
	rest, ok := strings.CutPrefix(run, gateRun+" ")
	if !ok {
		return codegatemirror.Config{}, fmt.Errorf("unregistered blocking step; add its check to .pyxis/config.yaml and invoke code-gate run --no-cache --group NAME")
	}
	var group string
	var phase int
	cheap := false
	tokens := strings.Fields(rest)
	for i := 0; i < len(tokens); {
		switch tokens[i] {
		case "--cheap":
			if cheap {
				return codegatemirror.Config{}, fmt.Errorf("remote selection %q is not registered as a shared group", rest)
			}
			cheap = true
			i++
		case "--group":
			if group != "" || i+1 >= len(tokens) {
				return codegatemirror.Config{}, fmt.Errorf("remote selection %q is not registered as a shared group", rest)
			}
			group = tokens[i+1]
			i += 2
		case "--phase":
			if phase != 0 || i+1 >= len(tokens) {
				return codegatemirror.Config{}, fmt.Errorf("remote selection %q is not registered as a shared group", rest)
			}
			n, err := parseHostedPhase(tokens[i+1])
			if err != nil {
				return codegatemirror.Config{}, err
			}
			phase = n
			i += 2
		default:
			return codegatemirror.Config{}, fmt.Errorf("remote selection %q is not registered as a shared group", rest)
		}
	}
	if cheap && (group != "" || phase != 0) {
		return codegatemirror.Config{}, fmt.Errorf("remote --cheap cannot combine with --group or --phase")
	}
	if cheap {
		selected, err := cfg.SelectCheap()
		if err != nil {
			return codegatemirror.Config{}, fmt.Errorf("remote cheap selection: %w", err)
		}
		return selected, nil
	}
	if group != "" && phase != 0 {
		selected, err := cfg.SelectGroupPhase(group, phase)
		if err != nil {
			return codegatemirror.Config{}, fmt.Errorf("remote group %q phase %d is not registered in .pyxis/config.yaml: %w", group, phase, err)
		}
		return selected, nil
	}
	if group != "" {
		// SelectGroup requires a literal name: no shell operators, quoting,
		// substitutions or globs, even if supplied by a malformed caller.
		selected, err := cfg.SelectGroup(group)
		if err != nil {
			return codegatemirror.Config{}, fmt.Errorf("remote group %q is not registered in .pyxis/config.yaml: %w", group, err)
		}
		return selected, nil
	}
	if phase != 0 {
		selected, err := cfg.SelectPhase(phase)
		if err != nil {
			return codegatemirror.Config{}, fmt.Errorf("remote phase %d is not registered in .pyxis/config.yaml: %w", phase, err)
		}
		return selected, nil
	}
	return codegatemirror.Config{}, fmt.Errorf("unregistered blocking step; add its check to .pyxis/config.yaml and invoke code-gate run --no-cache --group NAME")
}

func parseHostedPhase(s string) (int, error) {
	if s == "" || strings.ContainsFunc(s, func(r rune) bool { return r < '0' || r > '9' }) {
		return 0, fmt.Errorf("remote phase %q is not a literal phase number", s)
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 1 {
		return 0, fmt.Errorf("remote phase %q is not a literal phase number", s)
	}
	return n, nil
}

func checkPhaseOrder(workers map[string]hostedWorker, cfg codegatemirror.Config) error {
	for _, name := range slices.Sorted(maps.Keys(workers)) {
		w := workers[name]
		for _, dep := range w.needs {
			if _, ok := workers[dep]; !ok {
				return fmt.Errorf("job %s: dependency %q is missing, advisory, or not a verification worker", name, dep)
			}
		}
		phases := w.coverage.selected.Phases()
		if len(phases) == 0 {
			continue
		}
		if phases[len(phases)-1] <= codegatemirror.DefaultPhase {
			if len(w.needs) != 0 {
				return fmt.Errorf("job %s: phase-1 workers must stay independent", name)
			}
			continue
		}
		for _, phase := range phases {
			if phase <= codegatemirror.DefaultPhase {
				continue
			}
			if err := checkPhasePredecessor(name, w, workers, cfg, phase); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkPhasePredecessor(name string, w hostedWorker, workers map[string]hostedWorker, cfg codegatemirror.Config, phase int) error {
	have := map[string]bool{}
	for _, check := range w.coverage.selected.Checks {
		if check.Class == codegatemirror.ClassRequired && check.ExecutionPhase() < phase {
			have[check.Name] = true
		}
	}
	for _, dep := range w.needs {
		for _, check := range workers[dep].coverage.selected.Checks {
			if check.Class == codegatemirror.ClassRequired {
				have[check.Name] = true
			}
		}
	}
	for _, check := range cfg.Checks {
		if check.Class != codegatemirror.ClassRequired || check.ExecutionPhase() >= phase {
			continue
		}
		if !have[check.Name] {
			return fmt.Errorf("job %s runs phase %d before required earlier-phase check %q completed", name, phase, check.Name)
		}
	}
	return nil
}

func mergedEnv(layers ...map[string]string) map[string]string {
	env := map[string]string{}
	for _, layer := range layers {
		maps.Copy(env, layer)
	}
	return env
}
