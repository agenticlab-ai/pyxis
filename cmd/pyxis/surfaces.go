package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/cliguard"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/surfaces"
)

var (
	surfacesRepoRoot    = repoRoot
	surfacesInteractive = interactive
	surfacesAsk         = askSurfaces
	surfacesApply       = surfaces.Apply
)

// surfacesVerb — `pyxis surfaces [--all|--detected|--set a,b] [--preview]`.
//
// init resolves the agent-CLI surfaces once, by probing PATH, and a machine
// with no agent CLI installed records an empty set. That set is permanent:
// converge materializes what the config names, and an empty set names nothing,
// so installing Codex or Cursor afterwards wires nothing and says nothing.
// This verb is how an initialized repository re-answers the question.
//
// Bare, it reports and performs nothing — the same read-only posture doctor
// holds, so looking is never a mutation. A selection flag writes the knob and
// materializes. With a terminal and no flag it offers the choice directly.
//
// The selection is not limited to what is installed: an operator provisioning
// a repository for a team wires surfaces their own machine has never seen.
func surfacesVerb(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("surfaces", flag.ContinueOnError)
	all := fs.Bool("all", false, "wire every surface pyxis supports, installed or not")
	detected := fs.Bool("detected", false, "wire the surfaces found on this machine's PATH")
	set := fs.String("set", "", "wire exactly these surfaces (comma-separated)")
	preview := fs.Bool("preview", false, "report what would change and perform none of it")
	pos, err := cliguard.Positionals(fs, args)
	if err != nil {
		return misparse(stderr, "surfaces", err)
	}
	if len(pos) != 0 {
		return refuse(stderr, "surfaces", fmt.Sprintf("takes no arguments (got %q) — choose with --all, --detected, or --set", pos[0]))
	}
	chosen := 0
	for _, on := range []bool{*all, *detected, *set != ""} {
		if on {
			chosen++
		}
	}
	if chosen > 1 {
		return refuse(stderr, "surfaces", "--all, --detected and --set name three different selections — pass one")
	}

	root, err := surfacesRepoRoot()
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: %v\n", err)
		return 2
	}
	repo, cdef := config.Load(root)
	if len(cdef) > 0 {
		for _, d := range cdef {
			fmt.Fprintln(stderr, d.String())
		}
		return 1
	}
	st := surfaces.Read(repo)
	printSurfaces(stdout, st)

	// The selection, from a flag or — with a terminal and none — from the
	// operator. No flag and no terminal is a report, not a hang.
	var want []string
	switch {
	case *all:
		want = st.Supported
	case *detected:
		if want = st.Installed; len(want) == 0 {
			fmt.Fprintln(stderr, "pyxis: surfaces: --detected found no agent CLI on PATH — install one, or name surfaces with --set/--all")
			return 1
		}
	case *set != "":
		if want, err = surfaces.Resolve(strings.Split(*set, ",")); err != nil {
			fmt.Fprintf(stderr, "pyxis: %v\n", err)
			return 2
		}
	default:
		if !surfacesInteractive() {
			printSurfaceHints(stdout, st)
			return 0
		}
		answered := false
		if want, answered, err = surfacesAsk(stdout, os.Stdin, st); err != nil {
			fmt.Fprintf(stderr, "pyxis: surfaces: %v\n", err)
			return 1
		}
		if !answered {
			// stdin closed before an answer arrived — nobody was there to ask.
			printSurfaceHints(stdout, st)
			return 0
		}
		if want == nil {
			fmt.Fprintln(stdout, "\nnothing selected; no change was made")
			return 0
		}
	}

	if *preview {
		fmt.Fprintf(stdout, "\nwould wire: %s\n", strings.Join(want, ", "))
		fmt.Fprintln(stdout, "nothing was performed")
		return 0
	}
	actions, err := surfacesApply(root, repo, want)
	if err != nil {
		fmt.Fprintf(stderr, "pyxis: %v\n", err)
		return 1
	}
	fmt.Fprintf(stdout, "\nwired: %s\n", strings.Join(want, ", "))
	for _, a := range actions {
		fmt.Fprintf(stdout, "  %s\n", a.Path)
	}
	fmt.Fprintf(stdout, "%d path(s) written; `pyxis doctor` verifies them\n", len(actions))
	return 0
}

