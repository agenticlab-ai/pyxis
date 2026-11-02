package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
)

func maintenanceAdmissionFixture(t *testing.T) (string, *maintenanceFakeCron) {
	t.Helper()
	document := maintenanceDocument("daily", "inspect safely", true)
	root := maintenanceRepo(t, &document)
	writeFile(t, filepath.Join(root, ".gitignore"), []byte("/.pyxis/maintenance/logs.jsonl\n"))
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	fake := &maintenanceFakeCron{table: maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: "America/Los_Angeles"}}
	oldPort, oldExecutable, oldAccess := maintenanceCronPort, maintenanceExecutable, maintenanceProcessAccess
	t.Cleanup(func() {
		maintenanceCronPort, maintenanceExecutable, maintenanceProcessAccess = oldPort, oldExecutable, oldAccess
	})
	maintenanceCronPort = fake
	maintenanceProcessAccess = func(context.Context) error { return nil }
	bin := filepath.Join(t.TempDir(), "pyxis")
	writeFile(t, bin, []byte("fixture binary"))
	if err := os.Chmod(bin, 0o700); err != nil {
		t.Fatal(err)
	}
	maintenanceExecutable = func() (string, error) { return bin, nil }
	git, err := exec.LookPath("git")
	if err != nil {
		t.Fatal(err)
	}
	tools := t.TempDir()
	for name, target := range map[string]string{"git": git, "codex": "/usr/bin/true"} {
		if err := os.Symlink(target, filepath.Join(tools, name)); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", tools)
	t.Setenv("XDG_STATE_HOME", t.TempDir())
	return root, fake
}

func TestMaintenanceInstallAdmissionFailuresNeverInstall(t *testing.T) {
	for _, test := range []struct {
		name, want string
		change     func(*testing.T, string)
	}{
		{"unignored audit", "audit admission failed", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, ".gitignore"), nil)
		}},
		{"invalid policy", "operation failed", func(t *testing.T, root string) {
			writeFile(t, filepath.Join(root, ".pyxis/maintenance/jobs.yaml"), []byte("schema_version: ["))
		}},
		{"process guard", "injected process refusal", func(_ *testing.T, _ string) {
			maintenanceProcessAccess = func(context.Context) error { return errors.New("injected process refusal") }
		}},
		{"invalid machine state", "state", func(t *testing.T, _ string) {
			t.Setenv("XDG_STATE_HOME", "relative-state")
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, fake := maintenanceAdmissionFixture(t)
			test.change(t, root)
			var stdout, stderr bytes.Buffer
			if code := run([]string{"maintenance", "install", "--repo", root}, &stdout, &stderr); code != 1 || fake.installs != 0 || !strings.Contains(stderr.String(), test.want) || stdout.Len() != 0 {
				t.Fatalf("code=%d writes=%d stdout=%q stderr=%q", code, fake.installs, stdout.String(), stderr.String())
			}
			if test.name == "invalid machine state" {
				return
			}
			store, err := installation.Open(root)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := store.Load(t.Context()); !errors.Is(err, installation.ErrInstallationRequired) {
				t.Fatalf("refusal exposed installation authority: %v", err)
			}
			if test.name == "invalid policy" || test.name == "process guard" {
				state, err := store.LoadRefusals(t.Context())
				if err != nil || len(state.Counters) != 1 || state.Counters[0].Cause != installation.CauseInstallationFailed {
					t.Fatalf("installation refusal counter = %+v, %v", state, err)
				}
			}
		})
	}
}

func TestMaintenanceHostBindingRefusesUnstableOrMissingExecutable(t *testing.T) {
	for _, test := range []struct {
		name string
		path func(*testing.T, string) (string, error)
	}{
		{"lookup failed", func(_ *testing.T, _ string) (string, error) { return "", errors.New("injected executable lookup") }},
		{"missing executable", func(t *testing.T, _ string) (string, error) { return filepath.Join(t.TempDir(), "missing"), nil }},
		{"repository executable", func(t *testing.T, root string) (string, error) {
			path := filepath.Join(root, "pyxis")
			writeFile(t, path, nil)
			if err := os.Chmod(path, 0o700); err != nil {
				t.Fatal(err)
			}
			return path, nil
		}},
	} {
		t.Run(test.name, func(t *testing.T) {
			root, fake := maintenanceAdmissionFixture(t)
			maintenanceExecutable = func() (string, error) { return test.path(t, root) }
			var stdout, stderr bytes.Buffer
			if code := run([]string{"maintenance", "install", "--repo", root}, &stdout, &stderr); code != 1 || fake.installs != 0 || stdout.Len() != 0 {
				t.Fatalf("code=%d writes=%d stdout=%q stderr=%q", code, fake.installs, stdout.String(), stderr.String())
			}
		})
	}
}

func TestMaintenanceInstallPreviewRefusesRelativeToolPathWithoutMutation(t *testing.T) {
	root, fake := maintenanceAdmissionFixture(t)
	before := snapshot(t, root)
	t.Setenv("PATH", os.Getenv("PATH")+":relative-tool-directory")
	var stdout, stderr bytes.Buffer
	if code := run([]string{"maintenance", "install", "--repo", root, "--preview"}, &stdout, &stderr); code != 1 || fake.installs != 0 || !strings.Contains(stderr.String(), "absolute, normalized PATH") {
		t.Fatalf("code=%d writes=%d stdout=%q stderr=%q", code, fake.installs, stdout.String(), stderr.String())
	}
	if after := snapshot(t, root); !reflect.DeepEqual(before, after) {
		t.Fatal("refused preview created repository files")
	}
}
