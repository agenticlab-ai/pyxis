package cron

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

// Native cron interprets backslashes before passing command text to /bin/sh.
// This fixture follows Apple's do_command.c command scanner, not the package's
// own decoder: https://github.com/apple-oss-distributions/cron/blob/main/cron/do_command.c#L174-L198
// Cronie preserves doubled backslashes instead:
// https://github.com/cronie-crond/cronie/blob/master/src/do_command.c#L163-L187
// It exercises the boundary without inspecting or changing the host crontab.
func TestRenderNativeCronBackslashBoundary(t *testing.T) {
	for _, flavor := range []struct {
		name                string
		collapseBackslashes bool
	}{{"macos", true}, {"cronie", false}} {
		t.Run(flavor.name, func(t *testing.T) {
			for _, suffix := range []string{`slash\%percent`, `double\\slash`, `mixed\%\\'quote`} {
				t.Run(suffix, func(t *testing.T) {
					base := t.TempDir()
					root := filepath.Join(base, "repo "+suffix)
					if err := os.MkdirAll(filepath.Join(root, ".pyxis/logs/maintenance"), 0o700); err != nil {
						t.Fatal(err)
					}
					binary, err := os.Executable()
					if err != nil {
						t.Fatal(err)
					}
					executable := filepath.Join(base, "pyxis "+suffix)
					if err := os.Symlink(binary, executable); err != nil {
						t.Fatal(err)
					}
					toolPath := filepath.Join(base, "tools "+suffix)
					entries, err := Render(root, InstalledBinding{Executable: executable, ToolPath: toolPath, StableExecutable: true}, []jobs.Job{{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "7 3 * * *"}}})
					if err != nil {
						t.Fatal(err)
					}
					_, entry, ok := strings.Cut(string(entries[0].Text), "\n")
					if !ok {
						t.Fatal("missing owned entry")
					}
					entry = strings.TrimSuffix(entry, "\n")
					command, err := nativeCronCommand(strings.TrimPrefix(entry, "7 3 * * * "), flavor.collapseBackslashes)
					if err != nil {
						t.Fatalf("native cron rejected rendered command: %v", err)
					}
					process := exec.CommandContext(t.Context(), "/bin/sh", "-c", command)
					process.Env = append(os.Environ(), "PYXIS_CRON_HELPER=1")
					if output, err := process.CombinedOutput(); err != nil {
						t.Fatalf("native cron/shell boundary: %v: %s; command=%q", err, output, command)
					}
					logged, err := os.ReadFile(filepath.Join(root, ".pyxis/logs/maintenance/cron.log"))
					if err != nil {
						t.Fatal(err)
					}
					wantArgs := fmt.Sprintf("ARGS=%q", []string{"maintenance", "run", "--repo", root, "--job", "daily"})
					if !strings.Contains(string(logged), "PATH="+toolPath+"\n") || !strings.Contains(string(logged), wantArgs) {
						t.Fatalf("native cron changed literal binding: %q; want PATH=%q and %s", logged, toolPath, wantArgs)
					}
				})
			}
		})
	}
}

func nativeCronCommand(input string, collapseBackslashes bool) (string, error) {
	var output bytes.Buffer
	escaped := false
	for index := 0; index < len(input); index++ {
		value := input[index]
		if escaped {
			if value == '%' || collapseBackslashes && value == '\\' {
				output.Truncate(output.Len() - 1)
			}
			output.WriteByte(value)
			escaped = false
			continue
		}
		if value == '%' {
			return "", fmt.Errorf("unescaped percent begins cron stdin at byte %d", index)
		}
		output.WriteByte(value)
		escaped = value == '\\'
	}
	return output.String(), nil
}
