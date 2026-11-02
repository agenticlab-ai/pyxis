package cron

import (
	"bytes"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestAdmitOrderedEffectiveEnvironment(t *testing.T) {
	t.Parallel()
	first := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	second := mustRender(t, "/srv/other", "shared", "15 3 * * *")
	requirements := []EntryRequirement{
		{RepositoryHash: first.RepositoryHash, JobID: first.JobID, Timezone: "America/Los_Angeles"},
		{RepositoryHash: second.RepositoryHash, JobID: second.JobID, Timezone: "America/Los_Angeles"},
	}
	profile, _ := KnownProfile(ProviderCronie)
	table := append([]byte("MAILTO=ops@example.test\nSHELL=/bin/sh\nCRON_TZ=America/Los_Angeles\n"), first.Text...)
	table = append(table, []byte("ENV=/tmp/startup\nENV=\nRANDOM_DELAY=0\n")...)
	table = append(table, second.Text...)
	original := bytes.Clone(table)
	verdict, err := Admit(observedTable(table, profile, "America/Los_Angeles"), requirements)
	if err != nil || !verdict.Admitted || len(verdict.Environments) != 2 {
		t.Fatalf("Admit() = (%#v, %v)", verdict, err)
	}
	if verdict.Environments[0].SchedulerTimezone != "America/Los_Angeles" || verdict.Environments[1].ENV != "" || verdict.Environments[1].RandomDelay != "0" {
		t.Fatalf("effective environments = %#v", verdict.Environments)
	}
	if !bytes.Equal(table, original) {
		t.Fatal("admission changed retained bytes")
	}
}

func TestAdmitRefusesUnsafeEffectiveEnvironment(t *testing.T) {
	t.Parallel()
	entry := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	requirement := []EntryRequirement{{RepositoryHash: entry.RepositoryHash, JobID: entry.JobID, Timezone: "America/Los_Angeles"}}
	cronie, _ := KnownProfile(ProviderCronie)
	apple, _ := KnownProfile(ProviderAppleCron52)
	tests := []struct {
		name, prefix, host, contains string
		profile                      Profile
	}{
		{name: "matching host conflicting cron timezone", prefix: "CRON_TZ=UTC\n", host: "America/Los_Angeles", profile: cronie, contains: "CRON_TZ"},
		{name: "unicode name does not replace cron timezone", prefix: "CRON_TZ=UTC\nCRON_TZ\u00a0=America/Los_Angeles\n", host: "America/Los_Angeles", profile: cronie, contains: "CRON_TZ"},
		{name: "apple cron timezone unknown", prefix: "CRON_TZ=America/Los_Angeles\n", host: "America/Los_Angeles", profile: apple, contains: "unsupported CRON_TZ"},
		{name: "host mismatch", host: "UTC", profile: cronie, contains: "host timezone"},
		{name: "shell", prefix: "SHELL=/bin/zsh\n", host: "America/Los_Angeles", profile: cronie, contains: "SHELL"},
		{name: "posix startup", prefix: "ENV=/tmp/startup\n", host: "America/Los_Angeles", profile: cronie, contains: "ENV"},
		{name: "unicode posix startup", prefix: "ENV=\u00a0\n", host: "America/Los_Angeles", profile: cronie, contains: "ENV"},
		{name: "unquoted startup value", prefix: "ENV=foo\"bar\n", host: "America/Los_Angeles", profile: cronie, contains: "ENV"},
		{name: "bash startup", prefix: "BASH_ENV=/tmp/startup\n", host: "America/Los_Angeles", profile: cronie, contains: "BASH_ENV"},
		{name: "unicode bash startup", prefix: "BASH_ENV=\u00a0\n", host: "America/Los_Angeles", profile: cronie, contains: "BASH_ENV"},
		{name: "job timezone", prefix: "TZ=UTC\n", host: "America/Los_Angeles", profile: cronie, contains: "effective TZ"},
		{name: "random delay", prefix: "RANDOM_DELAY=12\n", host: "America/Los_Angeles", profile: cronie, contains: "RANDOM_DELAY"},
		{name: "unsupported delay", prefix: "RANDOM_DELAY=0\n", host: "America/Los_Angeles", profile: apple, contains: "RANDOM_DELAY"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			table := append([]byte(test.prefix), entry.Text...)
			verdict, err := Admit(observedTable(table, test.profile, test.host), requirement)
			if !errors.Is(err, ErrAdmissionRefused) || verdict.Admitted || !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("Admit() = (%#v, %v), want refusal containing %q", verdict, err, test.contains)
			}
		})
	}
}

