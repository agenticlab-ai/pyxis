package corpus

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The hardening corpus (plan docs/plans/argus-boundary-enforcement 002 T6;
// design docs/tech-design/argus-attribution-hardening.md §3.13; DOD-10, DOD-11).
//
// The shipped cross-layer corpus above (corpus.go / corpus_test.go) hand-seeds
// every attribution trailer directly, so it can prove the pre-hardening
// properties but NONE of the three the hardened derivation adds — because
// neither a producer nor a content binding existed when it was written:
//
//   - that the authorship signal is actually PRODUCED on a real drive-authored
//     commit (not assumed) — proven only by authoring THROUGH the live producer
//     (the built argus-stamp on git's core.hooksPath), never a hand-seeded
//     trailer (A-1, DOD-10);
//   - that a forged, a replayed, or a stripped signal cannot flip the derived
//     class in EITHER direction — never a human diff framed as Argus, never an
//     Argus protected-path diff served as human (A-3, DOD-11);
//   - that the both-signals collision refuses on the produced stamp — a
//     drive-plantable `Governing-Change: operator` attestation beside a produced
//     `Argus-Drive` stamp cannot launder a positively-Argus protected-path diff,
//     because positively-Argus wins the precedence (A-3, FND-1, DINV-5).
//
// These proofs run against a REAL commit graph built commit-by-commit (the
// local-substitutable seam of §3.13 — a real `git` read, never a mock), and the
// live-produced ones drive the real producer binary, so the corpus proves the
// signal end-to-end rather than restating the derivation's own unit fixtures.
// The corpus reads and asserts only; it never writes a protected path.

// --- the live producer, built once and installed on git's own hook surface ---

// buildProducer builds the argus-stamp producer binary — the same binary the
// repo's prepare-commit-msg shim delegates to. The live-produced proofs
// (A-1, the collision) need the real producer, not a hand-seeded trailer; when
// the go toolchain is unavailable the proof skips rather than silently degrades
// to a seeded stand-in (which would defeat DOD-10's whole point).
func buildProducer(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go toolchain unavailable — skipping the live-producer hardening proof")
	}
	bin := filepath.Join(t.TempDir(), "argus-stamp")
	cmd := exec.Command("go", "build", "-o", bin, "github.com/agenticlab-ai/pyxis/cmd/argus-stamp")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build argus-stamp: %v\n%s", err, out)
	}
	return bin
}

// installProducerHook wires the built producer onto git's own core.hooksPath
// surface as a delegate-only prepare-commit-msg shim — exactly the repo's own
// hook shape. Once installed, a commit whose $ARGUS_DRIVE_SLUG is set is stamped
// by construction; a commit with no slug is left untouched (a commit outside a
// drive carries no trailer — §3.1). The drive never reaches this surface: the
// harness plays the supervisor that sets the slug.
func installProducerHook(t *testing.T, root, bin string) {
	t.Helper()
	hooks := filepath.Join(root, ".githooks")
	if err := os.MkdirAll(hooks, 0o755); err != nil {
		t.Fatal(err)
	}
	shim := "#!/bin/sh\nexec \"" + bin + "\" \"$@\"\n"
	if err := os.WriteFile(filepath.Join(hooks, "prepare-commit-msg"), []byte(shim), 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, root, "config", "core.hooksPath", ".githooks")
}

// --- git harness (a fresh, hermetic repo per proof; scrubbed of any credential
// so the derivation is proven to need none, mirroring the shipped arms) ---

// newHardenRepo inits a throwaway git repo with gpg signing off and a stable
// identity — a real commit graph to build the hardening fixtures on.
func newHardenRepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q", "-b", "main")
	git(t, root, "config", "user.email", "t@example.com")
	git(t, root, "config", "user.name", "Test")
	git(t, root, "config", "commit.gpgsign", "false")
	return root
}

// hardenBase seeds a base commit (the merge base the gate evaluates from) and
// returns its sha.
func hardenBase(t *testing.T, root string) string {
	t.Helper()
	writeFile(t, root, "README.md", "seed\n")
	git(t, root, "add", "README.md")
	git(t, root, "commit", "-q", "-m", "chore: seed")
	return strings.TrimSpace(string(gitOut(t, root, "rev-parse", "HEAD")))
}

