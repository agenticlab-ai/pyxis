package installer

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

const publicationKey = "pyxis.init-publication"

// PublicationResult describes the configured destination and the verified
// initial publication. A successful push proves publication, not hosted CI.
type PublicationResult struct {
	Remote      string `json:"remote"`
	Branch      string `json:"branch"`
	Destination string `json:"destination"`
	Head        string `json:"head,omitempty"`
	Published   bool   `json:"published"`
	WouldPush   bool   `json:"would_push"`
}

// pendingPublication is installer-owned local Git state, outside the tracked
// config. Only the exact installation head can resume its initial publication.
type pendingPublication struct {
	Schema string `json:"schema_version"`
	RunID  string `json:"run_id"`
	Tree   string `json:"tree"`
	Head   string `json:"head"`
	Remote string `json:"remote"`
	Branch string `json:"branch"`
	URL    string `json:"url"`
}

var publicationGit = gitOutput

type publicationUncertainError struct{ cause error }

func (e *publicationUncertainError) Error() string { return e.cause.Error() }
func (e *publicationUncertainError) Unwrap() error { return e.cause }

// resolvePublicationDestination runs before writes. Local installations also
// need a destination; no external location is selected by default or on EOF.
func resolvePublicationDestination(ctx context.Context, opts *Options, knobs config.Knobs, pr *prompter) error {
	if strings.ContainsAny(opts.Answers.RemoteURL, "\r\n\x00") || strings.HasPrefix(opts.Answers.RemoteURL, "-") {
		return errors.New("init: invalid Git publication destination")
	}
	remote := config.Defaults().Remote
	if current, err := gitOutput(ctx, opts.Target, "remote", "get-url", remote); err == nil {
		if supplied := opts.Answers.RemoteURL; supplied != "" && strings.TrimSpace(current) != supplied {
			return fmt.Errorf("init: --remote-url differs from configured %s; align the destination before installing", remote)
		}
		_, err := publicationDestination(ctx, opts.Target, remote)
		return err
	}
	if opts.Integrations.Bindings.CodeHost != (integrations.Binding{}) || preflight.GHOpsImplied(knobs) {
		if opts.Answers.RemoteURL != "" {
			destination := wire.RemoteURL(opts.Answers.Coordinate)
			if opts.Integrations.Bindings.CodeHost != (integrations.Binding{}) {
				host, err := opts.Integrations.CodeHost(opts.Answers.Coordinate)
				if err != nil {
					return err
				}
				destination = host.BaseURL + "/" + host.Repository + ".git"
			}
			if opts.Answers.RemoteURL != destination {
				return errors.New("init: --remote-url differs from the configured code-host destination")
			}
		}
		return nil // the collected code-host coordinate owns remote materialization
	}
	if opts.Answers.RemoteURL == "" && pr != nil {
		url, err := pr.ask("Local Git destination (existing bare repository path or URL)", "")
		if err != nil {
			return err
		}
		opts.Answers.RemoteURL = strings.TrimSpace(url)
		return resolvePublicationDestination(ctx, opts, knobs, nil)
	}
	if opts.Answers.RemoteURL == "" {
		return fmt.Errorf("init: no Git publication destination — configure %s or pass --remote-url <bare-repository-path-or-URL>; nothing was written", remote)
	}
	return nil
}

func readPendingPublication(ctx context.Context, root string) (*pendingPublication, error) {
	top, err := gitOutput(ctx, root, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, nil
	} // admission diagnoses absent or invalid Git metadata
	actual, err := filepath.EvalSymlinks(root)
	if err != nil {
		return nil, err
	}
	if filepath.Clean(strings.TrimSpace(top)) != filepath.Clean(actual) {
		return nil, nil
	}
	body, err := gitOutput(ctx, root, "config", "--local", "--get", publicationKey)
	if err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) && exit.ExitCode() == 1 {
			return nil, nil
		}
		return nil, fmt.Errorf("init: read pending publication: %w", err)
	}
	var p pendingPublication
	dec := json.NewDecoder(strings.NewReader(body))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("init: invalid pending publication: %w", err)
	}
	var trailing json.RawMessage
	if err := dec.Decode(&trailing); err != io.EOF {
		return nil, errors.New("init: trailing data in pending publication")
	}
	if p.Schema != "pyxis-init-publication/v1" || !publicationSHA(p.Tree) || !publicationSHA(p.Head) || p.RunID == "" || p.URL == "" || p.Remote == "" || p.Branch == "" {
		return nil, errors.New("init: invalid pending publication fields")
	}
	return &p, nil
}

func publicationSHA(s string) bool {
	if len(s) != 40 && len(s) != 64 {
		return false
	}
	_, err := hex.DecodeString(s)
	return err == nil
}

