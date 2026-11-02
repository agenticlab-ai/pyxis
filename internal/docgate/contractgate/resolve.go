package contractgate

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/docgate/techdesigns"
)

// sourceMap is the read shape of a folded PRD source map
// (effective.source.json). Closed parse: unknown top-level keys are a defect,
// so a malformed or wrong-schema face fails closed as `error` rather than
// resolving to a lenient nil (design §3.11, DINV-3 — the gate reads the
// machine face, never re-parses effective.md). The unit shape mirrors the
// enriched fold projection (fold.UnitState) the later checks consume; T2 reads
// only its presence + fingerprint.
type sourceMap struct {
	SchemaVersion string             `json:"schema_version"`
	GeneratedBy   string             `json:"generated_by"`
	ToolVersion   string             `json:"tool_version"`
	AsOf          string             `json:"as_of"`
	Fingerprint   string             `json:"content_fingerprint"`
	Units         map[string]srcUnit `json:"units"`
}

type srcUnit struct {
	Status          string  `json:"status"`
	IntroducedBy    string  `json:"introduced_by"`
	Supersedes      *string `json:"supersedes"`
	SupersededBy    *string `json:"superseded_by"`
	SourceLoc       string  `json:"source_location"`
	CurrentLoc      string  `json:"current_location"`
	Priority        string  `json:"priority,omitempty"`
	HasFitCriterion bool    `json:"has_fit_criterion,omitempty"`
	VerifyMethod    string  `json:"verify_method,omitempty"`
}

// servingDesign is one live technical design whose Serves targets the
// feature's requirements — a resolved contract input alongside the source map.
type servingDesign struct {
	Slug        string
	Fingerprint string
	Serves      []string
}

// contract is a feature's fully-resolved, read-only assembled contract (design
// §3.1): the PRD source map, the live serving designs, and the DoD unit family
// (a subset of the source map — the units the structural check reads). It is
// the single object the check set runs against; T2 assembles it and the check
// tasks run the contract-scope subset over it.
type contract struct {
	Feature string
	// Root and Cfg are the resolution root + config the contract was resolved
	// against — carried so the link check (T4) can re-invoke the shared citation
	// resolver (gate.CheckCitationsInFiles) over the feature's own document files
	// (its ledger + serving designs), the SAME resolver the tree-wide
	// check-citations uses, scoped without altering its global behavior
	// (design §3.9, prd:contract-gate/NR-3). Read-only inputs only.
	Root string
	Cfg  config.Config
	// SourcePath is the repo-relative path of the resolved source map.
	SourcePath string
	Source     sourceMap
	Serving    []servingDesign
	// DoD holds the DOD-family unit IDs present in the source map (sorted).
	DoD []string
	// BuildsOn is the resolved design build-on graph — live design slug → the
	// slugs it builds on — read from the SAME computed graph the serving-design
	// resolution used (techdesigns.LiveGraph), so the compilability check reuses
	// T2's graph data and never re-derives it (design §3.9). The corpus-wide
	// supersession joins are already validated at resolution (a broken join is a
	// fail-closed `error` before any check runs); this exposes only the build-on
	// edges, whose cycle the compilability check adds (computeGraph flags
	// stale-builds-on but does not detect a build-on cycle).
	BuildsOn map[string][]string
	// Entries is the full live-design liveness graph (slug → computed state:
	// Status, Serves, BuildsOn, Flags) from the SAME techdesigns.LiveGraph call
	// the serving-design resolution makes — carried so the currency check (T4)
	// reads the shared stale-serves / stale-builds-on computation over the
	// feature's serving designs rather than re-deriving liveness (design §3.9,
	// prd:contract-gate/FR-2, NR-2). Every design (live and superseded) is
	// present, so a serving design's ref to a superseded target resolves here.
	Entries map[string]*techdesigns.IndexEntry
	// Inverse is the live serves-inverse (a Serves ref → the sorted live design
	// slugs serving it) from the SAME LiveGraph call — carried so the inverse-
	// coverage check (T4) reuses GenCoverage's requirement↔live-serves join
	// scoped to the feature, never a fork (design §3.9, prd:contract-gate/FR-3,
	// NR-2). A feature with no live serving design yields an empty slice for its
	// anchors, so every Must reds unserved (UB-5 — never vacuously clean).
	Inverse map[string][]string
	// Env is the read-only resolution environment (PRDUnits by feature → unit →
	// status, CorpusIDs) the graph was computed against — the coverage join's
	// requirement universe (the feature's live units) reads from it, so the
	// check runs the exact universe the tree-wide coverage view runs (NR-2).
	Env techdesigns.Env
}

