package gate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/planbasis"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// lockedPlanTargets is the citation universe of a draft-born plan's Basis.
// Only its own PRD and design are resolved here; foreign citations still use
// the document pipeline's ordinary universes.
type lockedPlanTargets struct {
	feature    string
	basis      string
	units      map[string]bool
	design     []byte
	projection *resolve.Projection
	err        error
}

func lockedPlanCitations(root, rel string, body []byte, cfg config.Config) lockedPlanTargets {
	feature, basis := planbasis.Of(rel, body, cfg)
	if basis == "" {
		return lockedPlanTargets{}
	}
	result := lockedPlanTargets{feature: feature, basis: basis}
	result.err = result.load(root)
	return result
}

// citationScanFiles adds configured plans outside docs/ to the citation
// check's scan set. The other corpus checks keep their existing scan set.
func citationScanFiles(root string, cfg config.Config, scan ScanScope) []string {
	files := corpusScanFiles(root, cfg, scan, true)
	seen := make(map[string]bool, len(files))
	for _, file := range files {
		seen[file] = true
	}
	add := func(rel string) {
		if planbasis.Feature(rel, cfg) == "" {
			return
		}
		abs := filepath.Join(root, filepath.FromSlash(rel))
		if fi, err := os.Stat(abs); err == nil && !fi.IsDir() && !seen[abs] {
			seen[abs] = true
			files = append(files, abs)
		}
	}
	if scan.epochActive {
		for rel := range scan.changed {
			add(rel)
		}
	} else {
		plansDir := filepath.Join(root, filepath.Clean(cfg.Plans))
		features, _ := os.ReadDir(plansDir)
		for _, feature := range features {
			if !feature.IsDir() {
				continue
			}
			plans, _ := os.ReadDir(filepath.Join(plansDir, feature.Name()))
			for _, plan := range plans {
				if !plan.IsDir() {
					add(filepath.ToSlash(filepath.Join(cfg.Plans, feature.Name(), plan.Name())))
				}
			}
		}
	}
	sort.Strings(files)
	return files
}

func (l *lockedPlanTargets) load(root string) error {
	ctx := context.Background()
	repo, err := gitsrc.Open(ctx, root)
	if err != nil {
		return fmt.Errorf("open repository: %w", err)
	}
	commit, err := repo.ResolveCommit(ctx, "refs/tags/"+l.basis)
	if err != nil {
		pending, ok := l.pendingLock(ctx, repo)
		if !ok {
			return fmt.Errorf("resolve %s: %w", l.basis, err)
		}
		commit = pending
	}
	c, err := l.contractAt(ctx, repo, commit)
	if err != nil {
		return err
	}
	set, projection, prd, design := c.set, c.projection, c.prd, c.design
	prdBody, designBody := set.Bodies[prd.ID], set.Bodies[design.ID]
	if len(prdBody) == 0 || len(designBody) == 0 {
		return fmt.Errorf("%s has an empty PRD or design Document", l.basis)
	}
	doc, defects := ledger.ParseProjection(filepath.Join(l.basis, "prd.md"), schema.Normalize(prdBody))
	if doc == nil || len(defects) != 0 {
		return fmt.Errorf("%s PRD cannot be projected: %v", l.basis, defects)
	}
	l.units = map[string]bool{}
	for _, id := range doc.UnitOrder {
		l.units[id] = true
	}
	l.design = designBody
	l.projection = projection
	return nil
}

// lockedContract is the feature's contract as the memory at one commit holds it.
type lockedContract struct {
	set         *store.Set
	projection  *resolve.Projection
	prd, design *record.Record
}

func (l lockedPlanTargets) contractAt(ctx context.Context, repo *gitsrc.Repo, commit string) (lockedContract, error) {
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: commit, Prefix: store.Dir})
	if err != nil {
		return lockedContract{}, fmt.Errorf("read %s memory: %w", l.basis, err)
	}
	if len(set.Problems) != 0 || len(set.Dangling) != 0 {
		return lockedContract{}, fmt.Errorf("%s has invalid memory", l.basis)
	}
	projection := resolve.New(set)
	if len(projection.Violations) != 0 || len(projection.Conflicts()) != 0 {
		return lockedContract{}, fmt.Errorf("%s has invalid memory projection", l.basis)
	}
	prd := projection.Head("prd/"+l.feature, "prd")
	design := projection.Head("design/"+l.feature, "design")
	if prd == nil || prd.Kind != record.Document || design == nil || design.Kind != record.Document {
		return lockedContract{}, fmt.Errorf("%s has no active PRD and design Documents", l.basis)
	}
	return lockedContract{set: set, projection: projection, prd: prd, design: design}, nil
}

// pendingLock returns the checked commit when it carries the lock the plan
// names before that lock is tagged. The change that prepares lock vN moves
// the live plan onto vN, and the tag appears only when that change merges.
// The commit must descend from the tagged lock v(N-1) and hold newer contract
// Documents than it, so a plan cannot name a lock no change prepares, and a
// checkout from before v(N-1) cannot stand in for vN.
func (l lockedPlanTargets) pendingLock(ctx context.Context, repo *gitsrc.Repo) (string, bool) {
	prefix := "lock/" + l.feature + "/v"
	n, err := strconv.Atoi(strings.TrimPrefix(l.basis, prefix))
	if err != nil || n < 2 {
		return "", false
	}
	previous, err := repo.ResolveCommit(ctx, "refs/tags/"+prefix+strconv.Itoa(n-1))
	if err != nil {
		return "", false
	}
	head, err := repo.ResolveCommit(ctx, "HEAD")
	if err != nil {
		return "", false
	}
	if base, err := repo.MergeBase(ctx, previous, head); err != nil || base != previous {
		return "", false
	}
	was, err := l.contractAt(ctx, repo, previous)
	if err != nil {
		return "", false
	}
	now, err := l.contractAt(ctx, repo, head)
	if err != nil || (now.prd.ID == was.prd.ID && now.design.ID == was.design.ID) {
		return "", false
	}
	return head, true
}

// resolve returns an empty reason only when this is not one of the plan's own
// locked citations, letting the existing document resolver handle that token.
func (l lockedPlanTargets) resolve(grammar, payload string) (bool, string) {
	if l.feature == "" {
		return false, ""
	}
	var target, anchor string
	switch grammar {
	case "prd":
		target, anchor, _ = strings.Cut(payload, "/")
	case "design":
		target, anchor = splitAnchor(payload)
	default:
		return false, ""
	}
	if target != l.feature {
		return false, ""
	}
	if l.err != nil {
		return false, l.err.Error()
	}
	if grammar == "prd" {
		if anchor == "" || l.units[firstOfRange(anchor)] {
			return true, ""
		}
		return false, fmt.Sprintf("unit %s not in %s PRD at %s", anchor, l.feature, l.basis)
	}
	if anchor == "" {
		return true, ""
	}
	anchor = firstAnchor(anchor)
	if strings.HasPrefix(anchor, "D-") {
		if claim := l.projection.Head("design/"+l.feature, anchor); claim != nil && claim.Kind == record.Decision {
			return true, ""
		}
		return false, fmt.Sprintf("decision %s not active at %s", anchor, l.basis)
	}
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(anchor) + `\b`)
	if re.Match(l.design) {
		return true, ""
	}
	return false, fmt.Sprintf("anchor %s not in design at %s", anchor, l.basis)
}
