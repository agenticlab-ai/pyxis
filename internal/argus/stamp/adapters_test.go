package stamp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClaudeAdapter_Registered(t *testing.T) {
	// The Claude Code commit path is wired and discoverable from the registry.
	a, ok := Lookup("claude")
	if !ok {
		t.Fatal("claude stamp adapter not registered")
	}
	if !strings.Contains(a.CommitPath, "git commit") {
		t.Errorf("claude commit path = %q, want it to name the git commit path", a.CommitPath)
	}
	if !strings.Contains(a.Wiring, ".pyxis/stamp/claude.md") {
		t.Errorf("claude wiring = %q, want it to point at the pointer config", a.Wiring)
	}
}

func TestPortability_StampLogicSingleSourcedAdaptersArePointers(t *testing.T) {
	// DINV-3 / AP-25: the trailer logic and the protected-path knowledge live
	// once (internal/argus/stamp Stamp, internal/argus/boundary). No adapter and
	// no vendor-neutral wiring file re-lists a protected path or re-implements
	// the trailer format — each CLI's wiring is a pointer, not a content copy.
	protectedLiterals := []string{
		"authority-contract.md",
		"docs/product-vision.md",
		"docs/architecture-principles.md",
		"docs/observability/incidents.md",
		"docs/problems/",
	}

	// (a) the adapter source files must name no protected path and carry no
	// trailer-building logic (they are registry metadata only). stamp.go holds
	// the one canonical trailer decision, so it is exempt from the check.
	adapterFiles, err := filepath.Glob("adapter_*.go")
	if err != nil {
		t.Fatal(err)
	}
	if len(adapterFiles) == 0 {
		t.Fatal("no stamp adapter files found — the per-CLI wiring is absent")
	}
	for _, f := range adapterFiles {
		body := readFile(t, f)
		for _, lit := range protectedLiterals {
			if strings.Contains(body, lit) {
				t.Errorf("%s contains protected-path literal %q — an adapter is a pointer, not a content copy", f, lit)
			}
		}
	}

	// (b) the vendor-neutral wiring/pointer files under .pyxis/stamp — each must
	// point at the canonical stamp (argus-stamp / cmd/argus-stamp) and name no
	// protected path.
	wiringDir := filepath.Join("..", "..", "..", ".pyxis", "stamp")
	wiring, err := filepath.Glob(filepath.Join(wiringDir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(wiring) == 0 {
		t.Fatalf("no vendor-neutral wiring files found under %s", wiringDir)
	}
	sawStampPointer := false
	for _, f := range wiring {
		body := readFile(t, f)
		if strings.Contains(body, "argus-stamp") {
			sawStampPointer = true
		}
		for _, lit := range protectedLiterals {
			if strings.Contains(body, lit) {
				t.Errorf("wiring file %s contains protected-path literal %q — it must be a pointer, not a content copy", f, lit)
			}
		}
	}
	if !sawStampPointer {
		t.Error("no vendor-neutral wiring file points at the argus-stamp producer")
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}
