// Review-wave checks (#54; docs/tech-design/docgate-fence-completion.md,
// serves-hardening.md, doc-link-integrity.md): the governing purity scan,
// the gate-policy key guard, system-map registry stability, and the
// repo-wide relative-link checker. Same discipline as every verb: report
// only, fail closed, bytes and git history — never judgment.
package gate

import (
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// mapAnchorRe matches a registry anchor opening a table row: | INV-33 | or | D7 |.
var mapAnchorRe = regexp.MustCompile(`^\|\s*(INV-[0-9]+|D[0-9]+)\s*\|`)

func mapAnchors(b []byte) map[string]bool {
	anchors := map[string]bool{}
	fenced := false
	for _, l := range strings.Split(string(schema.Normalize(b)), "\n") {
		if strings.HasPrefix(strings.TrimSpace(l), "```") {
			fenced = !fenced
			continue
		}
		if fenced {
			continue
		}
		if m := mapAnchorRe.FindStringSubmatch(l); m != nil {
			anchors[m[1]] = true
		}
	}
	return anchors
}

var (
	inlineLinkRe = regexp.MustCompile(`\]\(([^)\s]+)(?:\s+"[^"]*")?\)`)
	refDefRe     = regexp.MustCompile(`^\s*\[[^\]]+\]:\s+(\S+)`)
	codeSpanRe   = regexp.MustCompile("`[^`]*`")
	schemeRe     = regexp.MustCompile(`^[a-zA-Z][a-zA-Z0-9+.-]*:`)
)

func stripCodeSpans(l string) string { return codeSpanRe.ReplaceAllString(l, "") }

func isExternal(target string) bool { return schemeRe.MatchString(target) }

func linkTargets(line string) []string {
	var out []string
	for _, m := range inlineLinkRe.FindAllStringSubmatch(line, -1) {
		out = append(out, m[1])
	}
	if m := refDefRe.FindStringSubmatch(line); m != nil {
		out = append(out, m[1])
	}
	return out
}

func equalList(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
