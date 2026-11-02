package gate_test

// The authority-contract structure arm as the author-independent backstop
// (docs/tech-design/argus-authority-contract-structure-arm.md §3.7; plan 002
// T2 / issue #221). The hardened attribution gate owns an honest residual — a
// raw-commit-access forger who hand-writes a correctly-bound Argus-Drive
// trailer — and it LEANS ON this arm to close the authority-contract evade
// path the trailer alone cannot: a structurally-illegal authority contract must
// red at this arm regardless of what author or trailer the commit carries, and
// this arm must stay a SEPARATE check whose governed set is disjoint from the
// immutability check (NR-2). The parse / DINV-3/4/5 evaluation and the
// composition properties are covered elsewhere; this file locks the two
// backstop properties the hardening depends on.

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// authGitRunner returns a git command runner bound to root that authors and
// commits under an explicit identity — so a test can vary the diff's author.
func authGitRunner(t *testing.T, root, authorName, authorEmail string) func(args ...string) {
	t.Helper()
	return func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME="+authorName, "GIT_AUTHOR_EMAIL="+authorEmail,
			"GIT_COMMITTER_NAME="+authorName, "GIT_COMMITTER_EMAIL="+authorEmail)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

// armDefects returns only the structure arm's defects from a full gate run,
// serialized and sorted so two runs are byte-comparable.
func armDefects(ds []schema.Defect) string {
	var arm []schema.Defect
	for _, d := range ds {
		if d.Verb == authArmVerb {
			arm = append(arm, d)
		}
	}
	return authSerialize(arm)
}

// TestAuthorityArmAuthorIndependent is issue #221's first acceptance criterion:
// the same structurally-illegal authority contract reds at the arm with a
// BYTE-IDENTICAL, non-empty defect set no matter who authored the commit or what
// authorship trailer it carries — a plain human, a forged `Argus-Drive` trailer
// (the raw-commit-forger residual the attribution gate hands off), and an
// operator attestation. The structural verdict is a function of the file's
// content alone (the arm reads the file as it stands, never a commit trailer or
// identity), so an evade of the authorship signal cannot land a structurally
// illegal authority contract.
func TestAuthorityArmAuthorIndependent(t *testing.T) {
	variants := []struct {
		name        string
		authorName  string
		authorEmail string
		trailer     string // an authorship-signal trailer appended to the commit message
	}{
		{"plain human commit", "Ada Human", "ada@example.com", ""},
		{"forged Argus-Drive trailer (the raw-commit-forger residual)", "Mal Forger", "mal@example.com", "\n\nArgus-Drive: some-problem-slug"},
		{"operator attestation trailer", "Op Erator", "op@example.com", "\n\nGoverning-Change: operator"},
		{"anonymous / mismatched identity", "x", "x@x", "\n\nArgus-Drive: another-slug\nGoverning-Change: operator"},
	}

	var want string
	for i, v := range variants {
		root := t.TempDir()
		git := authGitRunner(t, root, v.authorName, v.authorEmail)
		p := filepath.Join(root, filepath.FromSlash(schema.AuthorityContractPath))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(illegalContract), 0o644); err != nil {
			t.Fatal(err)
		}
		git("init", "-q", "-b", "main")
		git("add", ".")
		git("commit", "-qm", "seed the illegal authority contract"+v.trailer)

		got := armDefects(gate.Gate(root, "HEAD", nil, config.Defaults()))
		if strings.TrimSpace(got) == "" {
			t.Errorf("%s: the arm raised no defect on a structurally-illegal contract — the backstop did not bite", v.name)
			continue
		}
		if i == 0 {
			want = got
			continue
		}
		if got != want {
			t.Errorf("%s: the arm's verdict differs by author/trailer — it must depend on the file alone\n want:\n%s\n got:\n%s", v.name, want, got)
		}
	}
}

// TestAuthorityArmDisjointFromImmutable is issue #221's third acceptance
// criterion (the NR-2 half): the authority-contract structure arm and the
// immutability check govern DISJOINT sets. The immutability check governs only
// the PRD-ledger and technical-design corpora; the authority contract lives
// outside both, so a *modified* authority contract never draws an
// immutability-check defect — the structure arm is the sole layer inspecting
// its structure, exactly as the attribution hardening relies on.
func TestAuthorityArmDisjointFromImmutable(t *testing.T) {
	cfg := config.Defaults()

	// Disjoint by construction: the authority contract is outside the two
	// corpora the immutability check governs.
	ac := schema.AuthorityContractPath
	if strings.HasPrefix(ac, cfg.PRDs+"/") || strings.HasPrefix(ac, cfg.TechDesigns+"/") {
		t.Fatalf("the authority contract %q is inside the immutability check's governed set (%q / %q) — the governed sets are not disjoint",
			ac, cfg.PRDs, cfg.TechDesigns)
	}

	// And disjoint in behavior: modify the authority contract relative to a
	// merge base and confirm check-immutable stays silent on it while the
	// structure arm is what fires.
	root := t.TempDir()
	git := authGitRunner(t, root, "t", "t@t")
	write := func(rel, content string) {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	git("init", "-q", "-b", "main")
	write(schema.AuthorityContractPath, illegalContract)
	git("add", ".")
	git("commit", "-qm", "base: authority contract present")
	base, rdef := gate.ResolveMergeBase(root, "main")
	if len(rdef) > 0 {
		t.Fatalf("ResolveMergeBase: %v", rdef)
	}

	// Modify the authority contract on top of the base — the change status the
	// immutability check keys on (M/D/R). It must still not govern this path.
	write(schema.AuthorityContractPath, illegalContract+"\n<!-- an edit relative to the base -->\n")
	git("add", ".")
	git("commit", "-qm", "modify the authority contract")

	for _, d := range gate.CheckImmutable(root, base, "", cfg) {
		if strings.Contains(d.File, "authority-contract") {
			t.Errorf("the immutability check governed the authority contract %q — the governed sets must stay disjoint: %+v", d.File, d)
		}
	}

	// The structure arm is the layer that actually inspects it.
	armFired := false
	for _, d := range gate.Gate(root, base, nil, cfg) {
		if d.Verb == authArmVerb {
			armFired = true
		}
	}
	if !armFired {
		t.Errorf("the structure arm did not fire on the illegal authority contract — it must be the sole layer inspecting its structure")
	}
}
