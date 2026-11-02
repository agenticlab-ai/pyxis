package gate

import (
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/authoritycontract"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The Argus authority-contract structure arm (docs/tech-design/
// argus-authority-contract-structure-arm.md §3.3): a base-free, self-registered
// check that parses the one authority-contract file against version-pinned
// closed vocabularies and reports a named defect on any DINV-3/4/5 violation —
// attribution present, current-tier-in-ladder with no bare pointer flip, and
// action-class token-membership. A malformed contract (wrong/absent version or
// an undelimitable region) is a named parse defect, never a crash or false
// pass (DINV-2). Report-only; base-free (it reads the file as it stands and
// needs no merge base — DINV-5), so it runs identically on the desk and CI
// paths. It governs one fixed file, so a tree without that file draws nothing
// and every non-authority-contract tree's verdict is byte-identical (DINV-4).
func init() {
	Register(Check{
		Verb:             authorityContractVerb,
		Help:             "the Argus authority contract is structurally legal (tier ladder, attribution, token-membership)",
		NeedsMergeBase:   false,
		RunOnPyxisSource: true,
		Run:              func(c Ctx) Defects { return CheckAuthorityContract(c.Root) },
	})
}

const authorityContractVerb = "check-authority-contract"

// CheckAuthorityContract parses the authority contract at its fixed home and
// evaluates DINV-3/4/5 over it. A tree without the file has nothing to govern
// (nil defects); a malformed file is a named parse defect (never a crash).
func CheckAuthorityContract(root string) []schema.Defect {
	rel := schema.AuthorityContractPath
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		// No authority contract in this tree — nothing for the arm to govern.
		return nil
	}
	c, pd := authoritycontract.Parse(authorityContractVerb, rel, b)
	if c == nil {
		return pd // a named parse defect (wrong/absent version, undelimitable region)
	}
	return authoritycontract.Evaluate(authorityContractVerb, rel, c)
}
