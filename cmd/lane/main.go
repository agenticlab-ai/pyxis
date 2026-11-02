// lane — the merge lane's executable surface (merge-lane-v1; plan T4's
// wiring; #514). Thin main: verb dispatch, JSON on stdout, defects on stderr.
// Exit codes follow the shape's CLI contract: 0 (verdict emitted / state
// resolved / handled), 2 (refusal — misparse, malformed or unratified policy,
// unclassifiable input, usage), 1 (infrastructure fault).
//
// Verbs:
//
//	classify — the single hermetic lane predicate over one (base, head) pair.
//	           Pure: git reads only; no network writes, no credential reads,
//	           no medium mutations. The binary and the policy load from the
//	           context it runs in — the workflow checks out the TRUSTED
//	           MAINLINE, so the proposal contributes only its diff (DINV-9).
//	handle   — the acting-adapter path the workflow invokes per proposal
//	           event: classify → record the verdict on the proposal →
//	           resolve state → the act gate (acts only in the enforcing
//	           state, which requires the operator's recorded activation).
//	sweep    — evaluate every open proposal from this host: the sequential
//	           fallback for the event-driven workflow (a harness primitive
//	           is an optimization; the fallback is stated). Idempotent per
//	           head; never acts.
//	state    — resolve and print the lane's operating state from the repo
//	           alone (merge-lane-v1 §3.4's debugging surface).
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/lane/acting"
	"github.com/agenticlab-ai/pyxis/internal/lane/classifier"
	"github.com/agenticlab-ai/pyxis/internal/lane/diffdata"
	"github.com/agenticlab-ai/pyxis/internal/lane/githost"
	"github.com/agenticlab-ai/pyxis/internal/lane/policy"
)

var processExit = os.Exit
var sweepGitRead = gitRead

