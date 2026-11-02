// Package host is the wake channel's per-host adapter set — the production
// implementations of the tick's three ports (wake-channel-v1 §3.2; plan T8's
// wiring; #515): the filesystem Ledgers read, the git/gh goal-object Prober,
// and the exec FrontDoor that invokes the agentic CLI once per due touch. The
// tick's dispatch and convergence are proven hermetically in internal/wake;
// this package owns exactly the medium touches, each behind an injectable
// seam. It carries no credential of its own: gh resolves auth from the host
// environment the operator already maintains, and the front-door command is
// the host's choice, passed by the invoker — never config-body or ledger
// content (DINV-1: the invoked CLI receives only the normalized wire signal).
package host

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/adapters/common/forgejo"
	"github.com/agenticlab-ai/pyxis/internal/adapters/common/github/githubauth"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/wake"
)

// Ledgers reads <problems>/*/delivery.md under the repo root — the tick's one
// bulk read (AP-19: it lists the problem directories and reads their delivery
// ledgers, walking nothing else). Problems is the configured problems root
// (config.Problems; config-driven-portability-v1 §3.2, FR-1); an empty value
// falls back to the built-in default (docs/problems) so a default-config sweep
// is byte-identical.
type Ledgers struct {
	Root     string
	Problems string
}

// problemsRel returns the repo-relative problems root the adapter reads under,
// defaulting to the built-in when unset — a single-source default (never a
// second target-facing literal; DINV-1).
func (l Ledgers) problemsRel() string {
	if l.Problems == "" {
		return config.Defaults().Problems
	}
	return filepath.ToSlash(filepath.Clean(l.Problems))
}

// List returns every delivery ledger under the problems root, in stable slug
// order. A repo with no problems directory is an empty (idle) sweep, not an
// error.
func (l Ledgers) List(ctx context.Context) ([]wake.Doc, error) {
	problems := l.problemsRel()
	dir := filepath.Join(l.Root, filepath.FromSlash(problems))
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("listing the problems root %s: %w", dir, err)
	}
	var docs []wake.Doc
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		rel := problems + "/" + e.Name() + "/delivery.md"
		b, err := os.ReadFile(filepath.Join(l.Root, filepath.FromSlash(rel)))
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // a problem dir without a delivery ledger is not the tick's concern
			}
			return nil, fmt.Errorf("reading %s: %w", rel, err)
		}
		docs = append(docs, wake.Doc{Path: rel, Bytes: b})
	}
	sort.Slice(docs, func(a, b int) bool { return docs[a].Path < docs[b].Path })
	return docs, nil
}

// Runner executes one external read and returns its stdout — the gh/git seam.
type Runner func(ctx context.Context, name string, args ...string) ([]byte, error)

func shellRun(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.Output()
	if err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) && len(ee.Stderr) > 0 {
			return nil, fmt.Errorf("%s %s: %s", name, strings.Join(args, " "), strings.TrimSpace(string(ee.Stderr)))
		}
		return nil, fmt.Errorf("%s %s: %w", name, strings.Join(args, " "), err)
	}
	return out, nil
}

// Prober performs the two exact goal-object reads the await grammar's closed
// schemes call for (AP-15/AP-18/AP-26): mainline ancestry for a merged
// proposal, record existence for a tracker item. Reads only; no credential
// beyond the host's own gh auth; fail-closed — a read it cannot perform is an
// error the tick surfaces, never a silent completion.
type Prober struct {
	Root   string // local clone (ancestry reads)
	GHRepo string // owner/name the bare `pr:<n>` / `item:<n>` refs resolve against
	// Remote and Branch are the configured git remote and default branch the
	// mainline ancestry read fetches and resolves against (config.Remote /
	// config.Branch; config-driven-portability-v1 §3.2, FR-2). Empty falls back to
	// the config schema's single source (config.Defaults — "origin"/"main"), so a
	// default-config tick is byte-identical (DINV-1/DINV-2). The composition root
	// (cmd/wake-tick) loads the config once and passes both, honoring a repo whose
	// mainline is not origin/main.
	Remote string
	Branch string
	Run    Runner // nil → the production shell runner
}

func (p Prober) run(ctx context.Context, name string, args ...string) ([]byte, error) {
	r := p.Run
	if r == nil {
		r = shellRun
	}
	return r(ctx, name, args...)
}

// remote and branch resolve the Prober's configured remote/branch, defaulting to
// the config schema's single source (config.Defaults — the ONE place those
// literals live, DINV-1); mainlineRef composes the remote-tracking ref the same
// way Config.MainlineRef does (AP-11), never baking a second "origin/main".
func (p Prober) remote() string {
	if p.Remote == "" {
		return config.Defaults().Remote
	}
	return p.Remote
}

func (p Prober) branch() string {
	if p.Branch == "" {
		return config.Defaults().Branch
	}
	return p.Branch
}

func (p Prober) mainlineRef() string { return p.remote() + "/" + p.branch() }

// prRefRe admits a bare PR number; qualifiedRe admits owner/repo#n for a
// cross-repo item or proposal ref. The set is closed: an unrecognized form is
// an error, never a guess (the probe mirrors probeAwait's fail-loud posture).
var (
	prRefRe     = regexp.MustCompile(`^#?(\d+)$`)
	qualifiedRe = regexp.MustCompile(`^([\w.-]+/[\w.-]+)#(\d+)$`)
)

