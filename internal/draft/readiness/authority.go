package readiness

import (
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/authoritycontract"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
)

// rulingAuthority checks declared authorship. The operator's later approval
// of the exact lock request vouches for that declaration; this is not identity
// authentication.
type rulingAuthority struct {
	refused map[string]bool // problem IDs
	unmet   []Unmet
}

func checkRulingAuthority(result compile.Result, sources Sources) rulingAuthority {
	state := rulingAuthority{refused: map[string]bool{}}
	argusGranted := argusMayAcceptRules(sources)
	for _, problem := range result.Problems {
		if problem.Ruling == nil {
			continue
		}
		author := strings.ToLower(strings.TrimSpace(problem.Ruling.Author))
		if operatorAuthor(author, sources.OperatorLogin) || author == "argus" && argusGranted {
			continue
		}
		state.refused[problem.ID] = true
		remedy := "record an operator ruling"
		if author == "argus" {
			remedy = "record an operator ruling or grant Argus rule-acceptance in the mainline authority contract"
		}
		state.unmet = append(state.unmet, Unmet{
			Condition: "FR-10", Item: "prd/" + result.Feature + "#" + problem.ID,
			Remedy: "ruling by " + problem.Ruling.Author + " lacks authority; " + remedy,
		})
	}
	return state
}

// operatorAuthor recognizes the "operator" role and the repository's configured
// operator login. author is already lower-cased; GitHub logins compare
// case-insensitively.
func operatorAuthor(author, operatorLogin string) bool {
	return author == "operator" || operatorLogin != "" && author == "@"+strings.ToLower(operatorLogin)
}

func argusMayAcceptRules(sources Sources) bool {
	body := sources.Documents[schema.AuthorityContractPath]
	if len(body) == 0 {
		return false
	}
	contract, parseDefects := authoritycontract.Parse("draft-ready", schema.AuthorityContractPath, body)
	if len(parseDefects) > 0 || len(authoritycontract.Evaluate("draft-ready", schema.AuthorityContractPath, contract)) > 0 {
		return false
	}
	if contract.CurrentTier == "revoked" {
		return false
	}
	rows := make(map[string][]string, len(contract.Ladder))
	for _, row := range contract.Ladder {
		rows[row.Tier] = row.Tokens
	}
	for _, tier := range contract.Class.TierOrder {
		if slices.Contains(rows[tier], "rule-acceptance") {
			return true
		}
		if tier == contract.CurrentTier {
			break
		}
	}
	return false
}

// AuthorizedChanges is the approval-request view of requirement edits. It
// preserves every edit but marks one paired with a refused ruling as unruled.
// The compiler's raw result retains the declared ruling as evidence.
func AuthorizedChanges(result compile.Result, sources Sources) []compile.RequirementChange {
	refused := checkRulingAuthority(result, sources).refused
	changes := append([]compile.RequirementChange(nil), result.Changes...)
	for i := range changes {
		if changes[i].Ruling != nil && refused[changes[i].Ruling.Problem] {
			changes[i].Unruled = true
			changes[i].Ruling = nil
		}
	}
	return changes
}

// A decline carries a declared operator ruling in the same author · reason
// form as a requirement ruling: "declined — operator · <reason>". Review
// authors may not close a blocking finding by merely describing a reason.
func disposed(s, operatorLogin string) bool {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "applied" {
		return true
	}
	rest, ok := strings.CutPrefix(s, "declined — ")
	if !ok {
		return false
	}
	author, reason, ok := strings.Cut(rest, " · ")
	return ok && operatorAuthor(strings.TrimSpace(author), operatorLogin) && substantive(reason)
}