func TestAdmitUsesDirectivePositionForUpdatedEntries(t *testing.T) {
	t.Parallel()
	first := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	second := mustRender(t, "/srv/other", "shared", "15 3 * * *")
	requirements := []EntryRequirement{
		{RepositoryHash: first.RepositoryHash, JobID: first.JobID, Timezone: "America/Los_Angeles"},
		{RepositoryHash: second.RepositoryHash, JobID: second.JobID, Timezone: "America/Los_Angeles"},
	}
	profile, _ := KnownProfile(ProviderCronie)
	table := append([]byte{}, first.Text...)
	table = append(table, []byte("CRON_TZ=UTC\n")...)
	table = append(table, second.Text...)
	if _, err := Admit(observedTable(table, profile, "America/Los_Angeles"), requirements); !errors.Is(err, ErrAdmissionRefused) || !strings.Contains(err.Error(), "/shared") {
		t.Fatalf("second-position refusal = %v", err)
	}
	table = append([]byte{}, first.Text...)
	table = append(table, []byte("CRON_TZ=UTC\nCRON_TZ='America/Los_Angeles'\n")...)
	table = append(table, second.Text...)
	if verdict, err := Admit(observedTable(table, profile, "America/Los_Angeles"), requirements); err != nil || !verdict.Admitted {
		t.Fatalf("reset directive = (%#v, %v)", verdict, err)
	}
}

func TestAdmitEvaluatesAnEntryUpdatedInPlace(t *testing.T) {
	t.Parallel()
	currentEntry := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	desiredEntry := mustRender(t, fixtureRoot, "daily", "45 6 * * *")
	current := append([]byte("CRON_TZ=UTC\n"), currentEntry.Text...)
	reconciled, err := Reconcile(current, fixtureHash, []CronEntry{desiredEntry})
	if err != nil || !reconciled.Changed {
		t.Fatalf("Reconcile() = (%#v, %v)", reconciled, err)
	}
	profile, _ := KnownProfile(ProviderCronie)
	requirements := []EntryRequirement{{RepositoryHash: fixtureHash, JobID: "daily", Timezone: "America/Los_Angeles"}}
	if _, err := Admit(observedTable(reconciled.Replacement, profile, "America/Los_Angeles"), requirements); !errors.Is(err, ErrAdmissionRefused) || !strings.Contains(err.Error(), "CRON_TZ") {
		t.Fatalf("updated entry admission = %v", err)
	}
}

func TestAdmitRequiresKnownProfileAndPresentRequirements(t *testing.T) {
	t.Parallel()
	entry := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	cronie, _ := KnownProfile(ProviderCronie)
	valid := observedTable(entry.Text, cronie, "America/Los_Angeles")
	tests := []struct {
		name         string
		table        Table
		requirements []EntryRequirement
		want         error
	}{
		{name: "absent table schema", table: Table{Bytes: entry.Text, Profile: cronie, HostTimezone: "America/Los_Angeles"}, want: ErrUnsupportedProvider},
		{name: "unknown table schema", table: Table{SchemaVersion: "pyxis-native-cron-table/v2", Bytes: entry.Text, Profile: cronie, HostTimezone: "America/Los_Angeles"}, want: ErrUnsupportedProvider},
		{name: "unknown profile", table: observedTable(entry.Text, Profile{SchemaVersion: ProfileSchemaVersion, ID: "other"}, "America/Los_Angeles"), want: ErrUnsupportedProvider},
		{name: "absent profile schema", table: observedTable(entry.Text, Profile{ID: ProviderCronie, CronTZ: true, RandomDelay: true}, "America/Los_Angeles"), want: ErrUnsupportedProvider},
		{name: "unknown profile schema", table: observedTable(entry.Text, Profile{SchemaVersion: "pyxis-native-cron-profile/v2", ID: ProviderCronie, CronTZ: true, RandomDelay: true}, "America/Los_Angeles"), want: ErrUnsupportedProvider},
		{name: "selected empty set", table: valid, requirements: []EntryRequirement{}, want: nil},
		{name: "extra requirement", table: valid, requirements: []EntryRequirement{{RepositoryHash: entry.RepositoryHash, JobID: entry.JobID, Timezone: "America/Los_Angeles"}, {RepositoryHash: fixtureHash, JobID: "other", Timezone: "America/Los_Angeles"}}, want: ErrAdmissionRefused},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if verdict, err := Admit(test.table, test.requirements); !errors.Is(err, test.want) || verdict.Admitted != (test.want == nil) {
				t.Fatalf("Admit() = (%#v, %v), want %v", verdict, err, test.want)
			}
		})
	}
}

