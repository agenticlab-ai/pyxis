package efficiency

import (
	"fmt"
	"os"
	"regexp"
)

// ledgerEntryRE matches a PRD ledger file's dense-numbered name —
// "<NNN>-<slug>.md" (docs/tech-design/prd-amendment-model.md §3): "000"
// is the immutable base, every other number an amendment or a rebase
// checkpoint. The generated projection (effective.md,
// effective.source.json) carries no numeric prefix and never matches.
var ledgerEntryRE = regexp.MustCompile(`^(\d{3})-.+\.md$`)

// AmendmentCount counts a PRD ledger directory's entries after 000-base —
// amendments and rebase checkpoints alike, each one instance of the PRD
// needing to change after approval (design §3.10: "rework from ...
// amendment count"). A ledger that does not exist yet (the phase hasn't
// been amended) counts as zero, not an error — a corpus artifact absent
// reads as no signal, never a fault (prd:pipeline-measurement §5.5).
func AmendmentCount(ledgerDir string) (int, error) {
	entries, err := os.ReadDir(ledgerDir)
	if err != nil {
		if os.IsNotExist(err) {
			return 0, nil
		}
		return 0, fmt.Errorf("amendment count %s: %w", ledgerDir, err)
	}
	n := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		m := ledgerEntryRE.FindStringSubmatch(e.Name())
		if m == nil {
			continue
		}
		if m[1] == "000" {
			continue
		}
		n++
	}
	return n, nil
}
