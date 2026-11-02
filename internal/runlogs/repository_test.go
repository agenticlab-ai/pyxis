package runlogs

import (
	"path/filepath"
	"testing"
)

func TestRepositoryBindingDoesNotLeakToSiblingContexts(t *testing.T) {
	workspace, owner, second := t.TempDir(), t.TempDir(), t.TempDir()
	parent := t.Context()
	bound := WithRepository(parent, owner)
	for _, tc := range []struct {
		name, workspace, want string
		bound                 bool
	}{
		{name: "bound", workspace: workspace, want: owner, bound: true},
		{name: "no workspace", want: owner, bound: true},
		{name: "sibling", workspace: workspace, want: workspace},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := parent
			if tc.bound {
				ctx = bound
			}
			if got := Repository(ctx, tc.workspace); got != tc.want {
				t.Fatalf("Repository=%s, want %s", got, tc.want)
			}
		})
	}
	if got := Repository(WithRepository(bound, " \t"), workspace); got != owner {
		t.Fatalf("empty owner cleared binding: %s", got)
	}
	if got := Repository(WithRepository(bound, second), workspace); got != second {
		t.Fatalf("nested owner=%s, want %s", got, second)
	}
	// Binding does not bypass the existing log-path confinement checks.
	if _, err := Resolve(bound, Repository(bound, workspace), Agents, filepath.Join(workspace, "outside.log")); err == nil {
		t.Fatal("binding permitted logs outside the owner's log tree")
	}
}