// driveCommit authors a commit THROUGH the live producer, keyed on the active
// drive's slug the harness sets ($ARGUS_DRIVE_SLUG) — the producer stamps the
// Argus-Drive trailer and its content binding by construction.
func driveCommit(t *testing.T, root, slug, msg string, paths ...string) {
	t.Helper()
	git(t, root, append([]string{"add"}, paths...)...)
	cmd := gitCmd(root, "commit", "-q", "-m", msg)
	cmd.Env = append(cmd.Env, "ARGUS_DRIVE_SLUG="+slug)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("drive commit (slug=%s): %v\n%s", slug, err, out)
	}
}

// rawCommit authors a plain commit with the message verbatim — no producer in
// the path, so any trailer in msg is exactly what the message carries. This is
// the forger's / stripper's surface: an actor with raw commit-authoring access
// writes whatever trailer (or none) they like, outside the honoring producer.
func rawCommit(t *testing.T, root, msg string, paths ...string) string {
	t.Helper()
	git(t, root, append([]string{"add"}, paths...)...)
	git(t, root, "commit", "-q", "-m", msg)
	return strings.TrimSpace(string(gitOut(t, root, "rev-parse", "HEAD")))
}

// authorArgusBound authors a GENUINE, content-bound Argus commit exactly as the
// producer would — the binding is computed over the commit's own tree+parents
// via the single-sourced attribution.ComputeBinding, so it re-derives at the
// gate. Used to mint the real trailer the replay proof lifts onto a different
// commit. Returns the new HEAD sha.
func authorArgusBound(t *testing.T, root, slug, msg string, paths ...string) string {
	t.Helper()
	git(t, root, append([]string{"add"}, paths...)...)
	tree := strings.TrimSpace(string(gitOut(t, root, "write-tree")))
	parents := []string{strings.TrimSpace(string(gitOut(t, root, "rev-parse", "HEAD")))}
	bind := attribution.ComputeBinding(tree, parents)
	full := msg + "\n\n" + attribution.TrailerKey + ": " + slug + "\n" + attribution.BindTrailerKey + ": " + bind + "\n"
	git(t, root, "commit", "-q", "-m", full)
	return strings.TrimSpace(string(gitOut(t, root, "rev-parse", "HEAD")))
}

// readArgusBind reads the `Argus-Bind` value a commit carries (or "" for none).
func readArgusBind(t *testing.T, root, ref string) string {
	t.Helper()
	body := string(gitOut(t, root, "log", "-1", "--format=%B", ref))
	prefix := attribution.BindTrailerKey + ":"
	for _, line := range strings.Split(body, "\n") {
		if strings.HasPrefix(line, prefix) {
			return strings.TrimSpace(strings.TrimPrefix(line, prefix))
		}
	}
	return ""
}

// gateRefusal runs the real gate over base..HEAD and returns the refusal for
// path (found, the defect). A file with no refusal returns found=false.
func gateRefusal(root, base, path string) (bool, schema.Defect) {
	for _, d := range attribution.Evaluate(root, base, corpusCfg) {
		if d.File == path {
			return true, d
		}
	}
	return false, schema.Defect{}
}

// --- A-1 / DOD-10 and the FND-1 collision: proven on the LIVE produced stamp ---

