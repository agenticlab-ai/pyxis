package cron

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/maintenance/jobs"
)

var (
	ErrUnsupportedProvider = errors.New("unsupported native cron provider")
	ErrAdmissionRefused    = errors.New("cron admission refused")
	ErrNoUserTable         = errors.New("user crontab is absent")
)

type ProviderID string

const (
	ProfileSchemaVersion = "pyxis-native-cron-profile/v1"
	TableSchemaVersion   = "pyxis-native-cron-table/v1"

	ProviderAppleCron52 ProviderID = "apple-cron-52"
	ProviderCronie      ProviderID = "cronie-1"
)

// Profile records only scheduler semantics used by admission.
type Profile struct {
	SchemaVersion string     `json:"schema_version"`
	ID            ProviderID `json:"provider_id"`
	CronTZ        bool       `json:"cron_tz"`
	RandomDelay   bool       `json:"random_delay"`
}

type Table struct {
	SchemaVersion string  `json:"schema_version"`
	Bytes         []byte  `json:"bytes"`
	Profile       Profile `json:"profile"`
	HostTimezone  string  `json:"host_timezone"`
}

// Port is owned by the maintenance consumer. Install requires the profile
// observed by Read so a provider replacement cannot cross admission unseen.
type Port interface {
	Read(context.Context) (Table, error)
	Install(context.Context, Profile, []byte) error
}

type EntryRequirement struct {
	RepositoryHash string
	JobID          string
	Timezone       string
}

type EffectiveEnvironment struct {
	RepositoryHash    string
	JobID             string
	Shell             string
	SchedulerTimezone string
	Timezone          string
	ENV               string
	BASHEnv           string
	RandomDelay       string
}

type Verdict struct {
	Admitted     bool
	Environments []EffectiveEnvironment
}

func KnownProfile(id ProviderID) (Profile, bool) {
	switch id {
	case ProviderAppleCron52:
		return Profile{SchemaVersion: ProfileSchemaVersion, ID: id}, true
	case ProviderCronie:
		return Profile{SchemaVersion: ProfileSchemaVersion, ID: id, CronTZ: true, RandomDelay: true}, true
	default:
		return Profile{}, false
	}
}

// Admit evaluates globals in retained byte order at every owned entry. It
// does not rewrite the table; native cron remains responsible for dispatch.
func Admit(table Table, requirements []EntryRequirement) (Verdict, error) {
	if table.SchemaVersion != TableSchemaVersion {
		return Verdict{}, fmt.Errorf("%w: table schema %q", ErrUnsupportedProvider, table.SchemaVersion)
	}
	known, ok := KnownProfile(table.Profile.ID)
	if !ok || known != table.Profile {
		return Verdict{}, fmt.Errorf("%w: profile %q", ErrUnsupportedProvider, table.Profile.ID)
	}
	if _, err := time.LoadLocation(table.HostTimezone); err != nil {
		return Verdict{}, fmt.Errorf("%w: unknown host timezone %q", ErrAdmissionRefused, table.HostTimezone)
	}
	owned, err := OwnedEntries(table.Bytes)
	if err != nil {
		return Verdict{}, err
	}
	wanted := make(map[string]string, len(requirements))
	for _, requirement := range requirements {
		key := requirement.RepositoryHash + "/" + requirement.JobID
		if !validHash(requirement.RepositoryHash) || !jobs.ValidID(requirement.JobID) || wanted[key] != "" {
			return Verdict{}, fmt.Errorf("%w: invalid or duplicate entry requirement", ErrAdmissionRefused)
		}
		if _, err := time.LoadLocation(requirement.Timezone); err != nil || requirement.Timezone == "" || requirement.Timezone == "Local" {
			return Verdict{}, fmt.Errorf("%w: entry %s has unknown timezone %q", ErrAdmissionRefused, key, requirement.Timezone)
		}
		wanted[key] = requirement.Timezone
	}
	environment := map[string]string{"SHELL": "/bin/sh"}
	present := map[string]bool{}
	verdict := Verdict{Environments: make([]EffectiveEnvironment, 0, len(owned))}
	matched := map[string]bool{}
	entryIndex := 0
	for offset := 0; offset < len(table.Bytes); {
		if entryIndex < len(owned) && offset == owned[entryIndex].Offset {
			entry := owned[entryIndex]
			key := entry.RepositoryHash + "/" + entry.JobID
			timezone, exists := wanted[key]
			if exists {
				effective, err := admitEnvironment(known, table.HostTimezone, timezone, entry, environment, present)
				if err != nil {
					return Verdict{}, err
				}
				verdict.Environments = append(verdict.Environments, effective)
				matched[key] = true
			}
			entryIndex++
		}
		end := bytes.IndexByte(table.Bytes[offset:], '\n')
		if end < 0 {
			end = len(table.Bytes)
		} else {
			end += offset + 1
		}
		if name, value, ok := environmentSetting(table.Bytes[offset:end]); ok {
			environment[name], present[name] = value, true
		}
		offset = end
	}
	if len(matched) != len(wanted) {
		return Verdict{}, fmt.Errorf("%w: required entries are absent from table", ErrAdmissionRefused)
	}
	verdict.Admitted = true
	return verdict, nil
}

