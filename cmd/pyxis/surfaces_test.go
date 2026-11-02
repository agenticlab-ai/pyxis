package main

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/agenticlab-ai/pyxis/internal/surfaces"
)

// surfacesRepo is a repository whose config names cfgSurfaces, on a PATH where
// exactly installed are present.
func surfacesRepo(t *testing.T, cfgSurfaces string, installed ...string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
		t.Fatal(err)
	}
	body := "schema_version: pyxis-config/v2\nsystem:\n  version: \"v0.1.471\"\n  mode: release\nknobs:\n  tracker: carina\n"
	if cfgSurfaces != "" {
		body += "  cli_surfaces: " + cfgSurfaces + "\n"
	}
	if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	bin := t.TempDir()
	for _, n := range installed {
		if err := os.WriteFile(filepath.Join(bin, n), []byte("#!/bin/sh\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", bin)
	t.Chdir(root)
	return root
}

func configBody(t *testing.T, root string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(root, ".pyxis", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The report is the whole point for the operator who does not yet know what is
// wrong: it must name every supported surface, not only the installed ones, so
// "claude is not listed" is never mistaken for "pyxis cannot do claude".
func TestSurfacesReportsInstalledAndWired(t *testing.T) {
	root := surfacesRepo(t, "", "codex", "cursor")
	var stdout, stderr bytes.Buffer
	if code := surfacesVerb(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	out := stdout.String()
	for _, want := range []string{
		"claude     no          no",
		"codex      yes         no",
		"cursor     yes         no",
		"opencode   no          no",
		"--detected",
		"--all",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("report missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(configBody(t, root), "cli_surfaces") {
		t.Error("the bare report wrote to the config — looking must not be a mutation")
	}
}

func TestSurfacesSelections(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		installed []string
		want      string // the cli_surfaces value expected in the config
		wantCode  int
		wantErr   string
	}{
		{
			name: "detected wires what is installed here",
			args: []string{"--detected"}, installed: []string{"codex", "cursor"},
			want: "[codex, cursor]",
		},
		{
			name: "all wires every supported surface, installed or not",
			args: []string{"--all"}, installed: []string{"codex"},
			want: "[claude, codex, cursor, opencode]",
		},
		{
			name: "set names them exactly, including uninstalled ones",
			args: []string{"--set", "claude,opencode"},
			want: "[claude, opencode]",
		},
		{
			name:     "detected with nothing installed refuses rather than wiring nothing",
			args:     []string{"--detected"},
			wantCode: 1, wantErr: "no agent CLI on PATH",
		},
		{
			name:     "an unwirable name is refused by name",
			args:     []string{"--set", "claude-code"},
			wantCode: 2, wantErr: "not a surface pyxis wires",
		},
		{
			name:     "two selections at once is a refusal, not a guess",
			args:     []string{"--all", "--detected"},
			wantCode: 2, wantErr: "pass one",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			root := surfacesRepo(t, "", tc.installed...)
			var stdout, stderr bytes.Buffer
			code := surfacesVerb(tc.args, &stdout, &stderr)
			if code != tc.wantCode {
				t.Fatalf("exit %d, want %d\n%s%s", code, tc.wantCode, stdout.String(), stderr.String())
			}
			if tc.wantErr != "" {
				if !strings.Contains(stderr.String(), tc.wantErr) {
					t.Errorf("stderr missing %q: %s", tc.wantErr, stderr.String())
				}
				if strings.Contains(configBody(t, root), "cli_surfaces") {
					t.Error("a refused selection still wrote the config")
				}
				return
			}
			if !strings.Contains(configBody(t, root), "cli_surfaces: "+tc.want) {
				t.Errorf("config does not carry %q:\n%s", tc.want, configBody(t, root))
			}
		})
	}
}

// --preview reports and performs nothing, the posture every other mutating
// verb here holds.
func TestSurfacesPreviewWritesNothing(t *testing.T) {
	root := surfacesRepo(t, "", "codex")
	var stdout, stderr bytes.Buffer
	if code := surfacesVerb([]string{"--preview", "--all"}, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "would wire: claude, codex, cursor, opencode") {
		t.Errorf("preview does not name what it would do:\n%s", stdout.String())
	}
	if strings.Contains(configBody(t, root), "cli_surfaces") {
		t.Error("--preview wrote the config")
	}
}

// A configured name with no wiring is invisible everywhere else: the knob's
// value set is open, so nothing rejects it and converge silently skips it.
func TestSurfacesSurfacesAnUnwirableConfiguredName(t *testing.T) {
	surfacesRepo(t, "[codex, claude-code]", "codex")
	var stdout, stderr bytes.Buffer
	if code := surfacesVerb(nil, &stdout, &stderr); code != 0 {
		t.Fatalf("exit %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "claude-code") ||
		!strings.Contains(stdout.String(), "wire nothing") {
		t.Errorf("the report does not flag the unwirable name:\n%s", stdout.String())
	}
}

// A bare Enter declines; a closed stdin means nobody was asked. Both arrive as
// an empty line and only the first is a decision.
func TestAskSurfacesTellsEnterFromAClosedStdin(t *testing.T) {
	st := surfaces.Status{Supported: surfaces.Supported(), Installed: []string{"codex"}}
	tests := []struct {
		name, in     string
		want         string
		wantAnswered bool
	}{
		{name: "a bare Enter declines", in: "\n", wantAnswered: true},
		{name: "a closed stdin was never asked", in: "", wantAnswered: false},
		{name: "all", in: "all\n", want: "claude,codex,cursor,opencode", wantAnswered: true},
		{name: "detected", in: "detected\n", want: "codex", wantAnswered: true},
		{name: "a named list", in: "cursor, claude\n", want: "claude,cursor", wantAnswered: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			sel, answered, err := askSurfaces(&out, strings.NewReader(tc.in), st)
			if err != nil {
				t.Fatalf("askSurfaces: %v", err)
			}
			if answered != tc.wantAnswered {
				t.Errorf("answered = %v, want %v", answered, tc.wantAnswered)
			}
			if got := strings.Join(sel, ","); got != tc.want {
				t.Errorf("selection = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestSurfacesVerbRejectsSyntaxPositionalsAndConfigDefects(t *testing.T) {
	for _, tc := range []struct {
		name string
		args []string
	}{
		{name: "unknown flag", args: []string{"--unknown"}},
		{name: "positional", args: []string{"unexpected"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			surfacesRepo(t, "")
			var stdout, stderr bytes.Buffer
			if code := surfacesVerb(tc.args, &stdout, &stderr); code != 2 {
				t.Fatalf("exit=%d, want usage; stderr=%s", code, stderr.String())
			}
		})
	}

	t.Run("malformed repository config", func(t *testing.T) {
		root := t.TempDir()
		if err := os.MkdirAll(filepath.Join(root, ".pyxis"), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(root, ".pyxis", "config.yaml"), []byte("unknown: true\n"), 0o644); err != nil {
			t.Fatal(err)
		}
		t.Chdir(root)
		var stdout, stderr bytes.Buffer
		if code := surfacesVerb(nil, &stdout, &stderr); code != 1 || stderr.Len() == 0 {
			t.Fatalf("exit=%d stderr=%q, want named config refusal", code, stderr.String())
		}
	})
}

func TestAskSurfacesReportsInputFailureAndMissingDetection(t *testing.T) {
	st := surfaces.Status{Supported: surfaces.Supported()}
	if _, _, err := askSurfaces(&bytes.Buffer{}, iotest.ErrReader(errors.New("read failed")), st); err == nil {
		t.Fatal("askSurfaces hid an input failure")
	}
	if _, answered, err := askSurfaces(&bytes.Buffer{}, strings.NewReader("detected\n"), st); err == nil || !answered {
		t.Fatalf("detected with no installed surface = answered %v, err %v", answered, err)
	}
}
