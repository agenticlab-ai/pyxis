package installer_test

// The repository question (#1043) driven black-box through installer.Init:
// one choice replaces the wiring.repo enum and the coordinate exchange, and
// "keep it local" settles the three GitHub-only knobs instead of asking
// them.

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/installer"
	"github.com/agenticlab-ai/pyxis/internal/preflight"
	"github.com/agenticlab-ai/pyxis/internal/wire"
)

// listingGH answers the repository listing with a scripted candidate set;
// everything else falls through to the shared happy-path fake.
type listingGH struct {
	nodes     string // raw JSON node rows
	failList  bool   // the listing cannot run at all
	truncated bool   // the window reported more pages behind it
	probe     string // the repository-state answer for the create branch
	calls     [][]string
}

func (g *listingGH) RunGH(ctx context.Context, args ...string) (string, string, int, error) {
	g.calls = append(g.calls, append([]string(nil), args...))
	switch {
	case isRepoListQuery(args):
		if g.failList {
			return "", "gh: could not reach github.com", 1, nil
		}
		next := "false"
		if g.truncated {
			next = "true"
		}
		return `{"data":{"viewer":{"repositories":{"pageInfo":{"hasNextPage":` + next + `},"nodes":[` + g.nodes + `]}}}}`, "", 0, nil
	case isRepoStateQuery(args):
		if g.probe != "" {
			return g.probe, "", 0, nil
		}
		return `{"data":{"repository":null}}`, "", 0, nil
	}
	return hermeticGH{}.RunGH(ctx, args...)
}

func emptyRepoNode(slug, perm string) string {
	return `{"nameWithOwner":"` + slug + `","isEmpty":true,"isArchived":false,"viewerPermission":"` + perm + `"}`
}

// repoQuestionOptions is a pure-interactive install with NO promptless
// coordinate, so the repository question is the surface under test.
func repoQuestionOptions(r release, target, stdin string, stdout *bytes.Buffer, gh wire.Runner) installer.Options {
	return installer.Options{
		Target:         target,
		Pin:            installer.Pin{Version: r.ver, ManifestSHA256: r.pin},
		ArtifactSource: "file://" + r.src,
		WireRunner:     gh,
		Stdin:          strings.NewReader(stdin),
		Stdout:         stdout,
		Logger:         quiet(),
	}
}

// Picking a listed repository settles both knobs at once: wiring.repo=use
// and the coordinate, with no second question about either.
func TestRepoQuestionPickingAListedRepository(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN") + "," + emptyRepoNode("octo/second", "WRITE")}
	var out bytes.Buffer
	// "1" picks the first listed repo; the five remaining questions default.
	rep, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "1\n\n\n\n\n\n", &out, gh))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if rep.Knobs.Wiring.Repo != "use" {
		t.Errorf("wiring.repo = %q, want use (an existing repository was chosen)", rep.Knobs.Wiring.Repo)
	}
	transcript := out.String()
	for _, want := range []string{
		"Where should this project live?",
		"1) octo/chosen",
		"empty · you have admin",
		"2) octo/second",
		"3) create a new GitHub repository…",
		"4) keep it local — no GitHub at all",
		"wiring GitHub (octo/chosen):",
	} {
		if !strings.Contains(transcript, want) {
			t.Errorf("transcript lacks %q:\n%s", want, transcript)
		}
	}
	// The two questions this one replaces must not also be asked.
	for _, gone := range []string{
		"Should pyxis use an existing GitHub repository, or create a new private one for you?",
		"No git remote is set — which GitHub repository",
	} {
		if strings.Contains(transcript, gone) {
			t.Errorf("the replaced question still fired (%q):\n%s", gone, transcript)
		}
	}
}

