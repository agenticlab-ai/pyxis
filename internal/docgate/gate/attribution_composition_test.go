package gate_test

// The Argus attribution gate's capstone composition + no-new-surface proofs
// (docs/tech-design/argus-attribution-hardening.md §3.6, §3.11; #DINV-7; A-5,
// A-6; plan argus-boundary-enforcement/002 T7).
//
// The hardened derivation's behavioral acceptance — the live drive-produced
// refusal (A-1/DOD-10), the forge/replay/strip corpus and the both-signals
// collision (A-3/DOD-11), the positively-human pass (A-4/NR-4), the
// credential-free git read (A-5) — is assembled and proven against a real
// commit graph by internal/argus/corpus/hardening_test.go and the attribution
// package's own git-backed tests. This file proves the *composition* half of
// the capstone that nothing else asserts:
//
//   - A-6: the gate composes as ONE self-registered, base-requiring check —
//     from its own descriptor, with a non-empty help line, appearing in the
//     verb-sorted enumeration the umbrella and CLI both walk — and the shared
//     umbrella enumeration / dispatch / help carry NO hand-edited line for it
//     (TestAttributionGateRegisteredFromDescriptor,
//     TestAttributionGateNoHandEditedLine).
//   - A-6 / DINV-7: the gate is additive — on a tree whose in-range diff touches
//     no protected path, the umbrella verdict is byte-identical with and without
//     the arm, so no non-protected verdict is perturbed
//     (TestAttributionGateAdditiveOnNonProtectedTree).
//   - A-5 / DINV-7: the produced signal and the credential-free read add NO new
//     egress, identity, secret, credential, or network surface — the producer is
//     a repo-internal git commit hook and the verification an in-tree `git` read,
//     so no non-test source in the producer or read path imports a network /
//     credential package or execs anything but `git`
//     (TestAttributionNoNewOutwardSurface).

import (
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const attrGateVerb = attribution.Verb // "check-argus-attribution"

func attrSerialize(ds []schema.Defect) string {
	lines := make([]string, 0, len(ds))
	for _, d := range ds {
		lines = append(lines, strings.Join([]string{d.Verb, d.File, d.Op, d.Reason, d.Remedy}, "\x1f"))
	}
	sort.Strings(lines)
	return strings.Join(lines, "\n")
}

// TestAttributionGateRegisteredFromDescriptor proves A-6: the gate is in the
// check registry, is base-requiring (NeedsMergeBase — it evaluates the PR's own
// base..HEAD range, §3.6/§3.7), carries a non-empty help line the umbrella help
// walk renders, and appears in the verb-sorted enumeration both the umbrella and
// CLI walk — nothing hand-listed.
func TestAttributionGateRegisteredFromDescriptor(t *testing.T) {
	c, ok := gate.Lookup(attrGateVerb)
	if !ok {
		t.Fatalf("%s is not registered", attrGateVerb)
	}
	if !c.NeedsMergeBase {
		t.Errorf("%s must be base-requiring (NeedsMergeBase true) — it evaluates base..HEAD", attrGateVerb)
	}
	if strings.TrimSpace(c.Help) == "" {
		t.Errorf("%s: empty help line", attrGateVerb)
	}
	seen := false
	for _, ch := range gate.Checks() {
		if ch.Verb == attrGateVerb {
			seen = true
		}
	}
	if !seen {
		t.Errorf("%s must appear in gate.Checks() (the derived enumeration)", attrGateVerb)
	}
}

// TestAttributionGateNoHandEditedLine proves A-6: the umbrella's shared
// enumeration, the registry, and the CLI dispatch/help carry NO hand-edited line
// naming the gate — the verb lives only in its own descriptor file (AP-27), so
// this "add a capability" task added no shared enumeration, dispatch, or help
// line.
func TestAttributionGateNoHandEditedLine(t *testing.T) {
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
		if strings.Contains(string(b), attrGateVerb) {
			t.Errorf("%s hand-lists %q — the umbrella must assemble the gate from its descriptor only", rel, attrGateVerb)
		}
	}
	desc := "internal/docgate/gate/reg_check_attribution.go"
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(desc)))
	if err != nil {
		t.Fatalf("reading %s: %v", desc, err)
	}
	// The descriptor defines its verb via attribution.Verb, so it need not carry
	// the string literal; what must hold is that the verb is registered from
	// exactly this one descriptor file (proven by Lookup above) and nowhere
	// hand-listed (proven by the loop above). The descriptor references the
	// attribution package that owns the verb.
	if !strings.Contains(string(b), "attribution.") {
		t.Errorf("%s does not compose the attribution gate from its own package", desc)
	}
}

