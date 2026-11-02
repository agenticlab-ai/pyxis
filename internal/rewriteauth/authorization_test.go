package rewriteauth

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestVerifyExactOneUseAuthorization(t *testing.T) {
	root, base := testRepo(t)
	write(t, root, "tracked.txt", "after\n", 0o644)
	write(t, root, "added.txt", "added\n", 0o644)
	git(t, root, "add", "--", "tracked.txt", "added.txt")

	p := policy{markerPath: MarkerPath, schemaVersion: "test/v1", authorizationID: "test", baseCommit: base}
	digest, err := ChangeSetDigest(root, base, "")
	if err != nil {
		t.Fatal(err)
	}
	writeMarker(t, root, marker{
		SchemaVersion: p.schemaVersion, AuthorizationID: p.authorizationID,
		BaseCommit: p.baseCommit, ChangeSetSHA256: digest,
	})
	got := verify(root, base, "", p)
	if !got.Active || !got.Authorized || got.Err != nil {
		t.Fatalf("verify = %+v", got)
	}
	for _, verb := range []string{"check-immutable", "check-ledger-history", "check-standalone-docs", "solution-shape"} {
		if !got.AllowsDocgate(verb) {
			t.Errorf("authorized result refused %s", verb)
		}
	}
	if got.AllowsDocgate("check-citations") || !got.AllowsMemoryGate() {
		t.Fatalf("authorization scope drift: %+v", got)
	}

	write(t, root, "added.txt", "changed after authorization\n", 0o644)
	got = verify(root, base, "", p)
	if !got.Active || got.Authorized || got.Err == nil || !strings.Contains(got.Err.Error(), "digest mismatch") {
		t.Fatalf("changed set was authorized: %+v", got)
	}

	write(t, root, "added.txt", "added\n", 0o644)
	git(t, root, "add", "--", MarkerPath)
	git(t, root, "commit", "-m", "authorized rewrite")
	got = verify(root, base, "HEAD", p)
	if !got.Authorized || got.Err != nil {
		t.Fatalf("committed authorization = %+v", got)
	}
	if public := Verify(root, base, "HEAD"); public.Active || public.Authorized || public.Err != nil {
		t.Fatalf("fixed production authorization activated in fixture: %+v", public)
	}
}

func TestVerifySelectsTheAuthorizationBoundToTheMergeBase(t *testing.T) {
	root, base := testRepo(t)
	write(t, root, "tracked.txt", "after\n", 0o644)
	git(t, root, "add", "--", "tracked.txt")
	const otherMarker = ".pyxis/direct-rewrite/other.json"
	other := policy{markerPath: otherMarker, schemaVersion: "test/v1", authorizationID: "other", baseCommit: strings.Repeat("0", 40)}
	bound := policy{markerPath: RepairMarkerPath, schemaVersion: "test/v1", authorizationID: "bound", baseCommit: base}
	digest, err := changeSetDigest(root, base, "", RepairMarkerPath)
	if err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(marker{SchemaVersion: bound.schemaVersion, AuthorizationID: bound.authorizationID, BaseCommit: base, ChangeSetSHA256: digest})
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, RepairMarkerPath, string(body), 0o644)

	got := verifyAny(root, base, "", []policy{other, bound})
	if !got.Active || !got.Authorized || got.Err != nil || got.Marker != RepairMarkerPath {
		t.Fatalf("verifyAny = %+v, want the policy bound to the merge base", got)
	}
	if got := verifyAny(root, base, "", []policy{other}); got.Active || got.Authorized || got.Err != nil || got.Marker != "" {
		t.Fatalf("an authorization bound to another base activated: %+v", got)
	}
	if got := verifyAny(root, "missing", "", []policy{other, bound}); got.Active {
		t.Fatalf("unresolvable base activated an authorization: %+v", got)
	}
	for _, p := range oneUsePolicies {
		if p.schemaVersion != SchemaVersion || !validGitObjectID(p.baseCommit) || p.markerPath == "" || p.authorizationID == "" {
			t.Fatalf("malformed production policy: %+v", p)
		}
	}
	if oneUsePolicies[0].baseCommit == oneUsePolicies[1].baseCommit || oneUsePolicies[0].markerPath == oneUsePolicies[1].markerPath {
		t.Fatal("production authorizations share a base or a marker")
	}
}

func TestChangeSetDigestExcludesOnlyTheActiveMarker(t *testing.T) {
	root, base := testRepo(t)
	write(t, root, "tracked.txt", "after\n", 0o644)
	git(t, root, "add", "--", "tracked.txt")
	before, err := changeSetDigest(root, base, "", RepairMarkerPath)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, RepairMarkerPath, `{}`, 0o644)
	git(t, root, "add", "--", RepairMarkerPath)
	withActive, err := changeSetDigest(root, base, "", RepairMarkerPath)
	if err != nil {
		t.Fatal(err)
	}
	if withActive != before {
		t.Fatalf("the active marker changed its own digest: %s != %s", withActive, before)
	}
	write(t, root, MarkerPath, `{}`, 0o644)
	git(t, root, "add", "--", MarkerPath)
	withOther, err := changeSetDigest(root, base, "", RepairMarkerPath)
	if err != nil {
		t.Fatal(err)
	}
	if withOther == before {
		t.Fatal("another authorization's marker escaped the bound change set")
	}
}

func validGitObjectID(value string) bool {
	return len(value) == 40 && strings.Trim(value, "0123456789abcdef") == ""
}