// "Keep it local" settles tracker, ci_mode, and protection — and those three
// questions are never put to the operator.
func TestRepoQuestionLocalSuppressesTheGitHubQuestions(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN")}
	var out bytes.Buffer
	// "3" is local (1 repo + create + local); then argus and surfaces only.
	rep, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "3\n\n\n"+filepath.Join(r.src, "local.git")+"\n", &out, gh))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if rep.Knobs.Tracker != "carina" || rep.Knobs.CIMode != "local" || rep.Knobs.Wiring.Protection != "none" {
		t.Fatalf("local derivation wrong: tracker=%q ci=%q protection=%q",
			rep.Knobs.Tracker, rep.Knobs.CIMode, rep.Knobs.Wiring.Protection)
	}
	// The whole point: this combination declares NO GitHub operation, so
	// nothing about GitHub can run or be asked.
	for _, op := range wire.Table(wire.Config{Repo: config.Config{Knobs: rep.Knobs}}) {
		if op.Transport == wire.TransportGH {
			t.Errorf("a local install still declares the gh op %q", op.Name)
		}
	}
	transcript := out.String()
	for _, gone := range []string{
		"Where should pyxis track issues and PRs?",
		"Where should changes be checked before they land?",
		"How strictly should the main branch be protected?",
		"No git remote is set — which GitHub repository",
	} {
		if strings.Contains(transcript, gone) {
			t.Errorf("a GitHub question was asked in a local install (%q):\n%s", gone, transcript)
		}
	}
	// What it settled is stated, not silently assumed.
	if !strings.Contains(transcript, "local install: tracker=carina · ci_mode=local · protection=none") {
		t.Errorf("the derivation was not disclosed:\n%s", transcript)
	}
	// Still a real install: argus and surfaces were asked, and it went green.
	if !strings.Contains(transcript, "Enable argus") || !strings.Contains(transcript, "Which coding agents") {
		t.Errorf("the two remaining questions were not asked:\n%s", transcript)
	}
	if len(rep.Commits) == 0 {
		t.Error("not a green install")
	}
}

// The create branch elicits a coordinate; a name the probe reports EMPTY is
// wired as `use` — the escape hatch for anything the list omitted.
func TestRepoQuestionCreateBranchUsesAnExistingEmptyRepository(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: "", probe: `{"data":{"repository":{"isEmpty":true}}}`}
	var out bytes.Buffer
	rep, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "1\nocto/omitted\n\n\n\n\n\n", &out, gh))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if rep.Knobs.Wiring.Repo != "use" {
		t.Errorf("wiring.repo = %q, want use — the named repository already exists and is empty", rep.Knobs.Wiring.Repo)
	}
	if !strings.Contains(out.String(), "wiring GitHub (octo/omitted):") {
		t.Errorf("the typed coordinate did not reach wiring:\n%s", out.String())
	}
}

// A name that resolves to a repository WITH content refuses pre-write: pyxis
// initializes an empty repository, and silently wiring into a populated one
// is not a recovery.
func TestRepoQuestionCreateBranchRefusesAPopulatedRepository(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: "", probe: `{"data":{"repository":{"isEmpty":false}}}`}
	var out bytes.Buffer
	_, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "1\nocto/busy\n", &out, gh))
	if err == nil {
		t.Fatal("Init accepted a populated repository, want a named refusal")
	}
	if !strings.Contains(err.Error(), "already exists and is not empty") {
		t.Errorf("err = %v, want the populated-repository refusal", err)
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Errorf("target holds %d entries after a pre-write refusal, want 0", len(entries))
	}
}

// A name this credential cannot resolve is AMBIGUOUS — GitHub answers the
// same 404 for "absent" and for "exists, private, invisible" — so the create
// bet is put to the operator pre-write, naming the account that asked.
// Confirming creates; declining refuses with nothing written.
func TestRepoQuestionUnresolvableNameIsCreatedNotReConfirmed(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)

	// "create a new GitHub repository…" then a name that does not resolve is
	// the ordinary case — it is what creating MEANS. The operator declared
	// the intent twice (the menu entry, then the name), so nothing further is
	// asked. The confirm this replaces defaulted to NO, so a bare Enter
	// declined the very thing just requested.
	target := t.TempDir()
	var out bytes.Buffer
	// Exactly six answers: the menu, the name, and the five that follow.
	rep, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "1\nocto/fresh\n\n\n\n\n\n", &out, &listingGH{nodes: ""}))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if rep.Knobs.Wiring.Repo != "create" {
		t.Errorf("wiring.repo = %q, want create", rep.Knobs.Wiring.Repo)
	}
	if got := out.String(); strings.Contains(got, "create it?") {
		t.Errorf("the operator was asked to confirm an intent already given twice:\n%s", got)
	}
	// The stdin script has no spare answer: had a confirm still been asked,
	// it would have eaten one of the five that follow and the knobs would
	// not have resolved to their defaults.
	if rep.Knobs.Tracker != "github" || rep.Knobs.CIMode != "both" {
		t.Errorf("knobs = %+v — a stray question consumed a later answer", rep.Knobs)
	}
}

