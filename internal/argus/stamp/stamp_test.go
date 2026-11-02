package stamp

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/attribution"
)

// testBind is a fixed, well-formed content binding for the pure message-logic
// tests (the real binding is attribution.ComputeBinding over the commit's
// tree+parents — computed by cmd/argus-stamp, not this pure core).
const testBind = "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"

// finalTrailerBlock returns the message's final paragraph, mirroring the gate
// reader's trailer-block convention, so a test can assert on the block the gate
// actually scans.
func finalTrailerBlock(message string) []string {
	lines := splitLines(message)
	start, end := finalBlock(lines)
	return lines[start:end]
}

func hasTrailerLine(message, key, value string) bool {
	for _, line := range finalTrailerBlock(message) {
		if strings.TrimSpace(line) == key+": "+value {
			return true
		}
	}
	return false
}

func TestStamp_NoSlugLeavesMessageUntouched(t *testing.T) {
	// A commit authored outside any drive (no slug) carries no trailer — the
	// message is returned byte-for-byte unchanged (§3.1; the acceptance's
	// "commit outside a drive carries no trailer"). The binding is ignored.
	for _, msg := range []string{
		"feat: something\n",
		"feat: something\n\nbody paragraph.\n",
		"subject\n\nbody\n\nCo-Authored-By: X <x@y.z>\n",
	} {
		got, err := Stamp(msg, "", testBind)
		if err != nil {
			t.Fatalf("Stamp(_, \"\") errored: %v", err)
		}
		if got != msg {
			t.Errorf("no-slug Stamp mutated the message:\n got %q\nwant %q", got, msg)
		}
		if strings.Contains(got, TrailerKey) {
			t.Errorf("no-slug Stamp added a %s trailer: %q", TrailerKey, got)
		}
	}
}

func TestStamp_KeysTrailerAndBindingOnActiveSlug(t *testing.T) {
	// A well-formed slug appends BOTH `Argus-Drive: <slug>` and its
	// `Argus-Bind: <binding>` to the final trailer block, keyed on the active
	// drive's slug (§3.1, §3.2, DINV-3/DINV-4).
	got, err := Stamp("feat: add widget\n", "argus-attribution-hardening", testBind)
	if err != nil {
		t.Fatalf("Stamp errored: %v", err)
	}
	if !hasTrailerLine(got, TrailerKey, "argus-attribution-hardening") {
		t.Errorf("stamp did not key the drive trailer on the slug:\n%q", got)
	}
	if !hasTrailerLine(got, BindTrailerKey, testBind) {
		t.Errorf("stamp did not carry the content binding:\n%q", got)
	}
	// The subject is separated from the stamp by a blank line (a proper trailer
	// paragraph), not glued below the subject.
	if !strings.Contains(got, "feat: add widget\n\n") {
		t.Errorf("stamp was not separated into its own trailer paragraph:\n%q", got)
	}
}

func TestStamp_JoinsExistingTrailerBlock(t *testing.T) {
	// When the message already ends with a trailer block (the repo's
	// Co-Authored-By footer), the stamp joins that block rather than starting a
	// new paragraph, so the final paragraph carries every trailer.
	msg := "feat: add widget\n\nbody.\n\nCo-Authored-By: Claude <noreply@anthropic.com>\n"
	got, err := Stamp(msg, "widget", testBind)
	if err != nil {
		t.Fatalf("Stamp errored: %v", err)
	}
	block := finalTrailerBlock(got)
	if len(block) != 3 {
		t.Fatalf("final trailer block = %q, want the Co-Authored-By trailer + the drive + bind stamps", block)
	}
	if !hasTrailerLine(got, TrailerKey, "widget") || !hasTrailerLine(got, BindTrailerKey, testBind) {
		t.Errorf("stamp not present in the joined trailer block: %q", got)
	}
	if !strings.Contains(got, "Co-Authored-By: Claude <noreply@anthropic.com>") {
		t.Errorf("stamp clobbered the existing trailer: %q", got)
	}
}

