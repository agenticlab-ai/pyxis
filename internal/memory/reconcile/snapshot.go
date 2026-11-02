package reconcile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
	"github.com/agenticlab-ai/pyxis/internal/memory/codeinventory"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/sessionlog"
)

// BuildSnapshot reads every input from one already-resolved git commit. It
// never consults the network or the mutable working tree.
func BuildSnapshot(ctx context.Context, repo *gitsrc.Repo, commit string, opts Options) (Snapshot, error) {
	if repo == nil || strings.TrimSpace(commit) == "" {
		return Snapshot{}, fmt.Errorf("build reconciliation snapshot: a repository and pinned commit are required")
	}
	resolved, err := repo.ResolveCommit(ctx, commit)
	if err != nil {
		return Snapshot{}, fmt.Errorf("resolve reconciliation commit %q: %w", commit, err)
	}
	commit = resolved
	opts = optionDefaults(opts)
	entries, err := repo.ListTree(ctx, commit, ".")
	if err != nil {
		return Snapshot{}, fmt.Errorf("list pinned source tree: %w", err)
	}
	extra := map[string]bool{}
	for _, p := range opts.ExtraSources {
		p = path.Clean(strings.TrimPrefix(p, "./"))
		if p != "." && !strings.HasPrefix(p, "../") && (!sessionlog.Path(p) || opts.LegacySessionLogs) {
			extra[p] = true
		}
	}
	var readPaths []string
	seenRead := map[string]bool{}
	features := map[string]bool{}
	ledgerFeatures := map[string]bool{}
	for _, entry := range entries {
		if feature, ok := LedgerFeature(entry.Path, opts.PRDs); ok && entry.Regular() {
			ledgerFeatures[feature] = true
		}
	}
	existingPaths := map[string]bool{}
	addRead := func(p string) {
		if !seenRead[p] {
			seenRead[p] = true
			readPaths = append(readPaths, p)
		}
	}
	for _, entry := range entries {
		existingPaths[entry.Path] = true
		if feature, ok := prdFeature(entry.Path, opts.PRDs); ok {
			features[feature] = true
		}
		selectedSource := snapshotSource(entry.Path, opts, extra, ledgerFeatures)
		selectedCode := codeinventory.CandidatePath(entry.Path)
		selectedCompiler := isLedgerEntry(entry.Path, opts.PRDs) || isPRDProjection(entry.Path, opts.PRDs) || entry.Path == path.Join(opts.TechDesigns, "index.json")
		if selectedSource && !entry.Regular() {
			return Snapshot{}, fmt.Errorf("authored source %s is not a regular file at %s", entry.Path, commit)
		}
		if selectedCompiler && !entry.Regular() {
			return Snapshot{}, fmt.Errorf("compiler input %s is not a regular file at %s", entry.Path, commit)
		}
		if (selectedSource || selectedCode || selectedCompiler) && entry.Regular() {
			addRead(entry.Path)
		}
	}
	sort.Strings(readPaths)
	blobs, err := repo.Files(ctx, commit, readPaths)
	if err != nil {
		return Snapshot{}, fmt.Errorf("read pinned source tree: %w", err)
	}
	for _, p := range readPaths {
		if _, ok := blobs[p]; !ok {
			return Snapshot{}, fmt.Errorf("read pinned source tree: listed path %s disappeared at immutable commit %s", p, commit)
		}
	}
	ciEntries := make([]codeinventory.Entry, 0, len(entries))
	for _, entry := range entries {
		ciEntries = append(ciEntries, codeinventory.Entry{Path: entry.Path, Mode: entry.Mode, Type: entry.Type})
	}
	var inv codeinventory.Inventory
	if opts.LegacyCodeInventory {
		inv, err = codeinventory.BuildLegacyV2(ciEntries, blobs)
	} else {
		inv, err = codeinventory.Build(ciEntries, blobs)
	}
	if err != nil {
		return Snapshot{}, err
	}

	s := Snapshot{
		Commit: commit, Files: map[string]File{}, ExistingPaths: existingPaths,
		PRDSourceMapUnits: map[string]map[string]string{},
		Code:              inv.Units, CodeInventorySHA256: inv.AggregateSHA256,
		SourceOptions: opts,
	}
	for feature := range features {
		s.PRDFeatures = append(s.PRDFeatures, feature)
	}
	sort.Strings(s.PRDFeatures)
	for _, entry := range entries {
		body, ok := blobs[entry.Path]
		if !ok {
			continue
		}
		f := File{Path: entry.Path, Mode: entry.Mode, SHA256: sourceDigest(body), Body: append([]byte(nil), body...)}
		s.Files[entry.Path] = f
		if snapshotSource(entry.Path, opts, extra, ledgerFeatures) {
			s.AuthoredSources = append(s.AuthoredSources, entry.Path)
		}
	}
	sort.Strings(s.AuthoredSources)
	s.AuthoredSources = compactStrings(s.AuthoredSources)
	s.SourceClosureSHA256 = sourceClosure(s.Files, s.AuthoredSources)

	if err := compilePRDs(&s, opts); err != nil {
		return Snapshot{}, err
	}
	if err := compileDesignIndex(&s, opts); err != nil {
		return Snapshot{}, err
	}
	return s, nil
}