func main() { processExit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func usage(stderr *os.File) int {
	_, _ = fmt.Fprintln(stderr, `usage: lane <verb> [flags]
verbs:
  classify --base <sha> --head <sha> [--repo <root>] [--checks <snapshot-id>]
           evaluate one proposal diff against the mainline risk policy and
           emit the verdict (JSON); exit 2 with named defects on any refusal
  handle   --pr <number> --base <sha> --head <sha> --gh-repo <owner/name>
           [--repo <root>] [--checks <snapshot-id>] [--machine <login>]
           run one proposal event end-to-end: classify, record the verdict on
           the proposal, resolve state, act only if enforcing; emits the
           result (JSON)
  sweep    --gh-repo <owner/name> [--repo <root>]
           evaluate every open proposal from this host (the event-driven
           workflow's sequential fallback); skips heads already recorded;
           emits a per-proposal report (JSON); never acts
  state    [--repo <root>] resolve and print the lane operating state`) // Best-effort usage; the command refuses regardless of terminal availability.
	return 2
}

func run(args []string, stdout, stderr *os.File) int {
	if len(args) < 1 {
		return usage(stderr)
	}
	ctx := context.Background()
	switch args[0] {
	case "classify":
		return runClassify(ctx, args[1:], stdout, stderr)
	case "handle":
		return runHandle(ctx, args[1:], stdout, stderr)
	case "sweep":
		return runSweep(ctx, args[1:], stdout, stderr)
	case "state":
		return runState(args[1:], stdout, stderr)
	default:
		_, _ = fmt.Fprintf(stderr, "lane: unknown verb %q\n", args[0]) // Best-effort diagnostic; exit status carries refusal.
		return usage(stderr)
	}
}

// emitDefects prints named defects, one JSON line each, to stderr — the same
// dual-face convention docgate holds (human-readable, machine-parsable).
func emitDefects(stderr *os.File, ds []schema.Defect) {
	for _, d := range ds {
		b, err := json.Marshal(d)
		if err != nil {
			_, _ = fmt.Fprintf(stderr, "lane: defect marshal: %v\n", err) // Best-effort diagnostic; the caller retains the defect outcome.
			continue
		}
		_, _ = fmt.Fprintln(stderr, string(b)) // Diagnostic output must not change the caller's already determined routing or refusal.
	}
}

// loadPolicy loads the risk policy from the trusted context root, emitting its
// defects. A load failure is not an error path for the caller: the lane is
// all-HUMAN with the defect named (DINV-8) — callers decide whether that is a
// refusal (classify: exit 2) or a recorded HUMAN routing (handle).
func loadPolicy(root string, stderr *os.File) (*policy.Policy, []schema.Defect) {
	p, defects := policy.Load(root)
	if len(defects) > 0 {
		emitDefects(stderr, defects)
	}
	return p, defects
}

func runClassify(ctx context.Context, args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("classify", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("repo", ".", "repo root — the TRUSTED context the policy and binary load from")
	base := fs.String("base", "", "merge-base sha the diff derives from (pinned)")
	head := fs.String("head", "", "head sha evaluated (pinned)")
	checks := fs.String("checks", "", "pinned check-state snapshot id")
	if err := fs.Parse(args); err != nil {
		return 2
	}

	p, loadDefects := loadPolicy(*root, stderr)
	cfg, _ := config.Load(*root)
	d, err := diffdata.Derive(ctx, *root, *base, *head, cfg)
	if err != nil {
		emitDefects(stderr, []schema.Defect{{Verb: "lane-classify", File: *root, Op: "diff",
			Reason: err.Error(), Remedy: "pass resolvable --base/--head shas present in the repo"}})
		return 2
	}

	v := classifier.Classify(classifier.Inputs{Diff: d, Base: *base, Head: *head, CheckState: *checks}, p, nil)
	if !emitRecord(stdout, stderr, v.Record()) {
		return 1
	}
	// The shape's CLI contract: a refusal-class input (unloadable policy —
	// malformed, unratified, absent) exits 2 with the defect named; the verdict
	// it still emitted routes HUMAN (DINV-8).
	if len(loadDefects) > 0 {
		return 2
	}
	return 0
}

func runHandle(ctx context.Context, args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("handle", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("repo", ".", "repo root — the TRUSTED context")
	ghRepo := fs.String("gh-repo", "", "owner/name of the code host repository")
	pr := fs.String("pr", "", "proposal (pull request) number")
	base := fs.String("base", "", "merge-base sha the diff derives from")
	head := fs.String("head", "", "head sha evaluated")
	checks := fs.String("checks", "", "pinned check-state snapshot id")
	machine := fs.String("machine", "", "dedicated machine identity login — ONLY the enforcing deployment sets this; empty = shadow (no acting power)")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *pr == "" {
		_, _ = fmt.Fprintln(stderr, "lane handle: --gh-repo and --pr are required") // Best-effort diagnostic; exit status carries refusal.
		return 2
	}

	p, _ := loadPolicy(*root, stderr)
	cfg, _ := config.Load(*root)
	d, err := diffdata.Derive(ctx, *root, *base, *head, cfg)
	if err != nil {
		emitDefects(stderr, []schema.Defect{{Verb: "lane-handle", File: *root, Op: "diff",
			Reason: err.Error(), Remedy: "pass resolvable --base/--head shas present in the repo"}})
		return 2
	}

	host, err := githost.NewConfigured(*ghRepo, *root, cfg.Remote, cfg.Branch)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lane handle: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 2
	}
	adapter := &acting.Adapter{Medium: host, Recorder: host}
	// The Machine is wired ONLY when the deployment names the machine identity
	// (the enforcing workflow holding the sealed credential). The shadow
	// workflow never passes --machine, so this deployment cannot act (DINV-5);
	// even with a Machine, the act gate requires the enforcing state, which
	// requires the operator's recorded activation acts (FR-9).
	if *machine != "" {
		adapter.Machine = &githost.Machine{Host: host, Name: *machine}
	}

	// Activation preconditions are READ as data (FR-9's pattern). The lane-state
	// ref-protection read is an activation-era medium feature; until the
	// operator's activation acts land it reads absent → false (fail-closed:
	// enforce cannot resolve without it, A-3).
	pc := acting.Preconditions{RefProtectionPresent: false, Halted: false}

	ev := acting.Event{Proposal: *pr, Diff: d, Base: *base, Head: *head, CheckSnapshot: *checks}
	res, err := adapter.Handle(ctx, ev, p, pc)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lane handle: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 1
	}
	emitDefects(stderr, res.Refusals)
	out, err := json.Marshal(res)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lane handle: result marshal: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 1
	}
	if !emitRecord(stdout, stderr, out) {
		return 1
	}
	return 0
}