// publicationDestination permits exactly one fetch/push destination. A remote
// with distinct push URLs cannot establish the configured mainline baseline.
func publicationDestination(ctx context.Context, root, remote string) (string, error) {
	fetch, err := gitOutput(ctx, root, "remote", "get-url", "--all", remote)
	if err != nil {
		return "", fmt.Errorf("first-push: resolve configured remote %s: %w", remote, err)
	}
	push, err := gitOutput(ctx, root, "remote", "get-url", "--push", "--all", remote)
	if err != nil {
		return "", err
	}
	fetchURLs, pushURLs := strings.Split(strings.TrimSpace(fetch), "\n"), strings.Split(strings.TrimSpace(push), "\n")
	if len(fetchURLs) != 1 || len(pushURLs) != 1 || fetchURLs[0] == "" || fetchURLs[0] != pushURLs[0] {
		return "", errors.New("first-push: installation requires one matching fetch/push URL for the configured remote")
	}
	return pushURLs[0], nil
}

func stageFirstPush(ctx context.Context, run *initRun) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(run.rep.Commits) != 2 || !run.rep.T8.GreenProof.Green {
		return errors.New("first-push: installation commits and green proof are required")
	}
	url, err := publicationDestination(ctx, run.opts.Target, run.repo.Remote)
	if err != nil {
		return err
	}
	p := pendingPublication{Schema: "pyxis-init-publication/v1", RunID: run.rep.RunID,
		Tree: run.rep.Commits[0].SHA, Head: run.rep.Commits[1].SHA, Remote: run.repo.Remote, Branch: run.branch, URL: url}
	body, err := json.Marshal(p)
	if err != nil {
		return err
	}
	if _, err := gitOutput(ctx, run.opts.Target, "config", "--local", publicationKey, string(body)); err != nil {
		return err
	}
	run.j.recordConfig(publicationKey)
	return publishInstallation(ctx, run, p)
}

func validatePublication(ctx context.Context, run *initRun, p pendingPublication) error {
	head, err := gitOutput(ctx, run.opts.Target, "rev-parse", "HEAD")
	if err != nil {
		return err
	}
	branch, err := gitOutput(ctx, run.opts.Target, "symbolic-ref", "--short", "HEAD")
	if err != nil {
		return err
	}
	parent, err := gitOutput(ctx, run.opts.Target, "rev-list", "--parents", "-n", "1", p.Head)
	if err != nil {
		return err
	}
	parents := strings.Fields(parent)
	if strings.TrimSpace(head) != p.Head || strings.TrimSpace(branch) != p.Branch || run.repo.Epoch.Commit != p.Tree || run.repo.Remote != p.Remote || run.repo.Branch != p.Branch || len(parents) != 2 || parents[1] != p.Tree {
		return errors.New("first-push: installation head, branch, or epoch changed; refusing to publish")
	}
	if _, err := gitOutput(ctx, run.opts.Target, "diff", "--quiet", p.Head, "--"); err != nil {
		return fmt.Errorf("first-push: installed tracked files changed: %w", err)
	}
	url, err := publicationDestination(ctx, run.opts.Target, p.Remote)
	if err != nil {
		return err
	}
	if url != p.URL {
		return errors.New("first-push: configured destination changed since installation; refusing to redirect publication")
	}
	dirs, err := gate.LedgerDirs(run.opts.Target, run.repo)
	if err != nil {
		return err
	}
	if defects := gate.GateAt(run.opts.Target, p.Tree, p.Head, dirs, run.repo); len(defects) > 0 {
		return fmt.Errorf("first-push: committed installation is not green: %s", defectsLine(defects))
	}
	return nil
}