func optionDefaults(o Options) Options {
	defaults := config.Defaults()
	if o.ProductVision == "" {
		o.ProductVision = defaults.ProductVision
	}
	if o.ArchitecturePrinciples == "" {
		o.ArchitecturePrinciples = defaults.ArchitecturePrinciples
	}
	if o.Incidents == "" {
		o.Incidents = defaults.Incidents
	}
	if o.PRDs == "" {
		o.PRDs = defaults.PRDs
	}
	if o.TechDesigns == "" {
		o.TechDesigns = defaults.TechDesigns
	}
	if o.Plans == "" {
		o.Plans = defaults.Plans
	}
	if o.Brainstorms == "" {
		o.Brainstorms = defaults.Brainstorms
	}
	if o.Problems == "" {
		o.Problems = defaults.Problems
	}
	if o.Reviews == "" {
		o.Reviews = defaults.Reviews
	}
	if o.Feedback == "" {
		o.Feedback = defaults.Feedback
	}
	return o
}

func isLedgerEntry(p, root string) bool {
	rel := strings.TrimPrefix(p, strings.TrimSuffix(root, "/")+"/")
	if rel == p {
		return false
	}
	parts := strings.Split(rel, "/")
	return len(parts) == 2 && ledgerNameRE.MatchString(parts[1])
}

// LedgerFeature identifies a PRD ledger entry without interpreting its content.
// The trusted-base check uses this predicate before granting native authority.
func LedgerFeature(p, root string) (string, bool) {
	if !isLedgerEntry(p, root) {
		return "", false
	}
	return prdFeature(p, root)
}

func prdFeature(p, root string) (string, bool) {
	rel := strings.TrimPrefix(p, strings.TrimSuffix(root, "/")+"/")
	if rel == p {
		return "", false
	}
	parts := strings.Split(rel, "/")
	if len(parts) < 2 || !validFeature(parts[0]) {
		return "", false
	}
	return parts[0], true
}

func isPRDProjection(p, root string) bool {
	rel := strings.TrimPrefix(p, strings.TrimSuffix(root, "/")+"/")
	if rel == p {
		return false
	}
	parts := strings.Split(rel, "/")
	if len(parts) != 2 {
		return false
	}
	return parts[1] == "effective.md" || parts[1] == "effective.source.json"
}

