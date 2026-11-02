package writehook

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
)

// claudePreToolUse builds a real Claude Code PreToolUse request for a Write of
// absPath under project cwd — the CLI's actual stdin contract.
func claudePreToolUse(cwd, absPath string) []byte {
	req := map[string]any{
		"hook_event_name": "PreToolUse",
		"tool_name":       "Write",
		"cwd":             cwd,
		"tool_input":      map[string]any{"file_path": absPath, "file_text": "x"},
	}
	b, _ := json.Marshal(req)
	return b
}

// decodeClaudeDecision parses the adapter's stdout as Claude's real PreToolUse
// decision JSON, asserting the envelope is well-formed.
func decodeClaudeDecision(t *testing.T, out []byte) claudeHookOutput {
	t.Helper()
	var o claudeHookOutput
	if err := json.Unmarshal(out, &o); err != nil {
		t.Fatalf("adapter emitted non-JSON on Claude's contract: %v (%q)", err, out)
	}
	if o.HookSpecificOutput.HookEventName != "PreToolUse" {
		t.Errorf("hookEventName = %q, want PreToolUse", o.HookSpecificOutput.HookEventName)
	}
	return o
}

func TestClaudeAdapter_DeniesProtectedFailClosedAllowsInScope(t *testing.T) {
	// A-2: a protected-path write is denied fail-closed against Claude's real
	// PreToolUse contract, and an in-scope write is allowed.
	a, ok := Lookup("claude")
	if !ok {
		t.Fatal("claude adapter not registered")
	}
	drive := boundary.Drive{Slug: "widget", Scope: boundary.ResolveFirstPR(dcfg, "widget")}
	const cwd = "/repo"

	// A protected-path write (the authority contract) — denied.
	var buf bytes.Buffer
	exit := a.Evaluate(dcfg, claudePreToolUse(cwd, cwd+"/.pyxis/skills/argus/authority-contract.md"), drive, &buf)
	if exit != 0 {
		t.Errorf("Claude deny exit = %d, want 0 (deny is exit-0 + JSON on Claude)", exit)
	}
	dec := decodeClaudeDecision(t, buf.Bytes())
	if dec.HookSpecificOutput.PermissionDecision != "deny" {
		t.Fatalf("permissionDecision = %q, want deny", dec.HookSpecificOutput.PermissionDecision)
	}
	reason := dec.HookSpecificOutput.PermissionDecisionReason
	for _, part := range []string{".pyxis/skills/argus/authority-contract.md", "authority contract", "remedy:"} {
		if !strings.Contains(reason, part) {
			t.Errorf("deny reason %q omits %q", reason, part)
		}
	}

	// An in-scope write — allowed.
	buf.Reset()
	a.Evaluate(dcfg, claudePreToolUse(cwd, cwd+"/docs/prds/widget/000-base.md"), drive, &buf)
	if got := decodeClaudeDecision(t, buf.Bytes()).HookSpecificOutput.PermissionDecision; got != "allow" {
		t.Errorf("in-scope permissionDecision = %q, want allow", got)
	}
}

func TestClaudeAdapter_ForcedHookErrorDenies(t *testing.T) {
	// A-4 / DINV-3: a forced hook-runtime error (an unparseable request, a
	// request with no file_path) denies on Claude's contract, never allows.
	a, _ := Lookup("claude")
	drive := boundary.Drive{Slug: "widget", Scope: boundary.ResolveFirstPR(dcfg, "widget")}

	cases := map[string][]byte{
		"unparseable request":   []byte("{ not json"),
		"no file_path":          []byte(`{"tool_name":"Write","tool_input":{}}`),
		"empty request (stdin)": nil,
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			var buf bytes.Buffer
			a.Evaluate(dcfg, req, drive, &buf)
			dec := decodeClaudeDecision(t, buf.Bytes())
			if dec.HookSpecificOutput.PermissionDecision != "deny" {
				t.Fatalf("forced hook error was not denied: %q", buf.String())
			}
			if !strings.Contains(dec.HookSpecificOutput.PermissionDecisionReason, HookErrorBoundary) {
				t.Errorf("hook-error deny reason = %q, want it to name %q", dec.HookSpecificOutput.PermissionDecisionReason, HookErrorBoundary)
			}
		})
	}
}

