package attribution

import (
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

func TestCoverage90NamingFallbacks(t *testing.T) {
	if got := Human.String(); got != "human-authored" {
		t.Fatalf("Human.String() = %q", got)
	}
	if got := Author(99).String(); got != "unknown-authorship" {
		t.Fatalf("unknown Author.String() = %q", got)
	}
	if got := opFor(boundary.Class(99)); got != "protected-path" {
		t.Fatalf("unknown op = %q", got)
	}
	if got := remedyFor(boundary.Class(99)); got != "a human lands this change by their own PR" {
		t.Fatalf("unknown remedy = %q", got)
	}
}

func TestCoverage90ScopeAndTrailerEmptyBoundaries(t *testing.T) {
	if got := gitScopeResolver("", "", config.Config{})(""); got != nil {
		t.Fatalf("empty slug scope = %+v, want nil", got)
	}
	if got := parseTrailer("subject\n \n\t"); got.drivePresent {
		t.Fatalf("plain message parsed as trailer: %+v", got)
	}
}
