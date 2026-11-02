package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "check-maps",
		Help:           "system-map registry anchors persist vs --merge-base",
		NeedsMergeBase: true,
		Run:            func(c Ctx) Defects { return CheckMaps(c.Root, c.MergeBase, c.Cfg) },
	})
}

// CheckMaps is FR-22: registry anchors (INV-<n>, D<n>) present in a
// system-map document at the merge base persist at HEAD — the living maps
// stay freely editable in everything but their permanent IDs
// (doc-link-integrity DINV-2).
func CheckMaps(root, mergeBase string, cfg config.Config) []schema.Defect {
	var defects []schema.Defect
	for _, rel := range cfg.SystemMaps {
		oldBytes, err := exec.Command("git", "-C", root, "show", mergeBase+":"+rel).Output()
		if err != nil {
			continue // absent at the base — nothing to protect yet
		}
		var newAnchors map[string]bool
		if newBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel))); err == nil {
			newAnchors = mapAnchors(newBytes)
		} else {
			newAnchors = map[string]bool{} // deleted map: maximal removal
		}
		oldAnchors := mapAnchors(oldBytes)
		var missing []string
		for a := range oldAnchors {
			if !newAnchors[a] {
				missing = append(missing, a)
			}
		}
		sort.Strings(missing)
		for _, a := range missing {
			defects = append(defects, vdef("check-maps", rel, "history",
				fmt.Sprintf("registry anchor %s removed or renumbered — map registry IDs are permanent", a),
				"anchors append under fresh numbers; update the numbered content, never the number"))
		}
	}
	return defects
}