func TestStamp_IdempotentRefreshesBinding(t *testing.T) {
	// Re-stamping with the SAME binding (a doubly-fired hook, a no-op amend) is a
	// byte-for-byte no-op — the drive never doubles.
	once, err := Stamp("feat: x\n", "widget", testBind)
	if err != nil {
		t.Fatal(err)
	}
	twice, err := Stamp(once, "widget", testBind)
	if err != nil {
		t.Fatal(err)
	}
	if twice != once {
		t.Errorf("re-stamp with the same binding changed the message:\n got %q\nwant %q", twice, once)
	}
	if n := strings.Count(twice, TrailerKey+":"); n != 1 {
		t.Errorf("message carries %d %s trailers, want 1:\n%q", n, TrailerKey, twice)
	}
	if n := strings.Count(twice, BindTrailerKey+":"); n != 1 {
		t.Errorf("message carries %d %s trailers, want 1:\n%q", n, BindTrailerKey, twice)
	}

	// Re-stamping after the tree changed (an amend) REFRESHES the binding to the
	// new value and never carries the stale one — a stale binding would derive
	// underivable and could slip an out-of-scope amend past the gate.
	const newBind = "fedcba9876543210fedcba9876543210fedcba9876543210fedcba9876543210"
	amended, err := Stamp(once, "widget", newBind)
	if err != nil {
		t.Fatal(err)
	}
	if !hasTrailerLine(amended, BindTrailerKey, newBind) {
		t.Errorf("amend did not refresh the binding to the new tree:\n%q", amended)
	}
	if strings.Contains(amended, testBind) {
		t.Errorf("amend carried the stale binding:\n%q", amended)
	}
	if n := strings.Count(amended, BindTrailerKey+":"); n != 1 {
		t.Errorf("refreshed message carries %d bind trailers, want 1:\n%q", n, amended)
	}
	if n := strings.Count(amended, TrailerKey+":"); n != 1 {
		t.Errorf("refreshed message carries %d drive trailers, want 1:\n%q", n, amended)
	}
}

func TestStamp_MalformedSlugFailsClosed(t *testing.T) {
	// A malformed slug is an error (a fail-closed abort), never a silently
	// mis-stamped (underivable) trailer.
	for _, bad := range []string{"Bad Slug", "UPPER", "-leading", "has_underscore", "slash/es"} {
		if _, err := Stamp("feat: x\n", bad, testBind); err == nil {
			t.Errorf("Stamp accepted malformed slug %q, want an error", bad)
		}
	}
}

func TestStamp_MissingBindingFailsClosed(t *testing.T) {
	// A well-formed slug with NO binding is an error: a produced trailer must be
	// content-bound, so a failed binding computation fails the commit closed
	// rather than emit an underivable (bare) trailer (§3.2).
	if _, err := Stamp("feat: x\n", "widget", ""); err == nil {
		t.Error("Stamp accepted a slug with an empty binding, want an error")
	}
	if _, err := Stamp("feat: x\n", "widget", "   "); err == nil {
		t.Error("Stamp accepted a slug with a blank binding, want an error")
	}
}

func TestStamp_PlacesTrailerBeforeCommentBlock(t *testing.T) {
	// On an editor commit the file carries git's `# …` comment template after
	// the message; the stamp must land before it so it survives git's cleanup
	// and stays in the final paragraph the gate reads.
	msg := "feat: x\n\nbody.\n\n# Please enter the commit message for your changes.\n# On branch main\n"
	got, err := Stamp(msg, "widget", testBind)
	if err != nil {
		t.Fatal(err)
	}
	stampAt := strings.Index(got, TrailerKey+":")
	commentAt := strings.Index(got, "# Please enter")
	if stampAt < 0 || commentAt < 0 || stampAt > commentAt {
		t.Errorf("stamp was not placed before the comment block:\n%q", got)
	}
	if bindAt := strings.Index(got, BindTrailerKey+":"); bindAt < 0 || bindAt > commentAt {
		t.Errorf("binding was not placed before the comment block:\n%q", got)
	}
}

func TestStamp_TrailerKeysSingleSourcedFromGate(t *testing.T) {
	// The producer's keys are single-sourced from the gate's reader so the two
	// can never drift (AP-6).
	if TrailerKey != attribution.TrailerKey {
		t.Errorf("producer TrailerKey %q != gate TrailerKey %q — the contract has drifted", TrailerKey, attribution.TrailerKey)
	}
	if BindTrailerKey != attribution.BindTrailerKey {
		t.Errorf("producer BindTrailerKey %q != gate BindTrailerKey %q — the contract has drifted", BindTrailerKey, attribution.BindTrailerKey)
	}
}
