// Package planbasis reads the lock a draft-born plan names as its Basis: the
// tag (refs/tags/lock/<feature>/v<N>) its own PRD and design citations resolve
// through. The citation check and the executor's recovery preflight share it,
// so both agree on which tags a checkout must hold.
package planbasis

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

var (
	planPath  = regexp.MustCompile(`^([a-z][a-z0-9-]+)/[0-9]{3}-plan\.md$`)
	lockBasis = regexp.MustCompile(`^lock/[a-z][a-z0-9-]*/v[1-9][0-9]*$`)
)

// Feature returns the feature of a plan at rel, a path relative to the
// repository root, or "" when rel is not a configured plan document.
func Feature(rel string, cfg config.Config) string {
	plans := filepath.ToSlash(filepath.Clean(cfg.Plans))
	withinPlans := rel
	if plans != "." {
		var ok bool
		withinPlans, ok = strings.CutPrefix(rel, plans+"/")
		if !ok {
			return ""
		}
	}
	m := planPath.FindStringSubmatch(withinPlans)
	if m == nil {
		return ""
	}
	return m[1]
}

// Of returns the plan's feature and Basis when its header names its own
// feature's PRD, design, and lock; otherwise both are empty.
func Of(rel string, body []byte, cfg config.Config) (feature, basis string) {
	feature = Feature(rel, cfg)
	if feature == "" {
		return "", ""
	}
	fields := map[string]string{}
	for line := range strings.SplitSeq(string(body), "\n") {
		if strings.HasPrefix(line, "## Tasks") {
			break
		}
		parts := strings.Split(strings.Trim(line, " |"), "|")
		if len(parts) == 2 {
			fields[strings.Trim(strings.TrimSpace(parts[0]), "* ")] = strings.TrimSpace(parts[1])
		}
	}
	basis = fields["Basis"]
	if fields["Feature"] != feature || fields["PRD"] != "prd/"+feature || fields["Design"] != "design/"+feature || !lockBasis.MatchString(basis) || !strings.HasPrefix(basis, "lock/"+feature+"/") {
		return "", ""
	}
	return feature, basis
}

// Required lists, sorted and once each, the Basis of every plan under root's
// configured plans directory: the lock tags a full citation check resolves.
func Required(root string, cfg config.Config) ([]string, error) {
	plansDir := filepath.Join(root, filepath.Clean(cfg.Plans))
	features, err := os.ReadDir(plansDir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	var bases []string
	for _, feature := range features {
		if !feature.IsDir() {
			continue
		}
		plans, err := os.ReadDir(filepath.Join(plansDir, feature.Name()))
		if err != nil {
			return nil, err
		}
		for _, plan := range plans {
			rel := filepath.ToSlash(filepath.Join(cfg.Plans, feature.Name(), plan.Name()))
			if plan.IsDir() || Feature(rel, cfg) == "" {
				continue
			}
			body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, err
			}
			if _, basis := Of(rel, body, cfg); basis != "" && !seen[basis] {
				seen[basis] = true
				bases = append(bases, basis)
			}
		}
	}
	sort.Strings(bases)
	return bases, nil
}
