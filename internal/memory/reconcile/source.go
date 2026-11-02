package reconcile

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/sessionlog"
)

var (
	ledgerNameRE              = regexp.MustCompile(`^\d{3}-[a-z0-9-]+\.md$`)
	invalidSourceClassCharsRE = regexp.MustCompile(`[^a-z0-9._-]+`)
)

// AuthoredSource reports whether a newly discovered path belongs to the
// authored context corpus. Previously bound provenance paths are always
// reconsidered separately, so this predicate only decides discovery.
func AuthoredSource(p string, o Options) bool {
	o = optionDefaults(o)
	p = path.Clean(strings.TrimPrefix(p, "./"))
	if sessionlog.Path(p) && !o.LegacySessionLogs {
		return false
	}
	if p == "." || strings.HasPrefix(p, "../") || strings.Contains(p, "/testdata/") || strings.Contains(p, "/fixtures/") || strings.Contains(p, "/node_modules/") {
		return false
	}
	base, ext := path.Base(p), strings.ToLower(path.Ext(p))
	if isPRDProjection(p, o.PRDs) || isFeedbackProjection(p, o.Feedback) || (path.Dir(p) == o.TechDesigns && (base == "index.md" || base == "index.json")) || (strings.HasPrefix(p, o.Plans+"/") && ext == ".json") {
		return false
	}
	switch {
	case p == "AGENTS.md", p == "README.md":
		return true
	case belowRoot(p, o.PRDs), belowRoot(p, o.TechDesigns), belowRoot(p, o.Plans), belowRoot(p, o.Brainstorms), belowRoot(p, o.Problems), belowRoot(p, o.Reviews), belowRoot(p, o.Feedback):
		return ext == ".md" || ext == ".markdown" || ext == ".yaml" || ext == ".yml" || ext == ".json"
	case strings.HasPrefix(p, "docs/"):
		return ext == ".md" || ext == ".markdown" || ext == ".yaml" || ext == ".yml" || ext == ".json"
	case strings.HasPrefix(p, "logs/"):
		return ext == ".md"
	case strings.HasPrefix(p, ".pyxis/authoring/"), strings.HasPrefix(p, ".pyxis/skills/"), strings.HasPrefix(p, ".pyxis/templates/"), strings.HasPrefix(p, ".pyxis/dist/"), strings.HasPrefix(p, ".pyxis/stamp/"), strings.HasPrefix(p, ".pyxis/writehook/"):
		return ext == ".md" || ext == ".markdown" || ext == ".yaml" || ext == ".yml" || ext == ".json"
	case strings.HasPrefix(p, ".pyxis/vendor/"):
		return base == "README.md"
	case path.Dir(p) == ".pyxis":
		return ext == ".md" || ext == ".yaml" || ext == ".yml" || ext == ".json"
	case p == ".pyxis/scripts/versions.env":
		return true
	case strings.HasPrefix(p, "cmd/"):
		return base == "README.md"
	default:
		return false
	}
}

// snapshotSource excludes plans whose feature has no PRD ledger at the pinned
// commit. A draft-born plan follows its lock outside memory; other authored
// sources and document-pipeline plans retain their ordinary classification.
func snapshotSource(p string, o Options, extra, ledgerFeatures map[string]bool) bool {
	if rel, ok := strings.CutPrefix(p, strings.TrimSuffix(o.Plans, "/")+"/"); ok {
		if feature, _, ok := strings.Cut(rel, "/"); ok && !ledgerFeatures[feature] {
			return false
		}
	}
	return AuthoredSource(p, o) || extra[p]
}

func isFeedbackProjection(p, root string) bool {
	return belowRoot(p, root) && path.Base(p) == "effective.source.json"
}

func belowRoot(name, root string) bool {
	root = strings.TrimSuffix(path.Clean(root), "/")
	return root != "." && strings.HasPrefix(name, root+"/")
}

func sourceDigest(body []byte) string {
	s := sha256.Sum256(body)
	return hex.EncodeToString(s[:])
}

