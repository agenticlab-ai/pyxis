// Package wire owns GitHub wiring as ONE declared-operation table executed
// through the gh CLI (installer-v1 §3.6; plan installer/001 T10). The table
// is the single contract (AP-11): Table derives the declared set from the
// operand's captured knobs exactly per the solution shape's wiring rows,
// Preview renders the would-do set from that same table, Execute walks
// exactly it, and the Audit — the EVT-4 `wiring_applied` event
// (installer-v1 §3.8) — records what actually ran. gh is transport, never
// policy: every mutation wiring performs against the user's account is a
// row here, so the executed set can never exceed the declared set
// (prd:installer/ABUSE-3) — Execute constructs no operation outside the
// table it is handed, and an op name outside the five declared rows runs
// nothing.
//
// The five rows and their knob conditions (the shape's wiring table):
//
//	create-repo     gh repo create               wiring.repo = create
//	ensure-labels   gh label list → create        tracker = github
//	push-workflow   converge write set + push     ci_mode ∈ {actions, both}
//	set-protection  gh api …/protection (PUT)     wiring.protection ≠ none
//	welcome-item    gh issue create               tracker = github
//
// push-workflow performs NO remote operation from this package: the
// workflow file lands via the converge write set and rides the initial
// push (the shape's row names exactly that transport). It stays in the
// table — declared, previewed, audited — so the account-facing contract a
// human reads is complete; its Transport is TransportConverge and Execute
// records it skipped-with-reason.
//
// Row order is execution's dependency order — create-repo first (nothing
// can target a repository that does not exist yet), then the shape's row
// order. The shape's table binds the SET and the conditions, never a walk
// order.
//
// Knob conditions match POSITIVELY over the closed enums (T2's Knobs): a
// zero value means "never captured" and declares nothing — in particular
// wiring.protection "" is not "≠ none".
//
// Failure posture (prd:installer/UB-4; installer-v1 §3.9 leg 3):
//   - gh absent or unrunnable → the named GitHub-mode prerequisite refusal
//     (UB-7 class) BEFORE any operation runs; a table with no gh-transport
//     rows never touches gh (the offline floor).
//   - a permission gap on an operation → every remaining permitted
//     operation still runs; the un-runnable remainder lands in a
//     copy-paste hand-off block of exact gh commands; Execute reports
//     partial, not an error.
//   - transient failures (429/5xx/network) → bounded retry (3 attempts,
//     capped backoff, injectable sleeper), then hand-off per UB-4's
//     extension.
//
// Idempotency (AP-22): ensure-labels checks-then-creates; set-protection
// is a full PUT (idempotent by nature); create-repo probes and
// skips-with-reason when the repository exists; welcome-item probes for
// WelcomeMarker in existing item bodies and skips-with-reason when found.
package wire

import (
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/plangate/dialect"
)

// The six declared operation names — the closed row vocabulary of the
// shape's wiring table. Execute runs nothing outside this set.
const (
	OpCreateRepo      = "create-repo"
	OpEnsureLabels    = "ensure-labels"
	OpPushWorkflow    = "push-workflow"
	OpSetProtection   = "set-protection"
	OpWelcomeItem     = "welcome-item"
	OpOnboardingItems = "onboarding-items"
)

// Transport names how a declared operation reaches the world.
type Transport string

const (
	// TransportGH — the operation executes through the gh CLI.
	TransportGH      Transport = "gh"
	TransportForgejo Transport = "forgejo"
	// TransportConverge — the operation is carried by the converge write
	// set and the initial push; wire performs no remote operation for it
	// (the shape's push-workflow row).
	TransportConverge Transport = "converge"
)

// RequiredCheckContext is the status-check context set-protection requires:
// the docs-gate job of the installed-repo workflow variant the payload
// ships (payload.InstalledWorkflowTarget — .github/workflows/docs-gate.yml,
// job id `docs-gate`; installer-v1 §3.10). A consistency test asserts this
// constant against the shipped workflow file so the two can never drift.
const RequiredCheckContext = "docs-gate"

// WelcomeMarker is the welcome item's stable idempotency key: Table ensures
// the item body carries it, and Execute's probe scans existing item bodies
// for it, skipping-with-reason on a hit (AP-22). Same HTML-comment marker
// convention as the projection's plan markers (dialect.PlanTaskMarker).
const WelcomeMarker = "<!-- pyxis:welcome-item -->"

