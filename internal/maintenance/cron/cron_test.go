package cron

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

const (
	fixtureRoot = "/srv/pyxis"
	fixtureHash = "2861aa6c79e595be59d6245707499455d5a51e76c155cab76fd8d525169f890a"
)

func TestMain(m *testing.M) {
	if os.Getenv("PYXIS_CRON_HELPER") == "1" {
		_, _ = fmt.Fprintf(os.Stdout, "PATH=%s\nARGS=%q\n", os.Getenv("PATH"), os.Args[1:])
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestGoldenReconciliation(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		jobs []jobs.Job
	}{
		{name: "install", jobs: fixtureJobs("0 2 * * *", true)},
		{name: "noop", jobs: fixtureJobs("0 2 * * *", true)},
		{name: "update", jobs: fixtureJobs("45 6 * * *", true)},
		{name: "removal", jobs: fixtureJobs("0 2 * * *", false)},
		{name: "empty", jobs: []jobs.Job{}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			current := readFixture(t, test.name+".current")
			want := readFixture(t, test.name+".want")
			entries, err := Render(fixtureRoot, fixtureBinding(), test.jobs)
			if err != nil {
				t.Fatal(err)
			}
			result, err := Reconcile(current, fixtureHash, entries)
			if err != nil {
				t.Fatal(err)
			}
			if string(current) == string(want) {
				if result.Changed || result.Replacement != nil {
					t.Fatalf("no-op result = %#v", result)
				}
				return
			}
			if !result.Changed || string(result.Replacement) != string(want) {
				t.Fatalf("replacement mismatch\n got: %q\nwant: %q", result.Replacement, want)
			}
		})
	}
}

func TestRenderQuotesShellArgumentsThenEscapesCronPercents(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native cron command grammar is POSIX")
	}
	base := t.TempDir()
	root := filepath.Join(base, "repo space ' back\\% combo")
	if err := os.MkdirAll(filepath.Join(root, ".pyxis/logs/maintenance"), 0o755); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(base, "pyxis tool ' back\\% combo")
	testBinary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(testBinary, executable); err != nil {
		t.Fatal(err)
	}
	toolPath := filepath.Join(base, "tool path ' back\\% combo")
	entries, err := Render(root, InstalledBinding{Executable: executable, ToolPath: toolPath, StableExecutable: true}, []jobs.Job{{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "7 3 * * *"}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || strings.Count(string(entries[0].Text), " maintenance run ") != 1 {
		t.Fatalf("rendered entries = %q", entries)
	}
	lines := strings.Split(string(entries[0].Text), "\n")
	command, err := unescapeCronPercents(lines[1])
	if err != nil {
		t.Fatal(err)
	}
	command = command[len("7 3 * * * "):]
	if strings.Contains(lines[1], "%") && !strings.Contains(lines[1], `\%`) {
		t.Fatalf("cron percent was not escaped: %q", lines[1])
	}
	process := exec.Command("/bin/sh", "-c", command)
	process.Env = append(os.Environ(), "PYXIS_CRON_HELPER=1")
	if output, err := process.CombinedOutput(); err != nil {
		redirected, _ := os.ReadFile(filepath.Join(root, ".pyxis/logs/maintenance/cron.log"))
		t.Fatalf("cron shell boundary: %v: %s; command=%q; redirected=%q", err, output, command, redirected)
	}
	logBytes, err := os.ReadFile(filepath.Join(root, ".pyxis/logs/maintenance/cron.log"))
	if err != nil {
		t.Fatal(err)
	}
	wantArgs := fmt.Sprintf("ARGS=%q", []string{"maintenance", "run", "--repo", root, "--job", "daily"})
	if !strings.Contains(string(logBytes), "PATH="+toolPath+"\n") || !strings.Contains(string(logBytes), wantArgs) {
		t.Fatalf("redirected helper output = %q, want PATH and %s", logBytes, wantArgs)
	}
}

func TestRenderRedirectsCommandLookupFailure(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("native cron command grammar is POSIX")
	}
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".pyxis/logs/maintenance"), 0o755); err != nil {
		t.Fatal(err)
	}
	entries, err := Render(root, InstalledBinding{Executable: filepath.Join(root, "missing-pyxis"), ToolPath: "/usr/bin:/bin", StableExecutable: true}, []jobs.Job{{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "7 3 * * *"}}})
	if err != nil {
		t.Fatal(err)
	}
	line := strings.Split(string(entries[0].Text), "\n")[1]
	command, err := unescapeCronPercents(line)
	if err != nil {
		t.Fatal(err)
	}
	process := exec.Command("/bin/sh", "-c", command[len("7 3 * * * "):])
	output, runErr := process.CombinedOutput()
	if runErr == nil || len(output) != 0 {
		t.Fatalf("lookup failure = (%v, %q), want redirected nonzero failure", runErr, output)
	}
	redirected, err := os.ReadFile(filepath.Join(root, ".pyxis/logs/maintenance/cron.log"))
	if err != nil || len(redirected) == 0 {
		t.Fatalf("redirected lookup diagnostic = (%q, %v)", redirected, err)
	}
}