// A listing that cannot run must not remove the local option: the operator
// whose credential cannot reach GitHub is exactly the one most likely to
// want "no GitHub at all", so the menu still renders with create and local.
func TestRepoQuestionListingFailureStillOffersLocal(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{failList: true}
	var out bytes.Buffer
	// "2" is local when the list is empty (create is 1, local is 2).
	rep, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "2\n\n\n"+filepath.Join(r.src, "local.git")+"\n", &out, gh))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	transcript := out.String()
	if !strings.Contains(transcript, "Where should this project live?") {
		t.Errorf("a listing failure removed the question entirely:\n%s", transcript)
	}
	for _, want := range []string{"1) create a new GitHub repository…", "2) keep it local — no GitHub at all"} {
		if !strings.Contains(transcript, want) {
			t.Errorf("the menu lacks %q with no listing behind it:\n%s", want, transcript)
		}
	}
	if rep.Knobs.Tracker != "carina" || rep.Knobs.CIMode != "local" {
		t.Errorf("local was not settled: tracker=%q ci=%q", rep.Knobs.Tracker, rep.Knobs.CIMode)
	}
	if len(rep.Commits) == 0 {
		t.Error("not a green install")
	}
}

// Selection honors FR-13's bound: out-of-range answers re-ask, and a third
// one aborts naming what was already collected.
func TestRepoQuestionBoundedReAsk(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN")}
	var out bytes.Buffer
	// Two bad answers, then a good one.
	if _, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "9\nnope\n1\n\n\n\n\n\n", &out, gh)); err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "is not one of 1-3") {
		t.Errorf("no re-ask notice:\n%s", out.String())
	}

	target2 := t.TempDir()
	var out2 bytes.Buffer
	_, err := installer.Init(t.Context(), repoQuestionOptions(r, target2, "8\n9\n10\n", &out2, &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN")}))
	if err == nil {
		t.Fatal("three out-of-range answers succeeded, want the bounded abort")
	}
	if !strings.Contains(err.Error(), "after 3 attempts") {
		t.Errorf("err = %v, want FR-13's bounded abort", err)
	}
	if entries, _ := os.ReadDir(target2); len(entries) != 0 {
		t.Errorf("target holds %d entries after a pre-write abort, want 0", len(entries))
	}
}

// A configured remote already names the repository: nothing is asked.
func TestRepoQuestionSkippedWhenARemoteResolves(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()
	mustGit(t, target, "init", "-b", "main")
	mustGit(t, target, "remote", "add", "origin", wire.RemoteURL("octo/widget"))

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN")}
	var out bytes.Buffer
	if _, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "\n\n\n\n\n", &out, gh)); err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "Where should this project live?") {
		t.Errorf("the repository question fired despite a configured remote:\n%s", out.String())
	}
	for _, c := range gh.calls {
		if isRepoListQuery(c) {
			t.Error("a listing was fetched for a target whose remote already names the repository")
		}
	}
}

// Promptless parity: a flags/answers run asks nothing and never lists.
func TestRepoQuestionAbsentFromPromptlessRuns(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN")}
	var out bytes.Buffer
	opts := r.baseOptions(target)
	opts.WireRunner = gh
	opts.Stdout = &out
	rep, err := installer.Init(t.Context(), opts)
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "Where should this project live?") {
		t.Errorf("a promptless run asked a question:\n%s", out.String())
	}
	for _, c := range gh.calls {
		if isRepoListQuery(c) {
			t.Error("a promptless run fetched a repository listing")
		}
	}
	// The knobs the answers file named, unchanged.
	if rep.Knobs.Wiring.Repo != "use" || rep.Knobs.Tracker != "github" {
		t.Errorf("promptless knobs drifted: %+v", rep.Knobs)
	}
}

// The list is capped, and what was omitted is stated rather than hidden.
func TestRepoQuestionCapsTheListAndSaysSo(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	var rows []string
	for i := range 18 {
		rows = append(rows, emptyRepoNode("octo/repo"+string(rune('a'+i)), "ADMIN"))
	}
	gh := &listingGH{nodes: strings.Join(rows, ",")}
	var out bytes.Buffer
	if _, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "1\n\n\n\n\n\n", &out, gh)); err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	transcript := out.String()
	if !strings.Contains(transcript, "3 more empty repositories not shown") {
		t.Errorf("the omission was not stated:\n%s", transcript)
	}
	// Exactly 15 repositories, then create (16) and local (17) — the two
	// standing options always come last, whatever the list length.
	if !strings.Contains(transcript, "15) octo/repo") {
		t.Errorf("the list stopped short of its cap:\n%s", transcript)
	}
	if strings.Contains(transcript, "16) octo/repo") {
		t.Errorf("a 16th repository was listed past the cap:\n%s", transcript)
	}
	if !strings.Contains(transcript, "16) create a new GitHub repository") || !strings.Contains(transcript, "17) keep it local") {
		t.Errorf("the standing options are not last after a capped list:\n%s", transcript)
	}
}