// WorkItem is one onboarding work item the onboarding-items op ensures
// (#947): the setup init could not do for the user, materialized in the
// tracker instead of terminal scrollback. Slug keys the item's idempotency
// marker; the installer authors the standing set (governing docs to write,
// the code-gate seed step) and the op itself derives one more from the
// walk's own hand-offs.
type WorkItem struct {
	Slug  string // stable marker key, kebab-case
	Title string
	Body  string
}

// OnboardingMarker renders a WorkItem's idempotency marker — the same
// HTML-comment convention as WelcomeMarker, keyed by slug.
func OnboardingMarker(slug string) string {
	return "<!-- pyxis:onboarding:" + slug + " -->"
}

// Config is Table's operand: the parsed repo config whose captured knobs
// drive the declared set (T2's surface), plus the inputs the knobs cannot
// carry — the GitHub coordinate and the welcome item's instantiated
// content (T8 owns rendering the welcome template; wire never renders it).
type Config struct {
	// Repo is the parsed operand config; Repo.Knobs decides which rows are
	// declared and Repo.Branch names set-protection's branch (blank resolves the
	// config schema's default branch, config.Defaults).
	Repo config.Config
	// Slug is the owner/name repository coordinate every gh-transport
	// operation targets — required whenever any gh row is declared;
	// Execute refuses by name pre-action when it is missing.
	Slug string
	// WelcomeTitle / WelcomeBody are the welcome item's instantiated
	// content. Table appends WelcomeMarker to the body when absent so the
	// audited argv is exactly what runs (one source for preview, audit,
	// and hand-off).
	WelcomeTitle string
	WelcomeBody  string
	// OnboardingItems is the installer-authored standing set the
	// onboarding-items op ensures (#947) — empty declares no op. Table
	// appends each item's marker at declaration time, same one-source
	// rationale as the welcome body.
	OnboardingItems []WorkItem
}

// Op is one declared row of the operation table — the single contract
// Preview renders, Execute walks, and the Audit records. Every field an
// execution needs is baked here at Table time; Execute adds nothing.
// Fields beyond Name/Transport/Condition/Slug are per-op payload,
// populated only for the row that consumes them.
type Op struct {
	Name      string    // one of the five declared names
	Transport Transport // gh | converge
	Condition string    // the shape-row knob condition that declared this op

	Slug       string                          // owner/name — every gh op's target
	Connection integrations.ResolvedRepository // configured host identity; never contains credentials

	// ensure-labels: the canonical routing-label set (the plan dialect's
	// vocabulary — AP-10, one source; presentation applies at creation only).
	Labels []dialect.Label

	// set-protection:
	Branch        string   // the protected branch (the captured branch knob)
	Contexts      []string // required status-check contexts (RequiredCheckContext)
	RequireReview bool     // true iff wiring.protection = checks+review

	// welcome-item (Body always carries WelcomeMarker):
	Title string
	Body  string

	// onboarding-items: the standing set (each Body already carries its
	// OnboardingMarker from Table); the op derives at most one more item
	// from the walk's own hand-offs at execution time.
	Items []WorkItem
}

// RoutingLabels is the label set ensure-labels ensures: the plan dialect's
// canonical vocabulary (prd:installer/FR-10 "the routing labels"), reused —
// never re-declared — so projection and wiring can't drift (AP-10).
func RoutingLabels() []dialect.Label { return dialect.Harbormaster().Labels() }

