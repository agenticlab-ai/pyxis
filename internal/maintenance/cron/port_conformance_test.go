package cron_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	nativecron "github.com/agenticlab-ai/pyxis/internal/adapters/cron/native"
	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
)

type providerObserver struct {
	platform     string
	timezone     string
	table        []byte
	absent       bool
	failRead     bool
	failInstall  bool
	blockOn      string
	badProbe     bool
	badMarker    bool
	appleProject string
	mu           sync.Mutex
	installed    []byte
}

func (observer *providerObserver) Platform() string { return observer.platform }

func (observer *providerObserver) Timezone(ctx context.Context) (string, error) {
	if observer.blockOn == "timezone" {
		<-ctx.Done()
		return "", ctx.Err()
	}
	return observer.timezone, nil
}

func (observer *providerObserver) Run(ctx context.Context, stdin []byte, name string, args ...string) (nativecron.Result, error) {
	operation := name + " " + strings.Join(args, " ")
	kind := ""
	if name == "/usr/bin/codesign" {
		kind = "probe"
	} else if len(args) == 1 && args[0] == "-l" {
		kind = "read"
	} else if len(args) == 1 && args[0] == "-" {
		kind = "install"
	}
	if kind != "" && kind == observer.blockOn {
		<-ctx.Done()
		return nativecron.Result{}, ctx.Err()
	}
	if name == "/usr/bin/codesign" {
		if observer.badProbe {
			return nativecron.Result{}, errors.New("signature rejected")
		}
		return nativecron.Result{}, nil
	}
	if name == "/usr/bin/what" {
		program := "crontab"
		if strings.Contains(operation, "/usr/sbin/cron") {
			program = "cron"
		}
		project := "cron-52"
		if observer.appleProject != "" {
			project = observer.appleProject
		}
		if observer.badMarker {
			project = "cron-unknown"
		}
		return nativecron.Result{Stdout: []byte("PROGRAM:" + program + "  PROJECT:" + project + "\n")}, nil
	}
	if len(args) == 1 && args[0] == "-V" {
		version := "cronie 1.7.2\n"
		if observer.badProbe && name == "/usr/sbin/crond" {
			version = "other cron\n"
		}
		return nativecron.Result{Stdout: []byte(version)}, nil
	}
	if len(args) == 1 && args[0] == "-l" {
		if observer.failRead {
			return nativecron.Result{Stderr: []byte("permission denied\n")}, errors.New("exit 1")
		}
		if observer.absent {
			diagnostic := "no crontab for test\n"
			if observer.platform == "darwin" {
				diagnostic = "crontab: no crontab for test\n"
			}
			return nativecron.Result{Stderr: []byte(diagnostic)}, errors.New("exit 1")
		}
		return nativecron.Result{Stdout: bytes.Clone(observer.table)}, nil
	}
	if len(args) == 1 && args[0] == "-" {
		if observer.failInstall {
			return nativecron.Result{Stderr: []byte("provider write failed\n")}, errors.New("exit 1")
		}
		observer.mu.Lock()
		observer.installed = bytes.Clone(stdin)
		observer.mu.Unlock()
		return nativecron.Result{}, nil
	}
	return nativecron.Result{}, fmt.Errorf("unexpected command %s", operation)
}

func TestNativePortConformanceForSupportedProviders(t *testing.T) {
	for _, platform := range []string{"darwin", "linux"} {
		t.Run(platform, func(t *testing.T) {
			observer := &providerObserver{platform: platform, timezone: "America/Los_Angeles", table: []byte("MAILTO=ops@example.test\n")}
			var port maintenancecron.Port = nativecron.New(observer)
			table, err := port.Read(t.Context())
			if err != nil || table.SchemaVersion != maintenancecron.TableSchemaVersion || !bytes.Equal(table.Bytes, observer.table) || table.HostTimezone != observer.timezone {
				t.Fatalf("Read() = (%#v, %v)", table, err)
			}
			if table.Profile.SchemaVersion != maintenancecron.ProfileSchemaVersion || platform == "darwin" && table.Profile.ID != maintenancecron.ProviderAppleCron52 || platform == "linux" && table.Profile.ID != maintenancecron.ProviderCronie {
				t.Fatalf("profile = %#v", table.Profile)
			}
			replacement := []byte("SHELL=/bin/sh\n")
			if err := port.Install(t.Context(), table.Profile, replacement); err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(observer.installed, replacement) {
				t.Fatalf("installed = %q", observer.installed)
			}
			observer.absent = true
			absent, err := port.Read(t.Context())
			if !errors.Is(err, maintenancecron.ErrNoUserTable) || absent.SchemaVersion != maintenancecron.TableSchemaVersion || absent.Profile != table.Profile || absent.Bytes != nil {
				t.Fatalf("absent Read() = (%#v, %v)", absent, err)
			}
			observer.absent, observer.failRead = false, true
			if _, err := port.Read(t.Context()); err == nil || errors.Is(err, maintenancecron.ErrNoUserTable) {
				t.Fatalf("permission failure = %v", err)
			}
		})
	}
}