func compilePRDs(s *Snapshot, o Options) error {
	byFeature := map[string][]ledger.Entry{}
	for p, f := range s.Files {
		if !isLedgerEntry(p, o.PRDs) {
			continue
		}
		rel := strings.TrimPrefix(p, strings.TrimSuffix(o.PRDs, "/")+"/")
		parts := strings.Split(rel, "/")
		byFeature[parts[0]] = append(byFeature[parts[0]], ledger.Entry{Name: parts[1], Bytes: f.Body})
	}
	for _, feature := range s.PRDFeatures {
		entries := byFeature[feature]
		sort.Slice(entries, func(i, j int) bool { return entries[i].Name < entries[j].Name })
		if defects := gate.LintStructure(feature, entries); len(defects) > 0 {
			return fmt.Errorf("lint PRD %s at %s: %s", feature, s.Commit, defects[0].String())
		}
		result := fold.Fold(entries)
		if len(result.Defects) > 0 {
			return fmt.Errorf("fold PRD %s at %s: %s", feature, s.Commit, result.Defects[0].String())
		}
		if defects := gate.VerifyRebase(entries); len(defects) > 0 {
			return fmt.Errorf("verify PRD rebase %s at %s: %s", feature, s.Commit, defects[0].String())
		}
		legacyDigests := historicalLegacyPRDUnitDigests(entries)
		for _, generated := range []struct {
			name string
			want []byte
		}{{"effective.md", result.EffectiveMD}, {"effective.source.json", result.SourceJSON}} {
			name, want := generated.name, generated.want
			p := path.Join(o.PRDs, feature, name)
			got, ok := s.Files[p]
			if !ok {
				return fmt.Errorf("verify PRD %s at %s: generated projection %s is missing", feature, s.Commit, p)
			}
			if !bytes.Equal(got.Body, want) {
				return fmt.Errorf("verify PRD %s at %s: committed %s differs from the pinned fold; run docgate fold and commit it", feature, s.Commit, p)
			}
		}
		prd := PRD{Feature: feature, Source: path.Join(o.PRDs, feature, "effective.md"), AsOf: result.AsOf, Fingerprint: result.Fingerprint, SourceMapSHA256: sourceDigest(result.SourceJSON)}
		staged, err := requirementsFirst(entries[0].Bytes, result.EffectiveMD)
		if err != nil {
			return fmt.Errorf("PRD %s design coverage: %w", feature, err)
		}
		prd.RequirementsFirst = staged
		for _, unit := range result.Resolved {
			projected := projectUnit(unit)
			projected.LegacySourceSHA256s = append([]string(nil), legacyDigests[legacyPRDIdentityKey(projected)]...)
			prd.Units = append(prd.Units, projected)
		}
		statuses, err := sourceMapStatuses(result.SourceJSON)
		if err != nil {
			return fmt.Errorf("decode folded PRD source map %s at %s: %w", feature, s.Commit, err)
		}
		s.PRDSourceMapUnits[feature] = statuses
		s.CompilerFindings = append(s.CompilerFindings, prdProjectionFindings(prd, statuses)...)
		s.PRDs = append(s.PRDs, prd)
	}
	return nil
}

// historicalLegacyPRDUnitDigests reconstructs only identities the v1
// reconciler could actually have emitted from a retained ledger prefix. A
// semantic-no-op rebase changes v1 lineage metadata, but the older immutable
// entries remain available here; pairing by the stable v2 semantic digest
// admits the old exact tag without widening compatibility to arbitrary hashes.
func historicalLegacyPRDUnitDigests(entries []ledger.Entry) map[string][]string {
	out := map[string][]string{}
	seen := map[string]map[string]bool{}
	for end := 1; end <= len(entries); end++ {
		result := fold.Fold(entries[:end])
		if len(result.Defects) > 0 {
			// A defective historical prefix could not have produced an admitted
			// v1 reconciliation record. Legacy corpora can contain such a prefix
			// followed by a corrective amendment, so omit it without weakening
			// the identities reconstructed from valid prefixes.
			continue
		}
		for _, unit := range result.Resolved {
			projected := projectUnit(unit)
			key := legacyPRDIdentityKey(projected)
			if seen[key] == nil {
				seen[key] = map[string]bool{}
			}
			for _, digest := range projected.LegacySourceSHA256s {
				if !validDigest(digest) || seen[key][digest] {
					continue
				}
				seen[key][digest] = true
				out[key] = append(out[key], digest)
			}
		}
	}
	for key := range out {
		sort.Strings(out[key])
	}
	return out
}

func legacyPRDIdentityKey(unit PRDUnit) string {
	return unit.ID + "\x00" + unit.SourceSHA256
}

func sourceMapStatuses(body []byte) (map[string]string, error) {
	var source struct {
		Units map[string]struct {
			Status string `json:"status"`
		} `json:"units"`
	}
	if err := json.Unmarshal(body, &source); err != nil {
		return nil, err
	}
	if source.Units == nil {
		return nil, fmt.Errorf("units is absent")
	}
	out := make(map[string]string, len(source.Units))
	for id, unit := range source.Units {
		out[id] = unit.Status
	}
	return out, nil
}

func prdProjectionFindings(prd PRD, sourceMap map[string]string) []Finding {
	resolved := make(map[string]string, len(prd.Units))
	for _, unit := range prd.Units {
		resolved[unit.ID] = unit.Status
	}
	ids := make(map[string]bool, len(resolved)+len(sourceMap))
	for id := range resolved {
		ids[id] = true
	}
	for id := range sourceMap {
		ids[id] = true
	}
	ordered := make([]string, 0, len(ids))
	for id := range ids {
		ordered = append(ordered, id)
	}
	schema.SortIDs(ordered)
	var out []Finding
	for _, id := range ordered {
		current, currentOK := resolved[id]
		mapped, mappedOK := sourceMap[id]
		if currentOK && mappedOK && current == mapped {
			continue
		}
		detail := fmt.Sprintf("effective projection status=%q (present=%t), source-map status=%q (present=%t)", current, currentOK, mapped, mappedOK)
		out = append(out, Finding{
			Code: "prd-projection-divergence", Scope: "prd/" + prd.Feature + "#" + id, Feature: prd.Feature,
			Detail:   detail,
			Remedy:   "repair the PRD fold so effective.md and effective.source.json describe the same unit universe and liveness, then regenerate both artifacts",
			Blocking: true,
		})
	}
	return out
}