// A closed stdin must choose NOTHING. The menu's answer names an external
// destination, so an unattended run (`pyxis init < /dev/null`, a CI wrapper)
// that inherited a default would wire a real repository nobody picked and
// run the whole account-facing wiring walk against it.
func TestRepoQuestionClosedStdinWiresNothing(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN") + "," + emptyRepoNode("octo/second", "WRITE")}
	var out bytes.Buffer
	_, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "", &out, gh))
	if err == nil {
		t.Fatal("a closed stdin produced an install, want the pre-write refusal")
	}
	var refusal *preflight.RefusalError
	if !errors.As(err, &refusal) || refusal.Outcome.Check != preflight.GitHubModeID {
		t.Fatalf("err = %v, want the GitHub-mode coordinate refusal (nothing chosen on the operator's behalf)", err)
	}
	if entries, _ := os.ReadDir(target); len(entries) != 0 {
		t.Errorf("target holds %d entries, want 0 — nothing may be written", len(entries))
	}
	// The decisive assertion: no repository was touched on GitHub.
	for _, c := range gh.calls {
		joined := strings.Join(c, " ")
		for _, mutation := range []string{"repo create", "label create", "issue create", "--method PUT"} {
			if strings.Contains(joined, mutation) {
				t.Errorf("an unattended run performed a GitHub mutation: gh %s", joined)
			}
		}
	}
}

// A blank answer is not an answer, but it is not a stop either: it re-asks
// within FR-13's bound rather than defaulting to somebody's repository.
func TestRepoQuestionBlankReAsksRatherThanDefaulting(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN")}
	var out bytes.Buffer
	if _, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "\n1\n\n\n\n\n\n", &out, gh)); err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "this one has no default, because it names where your work goes") {
		t.Errorf("a blank answer did not explain why it re-asked:\n%s", out.String())
	}
}

// The LAST listed repository is selectable — the boundary a menu off-by-one
// lives on, where picking it would silently become "keep it local".
func TestRepoQuestionPicksTheLastListedRepository(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/first", "ADMIN") + "," + emptyRepoNode("octo/last", "ADMIN")}
	var out bytes.Buffer
	rep, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "2\n\n\n\n\n\n", &out, gh))
	if err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if rep.Knobs.Wiring.Repo != "use" || rep.Knobs.Tracker != "github" {
		t.Fatalf("picking the last listed repository did not wire it: %+v", rep.Knobs)
	}
	if !strings.Contains(out.String(), "wiring GitHub (octo/last):") {
		t.Errorf("the last listed repository was not the chosen coordinate:\n%s", out.String())
	}
}

// Promptless parity, with NO coordinate supplied: the question must not run
// at all. Without its promptless guard this path reaches the menu with a nil
// prompter and dereferences it.
func TestRepoQuestionPromptlessWithoutACoordinateNeverAsks(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	gh := &listingGH{nodes: emptyRepoNode("octo/chosen", "ADMIN")}
	var out bytes.Buffer
	opts := r.baseOptions(target)
	opts.WireRunner = gh
	opts.Stdout = &out
	opts.Stdin = nil             // promptless
	opts.Answers.Coordinate = "" // and nothing names a repository
	opts.Answers.RemoteURL = filepath.Join(r.src, "local.git")
	opts.Answers.Tracker = "carina" // a wiring-free combination resolves
	opts.Answers.CIMode = "local"
	opts.Answers.WiringProtection = "none"
	if _, err := installer.Init(t.Context(), opts); err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if strings.Contains(out.String(), "Where should this project live?") {
		t.Errorf("a promptless run asked the repository question:\n%s", out.String())
	}
	for _, c := range gh.calls {
		if isRepoListQuery(c) {
			t.Error("a promptless run fetched a repository listing")
		}
	}
}

// A truncated window must never read as "you have no empty repositories".
// The query sees the 100 most recently updated, and empty repositories rank
// LOWEST on that axis, so an empty menu from a truncated window is a fact
// the listing does not have.
func TestRepoQuestionTruncatedWindowSaysSoRatherThanImplyingNone(t *testing.T) {
	isolateGit(t)
	ambientIdentityConfig(t, "Ada", "ada@example.com")
	r := newRelease(t)
	target := t.TempDir()

	// No candidates survived the filter, but more repositories exist.
	gh := &listingGH{nodes: "", truncated: true}
	var out bytes.Buffer
	// "2" is local; the run only has to reach the menu.
	if _, err := installer.Init(t.Context(), repoQuestionOptions(r, target, "2\n\n\n"+filepath.Join(r.src, "local.git")+"\n", &out, gh)); err != nil {
		t.Fatalf("Init: %v\n%s", err, out.String())
	}
	if !strings.Contains(out.String(), "only the 100 most recently updated were read") {
		t.Errorf("an empty menu from a truncated window implied none exist:\n%s", out.String())
	}
}
