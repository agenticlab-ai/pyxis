package reviewinvoke

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

func TestReviewLocalLaunchHasCanonicalInputsAtExactRevision(t *testing.T) {
	world := newLocalWorld(t)
	// Uncommitted caller state must never enter the review's frozen proposal.
	writeFile(t, filepath.Join(world.worktree, "pkg.go"), "uncommitted change\n")
	charter := "Frozen charter: preserve the widget contract.\n"
	rt := &fakeRuntime{launchCheck: func(spec port.LaunchSpec) error {
		root := filepath.Join(spec.RuntimeStateDir, "review-evidence", "input-root")
		diff := mustGit(t, world.worktree, "diff", "--no-ext-diff", "--no-textconv", world.base, world.tip, "--")
		inputs := pack.ReviewInputs{Subject: LocalProposalRef("o/r", world.branch), Proposal: diff,
			CodegateVerdict: []byte("All required repository checks were green at exactly " + world.tip + " when the trusted host admitted this review.\n"),
			Charter:         []byte(charter), Calibration: []byte("approved calibration\n"), DesignBindings: []byte("Relevant frozen design bindings\n")}
		for _, input := range []struct {
			name string
			data []byte
		}{
			{"proposal.diff", inputs.Proposal}, {"codegate.txt", inputs.CodegateVerdict},
			{"charter.md", inputs.Charter}, {"calibration.md", inputs.Calibration},
		} {
			path := filepath.Join(root, "review", input.name)
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, input.data) {
				return errors.New("host input not ready at launch: " + input.name)
			}
			info, err := os.Stat(path)
			if err != nil || info.Mode().Perm() != 0o600 {
				return errors.New("input permissions")
			}
		}
		if _, err := os.Stat(filepath.Join(root, "review", "design-bindings.md")); !os.IsNotExist(err) {
			return errors.New("host invented design bindings")
		}
		writeFile(t, filepath.Join(root, "review", "design-bindings.md"), string(inputs.DesignBindings))
		got, err := pack.BuildFromRoot(t.Context(), root, inputs.Subject)
		if err != nil {
			return err
		}
		want, err := pack.Build(inputs)
		if err != nil || !bytes.Equal(got, want) {
			return errors.New("host inputs changed canonical pack")
		}
		if !strings.Contains(spec.Brief, root) || !strings.Contains(spec.Brief, "--internal-review-context") {
			return errors.New("reviewer was not given exact input path and Go helper")
		}
		return nil
	}}
	i := localInvoker(t, rt, world.repo)
	result := i.ReviewLocal(t.Context(), port.Task{ID: "T5", Brief: charter}, localWorkdir{world.worktree, world.branch}, 1)
	if result.Err != nil || rt.launches != 1 {
		t.Fatalf("result=%+v launches=%d", result, rt.launches)
	}
}

