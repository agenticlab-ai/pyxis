package adopt

import (
	"fmt"
	"strings"
)

// unknownStackLine is the fixed degrade-to-unknown wording (design §3.6,
// prd:existing-repo-onboarding/UB-3): a repository no marker matches names
// this, never an error and never a fabricated command.
const unknownStackLine = "unknown — set your gate commands"

// Render is the plan's human face — a deterministic Markdown rendering of
// the SAME fields the machine face carries (Encode), for the reviewer who
// approves apply and the operator who reads an applied receipt (design
// §3.2: "rendered human-readable for review"). Both faces derive from the
// one Plan, so they can never diverge. Deterministic: no clock, and
// entries/paths render in the plan's own order — Render sorts nothing
// itself, it only formats what the producer already ordered.
func (p Plan) Render() string {
	var b strings.Builder

	fmt.Fprintf(&b, "# Adoption plan — %s\n\n", p.Status)
	fmt.Fprintf(&b, "- **Schema:** %s\n", p.SchemaVersion)
	fmt.Fprintf(&b, "- **Status:** %s\n", p.Status)
	epoch := p.Epoch
	if epoch == "" {
		epoch = "(not yet applied)"
	}
	fmt.Fprintf(&b, "- **Epoch:** %s\n", epoch)
	b.WriteString("\n")

	b.WriteString("## Entries\n\n")
	if len(p.Entries) == 0 {
		b.WriteString("(none)\n\n")
	} else {
		for _, e := range p.Entries {
			renderEntry(&b, e)
		}
		b.WriteString("\n")
	}

	b.WriteString("## Hooks\n\n")
	prior := p.Hooks.PriorHooksPath
	if prior == "" {
		prior = "(none displaced)"
	}
	fmt.Fprintf(&b, "- **Displaced prior hooks path:** %s\n\n", prior)

	b.WriteString("## Stack\n\n")
	if len(p.Stack.Detected) == 0 {
		fmt.Fprintf(&b, "- **Detected:** %s\n", unknownStackLine)
	} else {
		fmt.Fprintf(&b, "- **Detected:** %s\n", strings.Join(p.Stack.Detected, ", "))
	}
	if len(p.Stack.GateActions) == 0 {
		b.WriteString("- **Required gate actions:** (none)\n")
	} else {
		fmt.Fprintf(&b, "- **Required gate actions:** %s\n", strings.Join(p.Stack.GateActions, ", "))
	}
	b.WriteString("\n")

	b.WriteString("## Added paths (undo baseline)\n\n")
	if len(p.AddedPaths) == 0 {
		b.WriteString("(none)\n")
	} else {
		for _, a := range p.AddedPaths {
			fmt.Fprintf(&b, "- `%s` — sha256:%s\n", a.Path, a.SHA256)
		}
	}

	return b.String()
}

// renderEntry writes one entry as a bulleted `path — class` line. A
// single-line Detail renders inline after the class; a multi-line Detail
// (the common shape of a ClassCollideManual entry's exact lines to add,
// design §3.3) renders as an indented fenced block on the lines that
// follow, so arbitrary reviewed content — including a fenced block of its
// own — always embeds safely without markdown escaping.
func renderEntry(b *strings.Builder, e Entry) {
	fmt.Fprintf(b, "- `%s` — %s\n", e.Path, e.Class)
	if e.Detail == "" {
		return
	}
	if !strings.Contains(e.Detail, "\n") {
		fmt.Fprintf(b, "  %s\n", e.Detail)
		return
	}
	fence := codeFence(e.Detail)
	fmt.Fprintf(b, "  %s\n", fence)
	for _, line := range strings.Split(e.Detail, "\n") {
		if line == "" {
			b.WriteString("\n")
			continue
		}
		fmt.Fprintf(b, "  %s\n", line)
	}
	fmt.Fprintf(b, "  %s\n", fence)
}

// codeFence returns a backtick fence at least three symbols long and one
// symbol longer than the longest backtick run in s, so s always embeds
// safely inside it per CommonMark's fenced-code-block rule.
func codeFence(s string) string {
	longest, run := 0, 0
	for _, r := range s {
		if r == '`' {
			run++
			if run > longest {
				longest = run
			}
		} else {
			run = 0
		}
	}
	n := longest + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("`", n)
}
