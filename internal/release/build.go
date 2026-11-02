package release

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
)

// Build compiles the release's binary matrix — the scan-derived shipped set
// (AP-27) × Platforms — into OutDir, named exactly as Pack expects
// (<tool>-<version>-<platform>).
//
// This loop existed twice in prose (the system-release workflow's matrix
// step and the README's manual runbook), which is one place too many for
// the flags that decide what a released binary IS: -trimpath and
// CGO_ENABLED=0 are the difference between a reproducible static artifact
// and a machine-specific one. It lives here now, and both callers invoke
// the verb.
type BuildOptions struct {
	// Version is the system tag being built (vX.Y.Z) — it names the output
	// files, nothing more; provenance rides the manifest.
	Version string
	// RepoRoot is the source checkout to build from ("" = ".").
	RepoRoot string
	// OutDir receives the binaries; created if absent.
	OutDir string
	// Jobs bounds concurrent compiler invocations (0 = GOMAXPROCS). The
	// matrix is tools × 4 platforms — sequential is needlessly slow, and
	// unbounded would thrash the build cache.
	Jobs int
}

// BuildResult reports the scan-derived tool set and the produced files.
type BuildResult struct {
	Tools    []string
	Binaries []string // sorted file names in OutDir
}

// Build runs the matrix. Fail-closed and complete: every failure is
// collected and named in one error, so a broken tool does not hide behind
// the first one to fail.
func Build(ctx context.Context, opts BuildOptions) (BuildResult, error) {
	if err := checkVersion(opts.Version); err != nil {
		return BuildResult{}, err
	}
	repoRoot := opts.RepoRoot
	if repoRoot == "" {
		repoRoot = "."
	}
	if opts.OutDir == "" {
		return BuildResult{}, fmt.Errorf("release: build requires an output directory")
	}
	goBin, err := exec.LookPath("go")
	if err != nil {
		return BuildResult{}, fmt.Errorf("release: build: the Go toolchain is required to build a release from source: %w", err)
	}
	tools, err := ShippedTools(repoRoot)
	if err != nil {
		return BuildResult{}, err
	}
	if err := os.MkdirAll(opts.OutDir, 0o755); err != nil {
		return BuildResult{}, fmt.Errorf("release: build: out dir: %w", err)
	}
	outAbs, err := filepath.Abs(opts.OutDir)
	if err != nil {
		return BuildResult{}, fmt.Errorf("release: build: %w", err)
	}

	type job struct{ tool, plat string }
	jobs := make([]job, 0, len(tools)*len(Platforms))
	for _, tool := range tools {
		for _, plat := range Platforms {
			jobs = append(jobs, job{tool, plat})
		}
	}

	workers := opts.Jobs
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(jobs) {
		workers = len(jobs)
	}

	// A bounded pool with one explicit owner: this function starts every
	// worker, closes the queue, and waits for all of them before returning.
	// A failing target does NOT cancel its siblings — one run names every
	// broken tool, rather than whichever failed first.
	queue := make(chan job)
	var mu sync.Mutex
	var defects []string
	var produced []string
	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range queue {
				name := j.tool + "-" + opts.Version + "-" + j.plat
				err := buildOne(ctx, goBin, repoRoot, filepath.Join(outAbs, name), j.tool, j.plat, opts.Version)
				mu.Lock()
				if err != nil {
					defects = append(defects, err.Error())
				} else {
					produced = append(produced, name)
				}
				mu.Unlock()
			}
		}()
	}
	for _, j := range jobs {
		select {
		case queue <- j:
		case <-ctx.Done():
		}
	}
	close(queue)
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return BuildResult{}, fmt.Errorf("release: build: %w", err)
	}
	if len(defects) > 0 {
		sort.Strings(defects)
		return BuildResult{}, fmt.Errorf("release: build failed for %d of %d target(s):\n  %s", len(defects), len(jobs), strings.Join(defects, "\n  "))
	}
	if err := attestVersion(ctx, outAbs, tools, opts.Version); err != nil {
		return BuildResult{}, err
	}
	sort.Strings(produced)
	return BuildResult{Tools: tools, Binaries: produced}, nil
}

// attestVersion runs the just-built front-door binary for the build host's
// platform and requires it to report the version being built. The stamp
// (buildOne's -X) and the CLI's version var can drift apart silently — a
// released binary that self-reports "source" is exactly what shipped for
// every release through v0.1.3 (#868) — so the build refuses to hand over a
// set whose front door cannot attest its own version. Skipped only when the
// host platform is outside the release matrix (nothing built here can run)
// or the fixture set carries no front door.
func attestVersion(ctx context.Context, outDir string, tools []string, version string) error {
	host := runtime.GOOS + "-" + runtime.GOARCH
	runnable := false
	for _, p := range Platforms {
		if p == host {
			runnable = true
		}
	}
	shipsFrontDoor := false
	for _, t := range tools {
		if t == frontDoorTool {
			shipsFrontDoor = true
		}
	}
	if !runnable || !shipsFrontDoor {
		return nil
	}
	bin := filepath.Join(outDir, frontDoorTool+"-"+version+"-"+host)
	var stdout, stderr bytes.Buffer
	cmd := exec.CommandContext(ctx, bin, "version")
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("release: build refused — %s version failed (%v: %s); a released front door must attest the version it was built at", filepath.Base(bin), err, strings.TrimSpace(stderr.String()))
	}
	want := frontDoorTool + " " + version
	if got := strings.TrimSpace(stdout.String()); got != want {
		return fmt.Errorf("release: build refused — %s self-reports %q, want %q; the version stamp and the CLI's version var have drifted (#868)", filepath.Base(bin), got, want)
	}
	return nil
}

// buildOne compiles one tool for one platform. The flag set is the release's
// definition of a shipped binary: -trimpath (no build-machine paths in the
// artifact), -s -w (no debug tables), -X main.version (the binary attests
// the release it belongs to), CGO_ENABLED=0 (static, so a linux binary runs
// on any libc). Tools without a main.version var ignore the stamp.
func buildOne(ctx context.Context, goBin, repoRoot, out, tool, platform, version string) error {
	goos, goarch, ok := strings.Cut(platform, "-")
	if !ok {
		return fmt.Errorf("%s: %q is not a <goos>-<goarch> platform", tool, platform)
	}
	cmd := exec.CommandContext(ctx, goBin, "build", "-trimpath", "-ldflags", "-s -w -X main.version="+version, "-o", out, "./cmd/"+tool)
	cmd.Dir = repoRoot
	cmd.Env = append(os.Environ(), "GOOS="+goos, "GOARCH="+goarch, "CGO_ENABLED=0")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s %s: go build: %v: %s", tool, platform, err, strings.TrimSpace(stderr.String()))
	}
	return nil
}
