package gate_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
)

// TestCheckImmutableUsesForkPointNotTip pins the base semantics: a history
// check must diff against the commit where the branch forked from main, not
// main's moving tip. A branch cut from an older main that only ADDS files
// deletes nothing — but against the tip, every immutable doc merged after the
// cut reads as a phantom deletion. This is the false-fail that made every
// behind branch red once main moved (INV-41 CI half). ResolveMergeBase is the
// fix the CLI applies once for all history checks.
func TestCheckImmutableUsesForkPointNotTip(t *testing.T) {
	root := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	write := func(rel, content string) {
		t.Helper()
		p := filepath.Join(root, rel)
		os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	cfg := config.Defaults()
	run("init", "-q", "-b", "main")
	write("docs/prds/alpha/000-base.md", miniBase)
	run("add", ".")
	run("commit", "-qm", "alpha ledger")
	run("branch", "feature") // feature forks from main here

	// main moves ahead: a new immutable ledger merges AFTER feature forked.
	write("docs/prds/beta/000-base.md", miniBase)
	run("add", ".")
	run("commit", "-qm", "beta ledger (merged after feature forked)")

	// feature is behind but purely additive — it only adds its own ledger.
	// gamma's content is deliberately unlike beta's so git reports a clean
	// delete+add, not a rename — mirroring a real PR whose own new doc is
	// unrelated to the doc that merged onto main after the fork.
	run("checkout", "-q", "feature")
	gammaBase := "# gamma — PRD\n\n" + strings.Repeat("distinct feature-branch content.\n", 20)
	write("docs/prds/gamma/000-base.md", gammaBase)
	run("add", ".")
	run("commit", "-qm", "gamma ledger (feature's own addition)")

	// Against main's TIP, beta reads as a deletion feature never made — the bug.
	tip := gate.CheckImmutable(root, "main", "", cfg)
	phantom := false
	for _, d := range tip {
		if d.File == "docs/prds/beta/000-base.md" && strings.Contains(d.Reason, "deleted") {
			phantom = true
		}
	}
	if !phantom {
		t.Fatalf("expected the tip diff to phantom-delete beta (the bug being fixed); got %v", tip)
	}

	// Against the fork point (what --merge-base must resolve to), feature
	// deleted nothing: the behind-but-additive branch is clean.
	base, rdef := gate.ResolveMergeBase(root, "main")
	if len(rdef) > 0 {
		t.Fatalf("ResolveMergeBase: %v", rdef)
	}
	if d := gate.CheckImmutable(root, base, "", cfg); len(d) != 0 {
		t.Fatalf("behind-but-additive branch false-failed against fork point: %v", d)
	}
}