// The staged calibration is the exact revision's committed snapshot, never the
// caller's dirty worktree bytes, and every invocation of the same reviewer gets
// its own disposable private runtime root: no design bindings exist before the
// reviewer selects them, and no runtime state survives the invocation or is
// reused by the next one.
func TestReviewLocalStagesCommittedCalibrationInFreshRuntimePerInvocation(t *testing.T) {
	world := newLocalWorld(t)
	const calibrationPath = ".pyxis/skills/impl-review/calibration.md"
	committed := mustGit(t, world.worktree, "show", world.tip+":"+calibrationPath)
	dirty := []byte("caller-local uncommitted calibration\n")
	if bytes.Equal(committed, dirty) {
		t.Fatal("fixture degenerate: dirty bytes match the committed snapshot")
	}
	// Dirty the tracked calibration after the worktree's HEAD is committed, in
	// both the working tree and the index, so any reader that consults the
	// caller's checkout instead of the committed revision observes wrong bytes.
	writeFile(t, filepath.Join(world.worktree, calibrationPath), string(dirty))
	runGit(t, world.worktree, "add", calibrationPath)
	if status := strings.TrimSpace(string(mustGit(t, world.worktree, "status", "--porcelain", "--", calibrationPath))); status == "" {
		t.Fatal("fixture failed to dirty the caller's tracked calibration")
	}

	type launchObservation struct{ runtime, workdir string }
	var observed []launchObservation
	rt := &fakeRuntime{launchCheck: func(spec port.LaunchSpec) error {
		if spec.Purpose != port.LaunchPurposeReview {
			return errors.New("observed a non-review launch spec")
		}
		observed = append(observed, launchObservation{runtime: spec.RuntimeStateDir, workdir: spec.Workdir})
		if spec.Workdir == world.worktree || spec.Workdir == world.repo {
			return errors.New("review launched inside the caller's dirty checkout")
		}
		staged, err := os.ReadFile(filepath.Join(spec.RuntimeStateDir, "review-evidence", "input-root", "review", "calibration.md"))
		if err != nil {
			return err
		}
		if !bytes.Equal(staged, committed) {
			return fmt.Errorf("staged calibration %q is not the committed snapshot %q", staged, committed)
		}
		checkout, err := os.ReadFile(filepath.Join(spec.Workdir, calibrationPath))
		if err != nil || !bytes.Equal(checkout, committed) {
			return fmt.Errorf("review checkout calibration %q (%v) is not the committed snapshot", checkout, err)
		}
		if _, err := os.Stat(filepath.Join(spec.RuntimeStateDir, "review-evidence", "input-root", "review", "design-bindings.md")); !os.IsNotExist(err) {
			return errors.New("design bindings existed before reviewer selection")
		}
		return nil
	}}
	i := localInvoker(t, rt, world.repo)
	task := port.Task{ID: "T5", Brief: "Frozen charter: preserve the widget contract."}
	work := localWorkdir{world.worktree, world.branch}

	first := i.ReviewLocal(t.Context(), task, work, 1)
	if first.Err != nil || first.Verdict != implreport.VerdictApprove || first.Revision != world.tip || first.Round != 1 {
		t.Fatalf("first review = %+v", first)
	}
	if len(observed) != 1 {
		t.Fatalf("first review observed %d launches", len(observed))
	}
	if _, err := os.Stat(observed[0].runtime); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("first runtime root %s survived its invocation: %v", observed[0].runtime, err)
	}
	if got, err := os.ReadFile(filepath.Join(world.worktree, calibrationPath)); err != nil || !bytes.Equal(got, dirty) {
		t.Fatalf("caller's dirty calibration was modified: %q %v", got, err)
	}

	second := i.ReviewLocal(t.Context(), task, work, 2)
	if second.Err != nil || second.Verdict != implreport.VerdictApprove || second.Revision != world.tip || second.Round != 2 {
		t.Fatalf("second review = %+v", second)
	}
	if rt.launches != 2 || len(observed) != 2 {
		t.Fatalf("launches=%d observed=%d", rt.launches, len(observed))
	}
	if observed[0].runtime == observed[1].runtime || observed[0].workdir == observed[1].workdir {
		t.Fatalf("second invocation reused first-invocation state: %+v", observed)
	}
	for _, o := range observed {
		if _, err := os.Stat(o.runtime); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("runtime root %s survived its invocation: %v", o.runtime, err)
		}
	}
}

func TestPostGreenInputPreparationUsesRevisionMergeBase(t *testing.T) {
	world := newLocalWorld(t)
	// A tracking ref takes precedence over a divergent local mainline.
	runGit(t, world.repo, "update-ref", "refs/remotes/origin/main", world.base)
	i := &Invoker{repoDir: world.repo, remote: "origin", mainline: "main"}
	state := t.TempDir()
	if _, err := i.materializeReviewInputs(t.Context(), reviewInputPreparation{workdir: world.worktree, state: state, revision: world.tip, charter: "captured charter"}); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(state, "review-evidence", "input-root", "review", "proposal.diff"))
	want := mustGit(t, world.repo, "diff", world.base, world.tip, "--")
	if err != nil || !bytes.Equal(got, want) {
		t.Fatalf("wrong proposal: %q %v", got, err)
	}
}

