package installer

// The init-created registry content and welcome constants the tail seeds
// (installer-v1 §3.5 steps 4 + 9, record D-9: the registries are init-created,
// never shipped templates — a shipped template would be engine-verbatim, but
// these are repo-owned from birth so the operand grows them). Kept beside the
// tail stages that write them.

import (
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// emptyIncidentsRegistry is the empty-but-valid incident registry Seed writes
// at docs/observability/incidents.md (record D-9). The format legend sits in a
// fenced block so lint-incidents sees zero entries (the example is not a real
// `**I<n> …**` head outside a fence). This is the fresh shape the gate admits;
// entries append only through the coded writer (`pyxis exec docgate --
// add-incident`). It matches the seeded shape the payload conformance test
// (internal/payload) fixes as the operand's first registry.
const emptyIncidentsRegistry = "# Incidents registry\n" + `
The repository's paid-for lessons, one bounded entry each — the staging
ground principles graduate from (see ` + "`docs/architecture-principles.md`" + `).

## Format / legend

` + "```" + `
**I<n> (YYYY-MM-DD) [tag, tag, …]**

- **Decision:** what was decided / built.
- **Broke:** how it broke (the observed failure).
- **Lesson:** the distilled takeaway — what to do instead.
- **Links:** provenance; once graduated, Graduated: AP-<n>.
` + "```" + `

Entries append through the coded writer only; IDs run dense from 1 and are
never renumbered; a merged entry is immutable (its Links line may grow at
graduation).

## Entries
`

// welcomeItemTitle is the welcome work item's title (§3.5 step 9). Stable so
// the wire welcome-item op and the carina store agree.
const welcomeItemTitle = "Welcome — your factory is installed and green"

// WelcomeBanner heads the terminal epilogue (§3.5 step 9) — exported because
// the cmd report tail prints it as the run's closing block (#937); the item
// body carries the full guidance.
const WelcomeBanner = "pyxis init complete — your factory is seeded, gated, and green."

// codeGatePendingLine names the code gate's pending status and its enabling
// step (prd:init-hardening/FR-14, design init-hardening-v1 §3.9, #D-6): a
// fresh install never carries a verification manifest, so this line is
// knob-independent (unlike gatesActiveLine) and always true immediately
// after init. It routes to the codegate seed verb
// (prd:implementation-verification amendment 001, FR-20), never seeds.
const codeGatePendingLine = "pending: the code gate — once this repo holds code, run `pyxis exec codegate -- seed` to write its template manifest (yours to own)"

// OnboardingWorkItems is the standing onboarding set init files into the
// tracker (#947): the setup only the operator can complete, one item per
// action, paths resolved from the repo config (config-driven portability —
// an adopter's relocated governing docs are named at their real locations).
// The fourth, conditional item — finish-the-wiring — is derived from the
// walk's hand-offs, not authored here (wire.WiringRemainderItem).
func OnboardingWorkItems(cfg config.Config) []wire.WorkItem {
	return []wire.WorkItem{
		{
			Slug:  "author-product-vision",
			Title: "Author your product vision",
			Body: "Your factory is installed, but its north star is still a skeleton: `" + cfg.ProductVision + "`.\n\n" +
				"This document is yours alone to write — no tool will ever author it, by design. Every pipeline stage downstream judges work against it, and `pyxis doctor` lists it as unauthored until real content follows its authoring line.\n\n" +
				"Append below the authoring line inside the file (it is append-only from birth).\n\n" +
				"Close this item when the vision says what this product is for and what winning looks like.",
		},
		{
			Slug:  "author-architecture-principles",
			Title: "Author your architecture principles",
			Body: "The citable rules your designs will be judged against are still a skeleton: `" + cfg.ArchitecturePrinciples + "`.\n\n" +
				"Like the vision, this is yours alone to write. Principles carry permanent IDs; every technical design cites the ones it honors or deviates from, so the earlier they exist the more work they govern.\n\n" +
				"Append below the authoring line inside the file (append-only from birth).\n\n" +
				"Close this item when the first principles your designs must answer to are written.",
		},
		{
			Slug:  "seed-the-code-gate",
			Title: "Seed the code gate once this repository holds code",
			Body: "No verification manifest exists yet, so a task's code changes cannot clear the code gate until one does.\n\n" +
				"When the first real code lands, run:\n\n" +
				"```sh\npyxis exec codegate -- seed\n```\n\n" +
				"It detects your stack from marker files and writes a template manifest for you to own — real default commands where your stack ships them, named placeholders to edit otherwise. `pyxis doctor` names this gap whenever recognized code exists without the manifest.\n\n" +
				"Close this item when `.pyxis/codegate.json` is committed with your real verb commands.",
		},
	}
}
