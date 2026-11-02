// Package native provides the Darwin and Linux native-cron adapter.
package native

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"

	maintenancecron "github.com/agenticlab-ai/pyxis/internal/maintenance/cron"
)

type Result struct {
	Stdout []byte
	Stderr []byte
}

// Observer keeps host and command observations injectable and hermetic.
type Observer interface {
	Platform() string
	Timezone(context.Context) (string, error)
	Run(context.Context, []byte, string, ...string) (Result, error)
}

type Adapter struct {
	observer Observer
}

var (
	cronieVersion         = regexp.MustCompile(`^cronie 1\.(5|6|7)\.[0-9]+$`)
	darwinTimezoneRelease = regexp.MustCompile(`^[0-9]{4}[a-z]+(?:\.[0-9]+)+$`)
)

func New(observer Observer) *Adapter { return &Adapter{observer: observer} }

func System() *Adapter { return New(systemObserver{}) }

func (adapter *Adapter) Read(ctx context.Context) (maintenancecron.Table, error) {
	profile, timezone, command, err := adapter.identify(ctx)
	if err != nil {
		return maintenancecron.Table{}, err
	}
	result, err := adapter.observer.Run(ctx, nil, command, "-l")
	if err == nil {
		return maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Bytes: bytes.Clone(result.Stdout), Profile: profile, HostTimezone: timezone}, nil
	}
	if ctx.Err() != nil {
		return maintenancecron.Table{}, ctx.Err()
	}
	diagnostic := strings.TrimSpace(string(result.Stderr))
	if absentDiagnostic(profile.ID, diagnostic) {
		return maintenancecron.Table{SchemaVersion: maintenancecron.TableSchemaVersion, Profile: profile, HostTimezone: timezone}, maintenancecron.ErrNoUserTable
	}
	return maintenancecron.Table{}, fmt.Errorf("read native crontab: %w: %s", err, diagnostic)
}

func (adapter *Adapter) Install(ctx context.Context, expected maintenancecron.Profile, table []byte) error {
	known, ok := maintenancecron.KnownProfile(expected.ID)
	if !ok || known != expected {
		return fmt.Errorf("%w: profile %q", maintenancecron.ErrUnsupportedProvider, expected.ID)
	}
	profile, _, command, err := adapter.identify(ctx)
	if err != nil {
		return err
	}
	if profile != expected {
		return fmt.Errorf("%w: changed from %q to %q", maintenancecron.ErrUnsupportedProvider, expected.ID, profile.ID)
	}
	result, err := adapter.observer.Run(ctx, table, command, "-")
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return fmt.Errorf("install native crontab: %w: %s", err, strings.TrimSpace(string(result.Stderr)))
	}
	return nil
}

func (adapter *Adapter) identify(ctx context.Context) (maintenancecron.Profile, string, string, error) {
	timezone, err := adapter.observer.Timezone(ctx)
	if err != nil {
		return maintenancecron.Profile{}, "", "", fmt.Errorf("observe host timezone: %w", err)
	}
	if timezone == "" {
		return maintenancecron.Profile{}, "", "", errors.New("observe host timezone: empty observation")
	}
	var id maintenancecron.ProviderID
	var command string
	switch adapter.observer.Platform() {
	case "darwin":
		if err := adapter.identifyApple(ctx); err != nil {
			return maintenancecron.Profile{}, "", "", err
		}
		id, command = maintenancecron.ProviderAppleCron52, "/usr/bin/crontab"
	case "linux":
		if err := adapter.identifyCronie(ctx); err != nil {
			return maintenancecron.Profile{}, "", "", err
		}
		id, command = maintenancecron.ProviderCronie, "/usr/bin/crontab"
	default:
		return maintenancecron.Profile{}, "", "", fmt.Errorf("%w: platform %q", maintenancecron.ErrUnsupportedProvider, adapter.observer.Platform())
	}
	profile, _ := maintenancecron.KnownProfile(id)
	return profile, timezone, command, nil
}

