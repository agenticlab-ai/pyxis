package native

import (
	"context"
	"errors"
	"os"
	"runtime"
	"strings"
	"testing"
)

func TestSystemObserverHelperProcess(t *testing.T) {
	if len(os.Args) < 2 || os.Args[len(os.Args)-1] != "native-helper" {
		return
	}
	_, _ = os.Stdout.WriteString("stdout")
	_, _ = os.Stderr.WriteString("stderr")
	os.Exit(0)
}

func TestSystemObserverRunsLiteralCommandWithContext(t *testing.T) {
	t.Parallel()
	observer := systemObserver{}
	if observer.Platform() != runtime.GOOS || System() == nil {
		t.Fatal("system observer composition or platform mismatch")
	}
	result, err := observer.Run(t.Context(), nil, os.Args[0], "-test.run=^TestSystemObserverHelperProcess$", "--", "native-helper")
	if err != nil || string(result.Stdout) != "stdout" || !strings.HasPrefix(string(result.Stderr), "stderr") {
		t.Fatalf("Run() = (%q, %q, %v)", result.Stdout, result.Stderr, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := observer.Run(ctx, nil, os.Args[0], "-test.run=^TestSystemObserverHelperProcess$", "--", "native-helper"); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Run() = %v", err)
	}
}

func TestSystemObserverTimezoneParsingAndCancellation(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ target, want string }{
		{target: "/var/db/timezone/zoneinfo/America/Los_Angeles", want: "America/Los_Angeles"},
		{target: "/private/var/db/timezone/zoneinfo/America/Los_Angeles", want: "America/Los_Angeles"},
		{target: "/private/var/db/timezone/tz/2026c.1.0/zoneinfo/America/Los_Angeles", want: "America/Los_Angeles"},
		{target: "/usr/share/zoneinfo/UTC", want: "UTC"},
	} {
		if got, err := zoneName(test.target); err != nil || got != test.want {
			t.Fatalf("zoneName(%q) = (%q, %v)", test.target, got, err)
		}
	}
	for _, target := range []string{
		"/etc/localtime",
		"/tmp/zoneinfo/America/Los_Angeles",
		"/private/var/db/timezone/tz//zoneinfo/America/Los_Angeles",
		"/private/var/db/timezone/tz/not-a-release/zoneinfo/America/Los_Angeles",
		"/private/var/db/timezone/tz/2026c.1.0/other/America/Los_Angeles",
		"/private/var/db/timezone/tz/2026c.1.0/zoneinfo/Not/A_Zone",
		"/usr/share/zoneinfo/../passwd",
	} {
		if _, err := zoneName(target); err == nil {
			t.Errorf("zoneName(%q) unexpectedly succeeded", target)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := (systemObserver{}).Timezone(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("canceled Timezone() = %v", err)
	}
	if got, err := systemTimezone(t.Context(), func(string) (string, error) {
		return "/usr/share/zoneinfo/UTC", nil
	}); err != nil || got != "UTC" {
		t.Fatalf("systemTimezone() = (%q, %v)", got, err)
	}
	want := errors.New("observe failed")
	if _, err := systemTimezone(t.Context(), func(string) (string, error) { return "", want }); !errors.Is(err, want) {
		t.Fatalf("systemTimezone error = %v", err)
	}
	ctx, cancel = context.WithCancel(t.Context())
	if _, err := systemTimezone(ctx, func(string) (string, error) {
		cancel()
		return "/usr/share/zoneinfo/UTC", nil
	}); !errors.Is(err, context.Canceled) {
		t.Fatalf("mid-observation cancellation = %v", err)
	}
}

func TestAbsentDiagnosticRejectsAmbiguousOutput(t *testing.T) {
	t.Parallel()
	if absentDiagnostic("other", "no crontab for test") || absentDiagnostic("cronie-1", "no crontab for test\npermission denied") || absentDiagnostic("cronie-1", "no crontab for test: denied") {
		t.Fatal("ambiguous provider output was classified as an absent table")
	}
}
