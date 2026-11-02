package dispatch

import (
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

func TestBranchesSeparateTaskHostsAndKeepLegacyName(t *testing.T) {
	one := port.Task{ID: "T1", Number: 1, Title: "A task"}
	legacy := headBranchFor(one)
	if legacy != "feat/t1-a-task-1" {
		t.Fatalf("legacy branch changed: %q", legacy)
	}
	one.SourceKey = "forgejo:http://localhost:3000:o/r"
	two := one
	two.SourceKey = "forgejo:http://localhost:3001:o/r"
	if headBranchFor(one) == headBranchFor(two) || !strings.HasSuffix(headBranchFor(one), "/t1-a-task-1") {
		t.Fatal("task host branches collide")
	}
}