// resolve builds the feature's assembled contract read-only from the committed
// machine faces, or returns a fail-closed verdict (never nil, never clean on
// bad input; design §3.11). The resolution order fails closed at each edge:
//
//   - no such feature ledger directory        → configuration-error (UB-3/UB-4)
//   - source map file missing/unreadable       → configuration-error (UB-1)
//   - source map present but unparseable        → error                (UB-2)
//   - a referenced serving-design file absent    → configuration-error (UB-1)
//   - the design graph fails to compute          → error
//
// It makes zero mutations (read-only; DINV-1): only os.ReadFile / os.Stat and
// pure computation, no write, git, or network (DINV-5).
func resolve(root, feature string, cfg config.Config) (*contract, *Verdict) {
	if feature == "" {
		v := configErr(feature, "(feature)", "no feature slug given", "name the feature whose contract to gate: docgate contract-gate <feature>")
		return nil, &v
	}

	// The feature's ledger directory must exist — an unmerged or nonexistent
	// contract has no ledger to gate (design §3.11; UB-3/UB-4).
	ledgerDir := filepath.Join(root, filepath.FromSlash(cfg.PRDs), feature)
	rel := filepath.ToSlash(filepath.Join(cfg.PRDs, feature))
	if info, err := os.Stat(ledgerDir); err != nil || !info.IsDir() {
		v := configErr(feature, rel,
			fmt.Sprintf("no PRD ledger for feature %q — the contract does not exist or is not merged", feature),
			"gate a merged feature: its ledger directory must exist under "+cfg.PRDs)
		return nil, &v
	}

	// The folded source map is the contract's substance face. Missing/unreadable
	// is a missing input (configuration-error); present-but-unparseable is a
	// malformed machine face (error). Both fail closed — never clean.
	srcRel := filepath.ToSlash(filepath.Join(cfg.PRDs, feature, "effective.source.json"))
	srcPath := filepath.Join(ledgerDir, "effective.source.json")
	b, err := os.ReadFile(srcPath)
	if err != nil {
		v := configErr(feature, srcRel,
			fmt.Sprintf("PRD source map is missing or unreadable: %v", err),
			"fold the ledger first: go run ./cmd/docgate fold "+rel)
		return nil, &v
	}
	var src sourceMap
	dec := json.NewDecoder(strings.NewReader(string(b)))
	dec.DisallowUnknownFields() // closed parse — a wrong-schema face fails closed
	if err := dec.Decode(&src); err != nil {
		v := malformedErr(feature, srcRel,
			fmt.Sprintf("PRD source map does not parse against the closed schema: %v", err),
			"the source map is a generated machine face — re-fold, never hand-edit it")
		return nil, &v
	}
	if src.Units == nil {
		v := malformedErr(feature, srcRel,
			"PRD source map carries no units map",
			"re-fold the ledger; a folded source map always carries its units")
		return nil, &v
	}

	c := &contract{Feature: feature, Root: root, Cfg: cfg, SourcePath: srcRel, Source: src}
	for id, u := range src.Units {
		if _, pre, ok := schema.ParseID(id); ok && pre == "DOD" {
			c.DoD = append(c.DoD, id)
		}
		_ = u
	}
	schema.SortIDs(c.DoD)

	// Resolve the live serving designs — those whose Serves targets one of the
	// feature's requirements, from the design index's computed live inverse
	// (single liveness truth; DINV-3). The design bodies are never re-parsed for
	// substance — only their headers, as the index already reads them. The same
	// call returns the live build-on graph the compilability check reuses.
	serving, graph, verr := resolveServingDesigns(root, feature, cfg)
	if verr != nil {
		return nil, verr
	}
	c.Serving = serving
	c.BuildsOn = graph.buildsOn
	c.Entries = graph.entries
	c.Inverse = graph.inverse
	c.Env = graph.env
	return c, nil
}

// resolvedGraph is the live-design graph data resolveServingDesigns computes
// once (from a single techdesigns.LiveGraph call) and the checks reuse: the
// build-on edges (compilability), the full live entries + inverse (currency,
// coverage), and the environment the graph was computed against (coverage's
// requirement universe). One computation, one liveness truth (AP-11; §3.9).
type resolvedGraph struct {
	buildsOn map[string][]string
	entries  map[string]*techdesigns.IndexEntry
	inverse  map[string][]string
	env      techdesigns.Env
}