// runSweep evaluates every open proposal from this host — the sequential
// fallback for the event-driven workflow (the drive guide's rule: any harness
// primitive is an optimization with the sequential fallback stated). Per open
// PR: fetch its head as data, compute the merge base against the mainline,
// and run the same handle path (classify → record → resolve → act gate). A
// proposal whose recorded verdict already covers its current head is skipped —
// the workflow's per-synchronize idempotence, replayed host-side. Shadow by
// construction: no --machine is accepted here; the sweep can never act.
func runSweep(ctx context.Context, args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("sweep", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("repo", ".", "repo root — the TRUSTED context (must sit at the mainline)")
	ghRepo := fs.String("gh-repo", "", "owner/name of the code host repository")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	p, _ := loadPolicy(*root, stderr)
	cfg, _ := config.Load(*root)
	host, err := githost.NewConfigured(*ghRepo, *root, cfg.Remote, cfg.Branch)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lane sweep: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 2
	}
	adapter := &acting.Adapter{Medium: host, Recorder: host}
	pc := acting.Preconditions{RefProtectionPresent: false, Halted: false}

	proposals, err := host.OpenProposals(ctx)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lane sweep: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 1
	}
	type row struct {
		Proposal int    `json:"proposal"`
		Head     string `json:"head"`
		Outcome  string `json:"outcome"` // evaluated | skipped-unchanged | fault
		Lane     string `json:"lane,omitempty"`
		Reason   string `json:"reason,omitempty"`
	}
	var rows []row
	faults := 0
	for _, pr := range proposals {
		n := fmt.Sprintf("%d", pr.Number)
		last, err := host.LastRecordedHead(ctx, n)
		if err != nil {
			rows = append(rows, row{Proposal: pr.Number, Head: pr.Head, Outcome: "fault", Reason: err.Error()})
			faults++
			continue
		}
		if last == pr.Head {
			rows = append(rows, row{Proposal: pr.Number, Head: pr.Head, Outcome: "skipped-unchanged"})
			continue
		}
		if _, err := sweepGitRead(ctx, *root, "fetch", "--no-tags", "-q", cfg.Remote, pr.Head); err != nil {
			rows = append(rows, row{Proposal: pr.Number, Head: pr.Head, Outcome: "fault", Reason: err.Error()})
			faults++
			continue
		}
		// The merge base is computed against the configured mainline ref
		// (config-driven-portability-v1 §3.2, FR-2) — "origin/main" by default.
		baseOut, err := sweepGitRead(ctx, *root, "merge-base", cfg.MainlineRef(), pr.Head)
		if err != nil {
			rows = append(rows, row{Proposal: pr.Number, Head: pr.Head, Outcome: "fault", Reason: err.Error()})
			faults++
			continue
		}
		base := strings.TrimSpace(string(baseOut))
		d, err := diffdata.Derive(ctx, *root, base, pr.Head, cfg)
		if err != nil {
			rows = append(rows, row{Proposal: pr.Number, Head: pr.Head, Outcome: "fault", Reason: err.Error()})
			faults++
			continue
		}
		ev := acting.Event{Proposal: n, Diff: d, Base: base, Head: pr.Head,
			CheckSnapshot: "gh-checks@" + pr.Head}
		res, err := adapter.Handle(ctx, ev, p, pc)
		if err != nil {
			rows = append(rows, row{Proposal: pr.Number, Head: pr.Head, Outcome: "fault", Reason: err.Error()})
			faults++
			continue
		}
		emitDefects(stderr, res.Refusals)
		rows = append(rows, row{Proposal: pr.Number, Head: pr.Head, Outcome: "evaluated",
			Lane: string(res.Verdict.Lane), Reason: res.Verdict.Reason})
	}
	out, err := json.Marshal(rows)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lane sweep: report marshal: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 1
	}
	if !emitRecord(stdout, stderr, out) {
		return 1
	}
	if faults > 0 {
		return 1
	}
	return 0
}

// gitRead executes one git read under dir — the sweep's revision plumbing.
func gitRead(ctx context.Context, dir string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return out, nil
}

func runState(args []string, stdout, stderr *os.File) int {
	fs := flag.NewFlagSet("state", flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("repo", ".", "repo root")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	p, _ := loadPolicy(*root, stderr)
	state, defects := acting.Resolve(p, acting.Preconditions{})
	emitDefects(stderr, defects)
	summary := struct {
		State  string `json:"state"`
		Policy string `json:"policy"`
	}{State: string(state), Policy: policyVersion(p)}
	b, err := json.Marshal(summary)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "lane state: marshal: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return 1
	}
	if !emitRecord(stdout, stderr, b) {
		return 1
	}
	return 0
}

// A missing structured result is an infrastructure fault even if the lane's
// earlier host operations succeeded; callers must not mistake it for success.
func emitRecord(stdout, stderr *os.File, record []byte) bool {
	if _, err := fmt.Fprintln(stdout, string(record)); err != nil {
		_, _ = fmt.Fprintf(stderr, "lane: write result: %v\n", err) // Best-effort diagnostic; exit status carries failure.
		return false
	}
	return true
}

// policyVersion names the loaded policy for the state summary, or the
// all-HUMAN load posture when none loaded.
func policyVersion(p *policy.Policy) string {
	if p == nil {
		return "unloaded (all-HUMAN, DINV-8)"
	}
	return p.SchemaVersion
}

// interface guards — the adapter wiring must satisfy the acting seams.
var (
	_ acting.Medium   = (*githost.Host)(nil)
	_ acting.Recorder = (*githost.Host)(nil)
	_ acting.Machine  = (*githost.Machine)(nil)
)
