package tracker

import (
	"strings"
	"testing"
)

func TestCarinaPureMappingAndDecodeBoundaries(t *testing.T) {
	if got := carinaType("future-type"); got != "task" {
		t.Fatalf("unknown carina type = %q", got)
	}
	issue := (carinaItemView{ID: 7, Signals: nil}).issue()
	if issue.Labels == nil || len(issue.Labels) != 0 {
		t.Fatalf("nil signals mapped to %#v", issue.Labels)
	}
	c := NewCarina(t.TempDir(), "/bin/echo")
	if _, err := c.vocabulary(); err == nil || !strings.Contains(err.Error(), "parse signal vocabulary") {
		t.Fatalf("vocabulary error = %v", err)
	}
	if _, err := c.itemsToIssues([]byte("{")); err == nil {
		t.Fatal("itemsToIssues accepted malformed JSON")
	}
	if _, err := c.show(7); err == nil || !strings.Contains(err.Error(), "parse item") {
		t.Fatalf("show error = %v", err)
	}
}

func TestCarinaCommandFailureIncludesAvailableDiagnostics(t *testing.T) {
	missing := NewCarina(t.TempDir(), "/definitely/missing/carina")
	if _, err := missing.run("list"); err == nil || !strings.Contains(err.Error(), "carina list") {
		t.Fatalf("missing command error = %v", err)
	}

	withStderr := NewCarina(t.TempDir(), "/bin/sh")
	if _, err := withStderr.run("-c"); err == nil || !strings.Contains(err.Error(), "carina -c") {
		t.Fatalf("stderr command error = %v", err)
	}
}

func TestCarinaWrappersPropagateCommandFailures(t *testing.T) {
	c := NewCarina(t.TempDir(), "/definitely/missing/carina")
	if _, err := c.vocabulary(); err == nil {
		t.Fatal("vocabulary swallowed command failure")
	}
	if err := c.ensureSignal("kind:test", "ededed", "test"); err == nil {
		t.Fatal("ensureSignal swallowed vocabulary failure")
	}
	if _, err := c.CreateIssue("title", "body", nil, "task"); err == nil {
		t.Fatal("CreateIssue swallowed command failure")
	}
	if _, err := c.OpenIssues(); err == nil {
		t.Fatal("OpenIssues swallowed command failure")
	}
}
