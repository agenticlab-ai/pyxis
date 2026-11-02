package compile

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

var decisionHeading = regexp.MustCompile(`^### (D-[1-9][0-9]*) — (.+)$`)
var decisionRef = regexp.MustCompile(`^D-[1-9][0-9]*$`)

func compileTechSpec(feature string, body []byte) ([]Record, error) {
	var out []Record
	seen := map[string]bool{}
	var id, statement, serves, buildsOn, supersedes string
	flush := func() error {
		if id == "" {
			return nil
		}
		if statement == "" {
			return fmt.Errorf("compile draft %s tech spec %s: Decision line is missing", feature, id)
		}
		entry := Record{Draft: reconcile.Draft{
			Kind: record.Decision, Subject: "design/" + feature,
			Family: id, Statement: statement,
		}}
		linked := map[string]bool{}
		for raw := range strings.SplitSeq(serves, " · ") {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			target, ok := servedTarget(feature, raw)
			if !ok || linked[target] {
				return fmt.Errorf("compile draft %s tech spec %s: invalid or repeated Serves ID %q", feature, id, raw)
			}
			linked[target] = true
			entry.Links = append(entry.Links, Link{Type: record.Serves, Target: target})
		}
		for _, field := range []struct{ value, relation string }{{buildsOn, record.DependsOn}, {supersedes, record.Supersedes}} {
			linked = map[string]bool{}
			for raw := range strings.SplitSeq(field.value, " · ") {
				raw = strings.TrimSpace(raw)
				if raw == "" {
					continue
				}
				target, ok := decisionTarget(feature, raw)
				if !ok || linked[target] || field.relation == record.Supersedes && !strings.HasPrefix(target, "design/"+feature+"#") {
					return fmt.Errorf("compile draft %s tech spec %s: invalid or repeated %s target %q", feature, id, field.relation, raw)
				}
				linked[target] = true
				entry.Links = append(entry.Links, Link{Type: field.relation, Target: target})
			}
		}
		out = append(out, entry)
		return nil
	}
	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.HasPrefix(line, "### ") {
			if err := flush(); err != nil {
				return nil, err
			}
			id, statement, serves, buildsOn, supersedes = "", "", "", "", ""
			if strings.HasPrefix(line, "### D-") {
				match := decisionHeading.FindStringSubmatch(line)
				if match == nil || seen[match[1]] {
					return nil, fmt.Errorf("compile draft %s tech spec: invalid or repeated decision heading %q", feature, line)
				}
				id = match[1]
				seen[id] = true
			}
			continue
		}
		if id == "" {
			continue
		}
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "**Decision:**"); ok {
			statement = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "**Serves:**"); ok {
			serves = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "**Builds-on:**"); ok {
			buildsOn = strings.TrimSpace(value)
		}
		if value, ok := strings.CutPrefix(strings.TrimSpace(line), "**Supersedes:**"); ok {
			supersedes = strings.TrimSpace(value)
		}
	}
	if err := flush(); err != nil {
		return nil, err
	}
	return out, nil
}

func servedTarget(feature, raw string) (string, bool) {
	subject := "prd/" + feature
	id := raw
	if payload, ok := strings.CutPrefix(raw, "prd:"); ok {
		other, ref, found := strings.Cut(payload, "/")
		if !found || !record.ValidSubject("prd/"+other) {
			return "", false
		}
		subject, id = "prd/"+other, ref
	}
	ref, _, ok := schema.ParseID(id)
	return subject + "#" + id, ok && ref == id
}

func decisionTarget(feature, raw string) (string, bool) {
	subject, family := "design/"+feature, raw
	if payload, ok := strings.CutPrefix(raw, "design:"); ok {
		other, ref, found := strings.Cut(payload, "#")
		if !found || !record.ValidSubject("design/"+other) {
			return "", false
		}
		subject, family = "design/"+other, ref
	}
	return subject + "#" + family, decisionRef.MatchString(family)
}
