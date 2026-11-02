package lock

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	memorycli "github.com/agenticlab-ai/pyxis/internal/memory/cli"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/reconcile"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
)

func TestPrepareCapturesOnlyItsPinnedProblemLedgers(t *testing.T) {
	for _, tc := range []struct {
		configured, canonical string
		productionAttributes  bool
	}{
		{"docs/problems", "docs/problems", false},
		{"factory/problems", "factory/problems", false},
		{"./factory/problems", "factory/problems", false},
		{"docs/problems", "docs/problems", true},
	} {
		name := tc.configured
		if tc.productionAttributes {
			name += "-production-attributes"
		}
		t.Run(name, func(t *testing.T) {
			work, cfg, host := prepareFixture(t, true)
			cfg.Problems = tc.configured
			problemsRoot := tc.canonical
			fixtureFile(t, work, ".pyxis/config.yaml", []byte("paths:\n  problems: "+tc.configured+"\n"))
			bodies := map[string][]byte{
				problemsRoot + "/widget/problem.md":  []byte("# Problem\r\n\r\nLiteral {{unknown}} source.\r\n"),
				problemsRoot + "/widget/delivery.md": []byte("# Delivery\n\nKeep this journal.\n"),
			}
			for p, body := range bodies {
				fixtureFile(t, work, p, body)
			}
			other := problemsRoot + "/other/problem.md"
			fixtureFile(t, work, other, []byte("# Another problem\n"))
			gitFixture(t, work, "add", "--", problemsRoot, ".pyxis/config.yaml")
			if tc.productionAttributes {
				// Retain an already-staged CRLF source while introducing the
				// real repository attributes, as a legacy source can do.
				attrs, err := os.ReadFile("../../../.gitattributes")
				if err != nil {
					t.Fatal(err)
				}
				fixtureFile(t, work, ".gitattributes", attrs)
				gitFixture(t, work, "add", "--", ".gitattributes")
			}
			gitFixture(t, work, "commit", "-m", "add problem sources")
			gitFixture(t, work, "push", "origin", "draft/widget")
			gateCalls := 0
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "go" && len(args) > 1 && args[1] == "./cmd/memory-gate" {
					gateCalls++
					var output, diagnostic bytes.Buffer
					env := memorycli.Env{Dir: dir, Stdout: &output, Stderr: &diagnostic, Now: time.Now, Entropy: record.Entropy, Getenv: os.Getenv}
					if code := memorycli.Run(ctx, []string{"evidence", "--check", "--base", args[3], "--json"}, env); code != 0 {
						return nil, fmt.Errorf("native durable evidence refused: %s %s", output.String(), diagnostic.String())
					}
					repo, err := gitsrc.Open(ctx, dir)
					if err != nil {
						return nil, err
					}
					head, err := repo.Head(ctx)
					if err != nil {
						return nil, err
					}
					set, err := store.Load(ctx, gitsrc.TreeSource{Repo: repo, Ref: head, Prefix: store.Dir})
					if err != nil {
						return nil, err
					}
					if len(set.Problems) != 0 {
						return nil, fmt.Errorf("invalid captured pairs: %v", set.Problems)
					}
					found := map[string]bool{}
					for id, doc := range set.Records {
						if doc.Provenance.Source == other {
							return nil, fmt.Errorf("captured another problem's source")
						}
						body, wanted := bodies[doc.Provenance.Source]
						if !wanted {
							continue
						}
						family := strings.TrimSuffix(filepath.Base(doc.Provenance.Source), ".md")
						if doc.Subject != "problem/widget" || doc.Family != family || !doc.LiteralBody || doc.Provenance.Commit != "" || doc.Provenance.Evidence == nil || doc.Provenance.Evidence.Locator != record.EvidenceIntroduction || !bytes.Equal(set.Bodies[id], body) {
							return nil, fmt.Errorf("capture does not bind the pinned literal source: %+v", doc)
						}
						found[doc.Provenance.Source] = true
					}
					if len(found) != len(bodies) {
						return nil, fmt.Errorf("memory gate: missing own problem/delivery bindings")
					}
					return []byte("tested exact own ledger bindings\n"), nil
				}
				return host.run(ctx, dir, env, name, args...)
			}
			prepared, err := Prepare(t.Context(), work, cfg, "widget", run)
			if err != nil {
				t.Fatal(err)
			}
			if gateCalls != 1 || host.pushes != 1 || host.reviews != 1 {
				t.Fatalf("gate=%d pushes=%d reviews=%d", gateCalls, host.pushes, host.reviews)
			}
			if body := gitFixture(t, work, "log", "-1", "--format=%B"); strings.Contains(body, "Argus-Drive:") {
				t.Fatal("native generated commit was stamped as an authored drive commit")
			}
			changed := strings.Fields(gitFixture(t, work, "diff", "--name-only", prepared.Head+"^", prepared.Head))
			for p := range bodies {
				if slices.Contains(changed, p) {
					t.Fatalf("preparation changed ledger source %s", p)
				}
			}
			if tc.productionAttributes {
				checkout := t.TempDir()
				gitFixture(t, work, "worktree", "add", "--detach", checkout, prepared.Head)
				set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(checkout, store.Dir)})
				if err != nil || len(set.Problems) != 0 || len(set.DocumentIssues) != 0 || len(set.Dangling) != 0 {
					t.Fatalf("fresh checkout has invalid Document pairs: set=%+v error=%v", set, err)
				}
				found := 0
				for id, doc := range set.Records {
					if body, ok := bodies[doc.Provenance.Source]; ok {
						found++
						if !bytes.Equal(set.Bodies[id], body) {
							t.Fatalf("fresh checkout changed literal ledger body %s", doc.Provenance.Source)
						}
					}
				}
				if found != len(bodies) {
					t.Fatalf("fresh checkout retained %d/%d ledger bindings", found, len(bodies))
				}
			}
		})
	}
}

