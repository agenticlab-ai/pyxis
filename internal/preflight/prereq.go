// The suite's post-install applicability mode (design init-hardening-v1
// §3.1; plan init-hardening/001 T10): the prerequisite rows doctor renders
// delegate to THIS pass, so the presence/floor predicates and the
// gh-when-required applicability live exactly once (DINV-2; the I5
// two-evaluator drift made structurally impossible). It deliberately
// differs from init's probeToolchain in posture, not in predicate:
//
//   - presence is the row's core fact (doctor's historical contract);
//   - the floor leg is best-effort — a version that cannot be queried or
//     parsed NEVER fails a post-install pass (an installed, working repo is
//     not refused over `--version` output cosmetics), whereas init's
//     pre-write probe stays strict (an unparseable version there is an
//     infra error, probeToolchain's contract, unchanged);
//   - nothing here refuses: rows are data for the consumer's renderer;
//     doctor maps them onto its finding shapes and severities.
//
// The probed set reuses toolchainTools + toolchainFloors (one home for the
// toolchain predicate) and appends gh exactly when the captured knobs
// declare a gh-transport operation — githubModeApplicable, the same
// operation-table-derived predicate the GitHub-mode check runs on, never a
// second knob reading.
package preflight

import (
	"context"
	"os/exec"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// PrereqTool is one prerequisite row for a post-install consumer: the
// probed executable, whether it is on PATH, and — when a version was
// extractable — whether it clears the release-documented floor. Version and
// Floor are informative ("" when unknown); BelowFloor is set only from an
// extracted version, never from a probe failure.
type PrereqTool struct {
	Name       string
	Present    bool
	BelowFloor bool
	Version    string // extracted version, "" when not queryable/parseable
	Floor      string // the recorded floor, "" when none is recorded (gh)
}

// Prerequisites evaluates the post-install prerequisite rows against the
// captured knobs: every toolchain executable (presence + best-effort
// floor), plus gh (presence only — no floor is recorded for it) exactly
// when the knobs imply a gh-transport operation. Read-only; zero
// mutations; no network (LookPath and local `--version`/`gh --version`
// queries at most) — the posture doctor's zero-mutations contract requires.
func Prerequisites(ctx context.Context, knobs config.Knobs, runner wire.Runner) []PrereqTool {
	rows := make([]PrereqTool, 0, len(toolchainTools)+1)
	for _, tool := range toolchainTools {
		rows = append(rows, probePrereq(ctx, tool))
	}
	if githubModeApplicable(knobs) {
		if runner == nil {
			runner = wire.GHRunner{}
		}
		// The SAME evaluator the GitHub-mode check runs (probeGHPresence,
		// through the one wire.Runner seam) — never a bare LookPath beside
		// it: "present" means gh RUNS, for init and doctor alike, so a
		// broken-but-on-PATH gh can never split their verdicts (DINV-2).
		_, present := probeGHPresence(ctx, runner)
		rows = append(rows, PrereqTool{Name: "gh", Present: present})
	}
	return rows
}

// PrerequisitesForIntegrations retains the local prerequisite evaluator while
// deriving gh's presence from the actual selected operation transports.
func PrerequisitesForIntegrations(ctx context.Context, knobs config.Knobs, runner wire.Runner, profile integrations.Config) ([]PrereqTool, error) {
	if profile.SchemaVersion == "" {
		return Prerequisites(ctx, knobs, runner), nil
	}
	// A harmless fallback coordinate permits transport selection for legacy
	// bindings; this pass performs no remote lookup or repository operation.
	ops, err := wire.TableForIntegrations(wire.Config{Repo: config.Config{Knobs: knobs}, Slug: "prerequisite/probe"}, profile)
	if err != nil {
		return nil, err
	}
	requiresGH := false
	for _, op := range ops {
		if op.Transport == wire.TransportGH {
			requiresGH = true
			break
		}
	}
	probeKnobs := config.Knobs{}
	if requiresGH {
		probeKnobs.Tracker = "github"
	}
	return Prerequisites(ctx, probeKnobs, runner), nil
}

// probePrereq probes one toolchain executable in the tolerant post-install
// posture: absent → a not-present row; present with an extractable version
// below the recorded floor → a below-floor row; any version-query or parse
// failure → a present row with the floor question honestly unanswered.
func probePrereq(ctx context.Context, tool toolchainTool) PrereqTool {
	row := PrereqTool{Name: tool.name, Floor: toolchainFloors[tool.name]}
	path, err := exec.LookPath(tool.name)
	if err != nil {
		return row
	}
	row.Present = true
	out, err := exec.CommandContext(ctx, path, "--version").Output()
	if err != nil {
		return row
	}
	ver, ok := tool.extract(string(out))
	if !ok {
		return row
	}
	row.Version = ver
	if row.Floor != "" && !versionAtLeast(ver, row.Floor) {
		row.BelowFloor = true
	}
	return row
}