func TestRenderRefusesInvalidAndUnstableBindings(t *testing.T) {
	t.Parallel()
	valid := fixtureBinding()
	tests := []struct {
		name    string
		root    string
		binding InstalledBinding
		want    error
	}{
		{name: "relative root", root: "srv/pyxis", binding: valid, want: ErrInvalidInput},
		{name: "unclean root", root: "/srv/../pyxis", binding: valid, want: ErrInvalidInput},
		{name: "root newline", root: "/srv/pyxis\nother", binding: valid, want: ErrInvalidInput},
		{name: "root carriage return", root: "/srv/pyxis\rother", binding: valid, want: ErrInvalidInput},
		{name: "root NUL", root: "/srv/pyxis\x00other", binding: valid, want: ErrInvalidInput},
		{name: "unstable executable", root: fixtureRoot, binding: InstalledBinding{Executable: valid.Executable, ToolPath: valid.ToolPath}, want: ErrUnstableExecutable},
		{name: "relative executable", root: fixtureRoot, binding: InstalledBinding{Executable: "bin/pyxis", ToolPath: valid.ToolPath, StableExecutable: true}, want: ErrInvalidInput},
		{name: "executable newline", root: fixtureRoot, binding: InstalledBinding{Executable: "/opt/pyxis\n/bin", ToolPath: valid.ToolPath, StableExecutable: true}, want: ErrInvalidInput},
		{name: "tool path NUL", root: fixtureRoot, binding: InstalledBinding{Executable: valid.Executable, ToolPath: "/bin\x00:/usr/bin", StableExecutable: true}, want: ErrInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			entries, err := Render(test.root, test.binding, fixtureJobs("0 2 * * *", true))
			if !errors.Is(err, test.want) || entries != nil {
				t.Fatalf("Render() = (%v, %v), want nil, %v", entries, err, test.want)
			}
		})
	}
}

func TestRenderRefusesInvalidJobIdentityAndSchedule(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		jobs []jobs.Job
	}{
		{name: "invalid id", jobs: []jobs.Job{{ID: "NOT-valid", Enabled: true, Schedule: jobs.Schedule{Cron: "0 2 * * *"}}}},
		{name: "duplicate id", jobs: []jobs.Job{{ID: "daily"}, {ID: "daily"}}},
		{name: "invalid enabled schedule", jobs: []jobs.Job{{ID: "daily", Enabled: true, Schedule: jobs.Schedule{Cron: "* * * * *"}}}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			entries, err := Render(fixtureRoot, fixtureBinding(), test.jobs)
			if !errors.Is(err, ErrInvalidInput) || entries != nil {
				t.Fatalf("Render() = (%v, %v), want nil, %v", entries, err, ErrInvalidInput)
			}
		})
	}
}