func admitEnvironment(profile Profile, host, configured string, entry OwnedEntry, env map[string]string, present map[string]bool) (EffectiveEnvironment, error) {
	key := entry.RepositoryHash + "/" + entry.JobID
	if host != configured {
		return EffectiveEnvironment{}, fmt.Errorf("%w: entry %s host timezone %q does not match %q", ErrAdmissionRefused, key, host, configured)
	}
	scheduler := host
	if present["CRON_TZ"] {
		if !profile.CronTZ {
			return EffectiveEnvironment{}, fmt.Errorf("%w: entry %s uses unsupported CRON_TZ", ErrAdmissionRefused, key)
		}
		if env["CRON_TZ"] != "" {
			scheduler = env["CRON_TZ"]
		}
	}
	if scheduler != configured {
		return EffectiveEnvironment{}, fmt.Errorf("%w: entry %s effective CRON_TZ %q does not match %q", ErrAdmissionRefused, key, scheduler, configured)
	}
	if env["SHELL"] != "/bin/sh" {
		return EffectiveEnvironment{}, fmt.Errorf("%w: entry %s effective SHELL %q is unsupported", ErrAdmissionRefused, key, env["SHELL"])
	}
	for _, name := range []string{"ENV", "BASH_ENV"} {
		if env[name] != "" {
			return EffectiveEnvironment{}, fmt.Errorf("%w: entry %s has nonempty %s", ErrAdmissionRefused, key, name)
		}
	}
	if present["TZ"] && (env["TZ"] == "" || env["TZ"] != configured) {
		return EffectiveEnvironment{}, fmt.Errorf("%w: entry %s effective TZ %q does not match %q", ErrAdmissionRefused, key, env["TZ"], configured)
	}
	if present["RANDOM_DELAY"] && (!profile.RandomDelay || env["RANDOM_DELAY"] != "0") {
		return EffectiveEnvironment{}, fmt.Errorf("%w: entry %s has unsupported RANDOM_DELAY %q", ErrAdmissionRefused, key, env["RANDOM_DELAY"])
	}
	return EffectiveEnvironment{RepositoryHash: entry.RepositoryHash, JobID: entry.JobID, Shell: env["SHELL"], SchedulerTimezone: scheduler, Timezone: env["TZ"], ENV: env["ENV"], BASHEnv: env["BASH_ENV"], RandomDelay: env["RANDOM_DELAY"]}, nil
}

func environmentSetting(line []byte) (string, string, bool) {
	text := strings.TrimLeft(string(bytes.TrimSuffix(line, []byte("\n"))), " \t")
	if text == "" || text[0] == '#' {
		return "", "", false
	}
	equal := strings.IndexByte(text, '=')
	if equal < 1 {
		return "", "", false
	}
	name := strings.Trim(text[:equal], " \t\r\v\f")
	if len(name) >= 2 && (name[0] == '\'' || name[0] == '"') && name[len(name)-1] == name[0] {
		name = name[1 : len(name)-1]
	}
	if name == "" || strings.ContainsAny(name, " \t'\"") {
		return "", "", false
	}
	value := strings.Trim(text[equal+1:], " \t\r\v\f")
	if len(value) > 0 && (value[0] == '\'' || value[0] == '"') {
		if len(value) < 2 || value[len(value)-1] != value[0] {
			return "", "", false
		}
		value = value[1 : len(value)-1]
	}
	return name, value, true
}
