package payload

import (
	"fmt"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/authoritycontract"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// InitialAuthorityContract instantiates the shipped authoring template at its
// documented birth tier (installer-v1 §3.5 step 4). The ladder comes from that
// template, never the Source repository's live policy or promotion history.
// The result becomes repository-owned; only init calls this function.
func InitialAuthorityContract(template []byte, date string) ([]byte, error) {
	if _, err := time.Parse("2006-01-02", date); err != nil {
		return nil, fmt.Errorf("authority seed: invalid birth date: %w", err)
	}
	const birth = "| <YYYY-MM-DD> | none | <initial tier> | born at <tier> — the introducing PR |"
	const pointer = "| **Current tier** | <a tier name declared in the ladder below> |"
	body := string(template)
	for _, field := range []string{"<repo>", pointer, birth} {
		if strings.Count(body, field) != 1 {
			return nil, fmt.Errorf("authority seed: template must contain exactly one %q", field)
		}
	}
	tier := schema.AuthorityContractV1.InitialTier
	body = strings.Replace(body, "<repo>", "this repository", 1)
	body = strings.Replace(body, pointer, "| **Current tier** | "+tier+" |", 1)
	body = strings.Replace(body, birth, "| "+date+" | none | "+tier+" | born at "+tier+" — pyxis init with knobs.argus=true; installer-v1 §3.5 step 4 |", 1)
	// The template's standalone guidance paragraphs and structure checklist
	// guide human instantiation; neither belongs to the instantiated policy.
	var lines []string
	for line := range strings.SplitSeq(body, "\n") {
		if strings.HasPrefix(line, "## Appendix") {
			break
		}
		if strings.HasPrefix(line, "*") && strings.HasSuffix(line, "*") {
			continue
		}
		lines = append(lines, line)
	}
	result := []byte(strings.TrimSpace(strings.Join(lines, "\n")) + "\n")
	parsed, defects := authoritycontract.Parse("seed", AuthorityContractTarget, result)
	if len(defects) == 0 {
		defects = authoritycontract.Evaluate("seed", AuthorityContractTarget, parsed)
	}
	if len(defects) != 0 {
		return nil, fmt.Errorf("authority seed: invalid instantiated contract: %s", defects[0].Reason)
	}
	return result, nil
}