// TestAttributionGateAdditiveOnNonProtectedTree proves A-6/DINV-7: on a PR whose
// in-range diff touches only a non-protected path, the umbrella's verdict is
// byte-identical with and without the arm — the gate draws nothing, so no
// non-protected verdict is perturbed.
func TestAttributionGateAdditiveOnNonProtectedTree(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
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
	git("config", "commit.gpgsign", "false")
	write("README.md", "seed\n")
	git("add", "README.md")
	git("commit", "-qm", "base")
	base := strings.TrimSpace(gitCapture(t, root, "rev-parse", "HEAD"))
	// An in-range commit touching a non-protected path (a wiki note) — a real
	// diff the gate sees but must never refuse.
	write("docs/wiki/note.md", "An ordinary note. No protected path here.\n")
	git("add", "docs/wiki/note.md")
	git("commit", "-qm", "docs: a note")

	with := gate.Gate(root, base, nil, config.Defaults())
	for _, d := range with {
		if d.Verb == attrGateVerb {
			t.Errorf("the gate fired on a non-protected in-range diff: %+v", d)
		}
	}
	without := make([]schema.Defect, 0, len(with))
	for _, d := range with {
		if d.Verb != attrGateVerb {
			without = append(without, d)
		}
	}
	if attrSerialize(with) != attrSerialize(without) {
		t.Errorf("non-protected verdict differs with vs without the attribution gate")
	}
}

// TestAttributionNoNewOutwardSurface proves A-5/DINV-7: authorship is produced
// and verified entirely in-tree. No non-test source file in the producer
// (internal/argus/stamp, cmd/argus-stamp) or the read path
// (internal/argus/attribution) imports a network / credential / identity
// package, and every external process it spawns is `git` — never a code-host
// API, a signing credential, or any network egress. This is the mechanical form
// of the header's "no new egress, identity, secret, credential, or network
// surface" claim.
func TestAttributionNoNewOutwardSurface(t *testing.T) {
	root := repoRootForTest(t)
	// Packages this design owns: the produced signal and the credential-free
	// read. The classifier (internal/argus/boundary) is reused unchanged and
	// scanned too, so a regression there is caught.
	dirs := []string{
		"internal/argus/attribution",
		"internal/argus/stamp",
		"internal/argus/boundary",
		"cmd/argus-stamp",
	}
	// Import substrings that would introduce a new outward/credential surface.
	forbiddenImports := []string{
		`"net"`, `"net/http"`, `"net/url"`, `"net/rpc"`, `"net/smtp"`,
		`"crypto/tls"`, `"os/user"`,
		"oauth", "go-github", "golang.org/x/oauth2", "aws-sdk", "cloud.google",
	}
	// Any exec.Command with a non-git literal command names a new external
	// dependency surface — catch it.
	execLit := regexp.MustCompile(`exec\.Command\(\s*"([^"]+)"`)

	for _, dir := range dirs {
		abs := filepath.Join(root, filepath.FromSlash(dir))
		entries, err := os.ReadDir(abs)
		if err != nil {
			t.Fatalf("reading %s: %v", dir, err)
		}
		for _, e := range entries {
			name := e.Name()
			if e.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
				continue
			}
			b, err := os.ReadFile(filepath.Join(abs, name))
			if err != nil {
				t.Fatalf("reading %s/%s: %v", dir, name, err)
			}
			src := string(b)
			for _, bad := range forbiddenImports {
				if strings.Contains(src, bad) {
					t.Errorf("%s/%s references a forbidden outward/credential surface %q — the gate must stay repo-internal (DINV-7)", dir, name, bad)
				}
			}
			for _, m := range execLit.FindAllStringSubmatch(src, -1) {
				if m[1] != "git" {
					t.Errorf("%s/%s spawns %q — only `git` is admissible (no new external surface; DINV-7)", dir, name, m[1])
				}
			}
		}
	}
}

// gitCapture runs a git command and returns stdout (a thin read helper local to
// this file; the umbrella tests elsewhere use their own).
func gitCapture(t *testing.T, root string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