// TestHardening_ProducedSignalIsRealNotAssumed is the DOD-10 spine: a commit
// authored THROUGH the live producer hook carries the Argus-Drive stamp by
// construction, and the real gate refuses it on THAT produced signal — proving
// the signal is produced, not assumed. The collision subtest proves the same
// produced stamp wins the precedence over a planted human attestation (FND-1).
func TestHardening_ProducedSignalIsRealNotAssumed(t *testing.T) {
	bin := buildProducer(t)

	t.Run("live drive-produced protected-path refusal", func(t *testing.T) {
		root := newHardenRepo(t)
		installProducerHook(t, root, bin)
		base := hardenBase(t, root)

		// A genuine drive of a protected-path write — the governance vision —
		// stamped by the producer, not by hand.
		writeFile(t, root, governanceDocument, "governance edit\n")
		driveCommit(t, root, "widget", "feat: touch vision", governanceDocument)

		// The stamp is present BY CONSTRUCTION (both the trailer and its binding).
		body := string(gitOut(t, root, "log", "-1", "--format=%B", "HEAD"))
		if !strings.Contains(body, attribution.TrailerKey+": widget") {
			t.Fatalf("the producer did not stamp the drive trailer by construction:\n%q", body)
		}
		if !strings.Contains(body, attribution.BindTrailerKey+": ") {
			t.Fatalf("the producer did not stamp the content binding by construction:\n%q", body)
		}

		// The gate refuses it ON its produced signal, as Argus-authored.
		found, d := gateRefusal(root, base, governanceDocument)
		if !found {
			t.Fatal("the gate did not refuse the produced Argus protected-path diff")
		}
		if d.Op != "governance-document" || !strings.Contains(d.Reason, "Argus-authored") {
			t.Fatalf("refusal was not on the produced Argus signal: op=%q reason=%q", d.Op, d.Reason)
		}
	})

	t.Run("both-signals collision refuses on the produced stamp", func(t *testing.T) {
		root := newHardenRepo(t)
		installProducerHook(t, root, bin)
		base := hardenBase(t, root)

		// The drive authors the files AND plants the operator attestation in the
		// very commit body its produced stamp lands on — the both-signals collision.
		writeFile(t, root, governanceDocument, "vision + planted attestation\n")
		driveCommit(t, root, "widget", "feat: touch vision\n\nGoverning-Change: operator", governanceDocument)

		// Positively-Argus wins: the diff is refused as Argus-authored regardless of
		// the planted attestation — a drive-plantable human line cannot launder it.
		found, d := gateRefusal(root, base, governanceDocument)
		if !found {
			t.Fatal("the both-signals collision was not refused — positively-Argus must win the precedence")
		}
		if !strings.Contains(d.Reason, "Argus-authored") {
			t.Fatalf("the collision was not refused on the produced stamp (the planted attestation laundered it): reason=%q", d.Reason)
		}
	})
}

// --- A-3 / DOD-11: forge, replay, strip — the class flips in NEITHER direction ---