func TestReconcileRefusesMalformedOrDuplicateOwnershipWithoutReplacement(t *testing.T) {
	t.Parallel()
	entry := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	other := mustRender(t, "/srv/other", "shared", "15 3 * * *")
	alter := func(old, new string) []byte {
		t.Helper()
		return []byte(strings.Replace(string(entry.Text), old, new, 1))
	}
	tests := []struct {
		name    string
		current []byte
		desired []CronEntry
		want    error
	}{
		{name: "orphan marker", current: []byte("# pyxis-maintenance/v1 repo=" + fixtureHash + " job=daily\n"), want: ErrMalformedOwnership},
		{name: "malformed marker", current: []byte("# pyxis-maintenance/v1 repo=bad job=daily\n0 2 * * * echo no\n"), want: ErrMalformedOwnership},
		{name: "invalid schedule", current: alter("0 2 * * *", "* * * * *"), want: ErrMalformedOwnership},
		{name: "invalid tool quoting", current: alter("PATH='/usr", "PATH=/usr"), want: ErrMalformedOwnership},
		{name: "noncanonical tool quoting", current: alter("PATH='/usr/local", "PATH='/usr''/local"), want: ErrMalformedOwnership},
		{name: "invalid executable quoting", current: alter(" '/opt/pyxis", " /opt/pyxis"), want: ErrMalformedOwnership},
		{name: "invalid root quoting", current: alter("--repo '/srv", "--repo /srv"), want: ErrMalformedOwnership},
		{name: "invalid job quoting", current: alter("--job 'daily'", "--job daily"), want: ErrMalformedOwnership},
		{name: "invalid log quoting", current: alter(">> '/srv", ">> /srv"), want: ErrMalformedOwnership},
		{name: "job mismatch", current: alter("--job 'daily'", "--job 'other'"), want: ErrMalformedOwnership},
		{name: "root mismatch", current: alter("--repo '/srv/pyxis'", "--repo '/srv/wrong'"), want: ErrMalformedOwnership},
		{name: "trailing command", current: alter(" 2>&1\n", " 2>&1; echo bad\n"), want: ErrMalformedOwnership},
		{name: "unescaped percent", current: alter("PATH='/usr", "PATH='%/usr"), want: ErrMalformedOwnership},
		{name: "selected duplicate", current: append(append([]byte{}, entry.Text...), entry.Text...), want: ErrDuplicateOwnership},
		{name: "other duplicate", current: append(append([]byte{}, other.Text...), other.Text...), want: ErrDuplicateOwnership},
		{name: "desired duplicate", desired: []CronEntry{entry, entry}, want: ErrDuplicateOwnership},
		{name: "desired malformed", desired: []CronEntry{{RepositoryHash: fixtureHash, JobID: "daily", Text: []byte("not a pair\n")}}, want: ErrMalformedOwnership},
		{name: "desired other repository", desired: []CronEntry{other}, want: ErrInvalidInput},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			result, err := Reconcile(test.current, fixtureHash, test.desired)
			if !errors.Is(err, test.want) || result.Changed || result.Replacement != nil {
				t.Fatalf("Reconcile() = (%#v, %v), want zero result and %v", result, err, test.want)
			}
		})
	}
}

func TestReconcilePreservesArbitraryUnownedBytes(t *testing.T) {
	t.Parallel()
	current := []byte("A\r\nB\x00C")
	result, err := Reconcile(current, fixtureHash, nil)
	if err != nil {
		t.Fatal(err)
	}
	if result.Changed || result.Replacement != nil {
		t.Fatalf("unowned bytes changed: %#v", result)
	}
}

func fixtureBinding() InstalledBinding {
	return InstalledBinding{Executable: "/opt/pyxis/bin/pyxis", ToolPath: "/usr/local/bin:/usr/bin:/bin", StableExecutable: true}
}

func fixtureJobs(schedule string, enabled bool) []jobs.Job {
	return []jobs.Job{
		{ID: "daily", Enabled: enabled, Schedule: jobs.Schedule{Cron: schedule}},
		{ID: "disabled", Enabled: false, Schedule: jobs.Schedule{Cron: "30 4 * * *"}},
	}
}

func mustRender(t *testing.T, root, id, schedule string) CronEntry {
	t.Helper()
	entries, err := Render(root, fixtureBinding(), []jobs.Job{{ID: id, Enabled: true, Schedule: jobs.Schedule{Cron: schedule}}})
	if err != nil {
		t.Fatal(err)
	}
	return entries[0]
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}