func TestVerifyRefusesMalformedOrMissingMarker(t *testing.T) {
	root, base := testRepo(t)
	write(t, root, "tracked.txt", "after\n", 0o644)
	git(t, root, "add", "--", "tracked.txt")
	p := policy{markerPath: MarkerPath, schemaVersion: "test/v1", authorizationID: "test", baseCommit: base}
	digest, err := ChangeSetDigest(root, base, "")
	if err != nil {
		t.Fatal(err)
	}

	if got := verify(root, base, "", p); !got.Active || got.Authorized || got.Err == nil || !strings.Contains(got.Err.Error(), "read authorization marker") {
		t.Fatalf("missing marker = %+v", got)
	}
	cases := []struct {
		name string
		body string
		want string
	}{
		{"malformed", `{`, "decode authorization marker"},
		{"unknown field", `{"schema_version":"test/v1","authorization_id":"test","base_commit":"` + base + `","change_set_sha256":"` + digest + `","extra":true}`, "unknown field"},
		{"trailing value", `{"schema_version":"test/v1","authorization_id":"test","base_commit":"` + base + `","change_set_sha256":"` + digest + `"} {}`, "multiple JSON values"},
		{"wrong identity", `{"schema_version":"wrong","authorization_id":"test","base_commit":"` + base + `","change_set_sha256":"` + digest + `"}`, "identity"},
		{"bad digest", `{"schema_version":"test/v1","authorization_id":"test","base_commit":"` + base + `","change_set_sha256":"no"}`, "invalid change_set_sha256"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			write(t, root, MarkerPath, tc.body, 0o644)
			got := verify(root, base, "", p)
			if !got.Active || got.Authorized || got.Err == nil || !strings.Contains(got.Err.Error(), tc.want) {
				t.Fatalf("verify = %+v, want %q", got, tc.want)
			}
		})
	}
	if got := verify(root, "missing", "", p); got.Active || got.Authorized || got.Err != nil {
		t.Fatalf("unresolvable base activated authorization: %+v", got)
	}
}

func TestChangeSetDigestBindsStatusesModesAndMarkerExclusion(t *testing.T) {
	root, _ := testRepo(t)
	write(t, root, "deleted.txt", "delete\n", 0o644)
	write(t, root, "typed.txt", "regular\n", 0o644)
	git(t, root, "add", "--", "deleted.txt", "typed.txt")
	git(t, root, "commit", "-m", "more base files")
	base := strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))

	write(t, root, "tracked.txt", "modified\n", 0o644)
	if err := os.Remove(filepath.Join(root, "deleted.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, "typed.txt")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("tracked.txt", filepath.Join(root, "typed.txt")); err != nil {
		t.Fatal(err)
	}
	write(t, root, "regular.txt", "regular\n", 0o644)
	write(t, root, "executable", "#!/bin/sh\n", 0o755)
	write(t, root, "link-target", "target\n", 0o644)
	if err := os.Symlink("link-target", filepath.Join(root, "link")); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "--", "tracked.txt", "deleted.txt", "typed.txt", "regular.txt", "executable", "link-target", "link")
	beforeMarker, err := ChangeSetDigest(root, base, "")
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, MarkerPath, `{}`, 0o644)
	git(t, root, "add", "--", MarkerPath)
	afterMarker, err := ChangeSetDigest(root, base, "")
	if err != nil {
		t.Fatal(err)
	}
	if beforeMarker != afterMarker || !validDigest(afterMarker) {
		t.Fatalf("marker changed digest: %s != %s", beforeMarker, afterMarker)
	}
	git(t, root, "commit", "-m", "all change forms")
	committed, err := ChangeSetDigest(root, base, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if committed != beforeMarker {
		t.Fatalf("working/committed digest mismatch: %s != %s", beforeMarker, committed)
	}
}

func TestChangeSetDigestRejectsUnsafePathAndBadRepository(t *testing.T) {
	root, base := testRepo(t)
	write(t, root, "bad\npath", "bad\n", 0o644)
	git(t, root, "add", "--", "bad\npath")
	if _, err := ChangeSetDigest(root, base, ""); err == nil || !strings.Contains(err.Error(), "unsafe") {
		t.Fatalf("unsafe path error = %v", err)
	}
	if _, err := ChangeSetDigest(t.TempDir(), base, ""); err == nil || !strings.Contains(err.Error(), "list direct-rewrite changes") {
		t.Fatalf("bad repo error = %v", err)
	}
}

func TestHelpersRejectInvalidInputs(t *testing.T) {
	root, base := testRepo(t)
	if _, err := resolveCommit(root, ""); err == nil {
		t.Fatal("empty commit ref accepted")
	}
	if _, err := treeMode(root, base, "missing"); err == nil {
		t.Fatal("missing tree path accepted")
	}
	if _, err := refBlobHash(root, base, "missing"); err == nil {
		t.Fatal("missing blob accepted")
	}
	if _, _, err := worktreeBlob(root, "missing"); err == nil {
		t.Fatal("missing worktree path accepted")
	}
	if _, _, err := worktreeBlob(root, ".git"); err == nil || !strings.Contains(err.Error(), "not a regular file") {
		t.Fatalf("directory error = %v", err)
	}
	for _, bad := range []string{"", "sha256:abc", "sha256:" + strings.Repeat("z", 64)} {
		if validDigest(bad) {
			t.Errorf("validDigest(%q) = true", bad)
		}
	}
}

func testRepo(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "-q")
	git(t, root, "config", "user.name", "Test")
	git(t, root, "config", "user.email", "test@example.com")
	write(t, root, "tracked.txt", "before\n", 0o644)
	git(t, root, "add", "--", "tracked.txt")
	git(t, root, "commit", "-q", "-m", "base")
	return root, strings.TrimSpace(git(t, root, "rev-parse", "HEAD"))
}

func writeMarker(t *testing.T, root string, value marker) {
	t.Helper()
	body, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	write(t, root, MarkerPath, string(body), 0o644)
}

func write(t *testing.T, root, rel, body string, mode os.FileMode) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), mode); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, root string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return string(out)
}