func TestPrepareLedgerCaptureRefusalPreservesAuthorBranch(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	fixtureFile(t, work, "docs/problems/widget/problem.md", []byte("# Problem\n"))
	gitFixture(t, work, "add", "--", "docs/problems/widget/problem.md")
	gitFixture(t, work, "commit", "-m", "add problem source")
	gitFixture(t, work, "push", "origin", "draft/widget")
	before := gitFixture(t, work, "rev-parse", "HEAD")
	stop := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name == "go" {
			return nil, fmt.Errorf("unrelated memory defect remains")
		}
		return host.run(ctx, dir, env, name, args...)
	}
	if _, err := Prepare(t.Context(), work, cfg, "widget", stop); err == nil || !strings.Contains(err.Error(), "unrelated memory defect remains") {
		t.Fatalf("error = %v", err)
	}
	if after := gitFixture(t, work, "rev-parse", "HEAD"); after != before || host.pushes != 0 || host.reviews != 0 {
		t.Fatalf("refused preparation advanced author/host state: %s", after)
	}
	if dirty := gitFixture(t, work, "status", "--porcelain"); dirty != "" {
		t.Fatalf("refused preparation dirtied the author tree: %s", dirty)
	}
	if matches, err := filepath.Glob(filepath.Join(work, store.Dir, "problem/widget/Document-*")); err != nil || len(matches) != 0 {
		t.Fatalf("temporary captures leaked into the author tree: %v, %v", matches, err)
	}
}