func TestAdmitIgnoresOtherRepositoryScheduleButRetainsItsGlobals(t *testing.T) {
	t.Parallel()
	other := mustRender(t, "/srv/other", "shared", "15 3 * * *")
	selected := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	profile, _ := KnownProfile(ProviderCronie)
	table := append([]byte{}, other.Text...)
	table = append(table, []byte("CRON_TZ=America/Los_Angeles\n")...)
	table = append(table, selected.Text...)
	requirement := []EntryRequirement{{RepositoryHash: selected.RepositoryHash, JobID: selected.JobID, Timezone: "America/Los_Angeles"}}
	verdict, err := Admit(observedTable(table, profile, "America/Los_Angeles"), requirement)
	if err != nil || !verdict.Admitted || len(verdict.Environments) != 1 || verdict.Environments[0].JobID != "daily" {
		t.Fatalf("Admit() = (%#v, %v)", verdict, err)
	}
}

func TestBoundaryPayloadSchemasAndTags(t *testing.T) {
	t.Parallel()
	profile, ok := KnownProfile(ProviderCronie)
	if !ok || profile.SchemaVersion != ProfileSchemaVersion {
		t.Fatalf("KnownProfile() = (%#v, %v)", profile, ok)
	}
	for _, test := range []struct {
		value any
		tags  map[string]string
	}{
		{value: Profile{}, tags: map[string]string{"SchemaVersion": "schema_version", "ID": "provider_id", "CronTZ": "cron_tz", "RandomDelay": "random_delay"}},
		{value: Table{}, tags: map[string]string{"SchemaVersion": "schema_version", "Bytes": "bytes", "Profile": "profile", "HostTimezone": "host_timezone"}},
	} {
		typeOf := reflect.TypeOf(test.value)
		for field, want := range test.tags {
			got, _ := typeOf.FieldByName(field)
			if got.Tag.Get("json") != want {
				t.Errorf("%s.%s json tag = %q, want %q", typeOf.Name(), field, got.Tag.Get("json"), want)
			}
		}
	}
}

func observedTable(table []byte, profile Profile, timezone string) Table {
	return Table{SchemaVersion: TableSchemaVersion, Bytes: table, Profile: profile, HostTimezone: timezone}
}

func TestOwnedEntriesSharesOwnershipGrammar(t *testing.T) {
	t.Parallel()
	entry := mustRender(t, fixtureRoot, "daily", "0 2 * * *")
	table := append([]byte("MAILTO=ops@example.test\n"), entry.Text...)
	entries, err := OwnedEntries(table)
	if err != nil || len(entries) != 1 || entries[0].Offset != len("MAILTO=ops@example.test\n") || entries[0].JobID != "daily" {
		t.Fatalf("OwnedEntries() = (%#v, %v)", entries, err)
	}
	if _, err := OwnedEntries([]byte("# pyxis-maintenance/v1 broken\n")); !errors.Is(err, ErrMalformedOwnership) {
		t.Fatalf("malformed ownership error = %v", err)
	}
}
