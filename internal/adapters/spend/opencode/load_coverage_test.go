package opencode

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"
)

const openCodeExportHelperMode = "PYXIS_OPENCODE_EXPORT_HELPER_MODE"

func useOpenCodeExportHelper(t *testing.T, mode string) (calledName *string, calledArgs *[]string) {
	t.Helper()
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	previous := openCodeCommandContext
	t.Cleanup(func() { openCodeCommandContext = previous })
	t.Setenv(openCodeExportHelperMode, mode)
	var name string
	var args []string
	openCodeCommandContext = func(ctx context.Context, gotName string, gotArgs ...string) *exec.Cmd {
		name = gotName
		args = append([]string(nil), gotArgs...)
		return exec.CommandContext(ctx, executable, "-test.run=^TestOpenCodeExportHelperProcess$")
	}
	return &name, &args
}

func TestLoadUsesHermeticOpenCodeExport(t *testing.T) {
	name, args := useOpenCodeExportHelper(t, "success")
	a, err := Load(context.Background(), t.TempDir(), "ses_fixture")
	if err != nil {
		t.Fatal(err)
	}
	if *name != binary || !reflect.DeepEqual(*args, []string{"export", "ses_fixture"}) {
		t.Fatalf("command = %q %#v", *name, *args)
	}
	if a.Harness() != HarnessID || len(a.Turns()) != 0 {
		t.Fatalf("adapter = harness %q, turns %#v", a.Harness(), a.Turns())
	}
}

func TestLoadReportsExportAndParseFailures(t *testing.T) {
	t.Run("command", func(t *testing.T) {
		useOpenCodeExportHelper(t, "command-error")
		_, err := Load(context.Background(), t.TempDir(), "ses_failed")
		if err == nil || !strings.Contains(err.Error(), "fixture export failed") {
			t.Fatalf("command failure = %v", err)
		}
	})
	t.Run("invalid JSON", func(t *testing.T) {
		useOpenCodeExportHelper(t, "invalid-json")
		_, err := Load(context.Background(), t.TempDir(), "ses_invalid")
		if err == nil || !strings.Contains(err.Error(), "parse export JSON") {
			t.Fatalf("parse failure = %v", err)
		}
	})
}

func TestOpenCodeExportHelperProcess(t *testing.T) {
	switch os.Getenv(openCodeExportHelperMode) {
	case "":
		return
	case "success":
		fmt.Fprint(os.Stdout, `{"messages":[]}`)
	case "invalid-json":
		fmt.Fprint(os.Stdout, `{"messages":`)
	case "command-error":
		fmt.Fprint(os.Stderr, "fixture export failed")
		os.Exit(17)
	default:
		os.Exit(18)
	}
}
