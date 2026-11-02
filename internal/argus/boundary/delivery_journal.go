package boundary

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
)

var deliveryEntryHeadingRe = regexp.MustCompile(`^###\s+E-[1-9][0-9]*\s+—\s+[a-z][a-z-]*\s+—\s+\S.*$`)
var deliveryEntryFieldRe = regexp.MustCompile(`^-\s+\*\*([^*]+)\*\*\s*:\s*(.*?)\s*$`)
var deliveryAuthoritySeparatorRe = regexp.MustCompile(`\s*·\s*\*\*Authority-contract version\*\*\s*:\s*`)
var deliveryAuthorityVersionRe = regexp.MustCompile(`^authority-contract/v[1-9][0-9]*(?:$|\s|[·,;])`)
var deliveryClassRe = regexp.MustCompile(`\bclass\s*[:=]\s*(override|deadlock)\b`)
var deliveryAnswerRe = regexp.MustCompile(`\bA-[1-9][0-9]*\b`)
var deliveryTouchesRe = regexp.MustCompile(`(?i)\b(?:touch count|touches)\s*[:=]\s*([0-9]+)\b`)
var deliveryBreakingRe = regexp.MustCompile(`(?i)\b(?:breaking count|breaking)\s*[:=]\s*([0-9]+)\b`)
var deliverySpendRe = regexp.MustCompile(`(?i)\bspend\s*[:=]\s*\S+`)

// validDeliveryJournal checks the template's nonempty, dense typed journal and
// required entry fields before it can confer scope. Event-specific properties
// may ride the gist or labeled lines; their truth and external goal-object
// verification remain the owning drive's responsibility (argus-artifacts §3.2).
func validDeliveryJournal(ledger *argusledger.Ledger) bool {
	if len(ledger.Order) == 0 {
		return false
	}
	types, defects := argusledger.EffectiveEntryTypes("boundary", ledger)
	if len(defects) > 0 {
		return false
	}
	for _, line := range argusledger.ProseLines(ledger.EntryRegion.Bytes) {
		if strings.HasPrefix(line, "###") && !deliveryEntryHeadingRe.MatchString(line) {
			return false
		}
	}
	for i, id := range ledger.Order {
		if id != fmt.Sprintf("E-%d", i+1) {
			return false
		}
		entry := ledger.Entries[id]
		kind := types[id]
		fields, prose, ok := deliveryEntryFields(entry)
		if !ok {
			return false
		}
		for _, label := range []string{"Date", "Principal", "Authority-contract version", "Body"} {
			if fields[label] == "" {
				return false
			}
		}
		if !deliveryIdentityPresent(fields["Principal"]) {
			return false
		}
		if effect, present := fields["External effect"]; present && effect == "" {
			return false
		}
		if _, err := time.Parse(time.DateOnly, fields["Date"]); err != nil || !deliveryAuthorityVersionRe.MatchString(fields["Authority-contract version"]) {
			return false
		}
		switch kind {
		case argusledger.TypeItemMaterialized, argusledger.TypeBumpIssued:
			// These acts necessarily address an external tracker/proposal object.
			// Journal-only events may omit the field; the drive verifies any
			// external effects of those events against their actual goal objects.
			if !deliveryIdentityPresent(fields["External effect"]) {
				return false
			}
		case argusledger.TypeBriefEmitted:
			if class, present := fields["Class"]; (present && class != "override" && class != "deadlock") || (!present && !deliveryClassRe.MatchString(fields["Body"])) {
				return false
			}
		case argusledger.TypeAnswerReceived:
			if !deliveryAnswerRe.MatchString(fields["Body"]) {
				return false
			}
		case argusledger.TypeDelivered:
			body := strings.ToLower(fields["Body"])
			countsValid := deliveryCount(fields["Touch count"], prose, deliveryTouchesRe) && deliveryCount(fields["Breaking count"], prose, deliveryBreakingRe)
			spendPresent := fields["Spend"] != "" || deliverySpendRe.MatchString(fields["Body"])
			verificationPresent := fields["Mainline verification"] != "" || (strings.Contains(body, "mainline") && strings.Contains(body, "verif"))
			if i != len(ledger.Order)-1 || !countsValid || !spendPresent || !verificationPresent {
				return false
			}
		}
	}
	return true
}

func deliveryIdentityPresent(value string) bool {
	// The template permits a no-effect placeholder for journal-only events.
	// It cannot stand in for a principal or a necessarily external act.
	value = strings.ToLower(strings.TrimSpace(strings.Trim(strings.TrimSpace(value), "`")))
	if value == "" {
		return false
	}
	for _, sentinel := range []string{"—", "–", "-", "none", "n/a", "not applicable"} {
		suffix, matches := strings.CutPrefix(value, sentinel)
		if !matches {
			continue
		}
		if suffix == "" {
			return false
		}
		first, _ := utf8.DecodeRuneInString(suffix)
		if unicode.IsSpace(first) || strings.ContainsRune("`([{:,;.", first) {
			return false
		}
	}
	return true
}

func deliveryEntryFields(entry []byte) (map[string]string, string, bool) {
	fields := map[string]string{}
	var prose strings.Builder
	put := func(label, value string) bool {
		if _, duplicate := fields[label]; duplicate {
			return false
		}
		fields[label] = strings.TrimSpace(value)
		return true
	}
	for _, line := range argusledger.ProseLines(entry) {
		prose.WriteString(line)
		prose.WriteByte('\n')
		m := deliveryEntryFieldRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		label, value := strings.TrimSpace(m[1]), m[2]
		switch label {
		case "Principal":
			parts := deliveryAuthoritySeparatorRe.Split(value, -1)
			if len(parts) > 2 || !put(label, parts[0]) || (len(parts) == 2 && !put("Authority-contract version", parts[1])) {
				return nil, "", false
			}
		case "Date", "Authority-contract version", "External effect", "Body", "Class", "Touch count", "Breaking count", "Spend", "Mainline verification":
			if !put(label, value) {
				return nil, "", false
			}
		}
	}
	return fields, prose.String(), true
}

func deliveryCount(value, prose string, pattern *regexp.Regexp) bool {
	if value == "" {
		if match := pattern.FindStringSubmatch(prose); match != nil {
			value = match[1]
		}
	}
	_, err := strconv.ParseUint(value, 10, 64)
	return err == nil
}
