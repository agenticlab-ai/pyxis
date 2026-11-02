package argusledger

import (
	"crypto/sha256"
	"fmt"
	"path"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// TypeCorrectionVersion is a closed gist carried by a reconciliation event,
// implementing argus-artifacts §3.6 without adding an event type or field.
const TypeCorrectionVersion = "journal-type-correction/v1"

var correctionTargetRe = regexp.MustCompile(`^delivery:([a-z0-9][a-z0-9-]*)#(E-[1-9][0-9]*)$`)
var correctionDigestRe = regexp.MustCompile(`^[0-9a-f]{64}$`)
var correctionFromRe = regexp.MustCompile(`^[a-z][a-z-]*$`)
var correctionBodyRe = regexp.MustCompile(`^-\s+\*\*Body\*\*\s*:\s*(.*?)\s*$`)

// TypeCorrection binds a canonical classification to one unchanged earlier
// entry. SHA256 hashes the exact RawEntries block (not normalized Markdown).
type TypeCorrection struct {
	Target string
	SHA256 string
	From   string
	To     string
}

func (c TypeCorrection) Render() string {
	return fmt.Sprintf("%s; target=%s; sha256=%s; from=%s; to=%s", TypeCorrectionVersion, c.Target, c.SHA256, c.From, c.To)
}

// EntryDigest deliberately refuses synthesized ledgers lacking raw bytes.
func EntryDigest(l *Ledger, id string) (string, error) {
	if l == nil || len(l.RawEntries[id]) == 0 {
		return "", fmt.Errorf("entry %s has no original bytes", id)
	}
	return fmt.Sprintf("%x", sha256.Sum256(l.RawEntries[id])), nil
}

func parseTypeCorrection(gist string) (TypeCorrection, error) {
	var c TypeCorrection
	parts := strings.Split(gist, ";")
	if len(parts) != 5 || strings.TrimSpace(parts[0]) != TypeCorrectionVersion {
		return c, fmt.Errorf("want %s and exactly four key=value fields", TypeCorrectionVersion)
	}
	seen := map[string]bool{}
	for _, part := range parts[1:] {
		key, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || value == "" || seen[key] {
			return c, fmt.Errorf("malformed or duplicate correction field %q", key)
		}
		seen[key] = true
		switch key {
		case "target":
			c.Target = value
		case "sha256":
			c.SHA256 = value
		case "from":
			c.From = value
		case "to":
			c.To = value
		default:
			return c, fmt.Errorf("unknown correction field %q", key)
		}
	}
	if !correctionTargetRe.MatchString(c.Target) || !correctionDigestRe.MatchString(c.SHA256) ||
		!correctionFromRe.MatchString(c.From) || KnownEntryType(c.From) || !KnownEntryType(c.To) {
		return c, fmt.Errorf("correction requires a same-ledger E-n target, lowercase SHA256, unsupported from type and canonical to type")
	}
	return c, nil
}

// EffectiveEntryTypes reads the complete journal before assigning any types.
// An invalid, duplicate or unresolved correction produces defects and no map.
// Original metadata and effective type-specific obligations remain the typed
// journal validator's responsibility; a correction cannot rewrite them.
func EffectiveEntryTypes(verb string, l *Ledger) (map[string]string, []schema.Defect) {
	if l == nil {
		return nil, []schema.Defect{def(verb, "delivery.md", "journal-type", "missing delivery ledger", "read the actual delivery ledger")}
	}
	bad := func(id, reason string) []schema.Defect {
		return []schema.Defect{def(verb, l.Path, "journal-type", id+": "+reason,
			"keep merged entries unchanged; append a valid reconciliation with a "+TypeCorrectionVersion+" gist using docgate correct-journal")}
	}
	types := make(map[string]string, len(l.Order))
	positions := make(map[string]int, len(l.Order))
	for i, id := range l.Order {
		types[id] = EntryType(l.Entries[id])
		positions[id] = i
	}
	corrected := map[string]bool{}
	slug := path.Base(path.Dir(l.Path))
	for i, id := range l.Order {
		var gist string
		for _, line := range ProseLines(l.Entries[id]) {
			if m := correctionBodyRe.FindStringSubmatch(line); m != nil && strings.HasPrefix(m[1], "journal-type-correction/") {
				if gist != "" {
					return nil, bad(id, "duplicate correction body")
				}
				gist = m[1]
			}
		}
		if gist == "" {
			continue
		}
		if EntryType(l.Entries[id]) != TypeReconciliation {
			return nil, bad(id, "a type correction must be a reconciliation event")
		}
		c, err := parseTypeCorrection(gist)
		if err != nil {
			return nil, bad(id, err.Error())
		}
		match := correctionTargetRe.FindStringSubmatch(c.Target)
		target := match[2]
		position, exists := positions[target]
		if match[1] != slug || !exists || position >= i {
			return nil, bad(id, "correction target must be an earlier event in this ledger")
		}
		if corrected[target] {
			return nil, bad(id, "duplicate or conflicting correction for "+target)
		}
		digest, err := EntryDigest(l, target)
		if err != nil || digest != c.SHA256 || EntryType(l.Entries[target]) != c.From {
			return nil, bad(id, "correction digest/from does not match the original "+target)
		}
		corrected[target] = true
		types[target] = c.To
	}
	for _, id := range l.Order {
		if !KnownEntryType(types[id]) {
			return nil, bad(id, fmt.Sprintf("unsupported event type %q remains unresolved", types[id]))
		}
	}
	return types, nil
}
