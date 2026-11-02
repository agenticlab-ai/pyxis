package lock

import (
	"context"
	"fmt"
	"path"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/configlayer"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

// captureProblemLedgers retains exact source bindings in the native generated
// commit. It selects only this feature's two ledger paths from the reconciler;
// unrelated actions and all remaining defects stay for the full memory gate.
// Inputs come from the rebased immutable tree, never from an author-supplied
// envelope or the original worktree. An absent ledger has no capture obligation.
func captureProblemLedgers(ctx context.Context, root string, repo *gitsrc.Repo, head string, feature, issuer string) ([]string, error) {
	// The rebase may have changed repository source paths. Read its exact
	// configuration, rather than the caller's pre-fetch or local identity layer.
	configEntries, err := repo.ListTree(ctx, head, configlayer.GlobalPath)
	if err != nil {
		return nil, fmt.Errorf("list pinned repository config: %w", err)
	}
	for _, entry := range configEntries {
		if entry.Path != configlayer.GlobalPath || !entry.Regular() {
			return nil, fmt.Errorf("pinned repository config is not a regular file")
		}
	}
	configFiles, err := repo.Files(ctx, head, []string{configlayer.GlobalPath})
	if err != nil {
		return nil, fmt.Errorf("read pinned repository config: %w", err)
	}
	cfg, defects := config.Parse(configFiles[configlayer.GlobalPath])
	if len(defects) > 0 {
		return nil, fmt.Errorf("pinned repository config: %s", defects[0].String())
	}
	problems := path.Clean(strings.ReplaceAll(cfg.Problems, "\\", "/"))
	opts := reconcile.Options{
		ProductVision: cfg.ProductVision, ArchitecturePrinciples: cfg.ArchitecturePrinciples,
		Incidents: cfg.Incidents, PRDs: cfg.PRDs, TechDesigns: cfg.TechDesigns,
		Plans: cfg.Plans, Brainstorms: cfg.Brainstorms, Problems: problems,
		Reviews: cfg.Reviews, Feedback: cfg.Feedback,
		Grandfathered:           append([]string(nil), cfg.Grandfathered...),
		PrecedenceGrandfathered: append([]string(nil), cfg.PrecedenceGrandfathered...),
	}
	snapshot, err := reconcile.BuildSnapshot(ctx, repo, head, opts)
	if err != nil {
		return nil, err
	}
	set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: head, Prefix: store.Dir})
	if err != nil {
		return nil, err
	}
	if len(set.Problems) > 0 || len(set.DocumentIssues) > 0 || len(set.Dangling) > 0 {
		return nil, fmt.Errorf("source memory has unresolved store defects")
	}
	report := reconcile.Analyze(snapshot, set, resolve.New(set), opts)
	own := map[string]bool{
		path.Join(snapshot.SourceOptions.Problems, feature, "problem.md"):  true,
		path.Join(snapshot.SourceOptions.Problems, feature, "delivery.md"): true,
	}
	for _, finding := range report.Findings {
		if own[finding.Scope] && finding.Blocking && !finding.Writable {
			return nil, fmt.Errorf("%s at %s: %s", finding.Code, finding.Scope, finding.Detail)
		}
	}
	// The introduction witness binds these exact regular Git bytes, without
	// retaining a disposable topic commit as a mainline provenance pin. The
	// verifier rederives the witness at the generated record's introduction.
	evidence := reconcile.EvidenceSnapshot{
		Commit: head, Entries: map[string]gitsrc.TreeEntry{}, Files: map[string][]byte{},
	}
	for p := range own {
		if file, ok := snapshot.Files[p]; ok {
			evidence.Entries[p] = gitsrc.TreeEntry{Path: p, Mode: file.Mode, Type: "blob"}
			evidence.Files[p] = file.Body
		}
	}
	var entries []compile.Record
	for _, action := range report.Actions {
		if action.Code == "capture-document" && own[action.Scope] {
			draft := action.Draft
			witness, err := reconcile.CaptureEvidence(&record.Record{Provenance: draft.Provenance}, set, evidence, record.EvidenceIntroduction)
			if err != nil {
				return nil, err
			}
			draft.Provenance.Commit = ""
			draft.Provenance.Evidence = witness
			entries = append(entries, compile.Record{Draft: draft})
		}
	}
	if len(entries) == 0 {
		return nil, nil
	}
	_, paths, err := issueRecords(root, issuer, compile.Result{Feature: feature, Records: entries}, readiness.Sources{}, nil, 1)
	return paths, err
}