// AncestorOfMainline reports whether the `pr:<ref>` goal object's merged
// content is an ancestor of the mainline: the PR must report merged AND its
// merge commit must be an ancestor of the configured mainline ref (the
// remote-tracking `<remote>/<branch>`, origin/main on a default config) — a
// merged badge whose content is absent reads false, never a lying true
// (AP-26/I2).
func (p Prober) AncestorOfMainline(ctx context.Context, ref string) (bool, error) {
	repo, num, binding, err := p.resolveBinding(ref, false)
	if err != nil {
		return false, err
	}
	var pr struct {
		Merged         bool   `json:"merged"`
		MergeCommitSHA string `json:"merge_commit_sha"`
	}
	if err := p.read(ctx, binding, repo, "pulls/"+num, &pr); err != nil {
		return false, fmt.Errorf("decoding pr:%s: %w", ref, err)
	}
	if !pr.Merged || pr.MergeCommitSHA == "" {
		return false, nil
	}
	if _, err := p.run(ctx, "git", "-C", p.Root, "fetch", "--no-tags", "-q", p.remote(), p.branch()); err != nil {
		return false, fmt.Errorf("fetching mainline for the ancestry read: %w", err)
	}
	if _, err := p.run(ctx, "git", "-C", p.Root, "merge-base", "--is-ancestor", pr.MergeCommitSHA, p.mainlineRef()); err != nil {
		return false, nil // exit 1: not an ancestor — the completion has not happened
	}
	return true, nil
}

// RecordExists reports whether the `item:<ref>` goal object's tracker record
// exists — existence of the record itself, never a platform event (AP-18).
// Recognized forms: `<n>` / `#<n>` (an issue in the bound repo) and
// `owner/repo#<n>`. The form set grows additively per medium (AP-27).
func (p Prober) RecordExists(ctx context.Context, ref string) (bool, error) {
	repo, num, binding, err := p.resolveBinding(ref, true)
	if err != nil {
		return false, err
	}
	var it struct {
		Number int `json:"number"`
	}
	err = p.read(ctx, binding, repo, "issues/"+num, &it)
	if err != nil {
		if strings.Contains(err.Error(), "404") || strings.Contains(err.Error(), "Not Found") {
			return false, nil
		}
		return false, fmt.Errorf("reading item:%s: %w", ref, err)
	}
	want, parseErr := strconv.Atoi(num)
	if parseErr != nil || want <= 0 || it.Number != want {
		return false, fmt.Errorf("item:%s returned a different or invalid issue identity", ref)
	}
	return true, nil
}

func (p Prober) read(ctx context.Context, binding integrations.ResolvedRepository, repo, suffix string, out any) error {
	if binding.Provider == integrations.Forgejo {
		client, err := forgejo.New(binding.BaseURL, binding.Token())
		if err != nil {
			return err
		}
		return client.Request(ctx, "GET", forgejo.RepoPath(repo, suffix), nil, out)
	}
	args := []string{"api", "-H", "Cache-Control: no-cache", "repos/" + repo + "/" + suffix}
	var data []byte
	var err error
	if binding.Provider == integrations.GitHub && p.Run == nil {
		data, err = githubauth.RunForHost(ctx, binding.TokenEnv, "github.com", nil, args...)
	} else {
		data, err = p.run(ctx, "gh", args...)
	}
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out)
}

// resolve maps a closed-form ref to (repo, number). Unrecognized forms error.
func (p Prober) resolve(ref string) (repo, num string, err error) {
	ref = strings.TrimSpace(ref)
	if m := prRefRe.FindStringSubmatch(ref); m != nil {
		if p.GHRepo == "" {
			return "", "", fmt.Errorf("bare ref %q needs a bound repository (--gh-repo)", ref)
		}
		return p.GHRepo, m[1], nil
	}
	if m := qualifiedRe.FindStringSubmatch(ref); m != nil {
		return m[1], m[2], nil
	}
	return "", "", fmt.Errorf("goal-object ref %q is not a recognized form (`<n>`, `#<n>`, or `owner/repo#<n>`)", ref)
}

// Door invokes the agentic CLI once per due touch: the configured argv plus
// ONE appended argument — the signal's normalized wire form
// (`wake:<slug>:<reason>`). No shell interpolation, no template expansion, no
// byte beyond the wire token (DINV-1); the invoked CLI's own front door
// interprets it. DryRun records the would-be invocation without executing.
type Door struct {
	Argv   []string
	DryRun bool
	// Invoked records every wire token the door delivered (or would have, under
	// DryRun) — the report's evidence surface.
	Invoked []string
	Run     Runner // nil → the production shell runner
}

// Touch delivers one signal through the configured CLI.
func (d *Door) Touch(ctx context.Context, s wake.Signal) error {
	wire := s.Wire()
	d.Invoked = append(d.Invoked, wire)
	if d.DryRun {
		return nil
	}
	if len(d.Argv) == 0 {
		return errors.New("no front-door command configured (pass it after `--`)")
	}
	r := d.Run
	if r == nil {
		r = shellRun
	}
	args := append(append([]string{}, d.Argv[1:]...), wire)
	if _, err := r(ctx, d.Argv[0], args...); err != nil {
		return fmt.Errorf("front-door invocation for %s: %w", wire, err)
	}
	return nil
}
