package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "lint-incidents",
		Help:           "registry shape + append-only (with --merge-base)",
		NeedsMergeBase: false,
		Run: func(c Ctx) Defects {
			defects := LintIncidents(c.Root, c.Cfg)
			if c.MergeBase != "" {
				defects = append(defects, CheckIncidentsAppend(c.Root, c.MergeBase, c.Cfg)...)
			}
			return defects
		},
	})
}

// LintIncidents is the registry shape half (FR-25; append-only rides
// CheckIncidentsAppend). Reads the registry at its configured path.
func LintIncidents(root string, cfg config.Config) []schema.Defect {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cfg.Incidents)))
	if err != nil {
		return nil // registry absent — nothing shipped requires it yet
	}
	return LintIncidentBytes(cfg.Incidents, b)
}

// CheckIncidentsAppend is FR-15's history half. Append-only is an
// ENTRY-level invariant: committed **I<n>** blocks are immutable —
// except each entry's Links line, which may grow (graduation adds the
// `Graduated: AP-<n>` pointer per the registry's own format doc).
// Non-entry scaffolding (intro, legend, the seeded placeholder the
// first entry replaces) is ordinary editable prose.
func CheckIncidentsAppend(root, mergeBase string, cfg config.Config) []schema.Defect {
	oldBytes, err := exec.Command("git", "-C", root, "show", mergeBase+":"+cfg.Incidents).Output()
	if err != nil {
		return nil // registry absent at the base — everything is an append
	}
	newBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(cfg.Incidents)))
	if err != nil {
		return []schema.Defect{vdef("lint-incidents", cfg.Incidents, "history",
			"registry deleted", "the incident registry is permanent — restore it")}
	}
	oldEntries := incidentEntries(oldBytes)
	newEntries := incidentEntries(newBytes)
	var defects []schema.Defect
	var ids []int
	for id := range oldEntries {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	for _, id := range ids {
		oldE, newE := oldEntries[id], newEntries[id]
		if newE == nil {
			defects = append(defects, vdef("lint-incidents", cfg.Incidents, "history",
				fmt.Sprintf("committed entry I%d deleted — entries are append-only", id),
				"entries are permanent; append, never remove"))
			continue
		}
		if len(newE) < len(oldE) {
			defects = append(defects, vdef("lint-incidents", cfg.Incidents, "history",
				fmt.Sprintf("committed entry I%d truncated — entries are append-only", id),
				"entries are permanent; the Links line may only grow"))
			continue
		}
		for i, ol := range oldE {
			nl := newE[i]
			if nl == ol {
				continue
			}
			if strings.HasPrefix(strings.TrimSpace(ol), "- **Links:**") &&
				strings.HasPrefix(nl, strings.TrimRight(ol, " .")) {
				continue // graduation: the Links line grew append-only
			}
			defects = append(defects, vdef("lint-incidents", cfg.Incidents, "history",
				fmt.Sprintf("committed entry I%d modified — entries are append-only", id),
				"never rewrite a paid-for lesson; graduation appends to the Links line only"))
			break
		}
	}
	return defects
}
