package gitsrc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Explicit fetch exercises the transport boundary even on Git versions that
// honor GIT_NO_LAZY_FETCH. Repository configuration must not reopen a helper
// or network transport when older Git tries to fetch a missing promisor object.
func TestEvidenceGitDeniesRepositoryTransportOverrides(t *testing.T) {
	dir := newRepo(t)
	marker := filepath.Join(t.TempDir(), "executed")
	git(t, dir, "config", "protocol.ext.allow", "always")
	git(t, dir, "remote", "add", "untrusted", "ext::/usr/bin/touch "+marker)
	_, err := runGit(t.Context(), dir, nil, "fetch", "untrusted")
	if err == nil || !strings.Contains(err.Error(), "transport 'ext' not allowed") {
		t.Fatalf("repository reopened a transport: %v", err)
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Fatalf("repository executed a helper: %v", err)
	}
}