func compileDesignIndex(s *Snapshot, o Options) error {
	p := path.Join(o.TechDesigns, "index.json")
	f, ok := s.Files[p]
	var parsed []*techdesigns.Design
	for source, file := range s.Files {
		if path.Dir(source) != o.TechDesigns || !strings.HasSuffix(source, ".md") || strings.HasPrefix(path.Base(source), "index.") {
			continue
		}
		slug := strings.TrimSuffix(path.Base(source), ".md")
		design, defects := techdesigns.Parse(slug, file.Body)
		if len(defects) > 0 {
			return fmt.Errorf("parse technical design %s at %s: %s", source, s.Commit, defects[0].String())
		}
		if design != nil {
			parsed = append(parsed, design)
		}
	}
	sort.Slice(parsed, func(i, j int) bool { return parsed[i].Slug < parsed[j].Slug })
	if len(parsed) == 0 && !ok {
		return nil
	}
	if !ok {
		return fmt.Errorf("technical-design corpus exists but %s is missing at %s", p, s.Commit)
	}
	s.DesignIndexSHA256 = f.SHA256
	prdUnits := s.PRDSourceMapUnits
	if prdUnits == nil {
		prdUnits = map[string]map[string]string{}
	}
	grandfathered := map[string]bool{}
	for _, slug := range o.Grandfathered {
		grandfathered[slug] = true
	}
	precedenceGrandfathered := map[string]bool{}
	for _, slug := range o.PrecedenceGrandfathered {
		precedenceGrandfathered[slug] = true
	}
	corpusIDs := map[string]bool{}
	for source, file := range s.Files {
		if path.Dir(source) != o.PRDs || !strings.HasSuffix(source, ".md") {
			continue
		}
		for _, id := range schema.CorpusIDRe.FindAllString(string(file.Body), -1) {
			corpusIDs[id] = true
		}
	}
	env := techdesigns.Env{
		Grandfathered: grandfathered, PrecedenceGrandfathered: precedenceGrandfathered,
		PRDUnits: prdUnits, CorpusIDs: corpusIDs, NewDesigns: map[string]bool{},
		DocExists: func(name string) bool { return s.ExistingPaths[name] },
	}
	if defects := techdesigns.Lint(parsed, env); len(defects) > 0 {
		return fmt.Errorf("lint technical-design corpus at %s: %s", s.Commit, defects[0].String())
	}
	_, expected, defects := techdesigns.GenIndex(parsed, env)
	if len(defects) > 0 {
		return fmt.Errorf("compute technical-design graph at %s: %s", s.Commit, defects[0].String())
	}
	if !bytes.Equal(f.Body, expected) {
		return fmt.Errorf("technical-design index %s differs from the graph recomputed from %d pinned source design(s); run docgate gen-index and commit it", p, len(parsed))
	}
	entries, _, defects := techdesigns.LiveGraph(parsed, env)
	if len(defects) > 0 {
		return fmt.Errorf("compute technical-design graph at %s: %s", s.Commit, defects[0].String())
	}
	slugs := make([]string, 0, len(entries))
	for slug := range entries {
		slugs = append(slugs, slug)
	}
	sort.Strings(slugs)
	for _, slug := range slugs {
		e := entries[slug]
		source := path.Join(o.TechDesigns, slug+".md")
		sourceFile, ok := s.Files[source]
		if !ok {
			return fmt.Errorf("parse %s: design %q points to missing source %s at %s", p, slug, source, s.Commit)
		}
		d := Design{Slug: slug, Source: source, SHA256: sourceFile.SHA256, Status: e.Status, BuildsOn: append([]string(nil), e.BuildsOn...), Serves: append([]string(nil), e.Serves...), Flags: append([]string(nil), e.Flags...)}
		if e.CurrentSuccessor != nil {
			d.CurrentSuccessor = *e.CurrentSuccessor
		}
		sort.Strings(d.BuildsOn)
		sort.Strings(d.Serves)
		sort.Strings(d.Flags)
		s.Designs = append(s.Designs, d)
	}
	return nil
}

func compactStrings(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := in[:1]
	for _, s := range in[1:] {
		if s != out[len(out)-1] {
			out = append(out, s)
		}
	}
	return out
}
