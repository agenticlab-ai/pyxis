package boundary

import (
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

var deliveryFieldRowRe = regexp.MustCompile(`^\|\s*\*\*([^*|]+?)\*\*\s*\|\s*((?:\\.|[^|])*)\|\s*$`)
var deliveryTableHeadingRe = regexp.MustCompile(`^\|\s*Field\s*\|\s*Value\s*\|\s*$`)
var deliveryTableSeparatorRe = regexp.MustCompile(`^\|\s*:?-+:?\s*\|\s*:?-+:?\s*\|\s*$`)

// validDeliveryScopeShape checks the header and derived-state structure that
// ParseAs deliberately leaves to the structure arm. Region delimitation alone
// is insufficient evidence for granting committed delivery scope. This guard
// reads only live prose, binds the closed header to the driving slug, and
// requires the template's ordered regions and five nonempty derived fields. The derived values
// remain descriptive; their agreement with the journal does not confer a
// permission. The journal must independently pass its typed-entry shape check
// (argus-artifacts §3.2, DINV-6).
func validDeliveryScopeShape(slug string, committed []byte, ledger *argusledger.Ledger) bool {
	header := map[string]string{}
	derived := map[string]string{}
	regions := map[string]bool{}
	tableFrames := map[string]int{}
	section := ""
	for _, raw := range argusledger.ProseLines(schema.Normalize(committed)) {
		line := strings.TrimSpace(raw)
		if m := schema.LedgerRegionRe.FindStringSubmatch(raw); m != nil {
			section = m[1]
			if (section != "Derived state" && section != "Journal") || regions[section] ||
				(section == "Journal" && !regions["Derived state"]) {
				return false
			}
			regions[section] = true
			continue
		}
		if section == "Journal" || !strings.HasPrefix(line, "|") {
			continue
		}
		if deliveryTableHeadingRe.MatchString(line) {
			if tableFrames[section] != 0 {
				return false
			}
			tableFrames[section] = 1
			continue
		}
		if deliveryTableSeparatorRe.MatchString(line) {
			if tableFrames[section] != 1 {
				return false
			}
			tableFrames[section] = 2
			continue
		}
		m := deliveryFieldRowRe.FindStringSubmatch(line)
		if m == nil || tableFrames[section] != 2 {
			return false
		}
		label, value := strings.TrimSpace(m[1]), strings.TrimSpace(m[2])
		fields := derived
		if section == "" {
			fields = header
			if label != "Slug" && label != "Template version" {
				return false
			}
		} else {
			switch label {
			case "Status", "Stage position", "Open items", "Park state", "Cumulative spend":
			default:
				return false
			}
		}
		if _, duplicate := fields[label]; duplicate || value == "" {
			return false
		}
		fields[label] = value
	}
	if tableFrames[""] != 2 || tableFrames["Derived state"] != 2 ||
		len(header) != 2 || header["Slug"] != slug || header["Template version"] != schema.DeliveryLedgerVersion || len(derived) != 5 {
		return false
	}
	switch derived["Status"] {
	case "open", "parked", "delivered":
		return regions["Derived state"] && regions["Journal"] && validDeliveryJournal(ledger)
	default:
		return false
	}
}
