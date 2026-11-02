package surfaces

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// fakePATH puts executables named tools on a PATH containing nothing else, so
// detection sees exactly what the test says it sees.
func fakePATH(t *testing.T, tools ...string) {
	t.Helper()
	dir := t.TempDir()
	for _, name := range tools {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir)
}

func TestDetectProbesPATH(t *testing.T) {
	fakePATH(t, "codex", "cursor", "unrelated")
	got := strings.Join(Detect(), ",")
	if got != "codex,cursor" {
		t.Errorf("Detect() = %q, want codex,cursor", got)
	}
}

// The ordinary case for the repo this verb exists to fix.
func TestDetectFindsNothingWithoutAgentCLIs(t *testing.T) {
	fakePATH(t)
	if got := Detect(); len(got) != 0 {
		t.Errorf("Detect() = %v, want empty", got)
	}
}

func TestResolve(t *testing.T) {
	tests := []struct {
		name  string
		in    []string
		want  string
		wants string // substring of the expected error; "" = success
	}{
		{name: "sorted and deduplicated", in: []string{"cursor", "codex", "cursor"}, want: "codex,cursor"},
		{name: "case and spacing are forgiven", in: []string{" Codex ", "CURSOR"}, want: "codex,cursor"},
		{name: "empty items are skipped", in: []string{"codex", "", "  "}, want: "codex"},
		{
			// The knob's value set is open, so nothing downstream rejects a
			// typo — it just wires nothing, silently. Refusing here is the only
			// place that can tell the operator.
			name: "a name pyxis cannot wire is refused, not accepted", in: []string{"claude-code"},
			wants: "not a surface pyxis wires",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Resolve(tc.in)
			if tc.wants != "" {
				if err == nil {
					t.Fatalf("Resolve(%v) = %v, want a refusal", tc.in, got)
				}
				if !strings.Contains(err.Error(), tc.wants) {
					t.Errorf("refusal %q does not carry %q", err, tc.wants)
				}
				if !strings.Contains(err.Error(), "claude") {
					t.Errorf("refusal does not list what IS supported: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Resolve(%v): %v", tc.in, err)
			}
			if strings.Join(got, ",") != tc.want {
				t.Errorf("Resolve(%v) = %v, want %s", tc.in, got, tc.want)
			}
		})
	}
}

// Selection is deliberately not limited to what is installed: an operator
// provisioning a repository for a team wires surfaces their machine lacks.
func TestResolveAcceptsSurfacesThisMachineDoesNotHave(t *testing.T) {
	fakePATH(t) // nothing installed
	got, err := Resolve([]string{"claude", "opencode"})
	if err != nil {
		t.Fatalf("Resolve: %v", err)
	}
	if strings.Join(got, ",") != "claude,opencode" {
		t.Errorf("got %v", got)
	}
}

func TestStatusRows(t *testing.T) {
	fakePATH(t, "codex", "cursor")
	st := Read(config.Config{Knobs: config.Knobs{CLISurfaces: []string{"codex", "claude-code"}}})

	var got []string
	for _, r := range st.Rows() {
		got = append(got, r.Name+":"+yn(r.Installed)+yn(r.Wired))
	}
	want := "claude:nn codex:yy cursor:yn opencode:nn claude-code:ny"
	if strings.Join(got, " ") != want {
		t.Errorf("rows = %q\nwant  %q", strings.Join(got, " "), want)
	}
	// A configured name with no wiring is invisible everywhere else — the knob
	// accepts it and converge silently skips it — so the report must surface it.
	if u := strings.Join(st.Unsupported(), ","); u != "claude-code" {
		t.Errorf("Unsupported() = %q, want claude-code", u)
	}
}

// Supported is the wiring catalog, not a second list beside it.
func TestSupportedIsTheWiringCatalog(t *testing.T) {
	got := strings.Join(Supported(), ",")
	if got != "claude,codex,cursor,opencode" {
		t.Errorf("Supported() = %q", got)
	}
}

func yn(b bool) string {
	if b {
		return "y"
	}
	return "n"
}