func (adapter *Adapter) identifyApple(ctx context.Context) error {
	for _, candidate := range []struct{ path, identifier, program string }{
		{"/usr/bin/crontab", "com.apple.crontab", "crontab"},
		{"/usr/sbin/cron", "com.apple.cron", "cron"},
	} {
		requirement := `=identifier "` + candidate.identifier + `" and anchor apple`
		if _, err := adapter.observer.Run(ctx, nil, "/usr/bin/codesign", "--verify", "--strict", "-R", requirement, candidate.path); err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: unsigned %s", maintenancecron.ErrUnsupportedProvider, candidate.path)
		}
		result, err := adapter.observer.Run(ctx, nil, "/usr/bin/what", candidate.path)
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: unrecognized %s", maintenancecron.ErrUnsupportedProvider, candidate.path)
		}
		marker := "PROGRAM:" + candidate.program + "  PROJECT:cron-52"
		matched := false
		for line := range strings.SplitSeq(string(result.Stdout), "\n") {
			matched = matched || strings.Trim(line, " \t\r") == marker
		}
		if !matched {
			return fmt.Errorf("%w: unrecognized %s", maintenancecron.ErrUnsupportedProvider, candidate.path)
		}
	}
	return nil
}

func (adapter *Adapter) identifyCronie(ctx context.Context) error {
	var version string
	for _, command := range []string{"/usr/bin/crontab", "/usr/sbin/crond"} {
		result, err := adapter.observer.Run(ctx, nil, command, "-V")
		if err != nil {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			return fmt.Errorf("%w: %s version observation failed", maintenancecron.ErrUnsupportedProvider, command)
		}
		observed := strings.TrimSpace(string(result.Stdout))
		if !supportedCronie(observed) || version != "" && observed != version {
			return fmt.Errorf("%w: unrecognized or mismatched Cronie version", maintenancecron.ErrUnsupportedProvider)
		}
		version = observed
	}
	return nil
}

func supportedCronie(version string) bool {
	return cronieVersion.MatchString(version)
}

func absentDiagnostic(id maintenancecron.ProviderID, diagnostic string) bool {
	if strings.Contains(diagnostic, "\n") {
		return false
	}
	var prefix string
	switch id {
	case maintenancecron.ProviderAppleCron52:
		prefix = "crontab: no crontab for "
	case maintenancecron.ProviderCronie:
		prefix = "no crontab for "
	default:
		return false
	}
	username, ok := strings.CutPrefix(diagnostic, prefix)
	return ok && username != "" && !strings.ContainsAny(username, " \t:")
}

type systemObserver struct{}

func (systemObserver) Platform() string { return runtime.GOOS }

func (systemObserver) Timezone(ctx context.Context) (string, error) {
	return systemTimezone(ctx, filepath.EvalSymlinks)
}

func systemTimezone(ctx context.Context, evaluate func(string) (string, error)) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	target, err := evaluate("/etc/localtime")
	if err != nil {
		return "", err
	}
	if err := ctx.Err(); err != nil {
		return "", err
	}
	return zoneName(target)
}

func zoneName(target string) (string, error) {
	if !filepath.IsAbs(target) || filepath.Clean(target) != target {
		return "", errors.New("/etc/localtime is not a recognized zoneinfo symlink")
	}
	for _, prefix := range []string{"/var/db/timezone/zoneinfo/", "/private/var/db/timezone/zoneinfo/", "/usr/share/zoneinfo/"} {
		if strings.HasPrefix(target, prefix) && len(target) > len(prefix) {
			return validateZoneName(target[len(prefix):])
		}
	}
	const darwinRoot = "/private/var/db/timezone/tz/"
	if rest, ok := strings.CutPrefix(target, darwinRoot); ok {
		release, zone, found := strings.Cut(rest, "/zoneinfo/")
		if found && darwinTimezoneRelease.MatchString(release) {
			return validateZoneName(zone)
		}
	}
	return "", errors.New("/etc/localtime is not a recognized zoneinfo symlink")
}

func validateZoneName(zone string) (string, error) {
	if zone == "" || zone == "Local" || filepath.IsAbs(zone) || filepath.Clean(zone) != zone {
		return "", errors.New("/etc/localtime has an invalid zoneinfo name")
	}
	if _, err := time.LoadLocation(zone); err != nil {
		return "", fmt.Errorf("/etc/localtime has an invalid zoneinfo name %q: %w", zone, err)
	}
	return zone, nil
}

func (systemObserver) Run(ctx context.Context, stdin []byte, name string, args ...string) (Result, error) {
	command := exec.CommandContext(ctx, name, args...)
	command.Env = []string{"LANG=C", "LC_ALL=C"}
	command.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	if ctx.Err() != nil {
		return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, ctx.Err()
	}
	return Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes()}, err
}