func TestReviewInputPreparationRefusesMissingOrUnreadableInputs(t *testing.T) {
	for _, kind := range []string{"charter", "base", "merge-base", "diff", "calibration", "root", "write", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			world := newLocalWorld(t)
			i := &Invoker{repoDir: world.repo}
			in := reviewInputPreparation{workdir: world.worktree, state: t.TempDir(), revision: world.tip, base: world.base, charter: "charter"}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch kind {
			case "charter":
				in.charter = " \n"
			case "base":
				in.base = ""
				i.mainline = "absent"
			case "merge-base":
				in.base = ""
				in.revision = "missing-revision"
			case "diff":
				in.revision = "missing-revision"
			case "calibration":
				if err := os.Remove(filepath.Join(world.worktree, ".pyxis", "skills", "impl-review", "calibration.md")); err != nil {
					t.Fatal(err)
				}
			case "root":
				writeFile(t, filepath.Join(in.state, "review-evidence"), "not a directory")
			case "write":
				if err := os.MkdirAll(filepath.Join(in.state, "review-evidence", "input-root", "review", "proposal.diff"), 0o700); err != nil {
					t.Fatal(err)
				}
			case "cancelled":
				cancel()
			}
			if _, err := i.materializeReviewInputs(ctx, in); err == nil {
				t.Fatal("missing or unreadable input accepted")
			}
		})
	}
}

