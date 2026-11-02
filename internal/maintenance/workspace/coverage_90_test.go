package workspace

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	executorruntime "github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/maintenance/installation"
)

func TestCoverage90PreparationAuthorityAndRootRefusals(t *testing.T) {
	t.Run("control seam", func(t *testing.T) {
		f := newFixture(t)
		f.admitted.Control = nil
		if _, err := (Preparer{StateRoot: f.state}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("missing execution source seam accepted")
		}
	})
	t.Run("state authority", func(t *testing.T) {
		f := newFixture(t)
		t.Setenv("XDG_STATE_HOME", "relative")
		if _, err := (Preparer{Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("invalid default state root accepted")
		}
		if _, err := (Preparer{StateRoot: filepath.Join(f.state, "missing"), Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("missing explicit state root accepted")
		}
	})
	t.Run("claim mismatch", func(t *testing.T) {
		f := newFixture(t)
		mismatch := f.owned
		mismatch.Record.Timezone = "UTC"
		if _, err := (Preparer{StateRoot: f.state}).Prepare(t.Context(), f.admitted, mismatch); err == nil {
			t.Fatal("mismatched claim accepted")
		}
		f.admitted.DailyIdentity.JobDate = "2026-10-19"
		if _, err := (Preparer{StateRoot: f.state}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("inconsistent admission identity accepted")
		}
		f.admitted.DailyIdentity.JobDate = f.admitted.JobDate
		f.admitted.Job.Delivery.Branch = ""
		if _, err := (Preparer{StateRoot: f.state}).Prepare(t.Context(), f.admitted, f.owned); err == nil {
			t.Fatal("empty configured attempt branch accepted")
		}
	})
	t.Run("source identity", func(t *testing.T) {
		f := newFixture(t)
		source, _ := f.admitted.ExecutionSource()
		invalid := f.admitted
		invalid.Ready.Repository = installation.RepositoryIdentity{}
		if _, err := validateSource(invalid, source); err == nil {
			t.Fatal("invalid repository identity accepted")
		}
		bad := source
		bad.ControlRepository = "relative"
		if _, err := validateSource(f.admitted, bad); err == nil {
			t.Fatal("relative control path accepted")
		}
		if err := os.Chmod(source.ControlRepository, 0o755); err != nil {
			t.Fatal(err)
		}
		if _, err := validateSource(f.admitted, source); err == nil {
			t.Fatal("public control repository accepted")
		}
		if err := os.Chmod(source.ControlRepository, 0o700); err != nil {
			t.Fatal(err)
		}
		other := t.TempDir()
		bad = source
		bad.EnrolledRepository = canonical(t, other)
		if _, err := validateSource(f.admitted, bad); err == nil {
			t.Fatal("different enrolled identity accepted")
		}
		if err := os.Chmod(f.enrolled, 0o700); err != nil {
			t.Fatal(err)
		}
		bad = source
		bad.ControlRepository = f.enrolled
		if _, err := validateSource(f.admitted, bad); err == nil {
			t.Fatal("overlapping control repository accepted")
		}
	})
}

func TestCoverage90CloneAndPathRefusals(t *testing.T) {
	f := newFixture(t)
	prepared, err := (Preparer{StateRoot: f.state, Probe: allowProbe}).Prepare(t.Context(), f.admitted, f.owned)
	if err != nil {
		t.Fatal(err)
	}
	source, _ := f.admitted.ExecutionSource()
	protected, _ := validateSource(f.admitted, source)
	env := executorruntime.LaunchEnvironment(port.LaunchSpec{Purpose: port.LaunchPurposeImplementation, Workdir: prepared.Workdir, RuntimeStateDir: prepared.RuntimeStateDir})
	if _, err := validateClone(t.Context(), prepared.Workdir, prepared.RuntimeStateDir, strings.Repeat("f", 40), prepared.HeadBranch, env, protected); err == nil {
		t.Fatal("wrong expected commit accepted")
	}
	if _, err := validateClone(t.Context(), prepared.Workdir, prepared.Workdir, f.commit, prepared.HeadBranch, env, protected); err == nil {
		t.Fatal("overlapping runtime state accepted")
	}
	alternates := filepath.Join(prepared.GitDir, "objects", "info", "alternates")
	if err := os.WriteFile(alternates, []byte(source.ControlRepository+"/objects\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := validateClone(t.Context(), prepared.Workdir, prepared.RuntimeStateDir, f.commit, prepared.HeadBranch, env, protected); err == nil {
		t.Fatal("object alternates accepted")
	}

	if _, err := canonicalDirectory("relative"); err == nil {
		t.Fatal("relative directory accepted")
	}
	if _, err := canonicalDirectory(filepath.Join(f.state, "missing")); err == nil {
		t.Fatal("missing directory accepted")
	}
	file := filepath.Join(f.state, "file")
	if err := os.WriteFile(file, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalDirectory(file); err == nil {
		t.Fatal("regular file accepted as directory")
	}
	link := filepath.Join(f.state, "link")
	if err := os.Symlink(filepath.Join(f.state, "maintenance"), link); err != nil {
		t.Fatal(err)
	}
	if _, err := canonicalDirectory(link); err == nil {
		t.Fatal("redirected canonical directory accepted")
	}
	if err := privateDirectory(filepath.Join(f.state, "absent")); err == nil {
		t.Fatal("missing private directory accepted")
	}
	if err := privateDirectory(file); err == nil {
		t.Fatal("private regular file accepted")
	}
	blocked := filepath.Join(f.state, "blocked")
	if err := os.WriteFile(blocked, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := ensurePrivateChildren(blocked, "child"); err == nil {
		t.Fatal("blocked private child accepted")
	}
}

func TestCoverage90ClosedGitSettings(t *testing.T) {
	for _, test := range []struct {
		key, value string
		want       bool
	}{
		{"core.repositoryformatversion", "1", true},
		{"core.repositoryformatversion", "secret", false},
		{"core.bare", "false", true},
		{"core.bare", "true", false},
		{"core.ignorecase", "true", true},
		{"core.ignorecase", "secret", false},
		{"extensions.objectformat", "sha256", true},
		{"extensions.objectformat", "secret", false},
		{"credential.helper", "secret", false},
	} {
		if got := safeLocalConfig(test.key, test.value); got != test.want {
			t.Errorf("safeLocalConfig(%q, %q) = %v", test.key, test.value, got)
		}
	}
}

func TestCoverage90ProbeAndInvalidRecord(t *testing.T) {
	f := newFixture(t)
	invalid := f.owned
	invalid.Record.OwnerNonce = "invalid"
	if err := validateAuthority(f.admitted, invalid); err == nil {
		t.Fatal("invalid owned record accepted")
	}
	if err := probeAccess(t.Context(), &executorruntime.CommandAccess{}); err == nil {
		t.Fatal("invalid access probe succeeded")
	}
}
