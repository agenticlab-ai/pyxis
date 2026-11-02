package adopt

import (
	"path/filepath"
	"testing"
)

func TestHookChainGitFailuresAreReturned(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	if _, _, err := ResolveHookChain(t.Context(), missing); err == nil {
		t.Fatal("ResolveHookChain outside a repository: want error")
	}
	if _, err := ApplyHookChain(t.Context(), missing, Hooks{}); err == nil {
		t.Fatal("ApplyHookChain outside a repository: want error")
	}
	if err := gitConfigSet(t.Context(), missing, "core.hooksPath", ".githooks"); err == nil {
		t.Fatal("gitConfigSet outside a repository: want error")
	}
	if err := gitConfigUnset(t.Context(), missing, "core.hooksPath"); err == nil {
		t.Fatal("gitConfigUnset outside a repository: want error")
	}
}