func publishInstallation(ctx context.Context, run *initRun, p pendingPublication) error {
	if err := validatePublication(ctx, run, p); err != nil {
		return err
	}
	transport := run.opts.PublicationGit
	if transport == nil {
		transport = publicationGit
	}
	ref := "refs/heads/" + p.Branch
	read := func(ctx context.Context) (string, error) {
		out, err := transport(ctx, run.opts.Target, "ls-remote", "--refs", "--", p.URL, ref)
		if err != nil {
			return "", err
		}
		fields := strings.Fields(out)
		if len(fields) == 0 {
			return "", nil
		}
		if len(fields) != 2 || fields[1] != ref || !publicationSHA(fields[0]) {
			return "", errors.New("first-push: invalid destination ref advertisement")
		}
		return fields[0], nil
	}
	remoteHead, err := read(ctx)
	if err != nil {
		return fmt.Errorf("first-push: inspect destination: %w", err)
	}
	if remoteHead != "" && remoteHead != p.Head {
		return fmt.Errorf("first-push: destination %s already has a different %s; no remote ref was changed", p.Remote, p.Branch)
	}
	if remoteHead == "" {
		terminalPrintf(run.opts.Stdout, "publishing %s to %s (%s)…\n", p.Branch, p.Remote, p.URL)
		// The installer already gated this exact head. Scope hook suppression to
		// its own genesis publication, and atomically require the branch absent.
		// Ordinary pushes retain the installed hooks and require a remote base.
		_, pushErr := transport(ctx, run.opts.Target, "-c", "core.hooksPath=", "push", "--porcelain", "--no-follow-tags", "--recurse-submodules=no", "--force-with-lease="+ref+":", "--", p.URL, p.Head+":"+ref)
		checkCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 15*time.Second)
		remoteHead, err = read(checkCtx)
		cancel()
		if err != nil {
			return &publicationUncertainError{fmt.Errorf("first-push: cannot confirm publication after push (%v): %w", pushErr, err)}
		}
		if remoteHead != p.Head {
			if remoteHead != "" {
				return &publicationUncertainError{fmt.Errorf("first-push: destination changed during publication (%v)", pushErr)}
			}
			if pushErr != nil {
				// An absent ref immediately after a failed transport cannot prove
				// that the server will not finish the already-started receive.
				return &publicationUncertainError{fmt.Errorf("first-push: installation head is not confirmed after push: %w", pushErr)}
			}
			return &publicationUncertainError{errors.New("first-push: destination did not retain the installation head")}
		}
	}
	run.published = true
	run.rep.Publication = PublicationResult{Remote: p.Remote, Branch: p.Branch, Destination: p.URL, Head: p.Head, Published: true}
	// Publishing an exact SHA avoids a concurrent branch advancement. Establish
	// the local tracking ref and upstream explicitly after verifying the remote.
	for _, args := range [][]string{
		{"update-ref", "refs/remotes/" + p.Remote + "/" + p.Branch, p.Head},
		{"config", "--local", "branch." + p.Branch + ".remote", p.Remote},
		{"config", "--local", "branch." + p.Branch + ".merge", ref},
	} {
		if _, err := gitOutput(ctx, run.opts.Target, args...); err != nil {
			return err
		}
	}
	run.logger.LogAttrs(ctx, slog.LevelInfo, "install_first_push", slog.String("run_id", p.RunID), slog.String("remote", p.Remote), slog.String("branch", p.Branch), slog.String("head", p.Head))
	return nil
}

func stageCompletePublication(ctx context.Context, run *initRun) error {
	_, err := gitOutput(ctx, run.opts.Target, "config", "--local", "--unset", publicationKey)
	return err
}

func resumePublication(ctx context.Context, opts Options, rep Report, logger *slog.Logger, p pendingPublication) (Report, error) {
	started := now()
	repo, defects := config.Load(opts.Target)
	if len(defects) > 0 {
		return rep, fmt.Errorf("init: pending publication config: %s", defectsLine(defects))
	}
	profile, err := integrations.Load(opts.Target)
	if err != nil {
		return rep, err
	}
	opts.Integrations = profile
	opts.Pin = Pin{Version: repo.System.Version, ManifestSHA256: repo.System.ManifestSHA256}
	if opts.Answers.RemoteURL != "" && opts.Answers.RemoteURL != p.URL {
		return rep, errors.New("init: --remote-url differs from the pending publication destination")
	}
	rep.RunID, rep.Branch, rep.Knobs, rep.Resumed = p.RunID, p.Branch, repo.Knobs, true
	rep.Commits = []Commit{{SHA: p.Tree, Subject: commitTreeSubject}, {SHA: p.Head, Subject: commitEpochSubject}}
	rep.EpochWritten = true
	run := &initRun{opts: opts, logger: logger, knobs: repo.Knobs, branch: p.Branch, repo: repo, rep: &rep, j: newJournal(opts.Target)}
	if err := validatePublication(ctx, run, p); err != nil {
		return rep, fmt.Errorf("init: resume: %w", err)
	}
	if opts.Preview {
		rep.Publication = PublicationResult{Remote: p.Remote, Branch: p.Branch, Destination: p.URL, Head: p.Head, WouldPush: true}
		rep.Stages = []string{"green-proof", "first-push", "finish-wiring", "welcome", "complete-publication"}
		return rep, nil
	}
	if err := prepareWelcome(run); err != nil {
		return rep, err
	}
	for _, st := range []struct {
		name string
		run  stageFunc
	}{
		{"wiring", t8Slots.Wiring}, {"green-proof", t8Slots.GreenProof},
		{"first-push", func(ctx context.Context, run *initRun) error { return publishInstallation(ctx, run, p) }},
		{"finish-wiring", stageFinishWiring}, {"welcome", t8Slots.Welcome}, {"complete-publication", stageCompletePublication},
	} {
		if err := st.run(ctx, run); err != nil {
			return run.failAndUnwind(ctx, st.name, err)
		}
		rep.Stages = append(rep.Stages, st.name)
	}
	emitInstallCompleted(ctx, logger, rep, opts, repo.Knobs, now().Sub(started))
	return rep, nil
}