// Table derives the declared operation set from the captured knobs, exactly
// per the shape's wiring table — one Op per satisfied condition, in
// execution's dependency order (create-repo first). It is a pure
// derivation: no IO, no validation beyond baking the rows; Execute holds
// the pre-action refusals.
func Table(cfg Config) []Op {
	k := cfg.Repo.Knobs
	branch := cfg.Repo.Branch
	if branch == "" {
		// The blank-Repo fallback resolves the config schema's default branch
		// (config.Defaults — "main" by default, the ONE place that literal lives,
		// DINV-1), never a second literal here. A Repo parsed from config always
		// carries a Branch (Defaults sets it), so this only serves a zero-value
		// Config.Repo (config-driven-portability-v1 §3.2, FR-2).
		branch = config.Defaults().Branch
	}
	var ops []Op
	if k.Wiring.Repo == "create" {
		ops = append(ops, Op{
			Name: OpCreateRepo, Transport: TransportGH,
			Condition: "wiring.repo=create", Slug: cfg.Slug,
		})
	}
	if k.Tracker == "github" {
		ops = append(ops, Op{
			Name: OpEnsureLabels, Transport: TransportGH,
			Condition: "tracker=github", Slug: cfg.Slug,
			Labels: RoutingLabels(),
		})
	}
	if k.CIMode == "actions" || k.CIMode == "both" {
		ops = append(ops, Op{
			Name: OpPushWorkflow, Transport: TransportConverge,
			Condition: "ci_mode in {actions, both}",
		})
	}
	if k.Wiring.Protection == "required-checks" || k.Wiring.Protection == "checks+review" {
		ops = append(ops, Op{
			Name: OpSetProtection, Transport: TransportGH,
			Condition: "wiring.protection != none", Slug: cfg.Slug,
			Branch:        branch,
			Contexts:      []string{RequiredCheckContext},
			RequireReview: k.Wiring.Protection == "checks+review",
		})
	}
	if k.Tracker == "github" {
		ops = append(ops, Op{
			Name: OpWelcomeItem, Transport: TransportGH,
			Condition: "tracker=github", Slug: cfg.Slug,
			Title: cfg.WelcomeTitle, Body: withWelcomeMarker(cfg.WelcomeBody),
		})
	}
	if k.Tracker == "github" && len(cfg.OnboardingItems) > 0 {
		// Declared LAST: the op also derives a finish-the-wiring item from
		// the walk's accumulated hand-offs, so every earlier row must have
		// run before it fires.
		items := make([]WorkItem, 0, len(cfg.OnboardingItems))
		for _, it := range cfg.OnboardingItems {
			it.Body = withMarker(it.Body, OnboardingMarker(it.Slug))
			items = append(items, it)
		}
		ops = append(ops, Op{
			Name: OpOnboardingItems, Transport: TransportGH,
			Condition: "tracker=github", Slug: cfg.Slug,
			Items: items,
		})
	}
	return ops
}

// withWelcomeMarker guarantees the welcome body carries the idempotency
// marker, appending it when absent — done at Table time so preview, audit,
// and hand-off all show the exact body that runs.
func withWelcomeMarker(body string) string {
	return withMarker(body, WelcomeMarker)
}

// withMarker guarantees body carries marker, appending it when absent.
func withMarker(body, marker string) string {
	if strings.Contains(body, marker) {
		return body
	}
	if body == "" {
		return marker
	}
	return strings.TrimRight(body, "\n") + "\n\n" + marker
}

// probeArgv is the op's read-only existence probe ("" ⇒ none). Probes are
// external operations too, so Preview lists them (prd:installer/FR-19
// enumerates every external operation, reads included).
func (o Op) probeArgv() []string {
	switch o.Name {
	case OpCreateRepo:
		return []string{"repo", "view", o.Slug, "--json", "name"}
	case OpEnsureLabels:
		return []string{"label", "list", "--repo", o.Slug, "--json", "name", "--limit", "100"}
	case OpWelcomeItem, OpOnboardingItems:
		return []string{"issue", "list", "--repo", o.Slug, "--state", "all", "--json", "number,body", "--limit", "100"}
	}
	return nil
}

// repoCreateArgv creates the repository. Private is the deliberate default:
// the least-exposure posture for a freshly initialized operand (the
// operator flips visibility later at will; §3.9 leg 3's bounded-authority
// stance).
func (o Op) repoCreateArgv() []string {
	return []string{"repo", "create", o.Slug, "--private"}
}

// labelCreateArgv creates one routing label. force renders the blind-run
// (ensure) form — `--force` updates an existing label instead of failing —
// used by preview and hand-off, which must be safe to paste without
// knowing the live label set; Execute's own creates target labels the list
// probe proved missing and omit it.
func (o Op) labelCreateArgv(l dialect.Label, force bool) []string {
	args := []string{"label", "create", l.Name, "--repo", o.Slug, "--color", l.Color, "--description", l.Description}
	if force {
		args = append(args, "--force")
	}
	return args
}

// protectionArgv is the one full-state branch-protection PUT — idempotent
// by nature (AP-22): the endpoint replaces the whole protection object, so
// re-running converges. The body travels entirely in argv (auditable,
// copy-paste-able, nothing on stdin): gh's `-F` typed fields carry the
// booleans/null/integers and the `[]`/`[k]` bracket forms carry the nested
// object — both require gh ≥ 2.27, part of the minimum-gh contract the
// design's open Q-2 pins at implementation.
//
// Levels (installer-v1 §3.6): required-checks → the docs-gate context with
// no review requirement (required_pull_request_reviews=null); checks+review
// → the same checks plus a 1-approval review requirement. strict=false,
// enforce_admins=false, restrictions=null in both — the minimal protection
// the chosen level names, nothing beyond it.
func (o Op) protectionArgv() []string {
	args := []string{
		"api", "--method", "PUT",
		"-H", "Accept: application/vnd.github+json",
		fmt.Sprintf("repos/%s/branches/%s/protection", o.Slug, o.Branch),
		"-F", "required_status_checks[strict]=false",
	}
	for _, c := range o.Contexts {
		args = append(args, "-f", "required_status_checks[contexts][]="+c)
	}
	args = append(args, "-F", "enforce_admins=false")
	if o.RequireReview {
		args = append(args, "-F", "required_pull_request_reviews[required_approving_review_count]=1")
	} else {
		args = append(args, "-F", "required_pull_request_reviews=null")
	}
	args = append(args, "-F", "restrictions=null")
	return args
}