func TestLedgerCaptureReusesExactAndSupersedesStaleBindings(t *testing.T) {
	work, cfg, _ := prepareFixture(t, true)
	p := "docs/problems/widget/problem.md"
	fixtureFile(t, work, p, []byte("# Original source\n"))
	gitFixture(t, work, "add", "--", p)
	gitFixture(t, work, "commit", "-m", "add source")
	repo, err := gitsrc.Open(t.Context(), work)
	if err != nil {
		t.Fatal(err)
	}
	capture := func() []string {
		t.Helper()
		head := gitFixture(t, work, "rev-parse", "HEAD")
		paths, err := captureProblemLedgers(t.Context(), work, repo, head, "widget", "agent:agent")
		if err != nil {
			t.Fatal(err)
		}
		return paths
	}
	first := capture()
	if len(first) != 2 {
		t.Fatalf("initial capture paths = %v", first)
	}
	gitFixture(t, work, append([]string{"add", "--"}, first...)...)
	gitFixture(t, work, "commit", "-m", "retain initial binding")
	if reused := capture(); len(reused) != 0 {
		t.Fatalf("exact binding was duplicated: %v", reused)
	}
	old := map[string][]byte{}
	for _, p := range first {
		body, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(p)))
		if err != nil {
			t.Fatal(err)
		}
		old[p] = body
	}
	fixtureFile(t, work, p, []byte("# Revised source\n"))
	gitFixture(t, work, "add", "--", p)
	gitFixture(t, work, "commit", "-m", "revise source")
	second := capture()
	if len(second) != 2 {
		t.Fatalf("successor capture paths = %v", second)
	}
	for p, before := range old {
		after, err := os.ReadFile(filepath.Join(work, filepath.FromSlash(p)))
		if err != nil || !bytes.Equal(before, after) {
			t.Fatalf("capture rewrote immutable %s: %v", p, err)
		}
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(work, store.Dir)})
	if err != nil || len(set.Problems) != 0 {
		t.Fatalf("successor pairs are invalid: set=%+v, error=%v", set, err)
	}
	projection := resolve.New(set)
	if len(projection.Violations) != 0 || len(projection.Conflicts()) != 0 {
		t.Fatalf("invalid source lineage: violations=%v conflicts=%v", projection.Violations, projection.Conflicts())
	}
	head := projection.Head("problem/widget", "problem")
	if head == nil || len(head.Rel(record.Supersedes)) != 1 || !bytes.Equal(set.Bodies[head.ID], []byte("# Revised source\n")) {
		t.Fatalf("successor lost source supersession: %+v", head)
	}
	for _, finding := range ownSourceReport(t, work, cfg.Problems).Findings {
		if finding.Scope == p {
			t.Errorf("captured source still has %s", finding.Code)
		}
	}
}

func TestLedgerCaptureRejectsIrregularSources(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	p := "docs/problems/widget/problem.md"
	fixtureFile(t, work, "elsewhere.md", []byte("# Not a ledger source\n"))
	if err := os.MkdirAll(filepath.Dir(filepath.Join(work, p)), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("../../../elsewhere.md", filepath.Join(work, p)); err != nil {
		t.Fatal(err)
	}
	gitFixture(t, work, "add", "--", p, "elsewhere.md")
	gitFixture(t, work, "commit", "-m", "add irregular ledger")
	gitFixture(t, work, "push", "origin", "draft/widget")
	before := gitFixture(t, work, "rev-parse", "HEAD")
	if _, err := Prepare(t.Context(), work, cfg, "widget", host.run); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("irregular source error = %v", err)
	}
	if after := gitFixture(t, work, "rev-parse", "HEAD"); after != before || host.pushes != 0 || host.reviews != 0 {
		t.Fatalf("irregular source advanced author/host state: %s", after)
	}
}

