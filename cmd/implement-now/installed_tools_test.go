package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
)

var installedTools struct {
	sync.Mutex
	built map[string]bool
}

// Each installed CLI is built once into the package-owned helper directory.
// Every caller receives independent files, so its PATH and mutations stay local.
func installedCLITools(t *testing.T, names ...string) string {
	t.Helper()
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	shared := filepath.Join(e2eHelperBinDir, "installed")
	dst := t.TempDir()
	installedTools.Lock()
	defer installedTools.Unlock()
	if installedTools.built == nil {
		installedTools.built = make(map[string]bool)
	}
	built := installedTools.built
	if e2eHelperBinDir == "" {
		// TestMain does not create the POSIX helper directory on Windows.
		// Keep those callers' build products in test-owned temporary storage.
		shared = t.TempDir()
		built = make(map[string]bool)
	}
	if err := os.MkdirAll(shared, 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		src := filepath.Join(shared, name)
		if !built[name] {
			cmd := exec.CommandContext(t.Context(), "go", "build", "-o", src, "./cmd/"+name)
			cmd.Dir = root
			if output, err := cmd.CombinedOutput(); err != nil {
				t.Fatalf("build installed %s: %v\n%s", name, err, output)
			}
			built[name] = true
		}
		if err := copyExecutable(src, filepath.Join(dst, name)); err != nil {
			t.Fatal(err)
		}
	}
	return dst
}
