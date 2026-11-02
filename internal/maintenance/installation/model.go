// Package installation owns machine-local maintenance installation authority.
package installation

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

const SchemaVersion = "pyxis-maintenance-installation/v1"

var (
	ErrInvalidManifest      = errors.New("invalid maintenance installation manifest")
	ErrInstallationRequired = errors.New("maintenance installation required")
	ErrGenerationReplaced   = errors.New("maintenance installation generation replaced")
	ErrPublicationUncertain = errors.New("maintenance manifest publication durability uncertain")
)

type InstallationGeneration string
type State string

const (
	StateReady       State = "ready"
	StateInstalling  State = "installing"
	StateUnavailable State = "unavailable"
)

type InstallationManifest struct {
	SchemaVersion string                 `json:"schema_version"`
	Generation    InstallationGeneration `json:"generation"`
	State         State                  `json:"state"`
	Repository    RepositoryIdentity     `json:"repository"`
	Source        jobs.SourceBinding     `json:"source"`
	Host          HostBinding            `json:"host"`
	Schedules     []ScheduleFingerprint  `json:"schedules"`
}

type RepositoryIdentity struct {
	Root   string `json:"root"`
	SHA256 string `json:"sha256"`
}

type HostBinding struct {
	ExecutablePath string `json:"executable_path"`
	ToolSearchPath string `json:"tool_search_path"`
}

type ScheduleFingerprint struct {
	JobID  string `json:"job_id"`
	SHA256 string `json:"sha256"`
}

type ReadyBinding struct {
	Generation     InstallationGeneration `json:"generation"`
	Repository     RepositoryIdentity     `json:"repository"`
	Source         jobs.SourceBinding     `json:"source"`
	Host           HostBinding            `json:"host"`
	Schedules      []ScheduleFingerprint  `json:"schedules"`
	ManifestSHA256 string                 `json:"manifest_sha256"`
}

func NewGeneration() (InstallationGeneration, error) {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return "", fmt.Errorf("allocate installation generation: %w", err)
	}
	return InstallationGeneration(hex.EncodeToString(value[:])), nil
}

func Repository(root string) RepositoryIdentity {
	return RepositoryIdentity{Root: root, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(root)))}
}

// ScheduleFingerprints derives the installed binding without retaining prompts
// or runner settings. Disabled jobs have no installed schedule authority.
func ScheduleFingerprints(document jobs.Document) ([]ScheduleFingerprint, error) {
	if err := document.Validate(); err != nil {
		return nil, fmt.Errorf("derive installation schedules: %w", err)
	}
	result := make([]ScheduleFingerprint, 0, len(document.Jobs))
	for _, job := range document.Jobs {
		if job.Enabled {
			result = append(result, scheduleFingerprint(job.ID, job.Schedule.Cron, job.Schedule.Timezone))
		}
	}
	slices.SortFunc(result, func(a, b ScheduleFingerprint) int { return strings.Compare(a.JobID, b.JobID) })
	return result, nil
}

func scheduleFingerprint(jobID, cron, timezone string) ScheduleFingerprint {
	value := "pyxis-maintenance-schedule/v1\x00" + jobID + "\x00" + cron + "\x00" + timezone
	return ScheduleFingerprint{JobID: jobID, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte(value)))}
}

func (manifest InstallationManifest) Validate() error {
	if manifest.SchemaVersion != SchemaVersion || !validGeneration(manifest.Generation) {
		return invalid("unsupported schema or invalid generation")
	}
	if manifest.State != StateReady && manifest.State != StateInstalling && manifest.State != StateUnavailable {
		return invalid("unsupported state %q", manifest.State)
	}
	if !filepath.IsAbs(manifest.Repository.Root) || filepath.Clean(manifest.Repository.Root) != manifest.Repository.Root || manifest.Repository != Repository(manifest.Repository.Root) {
		return invalid("invalid canonical repository identity")
	}
	if !jobs.ValidSource(manifest.Source) || !validHost(manifest.Host) || manifest.Schedules == nil {
		return invalid("invalid source, host, or schedule binding")
	}
	previous := ""
	for _, schedule := range manifest.Schedules {
		if !jobs.ValidID(schedule.JobID) || !validHex(schedule.SHA256, 64) || schedule.JobID <= previous {
			return invalid("schedule fingerprints must be valid, unique, and sorted")
		}
		previous = schedule.JobID
	}
	return nil
}

func validHost(host HostBinding) bool {
	if !filepath.IsAbs(host.ExecutablePath) || filepath.Clean(host.ExecutablePath) != host.ExecutablePath || strings.ContainsAny(host.ExecutablePath+host.ToolSearchPath, "\x00\r\n") {
		return false
	}
	paths := filepath.SplitList(host.ToolSearchPath)
	if len(paths) == 0 {
		return false
	}
	for _, path := range paths {
		if !filepath.IsAbs(path) || filepath.Clean(path) != path {
			return false
		}
	}
	return true
}

func validGeneration(generation InstallationGeneration) bool { return validHex(string(generation), 32) }

func validHex(value string, size int) bool {
	if len(value) != size {
		return false
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

func invalid(format string, values ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidManifest, fmt.Sprintf(format, values...))
}
