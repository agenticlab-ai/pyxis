package main

import (
	"bytes"
	"testing"
)

// The real worker/adapter process boundary, including local review before
// publication, must permit source reads and scratch writes while refusing edits.
func TestEndToEnd_ReviewToolsUseSharedReadOnlyGuard(t *testing.T) {
	fixture := newImplementNowE2EFixture(t, e2eFixtureOptions{
		Review: true, ReviewAttemptsSourceWrite: true,
	})
	var stdout, stderr bytes.Buffer
	if exit := run(fixture.args(), &stdout, &stderr); exit != 0 {
		t.Fatalf("shared review guard failed: exit=%d\n%s\n%s", exit, stdout.String(), stderr.String())
	}
	calls := readE2ECalls(t, fixture.stateDir)
	if got := len(codexLaunchesWithArg(calls, "danger-full-access")); got != 1 {
		t.Fatalf("host-confined review launches = %d, want one", got)
	}
	if got := len(callsFor(calls, "codex", "sandbox")); got != 0 {
		t.Fatalf("nested sandbox launches = %d, want zero", got)
	}
	if got := len(githubCalls(calls, "POST", "repos/"+e2eRepository+"/pulls")); got != 1 {
		t.Fatalf("proposals after immutable-source review = %d, want one", got)
	}
}