func TestNativePortRechecksProfileBeforeInstall(t *testing.T) {
	t.Parallel()
	observer := &providerObserver{platform: "darwin", timezone: "UTC"}
	port := nativecron.New(observer)
	table, err := port.Read(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	observer.platform = "linux"
	if err := port.Install(t.Context(), table.Profile, []byte("MAILTO=\n")); !errors.Is(err, maintenancecron.ErrUnsupportedProvider) || observer.installed != nil {
		t.Fatalf("changed-provider install = %v, installed=%q", err, observer.installed)
	}
}

func TestNativePortCancellationBoundsProbeReadAndInstall(t *testing.T) {
	t.Parallel()
	for _, operation := range []string{"timezone", "probe", "read", "install"} {
		t.Run(strings.TrimSpace(operation), func(t *testing.T) {
			t.Parallel()
			observer := &providerObserver{platform: "darwin", timezone: "America/Los_Angeles", blockOn: operation}
			port := nativecron.New(observer)
			ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
			defer cancel()
			var err error
			if operation == "install" {
				profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderAppleCron52)
				err = port.Install(ctx, profile, []byte("x\n"))
			} else {
				_, err = port.Read(ctx)
			}
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("canceled %q = %v", operation, err)
			}
		})
	}
}

func TestNativePortRefusesUnknownProviderObservationsBeforeTableIO(t *testing.T) {
	t.Parallel()
	for _, test := range []struct {
		name, platform string
		badProbe       bool
		badMarker      bool
	}{
		{name: "platform", platform: "freebsd"},
		{name: "apple signature or project", platform: "darwin", badProbe: true},
		{name: "apple project marker", platform: "darwin", badMarker: true},
		{name: "cronie version mismatch", platform: "linux", badProbe: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			port := nativecron.New(&providerObserver{platform: test.platform, timezone: "UTC", badProbe: test.badProbe, badMarker: test.badMarker})
			if _, err := port.Read(t.Context()); !errors.Is(err, maintenancecron.ErrUnsupportedProvider) {
				t.Fatalf("Read() error = %v", err)
			}
		})
	}
}

func TestNativePortRefusesAppleProjectSuffixBeforeInstall(t *testing.T) {
	t.Parallel()
	for _, project := range []string{"cron-520", "cron-52-local"} {
		t.Run(project, func(t *testing.T) {
			t.Parallel()
			observer := &providerObserver{platform: "darwin", timezone: "UTC", appleProject: project}
			port := nativecron.New(observer)
			profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderAppleCron52)
			if err := port.Install(t.Context(), profile, []byte("MAILTO=\n")); !errors.Is(err, maintenancecron.ErrUnsupportedProvider) || observer.installed != nil {
				t.Fatalf("Install() = %v, installed=%q", err, observer.installed)
			}
		})
	}
}

func TestNativePortRefusesUnversionedInstallProfile(t *testing.T) {
	t.Parallel()
	observer := &providerObserver{platform: "darwin", timezone: "UTC"}
	port := nativecron.New(observer)
	if err := port.Install(t.Context(), maintenancecron.Profile{ID: maintenancecron.ProviderAppleCron52}, []byte("MAILTO=\n")); !errors.Is(err, maintenancecron.ErrUnsupportedProvider) || observer.installed != nil {
		t.Fatalf("Install() = %v, installed=%q", err, observer.installed)
	}
}

func TestNativePortReportsInstallFailure(t *testing.T) {
	t.Parallel()
	observer := &providerObserver{platform: "linux", timezone: "UTC", failInstall: true}
	port := nativecron.New(observer)
	profile, _ := maintenancecron.KnownProfile(maintenancecron.ProviderCronie)
	if err := port.Install(t.Context(), profile, []byte("MAILTO=\n")); err == nil || !strings.Contains(err.Error(), "provider write failed") || observer.installed != nil {
		t.Fatalf("Install() = %v, installed=%q", err, observer.installed)
	}
}
