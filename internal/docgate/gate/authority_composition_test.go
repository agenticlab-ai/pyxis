package gate_test

// The Argus authority-contract structure arm's composition proofs
// (docs/tech-design/argus-authority-contract-structure-arm.md §3.3, §3.9; plan
// T2). The parse and DINV-3/4/5 evaluation are covered in-process by the
// authoritycontract package's own tests; this file proves the *composition*
// properties: the arm self-registers from its own descriptor with no
// hand-edited umbrella line (A-4/A-6), runs base-free on every gate path
// (DINV-5), leaves every non-authority-contract tree's verdict byte-identical
// (A-4/DINV-4), and passes the repo's own real authority contract.

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const authArmVerb = "check-authority-contract"

// an intentionally illegal authority contract: current tier-4 with a wiped
// change history (the canonical bare-pointer-flip self-elevation) — its defect
// proves the arm actually ran.
const illegalContract = "# Argus authority contract — test\n\n" +
	"| Field | Value |\n|---|---|\n" +
	"| **Current tier** | tier-4 |\n" +
	"| **Template version** | authority-contract/v1 |\n\n" +
	"## The ladder\n\n" +
	"| Tier (ordered) | Bundle (action classes) |\n|---|---|\n" +
	"| `revoked` | ∅ — every act escalates; the kill switch |\n" +
	"| `floor` | dispose-advisory-finding |\n" +
	"| `tier-4` | + approve-merge |\n\n" +
	"## Change history\n\n" +
	"| Date | Move | Tier | Evidence |\n|---|---|---|---|\n"

func authTempRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	for rel, content := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q", "-b", "main")
	git("add", ".")
	git("commit", "-qm", "base")
	return root
}

func authSerialize(ds []schema.Defect) string {
	lines := make([]string, 0, len(ds))
	for _, d := range ds {
		lines = append(lines, strings.Join([]string{d.Verb, d.File, d.Op, d.Reason, d.Remedy}, "\x1f"))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestAuthorityArmRegisteredFromDescriptor proves A-6: the arm is in the check
// registry, base-free, with a non-empty help line the umbrella help walk
// renders, and appears in the verb-sorted enumeration both the umbrella and CLI
// walk — nothing hand-listed.
func TestAuthorityArmRegisteredFromDescriptor(t *testing.T) {
	c, ok := gate.Lookup(authArmVerb)
	if !ok {
		t.Fatalf("%s is not registered", authArmVerb)
	}
	if c.NeedsMergeBase {
		t.Errorf("%s must be base-free (NeedsMergeBase false)", authArmVerb)
	}
	if strings.TrimSpace(c.Help) == "" {
		t.Errorf("%s: empty help line", authArmVerb)
	}
	seen := false
	for _, ch := range gate.Checks() {
		if ch.Verb == authArmVerb {
			seen = true
		}
	}
	if !seen {
		t.Errorf("%s must appear in gate.Checks() (the derived enumeration)", authArmVerb)
	}
}

// TestAuthorityArmNoHandEditedLine proves A-4/A-6: the umbrella's shared
// enumeration, the registry, and the CLI dispatch/help carry NO hand-edited
// line naming the arm — the verb lives only in its own descriptor file (AP-27).
func TestAuthorityArmNoHandEditedLine(t *testing.T) {
	root := repoRootForTest(t)
	for _, rel := range []string{
		"internal/docgate/gate/gate.go",
		"internal/docgate/gate/registry.go",
		"cmd/docgate/main.go",
	} {
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatalf("reading %s: %v", rel, err)
		}
		if strings.Contains(string(b), authArmVerb) {
			t.Errorf("%s hand-lists %q — the umbrella must assemble the arm from its descriptor only", rel, authArmVerb)
		}
	}
	desc := "internal/docgate/gate/reg_check_authority_contract.go"
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(desc)))
	if err != nil {
		t.Fatalf("reading %s: %v", desc, err)
	}
	if !strings.Contains(string(b), authArmVerb) {
		t.Errorf("%s does not define its own verb %q", desc, authArmVerb)
	}
}

// TestAuthorityArmAdditiveOnNonAuthorityTree proves A-4/DINV-4: on a tree with
// no authority contract, the gate's verdict is byte-identical with and without
// the arm (it draws nothing), so no non-authority verdict is perturbed.
func TestAuthorityArmAdditiveOnNonAuthorityTree(t *testing.T) {
	root := authTempRepo(t, map[string]string{
		"docs/wiki/note.md": "An ordinary note. No authority contract here.\n",
	})
	with := gate.Gate(root, "HEAD", nil, config.Defaults())
	for _, d := range with {
		if d.Verb == authArmVerb {
			t.Errorf("the arm fired on a tree with no authority contract: %+v", d)
		}
	}
	without := make([]schema.Defect, 0, len(with))
	for _, d := range with {
		if d.Verb != authArmVerb {
			without = append(without, d)
		}
	}
	if authSerialize(with) != authSerialize(without) {
		t.Errorf("non-authority verdict differs with vs without the arm")
	}
}

// TestAuthorityArmRunsBaseFree proves DINV-5: the arm runs on the base-free
// gate path (no merge base) AND the base-present path, catching an illegal
// contract on both — it reads the file as it stands and needs no history.
func TestAuthorityArmRunsBaseFree(t *testing.T) {
	root := authTempRepo(t, map[string]string{
		schema.AuthorityContractPath: illegalContract,
	})
	for _, base := range []string{"", "HEAD"} {
		got := gate.Gate(root, base, nil, config.Defaults())
		fired := false
		for _, d := range got {
			if d.Verb == authArmVerb {
				fired = true
			}
		}
		if !fired {
			label := "base-free"
			if base != "" {
				label = "base-present"
			}
			t.Errorf("the arm did not run on the %s path (an illegal contract should have redded)", label)
		}
	}
}

// TestAuthorityArmRealContractGreen proves A-1's green half on the real object:
// the repo's own authority contract is structurally legal (zero defects).
func TestAuthorityArmRealContractGreen(t *testing.T) {
	root := repoRootForTest(t)
	if d := gate.CheckAuthorityContract(root); len(d) != 0 {
		t.Errorf("the repo's real authority contract must pass, got %d defects: %v", len(d), d)
	}
}