func TestLedgerCaptureRefusesConflictingMemory(t *testing.T) {
	for _, defect := range []string{"source-address-collision", "source-mode-conflict", "invalid-memory"} {
		t.Run(defect, func(t *testing.T) {
			work, _, _ := prepareFixture(t, true)
			p := "docs/problems/widget/problem.md"
			fixtureFile(t, work, p, []byte("# Current problem\n"))
			if defect == "invalid-memory" {
				fixtureFile(t, work, "memory/problem/widget/Document-invalid.json", []byte("{}\n"))
			} else {
				source := p
				if defect == "source-address-collision" {
					source = "docs/wiki/other.md"
					fixtureFile(t, work, source, []byte("# Other source\n"))
				}
				at := time.Now().UTC().Add(-time.Minute)
				doc := &record.Record{
					Schema: record.Schema, Kind: record.Document,
					ID:      record.NewID(record.Document, at, [10]byte{}),
					Subject: "problem/widget", Family: "problem", Statement: "Existing document",
					Issuer: "fixture", CreatedAt: at.Format(time.RFC3339),
					LiteralBody: defect != "source-mode-conflict",
					Provenance:  record.Provenance{Source: source},
				}
				if _, err := store.WriteDocument(filepath.Join(work, store.Dir), doc, []byte("# Old document\n")); err != nil {
					t.Fatal(err)
				}
			}
			gitFixture(t, work, "add", "--", "docs", "memory")
			gitFixture(t, work, "commit", "-m", "add conflicting source binding")
			before := gitFixture(t, work, "rev-parse", "HEAD")
			repo, err := gitsrc.Open(t.Context(), work)
			if err != nil {
				t.Fatal(err)
			}
			paths, err := captureProblemLedgers(t.Context(), work, repo, before, "widget", "agent:agent")
			want := defect
			if defect == "invalid-memory" {
				want = "unresolved store defects"
			}
			if err == nil || !strings.Contains(err.Error(), want) || len(paths) != 0 {
				t.Fatalf("paths=%v error=%v, want refusal %s", paths, err, want)
			}
			if dirty := gitFixture(t, work, "status", "--porcelain"); dirty != "" {
				t.Fatalf("capture wrote through a conflict: %s", dirty)
			}
		})
	}
}