func TestInputPreparationFailureStopsBothReviewLaunches(t *testing.T) {
	rt := &fakeRuntime{}
	i := lifecycleInvoker(t, rt, &fakeVerifier{}, 1e9)
	i.prepareInputs = func(context.Context, reviewInputPreparation) (*preparedReviewInputs, error) {
		return nil, errors.New("input unavailable")
	}
	result := i.Invoke(t.Context(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
	if result.Err == nil || rt.launches != 0 || !strings.Contains(result.Err.Error(), "input unavailable") {
		t.Fatalf("post-green result=%+v", result)
	}
	world := newLocalWorld(t)
	i = localInvoker(t, rt, world.repo)
	local := i.ReviewLocal(t.Context(), port.Task{ID: "T5"}, localWorkdir{world.worktree, world.branch}, 1)
	if local.Err == nil || rt.launches != 0 || !strings.Contains(local.Err.Error(), "missing frozen task charter") {
		t.Fatalf("local result=%+v", local)
	}
}

func TestReviewLocalRejectsTamperedHostInputsEvenAfterRestoration(t *testing.T) {
	for _, name := range []string{"proposal.diff", "codegate.txt", "charter.md", "calibration.md"} {
		for _, restore := range []bool{false, true} {
			t.Run(name+"/"+map[bool]string{false: "overwritten", true: "restored"}[restore], func(t *testing.T) {
				world := newLocalWorld(t)
				rt := &fakeRuntime{launchCheck: func(spec port.LaunchSpec) error {
					return forgePreparedPack(t, spec, LocalProposalRef("o/r", world.branch), name, restore)
				}}
				i := localInvoker(t, rt, world.repo)
				result := i.ReviewLocal(t.Context(), port.Task{ID: "T5", Brief: "frozen charter"}, localWorkdir{world.worktree, world.branch}, 1)
				if result.Err == nil || result.Artifact != "" || result.Verdict != "" || !strings.Contains(result.Err.Error(), "host-prepared review inputs") {
					t.Fatalf("tampered review accepted: %+v", result)
				}
			})
		}
	}
}

func forgePreparedPack(t *testing.T, spec port.LaunchSpec, subject, name string, restore bool) error {
	t.Helper()
	root := filepath.Join(spec.RuntimeStateDir, "review-evidence", "input-root")
	path := filepath.Join(root, "review", name)
	original, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	writeFile(t, path, "attacker-selected input\n")
	writeFile(t, filepath.Join(root, "review", "design-bindings.md"), "frozen design\n")
	body, err := pack.BuildFromRoot(t.Context(), root, subject)
	if err != nil {
		return err
	}
	writeFile(t, filepath.Join(spec.RuntimeStateDir, "review-evidence", "pack.txt"), string(body))
	if restore {
		writeFile(t, path, string(original))
	}
	return nil
}

func TestPostGreenRejectsTamperedPackBeforePublication(t *testing.T) {
	for _, name := range []string{"", "proposal.diff", "codegate.txt", "charter.md", "calibration.md"} {
		t.Run("input-"+name, func(t *testing.T) {
			world := newLocalWorld(t)
			rt := &fakeRuntime{}
			if name != "" {
				rt.launchCheck = func(spec port.LaunchSpec) error {
					return forgePreparedPack(t, spec, "github:o/r#548", name, true)
				}
			}
			publisher := &fakeVerifier{}
			i := lifecycleInvoker(t, rt, publisher, time.Minute)
			i.repoDir = world.repo
			// Keep the lifecycle fixture's publication machinery, while preparing
			// real committed inputs and retaining them in the production host.
			i.prepareInputs = func(ctx context.Context, in reviewInputPreparation) (*preparedReviewInputs, error) {
				in.workdir, in.base, in.revision, in.charter = world.worktree, world.base, world.tip, "captured frozen charter"
				return (&Invoker{repoDir: world.repo}).materializeReviewInputs(ctx, in)
			}
			result := i.Invoke(t.Context(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
			if name == "" {
				if result.Err != nil || publisher.publishes != 1 {
					t.Fatalf("valid review result=%+v publications=%d", result, publisher.publishes)
				}
			} else if result.Err == nil || publisher.publishes != 0 || !strings.Contains(result.Err.Error(), "host-prepared review inputs") {
				t.Fatalf("tampered pack reached publication: result=%+v publications=%d", result, publisher.publishes)
			}
		})
	}
}

func TestPreparedReviewInputsVerifyCanonicalPackAndRefuseUntrustedEvidence(t *testing.T) {
	for _, kind := range []string{"valid", "changed-design", "missing-state", "missing-design", "escaped-design", "directory-design", "oversized-design", "empty-subject", "missing-host-input", "missing-pack", "changed-pack", "wrong-subject", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			world := newLocalWorld(t)
			state := t.TempDir()
			p, err := (&Invoker{repoDir: world.repo}).materializeReviewInputs(t.Context(), reviewInputPreparation{workdir: world.worktree, state: state, revision: world.tip, base: world.base, charter: "frozen charter"})
			if err != nil {
				t.Fatal(err)
			}
			subject := "github:o/r#548"
			designPath := filepath.Join(state, "review-evidence", "input-root", "review", "design-bindings.md")
			packPath := filepath.Join(state, "review-evidence", "pack.txt")
			inputs := p.inputs
			inputs.Subject, inputs.DesignBindings = subject, []byte("reviewer-selected governing design\n")
			if kind == "changed-design" {
				inputs.DesignBindings = []byte("another applicable design selected by the reviewer\n")
			}
			writeFile(t, designPath, string(inputs.DesignBindings))
			if kind == "wrong-subject" {
				inputs.Subject = "github:o/r#999"
			}
			body, err := pack.Build(inputs)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, packPath, string(body))
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			switch kind {
			case "missing-state":
				state = filepath.Join(state, "absent")
			case "missing-design":
				if err := os.Remove(designPath); err != nil {
					t.Fatal(err)
				}
			case "escaped-design":
				if err := os.Remove(designPath); err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(t.TempDir(), "design.md")
				writeFile(t, outside, "outside runtime state")
				if err := os.Symlink(outside, designPath); err != nil {
					t.Fatal(err)
				}
			case "directory-design":
				if err := os.Remove(designPath); err != nil {
					t.Fatal(err)
				}
				if err := os.Mkdir(designPath, 0o700); err != nil {
					t.Fatal(err)
				}
			case "oversized-design":
				writeFile(t, designPath, strings.Repeat("x", (8<<20)+1))
			case "empty-subject":
				subject = ""
			case "missing-host-input":
				p.inputs.Charter = nil
			case "missing-pack":
				if err := os.Remove(packPath); err != nil {
					t.Fatal(err)
				}
			case "changed-pack":
				writeFile(t, packPath, "forged canonical pack")
			case "cancelled":
				cancel()
			}
			err = p.verify(ctx, state, subject)
			valid := kind == "valid" || kind == "changed-design"
			if (err == nil) != valid {
				t.Fatalf("verify %s: %v", kind, err)
			}
		})
	}
}
