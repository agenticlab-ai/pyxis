package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

func init() {
	Register(Check{
		Verb:           "check-config-policy",
		Help:           "gate-policy config keys need the operator trailer",
		NeedsMergeBase: true,
		Run:            func(c Ctx) Defects { return CheckConfigPolicy(c.Root, c.MergeBase) },
	})
}

// CheckConfigPolicy is FR-23: the gate's own policy keys (grandfather and
// exemption lists, the capability trigger set) change only with the
// operator trailer — the self-unlock fix (fence-completion DINV-3).
func CheckConfigPolicy(root, mergeBase string) []schema.Defect {
	const rel = ".pyxis/config.yaml"
	oldBytes, err := exec.Command("git", "-C", root, "show", mergeBase+":"+rel).Output()
	if err != nil {
		return nil // no config at the base — introduction is reviewed as a whole
	}
	newBytes, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		newBytes = nil // deletion: compare against empty (defaults)
	}
	oldCfg, _ := config.Parse(oldBytes)
	newCfg := config.Defaults()
	if newBytes != nil {
		newCfg, _ = config.Parse(newBytes)
	}
	changed := []string{}
	if !equalList(oldCfg.Grandfathered, newCfg.Grandfathered) {
		changed = append(changed, "grandfathered_designs")
	}
	if !equalList(oldCfg.GrandfatheredCitations, newCfg.GrandfatheredCitations) {
		changed = append(changed, "grandfathered_citations")
	}
	if !equalList(oldCfg.PrecedenceGrandfathered, newCfg.PrecedenceGrandfathered) {
		changed = append(changed, "precedence_grandfathered")
	}
	if !equalList(oldCfg.CapabilityPaths, newCfg.CapabilityPaths) {
		changed = append(changed, "capability_paths")
	}
	if len(changed) == 0 || hasOperatorTrailer(root, mergeBase) {
		return nil
	}
	var defects []schema.Defect
	for _, key := range changed {
		defects = append(defects, vdef("check-config-policy", rel, "history",
			fmt.Sprintf("gate-policy key %q changed without the `Governing-Change: operator` trailer", key),
			"the gate's exemption surface is policy: only the operator widens it (INV-19's pattern; #54 D1)"))
	}
	return defects
}
