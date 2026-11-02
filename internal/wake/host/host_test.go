package host

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/wake"
)

func TestLedgersList(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("docs/problems/beta/delivery.md", "# beta\n")
	write("docs/problems/alpha/delivery.md", "# alpha\n")
	write("docs/problems/gamma/problem.md", "# no delivery ledger here\n")
	write("docs/problems/stray.md", "# a file, not a problem dir\n")

	docs, err := Ledgers{Root: root}.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, d := range docs {
		paths = append(paths, d.Path)
	}
	want := []string{"docs/problems/alpha/delivery.md", "docs/problems/beta/delivery.md"}
	if strings.Join(paths, ",") != strings.Join(want, ",") {
		t.Fatalf("List = %v, want %v (stable order, ledgerless dirs skipped)", paths, want)
	}
}

func TestLedgersListNoProblemsRoot(t *testing.T) {
	docs, err := Ledgers{Root: t.TempDir()}.List(context.Background())
	if err != nil || docs != nil {
		t.Fatalf("want idle empty sweep for a repo without problems, got %v, %v", docs, err)
	}
}

func TestLedgersCustomPathAndUnreadableRoot(t *testing.T) {
	if got := (Ledgers{Problems: "custom/../problems"}).problemsRel(); got != "problems" {
		t.Fatalf("cleaned problems path = %q", got)
	}
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "problems"), []byte("not a directory"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := (Ledgers{Root: root, Problems: "problems"}).List(t.Context()); err == nil || !strings.Contains(err.Error(), "listing") {
		t.Fatalf("unreadable problems root error = %v", err)
	}
}

// scripted returns a Runner serving canned outputs keyed by a substring of
// "name args…", longest key first, and recording every invocation.
type scripted struct {
	calls     []string
	responses map[string]string
	errs      map[string]error
}

func (s *scripted) run(_ context.Context, name string, args ...string) ([]byte, error) {
	joined := name + " " + strings.Join(args, " ")
	s.calls = append(s.calls, joined)
	best, bestLen, isErr := "", -1, false
	for k := range s.responses {
		if strings.Contains(joined, k) && len(k) > bestLen {
			best, bestLen, isErr = k, len(k), false
		}
	}
	for k := range s.errs {
		if strings.Contains(joined, k) && len(k) > bestLen {
			best, bestLen, isErr = k, len(k), true
		}
	}
	if bestLen < 0 {
		return nil, fmt.Errorf("unscripted call: %s", joined)
	}
	if isErr {
		return nil, s.errs[best]
	}
	return []byte(s.responses[best]), nil
}

func TestProberResolveForms(t *testing.T) {
	p := Prober{GHRepo: "o/r"}
	for ref, want := range map[string][2]string{
		"7":       {"o/r", "7"},
		"#42":     {"o/r", "42"},
		"a/b#9":   {"a/b", "9"},
		" a/b#9 ": {"a/b", "9"},
	} {
		repo, num, err := p.resolve(ref)
		if err != nil || repo != want[0] || num != want[1] {
			t.Errorf("resolve(%q) = %s#%s, %v; want %s#%s", ref, repo, num, err, want[0], want[1])
		}
	}
	if _, _, err := p.resolve("junk ref"); err == nil {
		t.Error("want error for unrecognized ref form")
	}
	if _, _, err := (Prober{}).resolve("7"); err == nil {
		t.Error("want error for bare ref with no bound repo")
	}
}

func TestProberBoundaryFailures(t *testing.T) {
	if _, err := (Prober{}).AncestorOfMainline(t.Context(), "not-a-ref"); err == nil {
		t.Fatal("invalid proposal ref was accepted")
	}
	transport := errors.New("transport")
	p := Prober{GHRepo: "owner/repo", Run: func(context.Context, string, ...string) ([]byte, error) {
		return nil, transport
	}}
	if _, err := p.AncestorOfMainline(t.Context(), "1"); !errors.Is(err, transport) {
		t.Fatalf("proposal transport error = %v", err)
	}
	p.Run = func(context.Context, string, ...string) ([]byte, error) { return []byte("{"), nil }
	if _, err := p.AncestorOfMainline(t.Context(), "1"); err == nil || !strings.Contains(err.Error(), "decoding") {
		t.Fatalf("proposal decode error = %v", err)
	}
	calls := 0
	p.Run = func(context.Context, string, ...string) ([]byte, error) {
		calls++
		if calls == 1 {
			return []byte(`{"merged":true,"merge_commit_sha":"abc"}`), nil
		}
		return nil, transport
	}
	if _, err := p.AncestorOfMainline(t.Context(), "1"); !errors.Is(err, transport) || !strings.Contains(err.Error(), "fetching") {
		t.Fatalf("mainline fetch error = %v", err)
	}
	if _, err := (Prober{}).RecordExists(t.Context(), "not-a-ref"); err == nil {
		t.Fatal("invalid item ref was accepted")
	}
}