func sourceAddress(p string, o Options) (subject, family string) {
	p = path.Clean(p)
	if p == "logs/README.md" {
		return "history/session-logs", "index"
	}
	if strings.HasPrefix(p, "logs/") && path.Ext(p) == ".md" {
		return "session-log/" + slug(strings.TrimSuffix(path.Base(p), ".md")), "record"
	}
	if rel, ok := strings.CutPrefix(p, strings.TrimSuffix(o.PRDs, "/")+"/"); ok {
		parts := strings.Split(rel, "/")
		if len(parts) == 2 && ledgerNameRE.MatchString(parts[1]) {
			seq := strings.SplitN(parts[1], "-", 2)[0]
			if seq == "000" {
				return "prd/" + slug(parts[0]), "base"
			}
			return "prd/" + slug(parts[0]), "amendment-" + seq
		}
	}
	if rel := strings.TrimPrefix(p, strings.TrimSuffix(o.TechDesigns, "/")+"/"); rel != p && !strings.Contains(rel, "/") && strings.HasSuffix(rel, ".md") {
		return "design/" + slug(strings.TrimSuffix(rel, ".md")), "design"
	}
	if rel, ok := strings.CutPrefix(p, strings.TrimSuffix(o.Plans, "/")+"/"); ok {
		parts := strings.Split(rel, "/")
		if len(parts) == 2 && strings.HasSuffix(parts[1], ".md") {
			stem := strings.TrimSuffix(parts[1], ".md")
			fam := slug(stem)
			if planStem, ok := strings.CutSuffix(stem, "-plan"); ok {
				fam = "plan-" + planStem
			}
			return "plan/" + slug(parts[0]), ensureFamily(fam)
		}
	}
	if rel, ok := strings.CutPrefix(p, strings.TrimSuffix(o.Problems, "/")+"/"); ok {
		parts := strings.Split(rel, "/")
		if len(parts) == 2 && (parts[1] == "problem.md" || parts[1] == "delivery.md") {
			return "problem/" + slug(parts[0]), strings.TrimSuffix(parts[1], ".md")
		}
	}
	for _, rule := range []struct {
		root   string
		ns     string
		family string
	}{{o.Reviews, "review", "report"}, {o.Brainstorms, "brainstorm", "record"}} {
		root := strings.TrimSuffix(path.Clean(rule.root), "/") + "/"
		if rel := strings.TrimPrefix(p, root); rel != p && !strings.Contains(rel, "/") {
			return rule.ns + "/" + slug(strings.TrimSuffix(rel, path.Ext(rel))), rule.family
		}
	}
	for _, rule := range []struct {
		prefix string
		ns     string
		family string
	}{{"docs/wiki/", "wiki", "entry"}, {"docs/research/", "research", "study"}} {
		if strings.HasPrefix(p, rule.prefix) && !strings.Contains(strings.TrimPrefix(p, rule.prefix), "/") {
			return rule.ns + "/" + slug(strings.TrimSuffix(path.Base(p), path.Ext(p))), rule.family
		}
	}
	return "source/" + slug(strings.TrimSuffix(p, path.Ext(p))), "document"
}

func slug(s string) string {
	s = strings.ToLower(s)
	var b strings.Builder
	dash := false
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z', r >= '0' && r <= '9', r == '.', r == '_':
			b.WriteRune(r)
			dash = false
		default:
			if !dash && b.Len() > 0 {
				b.WriteByte('-')
				dash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-._")
	if out == "" || out[0] < 'a' || out[0] > 'z' {
		out = "x-" + out
	}
	return out
}

func ensureFamily(s string) string {
	return slug(s)
}

func sourceDraft(f File, subject, family, supersedes, commit string) Draft {
	rels := []record.Relation(nil)
	if supersedes != "" {
		rels = append(rels, record.Relation{Type: record.Supersedes, Target: supersedes})
	}
	tags := []string{"reconcile:source", "sha256:" + f.SHA256}
	if class := sourceClass(f.Path); class != "" {
		tags = append(tags, "class:"+class)
	}
	if strings.HasPrefix(f.Path, "logs/") {
		tags = append(tags, "historical", "non-governing")
	}
	sort.Strings(tags)
	return Draft{
		Kind: record.Document, Subject: subject, Family: family,
		Statement:   "Captured authored source: " + f.Path,
		Intent:      "Preserve the authored feature context until closeout proves its important content is represented in native memory.",
		Rationale:   "The offline reconciler binds exact source bytes to an immutable record at a pinned repository commit.",
		Tags:        tags,
		Relations:   rels,
		Provenance:  record.Provenance{Source: f.Path, Commit: commit, Note: "offline reconciliation"},
		LiteralBody: true,
		Body:        append([]byte(nil), f.Body...),
	}
}

func sourceClass(p string) string {
	p = path.Clean(p)
	switch {
	case p == "AGENTS.md", p == "README.md":
		return "repo-root"
	case strings.HasPrefix(p, "logs/"):
		return "logs"
	case strings.HasPrefix(p, "docs/"):
		parts := strings.Split(p, "/")
		if len(parts) == 2 {
			return "docs/root"
		}
		return "docs/" + normalizeSourceClassComponent(parts[1])
	case strings.HasPrefix(p, ".pyxis/"):
		parts := strings.Split(p, "/")
		return ".pyxis/" + normalizeSourceClassComponent(parts[1])
	case strings.HasPrefix(p, "cmd/"):
		parts := strings.Split(p, "/")
		if len(parts) >= 2 {
			return "cmd/" + normalizeSourceClassComponent(parts[1])
		}
	}
	return ""
}

func normalizeSourceClassComponent(component string) string {
	component = invalidSourceClassCharsRE.ReplaceAllString(strings.ToLower(component), "-")
	component = strings.Trim(component, "-")
	if component == "" {
		return "unknown"
	}
	return component
}

func sourceClosure(files map[string]File, paths []string) string {
	paths = append([]string(nil), paths...)
	sort.Strings(paths)
	var b strings.Builder
	for _, p := range paths {
		f, ok := files[p]
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "%s\x00%s\x00%s\n", f.Path, f.Mode, f.SHA256)
	}
	return sourceDigest([]byte(b.String()))
}
