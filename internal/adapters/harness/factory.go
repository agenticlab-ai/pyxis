// Package harness constructs configured runtimes for command composition roots.
// Engine packages consume the runtime port and do not import this package.
package harness

import (
	"fmt"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/claude"
	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/cursor"
	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/opencode"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/runtime"
	"github.com/agenticlab-ai/pyxis/internal/harnessconfig"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// New constructs a build runtime with the effective catalog. Claude receives
// WorkerTurns for its native --max-turns seam, as in implement-now.
func New(backend entity.Harness, cfg config.Config, hc harnessconfig.Config) (port.AgentRuntime, error) {
	return newForPurpose(backend, cfg, hc, false)
}

// NewReview constructs an independent review runtime. WorkerTurns is a build
// session control and must not leak into the review catalog's native launch.
func NewReview(backend entity.Harness, cfg config.Config, hc harnessconfig.Config) (port.AgentRuntime, error) {
	return newForPurpose(backend, cfg, hc, true)
}

func profilesFor(hc harnessconfig.Config, backend entity.Harness) (map[nativerouting.Role][]nativerouting.ProfileSpec, error) {
	if profiles := hc.Profiles(backend); len(profiles) != 0 {
		return profiles, nil
	}
	switch backend {
	case entity.Claude, entity.Codex, entity.Cursor, entity.OpenCode:
		return harnessconfig.LoadDefault(backend)
	default:
		return nil, nil
	}
}

func newForPurpose(backend entity.Harness, cfg config.Config, hc harnessconfig.Config, review bool) (port.AgentRuntime, error) {
	profiles, err := profilesFor(hc, backend)
	if err != nil {
		return nil, err
	}
	switch backend {
	case entity.Claude:
		var opts []claude.Option
		opts = append(opts, claude.WithProfiles(profiles))
		if !review {
			opts = append(opts, claude.WithMaxTurns(cfg.WorkerTurns()))
		}
		rt, err := claude.New(opts...)
		if err != nil {
			return nil, fmt.Errorf("build claude backend: %w", err)
		}
		return rt, nil
	case entity.Codex:
		rt, err := codex.New(codex.WithProfiles(profiles))
		if err != nil {
			return nil, fmt.Errorf("build codex backend: %w", err)
		}
		return rt, nil
	case entity.Cursor:
		rt, err := cursor.New(cursor.WithProfiles(profiles))
		if err != nil {
			return nil, fmt.Errorf("build cursor backend: %w", err)
		}
		return rt, nil
	case entity.OpenCode:
		rt, err := opencode.New(opencode.WithProfiles(profiles))
		if err != nil {
			return nil, fmt.Errorf("build opencode backend: %w", err)
		}
		return rt, nil
	default:
		rt, err := runtime.Resolve(backend)
		if err != nil {
			return nil, err
		}
		return rt, nil
	}
}