func TestLedgerCaptureNormalizesLegacyRoot(t *testing.T) {
	work, _, _ := prepareFixture(t, true)
	fixtureFile(t, work, ".pyxis/config.yaml", []byte("paths:\n  problems: ./factory/problems\n"))
	fixtureFile(t, work, "factory/problems/widget/problem.md", []byte("# Problem\n"))
	gitFixture(t, work, "add", "--", "factory/problems/widget/problem.md", ".pyxis/config.yaml")
	gitFixture(t, work, "commit", "-m", "add relocated problem")
	repo, err := gitsrc.Open(t.Context(), work)
	if err != nil {
		t.Fatal(err)
	}
	head := gitFixture(t, work, "rev-parse", "HEAD")
	paths, err := captureProblemLedgers(t.Context(), work, repo, head, "widget", "agent:agent")
	if err != nil || len(paths) != 2 {
		t.Fatalf("paths=%v error=%v", paths, err)
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(work, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	doc := resolve.New(set).Head("problem/widget", "problem")
	if doc == nil || doc.Provenance.Source != "factory/problems/widget/problem.md" {
		t.Fatalf("legacy root did not retain the native problem address: %+v", doc)
	}
}

func TestPrepareLedgerCaptureUsesRebasedConfiguration(t *testing.T) {
	work, cfg, host := prepareFixture(t, true)
	oldPath := "docs/problems/widget/problem.md"
	fixtureFile(t, work, oldPath, []byte("# Obsolete root\n"))
	gitFixture(t, work, "add", "--", oldPath)
	gitFixture(t, work, "commit", "-m", "add author ledger at original root")
	gitFixture(t, work, "push", "origin", "draft/widget")
	main := t.TempDir()
	gitFixture(t, work, "worktree", "add", main, "main")
	currentPath := "factory/problems/widget/problem.md"
	currentBody := []byte("# Current mainline root\n")
	fixtureFile(t, main, ".pyxis/config.yaml", []byte("paths:\n  problems: factory/problems\n"))
	fixtureFile(t, main, currentPath, currentBody)
	gitFixture(t, main, "add", "--", ".pyxis/config.yaml", currentPath)
	gitFixture(t, main, "commit", "-m", "relocate configured problem root on mainline")
	gitFixture(t, main, "push", "origin", "main")
	gate := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
		if name != "go" {
			return host.run(ctx, dir, env, name, args...)
		}
		set, err := store.Load(ctx, store.DirSource{Root: filepath.Join(dir, store.Dir)})
		if err != nil {
			return nil, err
		}
		for _, doc := range set.Records {
			if doc.Provenance.Source == oldPath {
				return nil, fmt.Errorf("captured stale pre-fetch problem root")
			}
		}
		doc := resolve.New(set).Head("problem/widget", "problem")
		if doc == nil || doc.Provenance.Source != currentPath || !bytes.Equal(set.Bodies[doc.ID], currentBody) {
			return nil, fmt.Errorf("missing current rebased-root ledger binding")
		}
		return nil, nil
	}
	if _, err := Prepare(t.Context(), work, cfg, "widget", gate); err != nil {
		t.Fatal(err)
	}
	if host.pushes != 1 || host.reviews != 1 {
		t.Fatalf("pushes=%d reviews=%d", host.pushes, host.reviews)
	}
}

func TestLedgerCaptureRefusesInvalidPinnedConfiguration(t *testing.T) {
	for _, defect := range []string{"malformed", "symlink", "canceled"} {
		t.Run(defect, func(t *testing.T) {
			work, _, _ := prepareFixture(t, true)
			fixtureFile(t, work, "docs/problems/widget/problem.md", []byte("# Problem\n"))
			if defect == "symlink" {
				fixtureFile(t, work, "elsewhere.yaml", []byte("paths:\n  problems: factory/problems\n"))
				if err := os.MkdirAll(filepath.Join(work, ".pyxis"), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("../elsewhere.yaml", filepath.Join(work, ".pyxis/config.yaml")); err != nil {
					t.Fatal(err)
				}
			} else {
				body := []byte("paths:\n  unknown: docs/problems\n")
				if defect == "canceled" {
					body = []byte("paths:\n  problems: docs/problems\n")
				}
				fixtureFile(t, work, ".pyxis/config.yaml", body)
			}
			gitFixture(t, work, "add", "--", ".pyxis/config.yaml", "docs/problems/widget/problem.md")
			gitFixture(t, work, "commit", "-m", "add pinned config refusal fixture")
			head := gitFixture(t, work, "rev-parse", "HEAD")
			repo, err := gitsrc.Open(t.Context(), work)
			if err != nil {
				t.Fatal(err)
			}
			ctx := t.Context()
			if defect == "canceled" {
				var cancel context.CancelFunc
				ctx, cancel = context.WithCancel(ctx)
				cancel()
			}
			paths, err := captureProblemLedgers(ctx, work, repo, head, "widget", "agent:agent")
			if err == nil || !strings.Contains(err.Error(), "pinned repository config") || len(paths) != 0 {
				t.Fatalf("paths=%v error=%v", paths, err)
			}
			if matches, err := filepath.Glob(filepath.Join(work, store.Dir, "problem/widget/Document-*")); err != nil || len(matches) != 0 {
				t.Fatalf("config refusal wrote captures: %v, %v", matches, err)
			}
		})
	}
}

func ownSourceReport(t *testing.T, root, problems string) reconcile.Report {
	// This helper uses the actual reconciler for the capture-only regression.
	// The complete source memory gate remains a separate required Prepare step.
	t.Helper()
	repo, err := gitsrc.Open(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	head, err := repo.Head(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	opts := reconcile.Options{Problems: problems}
	snap, err := reconcile.BuildSnapshot(t.Context(), repo, head, opts)
	if err != nil {
		t.Fatal(err)
	}
	set, err := store.Load(t.Context(), store.DirSource{Root: filepath.Join(root, store.Dir)})
	if err != nil {
		t.Fatal(err)
	}
	return reconcile.Analyze(snap, set, resolve.New(set), opts)
}