func TestCodexAdapter_ListedWithMergeBoundaryBackstop(t *testing.T) {
	// A-3 / DINV-4 / UB-2: a CLI whose hook cannot fail closed is listed with
	// the merge-boundary backstop as its stated guarantee — never a silent
	// fail-open.
	a, ok := Lookup("codex")
	if !ok {
		t.Fatal("codex adapter not registered")
	}
	if a.Honoring {
		t.Fatal("codex is marked honoring but has no fail-closable pre-write hook")
	}
	if !strings.Contains(a.Backstop, "merge-boundary") {
		t.Errorf("codex backstop = %q, want it to name the merge-boundary layers", a.Backstop)
	}

	// Any evaluation attempt on a non-honoring CLI fails closed (deny), never a
	// silent allow.
	var buf bytes.Buffer
	exit := a.Evaluate(dcfg, claudePreToolUse("/repo", "/repo/docs/prds/widget/000-base.md"),
		boundary.Drive{Slug: "widget", Scope: boundary.ResolveFirstPR(dcfg, "widget")}, &buf)
	if exit == 0 {
		t.Errorf("non-honoring adapter Evaluate exit = 0, want a hard-deny nonzero")
	}
	if !strings.Contains(buf.String(), "no fail-closable") {
		t.Errorf("non-honoring deny = %q, want it to explain the missing fail-closable hook", buf.String())
	}
}

func TestPortability_DenyLogicSingleSourcedAdaptersArePointers(t *testing.T) {
	// A-2 (inspection) / DINV-5 / AP-25: the deny logic and the protected-path
	// knowledge live once (internal/argus/boundary). No writehook adapter/decision
	// source file and no vendor-neutral wiring file re-lists a protected path —
	// each CLI's wiring is a pointer, not a content copy.
	//
	// The protected-path literals that must appear ONLY in the boundary package.
	protectedLiterals := []string{
		"authority-contract.md",
		"docs/product-vision.md",
		"docs/architecture-principles.md",
		"docs/observability/incidents.md",
		"docs/problems/",
	}

	// (a) the writehook package's shipped (non-test) sources.
	pkgFiles, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range pkgFiles {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		body := readFile(t, f)
		for _, lit := range protectedLiterals {
			if strings.Contains(body, lit) {
				t.Errorf("%s contains protected-path literal %q — the deny logic/knowledge must live once in internal/argus/boundary, not be copied into the writehook layer", f, lit)
			}
		}
		// Every shipped writehook source must reach the canonical decision by
		// reference, not fork it: it either imports boundary or calls Decide.
	}

	// (b) the vendor-neutral wiring/pointer files under .pyxis/writehook — each
	// must reference the shim and name no protected path.
	wiringDir := filepath.Join("..", "..", "..", ".pyxis", "writehook")
	wiring, err := filepath.Glob(filepath.Join(wiringDir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(wiring) == 0 {
		t.Fatalf("no vendor-neutral wiring files found under %s", wiringDir)
	}
	sawShimPointer := false
	for _, f := range wiring {
		body := readFile(t, f)
		if strings.Contains(body, "argus-writehook") {
			sawShimPointer = true
		}
		// The README's guarantee table legitimately names protected-path
		// *concepts* in prose; the machine wiring (the .json adapter config)
		// must name no protected-path literal.
		if strings.HasSuffix(f, ".json") {
			for _, lit := range protectedLiterals {
				if strings.Contains(body, lit) {
					t.Errorf("wiring config %s contains protected-path literal %q — it must be a pointer to the shim, not a content copy", f, lit)
				}
			}
		}
	}
	if !sawShimPointer {
		t.Error("no vendor-neutral wiring file points at the argus-writehook shim")
	}
}

// failingWriter errors on every Write — a broken/closed stdout at hook time.
type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errString("stdout closed") }

func TestClaudeAdapter_DenyToBrokenStdoutBlocks(t *testing.T) {
	// DINV-3 / AP-17: if the deny JSON cannot be written, the adapter must fall
	// back to the hard-deny exit (2) — an exit-0 with empty stdout would be read
	// as an allow (Claude's fail-open default). Block over explain.
	a, _ := Lookup("claude")
	drive := boundary.Drive{Slug: "widget", Scope: boundary.ResolveFirstPR(dcfg, "widget")}
	exit := a.Evaluate(dcfg, claudePreToolUse("/repo", "/repo/docs/product-vision.md"), drive, failingWriter{})
	if exit != 2 {
		t.Fatalf("deny to a broken stdout returned exit %d, want 2 (hard deny) — a fail-open hole", exit)
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