// printSurfaces renders the two states that decide everything: what this
// machine has, and what this repository wires.
func printSurfaces(w io.Writer, st surfaces.Status) {
	fmt.Fprintln(w, "pyxis surfaces — agent-CLI skill surfaces for this repository")
	fmt.Fprintln(w)
	fmt.Fprintf(w, "  %-10s %-11s %s\n", "surface", "installed", "wired")
	for _, r := range st.Rows() {
		fmt.Fprintf(w, "  %-10s %-11s %s\n", r.Name, yesNo(r.Installed), yesNo(r.Wired))
	}
	if extra := st.Unsupported(); len(extra) > 0 {
		fmt.Fprintf(w, "\n  configured but not a surface pyxis wires: %s\n", strings.Join(extra, ", "))
		fmt.Fprintln(w, "  those names wire nothing — check the spelling against the list above")
	}
}

// printSurfaceHints is the non-interactive tail: what to run next, phrased for
// what this machine actually has.
func printSurfaceHints(w io.Writer, st surfaces.Status) {
	fmt.Fprintln(w)
	if len(st.Installed) > 0 {
		fmt.Fprintf(w, "  pyxis surfaces --detected      wire what is installed here (%s)\n", strings.Join(st.Installed, ", "))
	}
	fmt.Fprintf(w, "  pyxis surfaces --all           wire all %d supported surfaces\n", len(st.Supported))
	fmt.Fprintf(w, "  pyxis surfaces --set a,b       wire exactly these\n")
	fmt.Fprintln(w, "\nnothing was performed")
}

// askSurfaces offers the choice on a terminal. A bare Enter selects nothing
// and changes nothing: this verb rewrites a knob, so silence is not consent.
//
// answered distinguishes that Enter from a stdin that was closed before anyone
// could answer — both arrive as an empty line, and only the first is a person
// declining. The installer's own prompter draws the same line for the same
// reason.
func askSurfaces(w io.Writer, in io.Reader, st surfaces.Status) (sel []string, answered bool, err error) {
	fmt.Fprintln(w)
	if len(st.Installed) > 0 {
		fmt.Fprintf(w, "  detected: %s\n", strings.Join(st.Installed, ", "))
	}
	fmt.Fprintf(w, "Which surfaces should this repository wire?\n")
	fmt.Fprintf(w, "  [detected | all | a,b | Enter to leave unchanged]: ")

	line, rerr := bufio.NewReader(in).ReadString('\n')
	if rerr != nil && rerr != io.EOF {
		return nil, false, fmt.Errorf("reading your answer: %w", rerr)
	}
	if rerr == io.EOF && strings.TrimSpace(line) == "" {
		return nil, false, nil // closed, not declined
	}
	switch answer := strings.ToLower(strings.TrimSpace(line)); answer {
	case "":
		return nil, true, nil
	case "all":
		return st.Supported, true, nil
	case "detected":
		if len(st.Installed) == 0 {
			return nil, true, fmt.Errorf("no agent CLI is installed on this machine — name surfaces instead, or choose all")
		}
		return st.Installed, true, nil
	default:
		sel, err = surfaces.Resolve(strings.Split(answer, ","))
		return sel, true, err
	}
}

// interactive reports whether there is a person to ask: BOTH stdin and stdout
// must be terminals. Stdin alone is not enough — /dev/null is a character
// device too, so a redirected run would otherwise read as a terminal and print
// a question nobody sees. Requiring stdout as well means a piped or captured
// run reports instead of prompting.
func interactive() bool {
	return charDevice(os.Stdin) && charDevice(os.Stdout)
}

func charDevice(f *os.File) bool {
	info, err := f.Stat()
	return err == nil && info.Mode()&os.ModeCharDevice != 0
}

func yesNo(b bool) string {
	if b {
		return "yes"
	}
	return "no"
}
