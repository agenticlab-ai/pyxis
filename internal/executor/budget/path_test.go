package budget

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A-21: path keys are injective in practice and hostile-input safe.
//
// Sanitize-and-truncate alone is NOT injective — "a/b" and "a?b" both reduce
// to "a_b", long shared prefixes collide once truncated, and "." and ".."
// pass an allowed-character filter untouched. The digest is what separates
// them, and it is taken over the RAW bytes before any of that.
func TestAtomSeparatesInputsSanitizingWouldCollide(t *testing.T) {
	for _, pair := range [][2]string{
		{"a/b", "a?b"},
		{"a/b", "a_b"},
		{strings.Repeat("x", 40) + "one", strings.Repeat("x", 40) + "two"},
		{".", ".."},
		{"", "_"},
	} {
		if atom(pair[0]) == atom(pair[1]) {
			t.Errorf("atom(%q) == atom(%q) — a sanitizing collision survived", pair[0], pair[1])
		}
	}
}

// No atom can name a parent, be empty, or traverse — the structural
// guarantees the digest suffix provides for free.
func TestAtomIsAlwaysASafeSegment(t *testing.T) {
	for _, raw := range []string{"", ".", "..", "../../etc/passwd", "a/b", strings.Repeat("z", 200), "octocat"} {
		got := atom(raw)
		switch {
		case got == "" || got == "." || got == "..":
			t.Errorf("atom(%q) = %q — names a parent or nothing", raw, got)
		case strings.ContainsAny(got, `/\`):
			t.Errorf("atom(%q) = %q — contains a separator", raw, got)
		case len(got) > readableMax+1+digestHex:
			t.Errorf("atom(%q) is %d chars, over the bound", raw, len(got))
		}
	}
}

// The host is encoded by the same rule, so two hosts differing only in
// disallowed characters do not share a directory.
func TestHostIsEncodedToo(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	a, err := HostPath("ghe/example.com")
	if err != nil {
		t.Fatal(err)
	}
	b, err := HostPath("ghe?example.com")
	if err != nil {
		t.Fatal(err)
	}
	if a == b {
		t.Error("two hosts differing only in disallowed characters share one umbrella")
	}
	if !strings.HasSuffix(a, "_host.json") {
		t.Errorf("umbrella path = %s, want a _host.json leaf", a)
	}
}

// Distinct identities and hosts meter separately; the same pair always names
// one ledger regardless of which repository is being written to — there is no
// repository input at all, so the partition cannot be reintroduced.
func TestPathSeparatesIdentitiesAndHostsButNotRepositories(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())

	octocat, _ := Path("github.com", "octocat")
	hubber, _ := Path("github.com", "hubber")
	if octocat == hubber {
		t.Error("two identities share one ledger")
	}
	if enterprise, _ := Path("ghe.example.com", "octocat"); enterprise == octocat {
		t.Error("two hosts share one ledger")
	}
	if again, _ := Path("github.com", "octocat"); again != octocat {
		t.Error("the same (host, identity) resolved two ledgers")
	}
}

// The umbrella and the marker live beside the identity ledgers they govern,
// and neither can be confused for one.
func TestHostRecordsAreDistinctFromIdentityLedgers(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	umbrella, _ := HostPath("github.com")
	marker, _ := MarkerPath("github.com")
	identity, _ := Path("github.com", "octocat")

	if umbrella == identity || marker == identity || umbrella == marker {
		t.Fatalf("records collide:\n  umbrella %s\n  marker   %s\n  identity %s", umbrella, marker, identity)
	}
	if filepath.Dir(umbrella) != filepath.Dir(identity) || filepath.Dir(marker) != filepath.Dir(identity) {
		t.Error("host records do not sit beside the ledgers they govern")
	}
}

// Legacy locations are named so an operator can clean them, and none of them
// is where the ledgers now live.
func TestStaleLegacyPathsReportsLedgerAndDebris(t *testing.T) {
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	checkout := t.TempDir()
	if got := StaleLegacyPaths(checkout); len(got) != 0 {
		t.Errorf("a clean checkout reported stale files: %v", got)
	}

	stale := filepath.Join(checkout, ".pyxis", "state")
	if err := os.MkdirAll(stale, 0o700); err != nil {
		t.Fatal(err)
	}
	bucket := filepath.Join(stale, "github-content-budget.json")
	for _, p := range []string{bucket, bucket + lockSuffix, filepath.Join(stale, tempPrefix+"123")} {
		if err := os.WriteFile(p, []byte("{}"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	got := StaleLegacyPaths(checkout)
	for _, want := range []string{bucket, bucket + lockSuffix, filepath.Join(stale, tempPrefix+"123")} {
		found := false
		for _, g := range got {
			if g == want {
				found = true
			}
		}
		if !found {
			t.Errorf("stale legacy paths omit %s:\n  %v", want, got)
		}
	}
	live, _ := Path("github.com", "octocat")
	for _, g := range got {
		if g == live {
			t.Errorf("the live ledger is reported as legacy: %s", live)
		}
	}
}
