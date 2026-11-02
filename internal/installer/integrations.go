package installer

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/integrations"
)

func prepareIntegrations(opts *Options) error {
	if opts.Integrations.SchemaVersion == "" {
		if opts.Answers.Tracker == integrations.Forgejo {
			return fmt.Errorf("init: forgejo requires --integrations with a configured connection and repository binding")
		}
		return nil
	}
	if _, err := opts.Integrations.Render(); err != nil {
		return fmt.Errorf("init integrations: %w", err)
	}
	if opts.Integrations.Bindings.CodeHost != (integrations.Binding{}) {
		host, err := opts.Integrations.CodeHost(opts.Answers.Coordinate)
		if err != nil {
			return err
		}
		if host.Provider == integrations.LocalGit {
			return fmt.Errorf("%w: init with an explicit local Git binding is not available in this build; bind local-git only in an already initialized repository with its existing bare origin", codehost.ErrWorkflowUnavailable)
		}
		opts.Answers.Coordinate = host.Repository
	}
	if opts.Integrations.Bindings.Tasks != (integrations.Binding{}) {
		tasks, err := opts.Integrations.Tasks(opts.Answers.Coordinate, opts.Answers.Tracker, "")
		if err != nil {
			return err
		}
		opts.Answers.Tracker = tasks.Provider
		if tasks.Provider == integrations.Carina && tasks.Root != "" && opts.Target != "" {
			target, err := filepath.Abs(opts.Target)
			if err != nil {
				return fmt.Errorf("init: resolve target: %w", err)
			}
			operand := tasks.Root
			if !filepath.IsAbs(operand) {
				operand = filepath.Join(target, operand)
			}
			if actual, err := filepath.EvalSymlinks(target); err == nil {
				target = actual
			}
			if actual, err := filepath.EvalSymlinks(operand); err == nil {
				operand = actual
			}
			if filepath.Clean(operand) != filepath.Clean(target) {
				return fmt.Errorf("init: Carina tasks.root must name the target operand (use root: . or omit root); initialize this repository first, then configure an external Carina operand for runtime use")
			}
		}
	}
	if opts.Integrations.CI.Mode != "" {
		opts.Answers.CIMode = opts.Integrations.CI.Mode
	} else if opts.Answers.CIMode == "" && opts.Integrations.Bindings.CodeHost != (integrations.Binding{}) {
		opts.Answers.CIMode = "both"
	}
	opts.AnswersProvided = true
	return nil
}

func appendIntegrations(config []byte, c integrations.Config) ([]byte, error) {
	b, err := c.Render()
	if err != nil {
		return nil, err
	}
	return append(config, b...), nil
}

func stageIntegrationRemote(ctx context.Context, run *initRun) error {
	if run.opts.Integrations.Bindings.CodeHost == (integrations.Binding{}) && run.knobs.Tracker == integrations.Carina && run.knobs.Wiring.Protection == "none" {
		return nil
	}
	host, err := run.opts.Integrations.CodeHost(run.opts.Answers.Coordinate)
	if err != nil {
		return err
	}
	remoteURL := host.BaseURL + "/" + host.Repository + ".git"
	if existing, err := gitOutput(ctx, run.opts.Target, "remote", "get-url", run.repo.Remote); err == nil {
		if strings.TrimSuffix(strings.TrimSpace(existing), ".git") != strings.TrimSuffix(remoteURL, ".git") {
			return fmt.Errorf("remote: configured code_host %s differs from existing %s remote; align the binding before init", host.Key(), run.repo.Remote)
		}
		return nil
	}
	if _, err := gitOutput(ctx, run.opts.Target, "remote", "add", run.repo.Remote, remoteURL); err != nil {
		return fmt.Errorf("configure code-host remote: %w", err)
	}
	run.j.recordRemote(run.repo.Remote)
	return nil
}