// TestHardening_ForgeReplayStripDoNotFlipTheClass is the DOD-11 spine: a forged,
// a replayed, and a stripped signal each leave the derived class un-flipped —
// no forged signal frames a human diff as Argus, and no stripped/replayed signal
// masquerades an Argus protected-path diff as human. Each derives underivable
// (never Argus, never a silent human pass), the honestly-owned direction closed.
func TestHardening_ForgeReplayStripDoNotFlipTheClass(t *testing.T) {
	// A binding value that is well-formed in shape but never a real content
	// binding — the forger cannot produce the true one for an arbitrary commit.
	const forgedBind = "ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"

	t.Run("forge does not frame a human diff as Argus", func(t *testing.T) {
		// A raw-commit forger hand-writes an Argus trailer with a fabricated
		// binding on a NON-protected human diff (the frame vector: make a human's
		// change derive as Argus). The binding does not re-derive, so it derives
		// underivable — never Argus — and on a non-protected path it is not refused
		// at all: no frame.
		root := newHardenRepo(t)
		base := hardenBase(t, root)
		writeFile(t, root, "internal/tool/x.go", "package tool\n")
		rawCommit(t, root, "feat: code\n\n"+attribution.TrailerKey+": widget\n"+attribution.BindTrailerKey+": "+forgedBind, "internal/tool/x.go")
		if found, d := gateRefusal(root, base, "internal/tool/x.go"); found {
			t.Fatalf("a forged trailer framed a non-protected human diff (it must derive underivable, not Argus): %q", d.Reason)
		}

		// On a PROTECTED path the same forgery fails closed — but as UNDERIVABLE,
		// never accepted as Argus: the forgery cannot masquerade as a genuine Argus
		// attribution (the in-tree binding rejects it).
		root2 := newHardenRepo(t)
		base2 := hardenBase(t, root2)
		writeFile(t, root2, authorityContract, "policy\n")
		rawCommit(t, root2, "feat: policy\n\n"+attribution.TrailerKey+": widget\n"+attribution.BindTrailerKey+": "+forgedBind, authorityContract)
		found, d := gateRefusal(root2, base2, authorityContract)
		if !found {
			t.Fatal("a forged trailer on a protected path did not fail closed")
		}
		if strings.Contains(d.Reason, "Argus-authored") || !strings.Contains(d.Reason, "underivable") {
			t.Fatalf("the forgery was accepted as an Argus attribution rather than rejected as underivable: %q", d.Reason)
		}
	})

	t.Run("replay masquerades neither as Argus nor as human", func(t *testing.T) {
		// A GENUINE, content-bound Argus commit on an in-scope path — its binding is
		// derived over its own tree+parents, so it re-derives (in-scope → not refused).
		root := newHardenRepo(t)
		base := hardenBase(t, root)
		writeFile(t, root, "docs/problems/widget/problem.md", "p\n")
		legit := authorArgusBound(t, root, "widget", "feat: legit in-scope", "docs/problems/widget/problem.md")
		lifted := readArgusBind(t, root, legit)
		if lifted == "" {
			t.Fatal("the genuine Argus commit carried no content binding to lift")
		}

		// Lift that exact trailer+binding onto a DIFFERENT commit touching a
		// protected path. The binding no longer re-derives (a different tree+parents),
		// so the replay is underivable and fails closed — not served as human, not
		// re-accepted as Argus.
		writeFile(t, root, authorityContract, "policy\n")
		rawCommit(t, root, "feat: replay\n\n"+attribution.TrailerKey+": widget\n"+attribution.BindTrailerKey+": "+lifted, authorityContract)

		if found, _ := gateRefusal(root, base, "docs/problems/widget/problem.md"); found {
			t.Fatal("the genuine in-scope Argus commit was refused")
		}
		found, d := gateRefusal(root, base, authorityContract)
		if !found {
			t.Fatal("the replayed trailer did not fail closed on a protected path (it masqueraded as human)")
		}
		if strings.Contains(d.Reason, "Argus-authored") || !strings.Contains(d.Reason, "underivable") {
			t.Fatalf("the replay was not detected as underivable: %q", d.Reason)
		}
	})

	t.Run("strip masquerades an Argus diff as human", func(t *testing.T) {
		// The produced signal was STRIPPED — a bare commit touching a protected
		// path, no trailer, no attestation. Absence is no longer human: it derives
		// underivable and fails closed, never served as a masquerading human edit.
		root := newHardenRepo(t)
		base := hardenBase(t, root)
		writeFile(t, root, governanceDocument, "stripped edit\n")
		rawCommit(t, root, "docs: stripped vision edit", governanceDocument)
		found, d := gateRefusal(root, base, governanceDocument)
		if !found {
			t.Fatal("a stripped (unattested) protected-path diff did not fail closed — it masqueraded as human")
		}
		if !strings.Contains(d.Reason, "underivable") {
			t.Fatalf("the stripped diff was not derived underivable: %q", d.Reason)
		}
	})
}

// --- A-4 / NR-4: the legitimate positively-human protected-path pass ---

// TestHardening_PositivelyHumanPassesZeroFalseRefusals is the A-4 half: a
// legitimate human PR to protected paths — carrying the positive attestation the
// governance/authority classes already require, and NO Argus stamp — derives
// positively-human and passes with ZERO false refusals. Establishing human
// authorship demands no step beyond the ordinary attestation, so the anti-segment
// stays served alongside the fail-closed guarantee.
func TestHardening_PositivelyHumanPassesZeroFalseRefusals(t *testing.T) {
	root := newHardenRepo(t)
	base := hardenBase(t, root)
	writeFile(t, root, authorityContract, "policy\n")
	writeFile(t, root, governanceDocument, "vision\n")
	rawCommit(t, root, "docs: operator lands a governing change\n\nGoverning-Change: operator", authorityContract, governanceDocument)

	if defects := attribution.Evaluate(root, base, corpusCfg); len(defects) != 0 {
		t.Fatalf("FALSE REFUSAL — a positively-human protected-path PR was refused: %v", defects)
	}
}
