package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"regexp"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/shellinit"
	"github.com/agenticlab-ai/pyxis/scripts/startup"
)

// The launcher passes both names to git, and a repository's tracked
// configuration supplies them. Plain names only: never an option or a URL.
var (
	mainlineRemote = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)
	mainlineBranch = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*(/[A-Za-z0-9][A-Za-z0-9._-]*)*$`)
)

func shellVerb(args []string, stdout, stderr io.Writer) int {
	if len(args) == 1 && (args[0] == "--help" || args[0] == "-h") ||
		len(args) == 2 && args[0] == "install" && (args[1] == "--help" || args[1] == "-h") {
		if _, err := fmt.Fprintln(stdout, "usage: pyxis shell install\nInstall interactive worktree startup for zsh in ${ZDOTDIR:-$HOME}/.zshrc."); err != nil {
			return 1
		}
		return 0
	}
	if len(args) == 2 && args[0] == "mainline" && args[1] != "" && args[1][0] != '-' {
		return shellMainline(args[1], stdout, stderr)
	}
	if len(args) != 1 || args[0] != "install" {
		return refuse(stderr, "shell", "expected install or mainline <repository>")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "pyxis: shell: %v\n", err) // Best-effort diagnostic before returning failure.
		return 1
	}
	rep, err := shellinit.Install(context.Background(), shellinit.Options{
		Home: home, ZDotDir: os.Getenv("ZDOTDIR"), DataHome: os.Getenv("XDG_DATA_HOME"), Assets: startup.Files,
	})
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "pyxis: shell: %v\n", err) // Best-effort diagnostic before returning failure.
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "pyxis: interactive worktrees enabled for zsh in %s\nOpen a new zsh terminal to activate them.\n", rep.RCPath); err != nil {
		return 1
	}
	return 0
}

// shellMainline prints the repository's configured remote and default branch,
// one per line. The zsh launcher starts worktrees from that pair and carries
// no copy of either (config-driven-portability-v1 DINV-4).
func shellMainline(root string, stdout, stderr io.Writer) int {
	cfg, defects := config.Load(root)
	if len(defects) > 0 {
		_, _ = fmt.Fprintf(stderr, "pyxis: shell: configuration: %v\n", defects) // Best-effort diagnostic before returning failure.
		return 1
	}
	if !mainlineRemote.MatchString(cfg.Remote) || !mainlineBranch.MatchString(cfg.Branch) {
		_, _ = fmt.Fprintf(stderr, "pyxis: shell: remote %q and branch %q must be plain git names\n", cfg.Remote, cfg.Branch) // Best-effort diagnostic before returning failure.
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "%s\n%s\n", cfg.Remote, cfg.Branch); err != nil {
		return 1
	}
	return 0
}