// resolveServingDesigns computes the live designs serving the feature's
// requirements via techdesigns.LiveGraph (the same graph gen-index / coverage
// use). A referenced design file that cannot be read is a missing input
// (configuration-error); a graph-computation defect (e.g. a broken supersession
// join in the design corpus) is a malformed input (error). Read-only.
func resolveServingDesigns(root, feature string, cfg config.Config) ([]servingDesign, resolvedGraph, *Verdict) {
	designs, ddef := gate.LoadDesigns(root, cfg)
	// A design directory that cannot be read at all is a missing input.
	for _, d := range ddef {
		if d.Op == "read" {
			v := configErr(feature, d.File, "a technical-design input is missing or unreadable: "+d.Reason,
				"every serving design must be readable to resolve the contract")
			return nil, resolvedGraph{}, &v
		}
	}
	env := designEnv(root, cfg)
	entries, inverse, gdef := techdesigns.LiveGraph(designs, env)
	// A design-graph defect (broken supersession, cycle) means the contract's
	// design plane is malformed — fail closed as error.
	if len(gdef) > 0 {
		v := malformedErr(feature, cfg.TechDesigns, "the technical-design graph is malformed: "+gdef[0].Reason,
			"fix the design corpus (supersession joins) then re-gate")
		return nil, resolvedGraph{}, &v
	}

	prefix := "prd:" + feature + "/"
	fpBySlug := map[string]string{}
	for _, d := range designs {
		fpBySlug[d.Slug] = designFingerprint(root, cfg, d.Slug)
	}
	var out []servingDesign
	// buildsOn: the live design build-on graph the compilability check reuses —
	// read from the same computed entries, never re-derived (design §3.9).
	buildsOn := map[string][]string{}
	for slug, e := range entries {
		if e.Status != "live" {
			continue // a superseded design is not part of the live contract
		}
		if len(e.BuildsOn) > 0 {
			bo := append([]string(nil), e.BuildsOn...)
			sort.Strings(bo)
			buildsOn[slug] = bo
		}
		var hits []string
		for _, ref := range e.Serves {
			if strings.HasPrefix(ref, prefix) {
				hits = append(hits, ref)
			}
		}
		if len(hits) == 0 {
			continue
		}
		sort.Strings(hits)
		out = append(out, servingDesign{Slug: slug, Fingerprint: fpBySlug[slug], Serves: hits})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Slug < out[j].Slug })
	// Carry the full live graph (entries + inverse + env) the currency and
	// coverage checks reuse — the SAME single LiveGraph computation, never a
	// re-derivation (design §3.9, prd:contract-gate/FR-2/FR-3/NR-2).
	return out, resolvedGraph{buildsOn: buildsOn, entries: entries, inverse: inverse, env: env}, nil
}

// designFingerprint hashes a design file's normalized bytes — the design's
// content identity recorded in the verdict (AP-13). A design carries no
// fold-style fingerprint of its own, so the gate computes one deterministically
// (normalized bytes, sha256). An unreadable design yields "" (recorded
// honestly; the file-presence edge is caught upstream in LoadDesigns).
func designFingerprint(root string, cfg config.Config, slug string) string {
	path := filepath.Join(root, filepath.FromSlash(cfg.TechDesigns), slug+".md")
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(schema.Normalize(b))
	return "sha256:" + hex.EncodeToString(sum[:])
}

// designEnv assembles the read-only resolution inputs the design graph needs —
// grandfather sets, each ledger's unit→status source map, and the corpus-ID
// universe — with NO merge base (the gate is history-free and network-free,
// DINV-5). It mirrors the gate package's own designEnv but omits the git diff
// entirely: contract resolution reads only committed machine faces.
func designEnv(root string, cfg config.Config) techdesigns.Env {
	env := techdesigns.Env{
		Grandfathered:           map[string]bool{},
		PrecedenceGrandfathered: map[string]bool{},
		PRDUnits:                map[string]map[string]string{},
		CorpusIDs:               gate.CorpusIDs(root, cfg),
		NewDesigns:              map[string]bool{},
	}
	for _, s := range cfg.Grandfathered {
		env.Grandfathered[s] = true
	}
	for _, s := range cfg.PrecedenceGrandfathered {
		env.PrecedenceGrandfathered[s] = true
	}
	if dirs, err := gate.LedgerDirs(root, cfg); err == nil {
		for _, dir := range dirs {
			b, err := os.ReadFile(filepath.Join(dir, "effective.source.json"))
			if err != nil {
				continue
			}
			var s struct {
				Units map[string]struct {
					Status string `json:"status"`
				} `json:"units"`
			}
			if json.Unmarshal(b, &s) != nil {
				continue
			}
			units := map[string]string{}
			for id, u := range s.Units {
				units[id] = u.Status
			}
			env.PRDUnits[filepath.Base(dir)] = units
		}
	}
	env.DocExists = func(relp string) bool {
		_, err := os.Stat(filepath.Join(root, filepath.FromSlash(relp)))
		return err == nil
	}
	return env
}