// welcomeCreateArgv creates the welcome item, template-bodied content
// verbatim (Body already carries WelcomeMarker from Table).
func (o Op) welcomeCreateArgv() []string {
	return []string{"issue", "create", "--repo", o.Slug, "--title", o.Title, "--body", o.Body}
}

// itemCreateArgv creates one onboarding item (its Body already carries the
// item's OnboardingMarker).
func (o Op) itemCreateArgv(it WorkItem) []string {
	return []string{"issue", "create", "--repo", o.Slug, "--title", it.Title, "--body", it.Body}
}

// ensureCommands is the op's declared mutation set in blind-run-safe form —
// the ONE rendering preview and the hand-off block share (AP-11): label
// creates carry --force (paste-safe against any live label set), the
// protection PUT and issue/repo creates are their exact argv. Execute's
// runtime walk may narrow it (create only the labels the probe proved
// missing); the audit records that narrowed argv.
func (o Op) ensureCommands() [][]string {
	switch o.Name {
	case OpCreateRepo:
		return [][]string{o.repoCreateArgv()}
	case OpEnsureLabels:
		cmds := make([][]string, 0, len(o.Labels))
		for _, l := range o.Labels {
			cmds = append(cmds, o.labelCreateArgv(l, true))
		}
		return cmds
	case OpSetProtection:
		return [][]string{o.protectionArgv()}
	case OpWelcomeItem:
		return [][]string{o.welcomeCreateArgv()}
	case OpOnboardingItems:
		cmds := make([][]string, 0, len(o.Items))
		for _, it := range o.Items {
			cmds = append(cmds, o.itemCreateArgv(it))
		}
		return cmds
	}
	return nil
}

// reasonConvergeCarried is push-workflow's audited outcome reason.
const reasonConvergeCarried = "carried by the converge write set and the initial push — no remote operation from wire (installer-v1 §3.6; the shape's push-workflow row)"

// Preview renders the would-do set from the same table Execute walks — one
// entry per declared op, in table order (AP-11: one source; prd:installer/
// FR-19's external-operation enumeration). Each gh entry lists the probe
// and the ensure-form commands exactly as the hand-off block would render
// them; the converge entry states that no remote operation runs.
func Preview(ops []Op) []string {
	out := make([]string, 0, len(ops))
	for _, op := range ops {
		out = append(out, previewEntry(op))
	}
	return out
}

func previewEntry(op Op) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s (%s; transport %s)", op.Name, op.Condition, op.Transport)
	if op.Transport == TransportForgejo {
		fmt.Fprintf(&b, " — %s repository %s; credential environment %s", op.Connection.BaseURL, op.Slug, op.Connection.TokenEnv)
		return b.String()
	}
	if op.Transport == TransportConverge {
		b.WriteString(" — " + reasonConvergeCarried)
		return b.String()
	}
	if p := op.probeArgv(); p != nil {
		fmt.Fprintf(&b, "\n  probe: %s", shellJoin(p))
	}
	for _, c := range op.ensureCommands() {
		fmt.Fprintf(&b, "\n  %s", shellJoin(c))
	}
	return b.String()
}

// shellJoin renders a gh argv as the exact copy-paste command line the
// hand-off block promises (UB-4): `gh` plus each argument, quoted only
// where the shell demands it.
func shellJoin(args []string) string {
	var b strings.Builder
	b.WriteString("gh")
	for _, a := range args {
		b.WriteByte(' ')
		b.WriteString(shellQuote(a))
	}
	return b.String()
}

// shellQuote single-quotes an argument unless it is entirely
// shell-neutral; an embedded single quote is spliced the standard way
// (close quote, escaped quote, reopen quote). Newlines survive inside
// single quotes, so multi-line issue bodies stay one pasteable command.
func shellQuote(s string) string {
	if s != "" && strings.IndexFunc(s, func(r rune) bool { return !isShellSafe(r) }) < 0 {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func isShellSafe(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	}
	return strings.ContainsRune("@%_+=:,./-", r)
}
