package coveragegate

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

var measureMkdirTemp = os.MkdirTemp

// Measure runs the default Go tests once with repository-wide coverage
// instrumentation, then returns their de-duplicated statement
// coverage. The suite is cacheable: Go
// stores coverprofiles in the test cache, so an unchanged tree reprints the
// same profile instead of re-running every package. Test output is streamed
// to output so a red check remains diagnosable through the enclosing
// code-gate report. The repository's slowest packages run split across
// concurrent invocations, and packages whose tests are disabled are left out
// of the report.
func Measure(ctx context.Context, dir, pattern string, output io.Writer) (Report, error) {
	report, err := repositoryPlan.Measure(ctx, dir, pattern, output)
	if err != nil {
		return Report{}, err
	}
	return report.Without(repositoryExcluded), nil
}

// Measure measures like the package-level Measure, splitting p's packages
// across concurrent invocations whose output lines are never interleaved.
func (p Plan) Measure(ctx context.Context, dir, pattern string, output io.Writer) (Report, error) {
	if pattern == "" {
		return Report{}, fmt.Errorf("coverage package pattern is empty")
	}
	temp, err := measureMkdirTemp("", "pyxis-coverage-")
	if err != nil {
		return Report{}, fmt.Errorf("create coverage directory: %w", err)
	}
	defer func() {
		// Temporary profile cleanup is best-effort after the result is read.
		_ = os.RemoveAll(temp)
	}()

	runs := p.invocations(ctx, dir, pattern)
	if len(runs) == 1 {
		profile := filepath.Join(temp, "coverage.out")
		if err := goTest(ctx, dir, pattern, profile, runs[0].args, output); err != nil {
			return Report{}, fmt.Errorf("go test with coverage: %w", err)
		}
		return readProfiles([]string{profile})
	}

	// Concurrent invocations would otherwise each compile and link the same
	// instrumented packages. A failed build is not reported here: the measured
	// invocations repeat it with their diagnostics.
	warm := exec.CommandContext(ctx, "go", "test", "-c", "-o", os.DevNull, "-covermode=atomic", "-coverpkg="+pattern, pattern)
	warm.Dir = dir
	_ = warm.Run()

	var mu sync.Mutex
	profiles := make([]string, len(runs))
	failures := make([]error, len(runs))
	queue := make(chan int, len(runs))
	for i := range runs {
		profiles[i] = filepath.Join(temp, fmt.Sprintf("coverage-%d.out", i))
		queue <- i
	}
	close(queue)
	var workers sync.WaitGroup
	for range min(len(runs), runtime.NumCPU()) {
		workers.Go(func() {
			for i := range queue {
				lines := &lineWriter{mu: &mu, w: output}
				err := errors.Join(goTest(ctx, dir, pattern, profiles[i], runs[i].args, lines), lines.flush())
				if err != nil {
					failures[i] = fmt.Errorf("%s: %w", runs[i].label, err)
				}
			}
		})
	}
	workers.Wait()
	if err := errors.Join(failures...); err != nil {
		return Report{}, fmt.Errorf("go test with coverage: %w", err)
	}
	return readProfiles(profiles)
}

func goTest(ctx context.Context, dir, pattern, profile string, selection []string, output io.Writer) error {
	args := append([]string{
		"test",
		// The shared check already owns a 35-minute outer deadline. Avoid
		// Go's implicit 10-minute package alarm cutting off an active suite.
		"-timeout=35m",
		"-covermode=atomic",
		"-coverpkg=" + pattern,
		"-coverprofile=" + profile,
	}, selection...)
	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = dir
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}

// readProfiles parses the concatenated profiles. Every invocation writes the
// same mode header; like go test merging several test binaries' profiles,
// only the first is kept, so ParseProfile still rejects a conflicting mode.
func readProfiles(paths []string) (Report, error) {
	var files []*os.File
	defer func() {
		// The profiles are read-only; ParseProfile reports all read failures.
		for _, f := range files {
			_ = f.Close()
		}
	}()
	var readers []io.Reader
	header := ""
	for i, path := range paths {
		f, err := os.Open(path)
		if err != nil {
			return Report{}, fmt.Errorf("open coverage profile: %w", err)
		}
		files = append(files, f)
		r := bufio.NewReader(f)
		first, _ := r.ReadString('\n') // a read failure recurs for ParseProfile
		if i == 0 {
			header = first
		}
		if i == 0 || first != header {
			readers = append(readers, strings.NewReader(first))
		}
		readers = append(readers, r, strings.NewReader("\n"))
	}
	return ParseProfile(io.MultiReader(readers...))
}

// lineWriter forwards only whole lines, so concurrent invocations sharing one
// output never splice a failure marker into another invocation's line.
type lineWriter struct {
	mu      *sync.Mutex
	w       io.Writer
	pending []byte
}

func (l *lineWriter) Write(p []byte) (int, error) {
	l.pending = append(l.pending, p...)
	end := bytes.LastIndexByte(l.pending, '\n') + 1
	if end == 0 {
		return len(p), nil
	}
	if err := l.emit(l.pending[:end]); err != nil {
		return 0, err
	}
	l.pending = append(l.pending[:0], l.pending[end:]...)
	return len(p), nil
}

func (l *lineWriter) flush() error {
	if len(l.pending) == 0 {
		return nil
	}
	err := l.emit(append(l.pending, '\n'))
	l.pending = l.pending[:0]
	return err
}

func (l *lineWriter) emit(b []byte) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, err := l.w.Write(b)
	return err
}