func TestAncestorOfMainline(t *testing.T) {
	t.Run("merged and ancestor", func(t *testing.T) {
		s := &scripted{responses: map[string]string{
			"pulls/7":    `{"merged":true,"merge_commit_sha":"mc"}`,
			"fetch":      "",
			"merge-base": "",
		}}
		p := Prober{Root: ".", GHRepo: "o/r", Run: s.run}
		got, err := p.AncestorOfMainline(context.Background(), "7")
		if err != nil || !got {
			t.Fatalf("want true, got %v, %v", got, err)
		}
	})
	t.Run("merged badge, content not an ancestor", func(t *testing.T) {
		s := &scripted{
			responses: map[string]string{"pulls/7": `{"merged":true,"merge_commit_sha":"mc"}`, "fetch": ""},
			errs:      map[string]error{"merge-base": errors.New("exit 1")},
		}
		p := Prober{Root: ".", GHRepo: "o/r", Run: s.run}
		got, err := p.AncestorOfMainline(context.Background(), "7")
		if err != nil || got {
			t.Fatalf("want false for lying badge, got %v, %v", got, err)
		}
	})
	t.Run("unmerged", func(t *testing.T) {
		s := &scripted{responses: map[string]string{"pulls/7": `{"merged":false}`}}
		p := Prober{Root: ".", GHRepo: "o/r", Run: s.run}
		got, err := p.AncestorOfMainline(context.Background(), "7")
		if err != nil || got {
			t.Fatalf("want false for unmerged, got %v, %v", got, err)
		}
	})

	// The ancestry read fetches and resolves against the CONFIGURED remote/branch,
	// not a baked origin/main — the config-driven-portability lift (FR-2, DINV-1).
	// A default (blank) Prober is byte-identical to the old literals; a configured
	// non-default pair changes both the fetch and the merge-base ref.
	t.Run("resolves the configured remote and branch", func(t *testing.T) {
		cases := []struct {
			name, remote, branch, wantFetch, wantRef string
		}{
			{"default blanks resolve origin/main", "", "", "fetch --no-tags -q origin main", "merge-base --is-ancestor mc origin/main"},
			{"configured non-default", "upstream", "trunk", "fetch --no-tags -q upstream trunk", "merge-base --is-ancestor mc upstream/trunk"},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				s := &scripted{responses: map[string]string{
					"pulls/7":    `{"merged":true,"merge_commit_sha":"mc"}`,
					"fetch":      "",
					"merge-base": "",
				}}
				p := Prober{Root: ".", GHRepo: "o/r", Remote: tc.remote, Branch: tc.branch, Run: s.run}
				if _, err := p.AncestorOfMainline(context.Background(), "7"); err != nil {
					t.Fatal(err)
				}
				var sawFetch, sawRef bool
				for _, c := range s.calls {
					if strings.Contains(c, tc.wantFetch) {
						sawFetch = true
					}
					if strings.Contains(c, tc.wantRef) {
						sawRef = true
					}
				}
				if !sawFetch {
					t.Errorf("no fetch of the configured mainline; calls=%v want substring %q", s.calls, tc.wantFetch)
				}
				if !sawRef {
					t.Errorf("merge-base did not target the configured mainline ref; calls=%v want substring %q", s.calls, tc.wantRef)
				}
			})
		}
	})
}

func TestRecordExists(t *testing.T) {
	t.Run("record exists", func(t *testing.T) {
		s := &scripted{responses: map[string]string{"issues/500": `{"number":500}`}}
		p := Prober{GHRepo: "o/r", Run: s.run}
		got, err := p.RecordExists(context.Background(), "500")
		if err != nil || !got {
			t.Fatalf("want true, got %v, %v", got, err)
		}
	})
	t.Run("absent record is a clean false, not an error", func(t *testing.T) {
		s := &scripted{errs: map[string]error{"issues/9": errors.New("gh: Not Found (HTTP 404)")}}
		p := Prober{GHRepo: "o/r", Run: s.run}
		got, err := p.RecordExists(context.Background(), "9")
		if err != nil || got {
			t.Fatalf("want clean false for 404, got %v, %v", got, err)
		}
	})
	t.Run("infrastructure fault surfaces as error", func(t *testing.T) {
		s := &scripted{errs: map[string]error{"issues/9": errors.New("network unreachable")}}
		p := Prober{GHRepo: "o/r", Run: s.run}
		if _, err := p.RecordExists(context.Background(), "9"); err == nil {
			t.Fatal("want error for non-404 fault")
		}
	})
}

func TestDoorTouch(t *testing.T) {
	sig, ok := wake.NewSignal("my-problem", "merge")
	if !ok {
		t.Fatal("fixture signal invalid")
	}
	t.Run("dry-run records, executes nothing", func(t *testing.T) {
		d := &Door{Argv: []string{"echo"}, DryRun: true, Run: func(context.Context, string, ...string) ([]byte, error) {
			t.Fatal("dry-run must not execute")
			return nil, nil
		}}
		if err := d.Touch(context.Background(), sig); err != nil {
			t.Fatal(err)
		}
		if len(d.Invoked) != 1 || d.Invoked[0] != "wake:my-problem:merge" {
			t.Fatalf("Invoked = %v", d.Invoked)
		}
	})
	t.Run("delivery appends exactly the wire token as one argument", func(t *testing.T) {
		var gotName string
		var gotArgs []string
		d := &Door{Argv: []string{"claude", "-p"}, Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
			gotName, gotArgs = name, args
			return nil, nil
		}}
		if err := d.Touch(context.Background(), sig); err != nil {
			t.Fatal(err)
		}
		if gotName != "claude" || strings.Join(gotArgs, " ") != "-p wake:my-problem:merge" {
			t.Fatalf("invoked %s %v", gotName, gotArgs)
		}
	})
	t.Run("no configured command is a named error", func(t *testing.T) {
		d := &Door{}
		if err := d.Touch(context.Background(), sig); err == nil {
			t.Fatal("want error for missing front-door command")
		}
	})
}
