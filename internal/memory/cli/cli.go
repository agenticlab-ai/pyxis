// Package cli is the memory tool: every verb a person or an agent runs
// against the records under memory/. It is the only package that knows git;
// everything else works on record sets.
//
// Exit codes: 0 done, 1 refused or blocked (the reason is on stderr), 2 usage
// or environment.
package cli

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/memory/ask"
	"github.com/agenticlab-ai/pyxis/internal/memory/association"
	"github.com/agenticlab-ai/pyxis/internal/memory/capture"
	"github.com/agenticlab-ai/pyxis/internal/memory/gitsrc"
	"github.com/agenticlab-ai/pyxis/internal/memory/pack"
	"github.com/agenticlab-ai/pyxis/internal/memory/packet"
	"github.com/agenticlab-ai/pyxis/internal/memory/policy"
	"github.com/agenticlab-ai/pyxis/internal/memory/record"
	"github.com/agenticlab-ai/pyxis/internal/memory/resolve"
	"github.com/agenticlab-ai/pyxis/internal/memory/search"
	"github.com/agenticlab-ai/pyxis/internal/memory/sessionlog"
	"github.com/agenticlab-ai/pyxis/internal/memory/store"
	"github.com/agenticlab-ai/pyxis/internal/memory/view"
)

const (
	exitOK      = 0
	exitRefused = 1
	exitUsage   = 2
)

// Env is what the tool needs from its surroundings; tests inject all of it.
type Env struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Dir     string
	Now     func() time.Time
	Entropy func() ([10]byte, error)
	Getenv  func(string) string
	// pinned carries one invocation's repository handle and pinned state into
	// the verbs a composite verb runs, so the gate's steps read each pinned
	// commit once between them. An ordinary verb leaves it nil.
	pinned *pinnedState
}

// OSEnv is the environment of the real process.
func OSEnv() Env {
	dir, err := os.Getwd()
	if err != nil {
		dir = "."
	}
	return Env{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr, Dir: dir, Now: time.Now, Entropy: record.Entropy, Getenv: os.Getenv}
}

const usageText = `usage: memory <verb> [flags]

writing
  init                                  create memory/policy.json with the default policy
  add --kind K --subject S [--family F] --statement "…" [--intent …] [--rationale …] [--fit …]
      [--learned-from x]… [--alternative x]… [--tag t]… [--supersedes id]… [--serves id|subject#family]…
      [--depends-on id]… [--about id]… [--refines id]… [--consequence low|medium|high] [--expires RFC3339]
      [--source path] [--pr N] [--commit sha] [--note text] [--actor who] [--json]   (--json reads the fields from stdin instead)
      a Document (--kind Document --family prd|design… --statement "title" --body-file doc.md) is the narrative of a
      subject: markdown that embeds claims as {{FR-3}} or {{prd/x#FR-3}} and never restates them; --literal-body
      preserves imported legacy/template bytes without interpreting brace-shaped text as claim embeds
  revise <subject#slot> --body-file doc.md [--statement title] [--intent …] [--rationale …]
                                        the next version of a document; the previous one stays as history
  revoke <id> --reason "…"              withdraw a record; lists what rested on it
  ask --subject S --text "…"            ask once: returns the existing answer, or the open question, or records a new one
  answer <question-id> --text "…" [--replace]   answer a question; a question has one active answer, so a correction
                                        must say --replace and supersedes the answer it corrects

reading
  render <subject> [--slot prd|design] [--at ref]   the human document: the narrative with every embedded claim expanded
                                        to its current head; --at renders it as the records stood at a ref
  view <subject> [--since ref]          the PRD (prd/…) or design (design/…) rendered from the records
  why <id | subject#family>             the chain behind a claim: every version and why it moved
  delivery <subject>                    which claims a decision serves and which are undesigned
  status                                every subject, and everything that needs a human
  search "text" [--subject S] [--kind K] [--tag t]… [--code path]… [-k N] [--all]
  collide --statement "…" [--subject S] [--family F] [--kind K] [--intent …] [--rationale …] [--tag t]… [--code p]…
      [-k N] [--judge-cmd "command"] [--json]  what a draft supersedes, conflicts with, or leaves alone
  pack [--subject S]… [--family F]… [--code path]… [--budget tokens] [--json]   task context (subject or code required)
  ready <feature> [--base ref] [--head ref] emit one draft-readiness/v1 verdict; exit 1 if unmet
  ready --all [--base ref] [--head ref]     judge changed draft heads; print "no draft" if none

integrity (offline, pinned to HEAD)
  evidence --check [--base REF] [--json]               verify exact evidence independently of the debt baseline
  evidence --record ID… [--base REF] [--source REF] [--write] [--json]
                                                      preview or bind explicitly selected unlanded records;
                                                      readers must be upgraded before writing memory-evidence/v1
  discard-proposal --record ID… --reason text --check|--write [--json]
                                                      withdraw unlanded source Documents and exact source-only Decision companions as a closed batch;
                                                      archive exact bytes; never change mainline records or Git history
  reconcile --check [--baseline .github/memory-reconciliation-baseline.json [--baseline-ref trusted-commit [--baseline-replay-ref original-snapshot]]] [--json]
  reconcile --write [--action 'code|scope']… [--actor who] [--json] | --baseline-generate
  reconcile --baseline-sync --baseline-ref trusted-commit [--migrate-v2] [--baseline-replay-ref original-snapshot] [--json]
                                                      compare authored sources, compiled contracts, code, and memory;
                                                      --baseline requires the exact checked-in debt inventory;
                                                      --baseline-ref also proves it only tightens its trusted predecessor;
                                                      --baseline-replay-ref must reproduce that predecessor's unchanged baseline and exact inputs;
                                                      --baseline-generate emits the current exact v1 inventory;
                                                      --baseline-sync writes only changed v2 allowance members, with no new debt;
                                                      --migrate-v2 explicitly preserves the trusted v1 inventory in a data-only cutover;
                                                      --write appends lossless Documents, exact PRD rows/questions, and explicit design Decisions;
                                                      repeat --action for an exact bounded set, or omit it to write all
  closeout --feature name --check [--json]
  retire-source --source path [--source path...] --reason text --check|--write [--actor who] [--json]
                                               seal standalone source bytes before a later deletion
  closeout --feature name --write [--actor who] [--json]   seal a complete feature from the configured trusted mainline;
                                                           run on a closeout-only branch before retiring source context
  verify --offline [--json]                    validate memory, current code, and every active closeout receipt without a network

proposing (needs git)
  packet [--base ref] [--body] [--description "…"]   what this branch adds, moves, reopens; --body prints the PR body
  check --base ref                                    the merge check: exit 1 on blockers; the base must come from a trusted
                                                      source (in CI the pull request's base ref), never from the candidate

The packet's base defaults to the mainline named in .pyxis/config.yaml (remote-tracking first, then local);
a base that is the branch's own commit is refused.
The actor is --actor, then $MEMORY_ACTOR, then git user.email.
`

// Run executes one invocation.
func Run(ctx context.Context, args []string, env Env) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		if _, err := fmt.Fprint(env.Stderr, usageText); err != nil {
			return exitRefused
		}
		return exitUsage
	}
	verb, rest := args[0], args[1:]
	var err error
	code := exitOK
	switch verb {
	case "init":
		err = runInit(ctx, env)
	case "evidence":
		code, err = runEvidence(ctx, rest, env)
	case "discard-proposal":
		code, err = runDiscardProposal(ctx, rest, env)
	case "add":
		code, err = runAdd(ctx, rest, env)
	case "revise":
		code, err = runRevise(ctx, rest, env)
	case "render":
		err = runRender(ctx, rest, env)
	case "revoke":
		code, err = runRevoke(ctx, rest, env)
	case "ask":
		code, err = runAsk(ctx, rest, env)
	case "answer":
		code, err = runAnswer(ctx, rest, env)
	case "view":
		err = runView(ctx, rest, env)
	case "why":
		err = runWhy(ctx, rest, env)
	case "delivery":
		err = runDelivery(ctx, rest, env)
	case "status":
		err = runStatus(ctx, rest, env)
	case "search":
		err = runSearch(ctx, rest, env)
	case "collide":
		err = runCollide(ctx, rest, env)
	case "pack":
		err = runPack(ctx, rest, env)
	case "ready":
		code, err = runReady(ctx, rest, env)
	case "reconcile":
		code, err = runReconcile(ctx, rest, env)
	case "closeout":
		code, err = runCloseout(ctx, rest, env)
	case "retire-source":
		code, err = runRetireSource(ctx, rest, env)
	case "verify":
		code, err = runVerify(ctx, rest, env)
	case "packet":
		code, err = runPacket(ctx, rest, env, false)
	case "check":
		code, err = runPacket(ctx, rest, env, true)
	default:
		if _, err := fmt.Fprintf(env.Stderr, "memory: unknown verb %q\n%s", verb, usageText); err != nil {
			return exitRefused
		}
		return exitUsage
	}
	if err != nil {
		var ue usageError
		if errors.As(err, &ue) {
			if _, err := fmt.Fprintf(env.Stderr, "memory %s: %v\n", verb, err); err != nil {
				return exitRefused
			}
			return exitUsage
		}
		if _, err := fmt.Fprintf(env.Stderr, "memory %s: %v\n", verb, err); err != nil {
			return exitRefused
		}
		if code == exitOK {
			code = exitRefused
		}
		return code
	}
	return code
}

type usageError struct{ msg string }

func (u usageError) Error() string { return u.msg }

func usage(format string, a ...any) error { return usageError{fmt.Sprintf(format, a...)} }

// multi is a repeatable string flag.
type multi []string

func (m *multi) String() string     { return strings.Join(*m, ",") }
func (m *multi) Set(v string) error { *m = append(*m, v); return nil }

func newFlags(verb string, env Env) *flag.FlagSet {
	fs := flag.NewFlagSet("memory "+verb, flag.ContinueOnError)
	fs.SetOutput(env.Stderr)
	return fs
}

// parse accepts flags before, between and after positional arguments (the
// standard library stops at the first positional) and returns the positionals.
func parse(fs *flag.FlagSet, args []string) ([]string, error) {
	var pos []string
	for {
		if err := fs.Parse(args); err != nil {
			return nil, usage("%v", err)
		}
		rest := fs.Args()
		if len(rest) == 0 {
			return pos, nil
		}
		pos = append(pos, rest[0])
		args = rest[1:]
	}
}

// ---------------------------------------------------------------- opening the memory

type memory struct {
	reconciliationBase string // trusted predecessor supplied by the ratchet caller
	root               string // repository root
	dir                string // the memory directory
	remote             string // the git remote of the mainline, from the repository's config
	branch             string // the mainline branch, from the repository's config
	repo               *gitsrc.Repo
	set                *store.Set
	proj               *resolve.Projection
	pol                policy.Policy
	cfg                config.Config
	evidence           *evidenceSession // capture cache scoped to this writer and its pinned inputs
	pinned             *pinnedState     // what this invocation derived from pinned commits; nil for a hand-built memory
}

// lockWait is how long a writer waits for another writer to finish.
const lockWait = 10 * time.Second

// lockFile is the write lock's name inside the repository's git directory,
// which every writer of the same repository shares and git never commits.
const lockFile = "memory-write.lock"

// openLocked is open for writers: it takes the memory's write lock first and
// only then loads, so the admission decision and the write that follows it
// see the same files. The release must be deferred by the caller.
func openLocked(ctx context.Context, env Env) (*memory, func(), error) {
	repo, err := locate(ctx, env)
	if err != nil {
		return nil, nil, err
	}
	common, err := repo.CommonDir(ctx)
	if err != nil {
		return nil, nil, err
	}
	release, err := store.Lock(filepath.Join(common, lockFile), lockWait)
	if err != nil {
		return nil, nil, err
	}
	m, err := open(ctx, env)
	if err != nil {
		release()
		return nil, nil, err
	}
	return m, release, nil
}

// locate finds the repository the memory lives in. The memory is shared
// through git, so there is no memory outside a repository: a plain directory
// is a usage error and a broken git is an environment error, never a
// standalone tree that writes would silently land in.
func locate(ctx context.Context, env Env) (*gitsrc.Repo, error) {
	repo, err := gitsrc.Open(ctx, env.Dir)
	switch {
	case errors.Is(err, gitsrc.ErrNotRepo):
		return nil, usage("%s is not inside a git repository; the memory lives in one", env.Dir)
	case err != nil:
		return nil, usage("%v", err)
	}
	return repo, nil
}

func open(ctx context.Context, env Env) (*memory, error) {
	state := env.pinned
	if state == nil {
		repo, err := locate(ctx, env)
		if err != nil {
			return nil, err
		}
		state = newPinnedState(repo)
	}
	m := &memory{root: state.repo.Root, repo: state.repo, pinned: state}
	// the mainline's remote and branch come from the pyxis config surface, the
	// single source of every target-facing value (DINV-1); an absent config
	// reads as its schema defaults, a defective one stops every verb
	cfg, defects := config.Load(m.root)
	if len(defects) > 0 {
		msgs := make([]string, 0, len(defects))
		for _, d := range defects {
			msgs = append(msgs, d.String())
		}
		return nil, fmt.Errorf("the repository config has %d defect(s); the memory does not run on a broken config: %s", len(defects), strings.Join(msgs, "; "))
	}
	m.remote, m.branch = cfg.Remote, cfg.Branch
	m.cfg = cfg
	if m.remote == "" {
		m.remote = config.Defaults().Remote
	}
	if m.branch == "" {
		m.branch = config.Defaults().Branch
	}
	m.dir = filepath.Join(m.root, store.Dir)
	set, err := store.Load(ctx, store.DirSource{Root: m.dir})
	if err != nil {
		return nil, err
	}
	m.set = set
	m.proj = resolve.New(set)
	m.pol, err = policy.Parse(set.Policy)
	if err != nil {
		return nil, err
	}
	return m, nil
}

func (m *memory) needGit() error {
	if m.repo == nil {
		return usage("%s is not inside a git repository; this verb compares the branch with its base", m.root)
	}
	return nil
}

// LockPath is where the memory's write lock lives for the repository at dir;
// exported for tests that assert nothing is left behind.
func LockPath(ctx context.Context, dir string) (string, error) {
	repo, err := gitsrc.Open(ctx, dir)
	if err != nil {
		return "", err
	}
	common, err := repo.CommonDir(ctx)
	if err != nil {
		return "", err
	}
	return filepath.Join(common, lockFile), nil
}

// baseRef is a proposal's base, pinned to one commit for the whole run.
type baseRef struct {
	name string
	oid  string
}

func (b baseRef) String() string { return b.name + "@" + b.oid[:7] }

// base resolves the base a proposal is measured against. A gate (check)
// takes it only from --base, because the configured mainline lives in the
// candidate's own tree and cannot be trusted to judge that tree; the packet
// may default to the configured mainline for convenience. Either way the
// base is pinned to a commit once. A base that is the branch's own commit
// while the working tree holds nothing beyond it is refused: that compares a
// tree with itself and can only report nothing. With uncommitted work the
// comparison is meaningful (an author checking before committing).
func (m *memory) base(ctx context.Context, flag string, gate bool) (baseRef, error) {
	if err := m.needGit(); err != nil {
		return baseRef{}, err
	}
	name := flag
	if name == "" {
		if gate {
			return baseRef{}, usage("check needs an explicit --base from a trusted source (in CI, the pull request's base ref); the mainline configured in the candidate's own tree is not trusted for a gate")
		}
		name = m.repo.DefaultBase(ctx, m.remote, m.branch)
		if name == "" {
			return baseRef{}, usage("no base ref: neither %s/%s nor %s exists; pass --base", m.remote, m.branch, m.branch)
		}
	}
	oid, err := m.repo.ResolveCommit(ctx, name)
	if err != nil {
		return baseRef{}, usage("base ref %q does not exist", name)
	}
	head, err := m.repo.Head(ctx)
	if err != nil {
		return baseRef{}, err
	}
	if oid == head {
		dirty, err := m.repo.Dirty(ctx, store.Dir)
		if err != nil {
			return baseRef{}, err
		}
		if !dirty {
			return baseRef{}, fmt.Errorf("base %s is this branch's own commit %s and the working tree holds nothing beyond it; a proposal cannot be checked against itself — pass the base it will merge into", name, oid[:7])
		}
	}
	return baseRef{name: name, oid: oid}, nil
}

// mainline is the base other verbs measure in-flight work against, or zero
// when the repository has no mainline yet or the mainline is this commit.
func (m *memory) mainline(ctx context.Context) baseRef {
	if m.repo == nil {
		return baseRef{}
	}
	b, err := m.base(ctx, "", false)
	if err != nil {
		return baseRef{}
	}
	return b
}

// inFlight collects, per other branch, the records it adds relative to the
// base. A remote-tracking branch is skipped only when its local branch points
// at the same commit, so one piece of work is reported once but a remote
// that moved on is never hidden behind a stale local branch.
func (m *memory) inFlight(ctx context.Context, base baseRef) (map[string]*store.Set, error) {
	if m.repo == nil {
		return nil, nil
	}
	branches, err := m.repo.Branches(ctx)
	if err != nil {
		return nil, err
	}
	current, err := m.repo.CurrentBranch(ctx)
	if err != nil {
		return nil, err
	}
	head, err := m.repo.Head(ctx)
	if err != nil {
		return nil, err
	}
	remotes, err := m.repo.Remotes(ctx)
	if err != nil {
		return nil, err
	}
	// remoteOf returns the remote a tracking branch belongs to, or "" for a local branch
	remoteOf := func(b string) string {
		for _, r := range remotes {
			if strings.HasPrefix(b, r+"/") {
				return r
			}
		}
		return ""
	}
	local := map[string]string{} // local branch → commit
	for _, b := range branches {
		if remoteOf(b.Name) == "" {
			local[b.Name] = b.OID
		}
	}
	out := map[string]*store.Set{}
	for _, br := range branches {
		b := br.Name
		if b == current || b == base.name {
			continue
		}
		if r := remoteOf(b); r != "" {
			name := strings.TrimPrefix(b, r+"/")
			if name == current && br.OID == head {
				continue // the checked-out branch stands for its own remote twin
			}
			if oid, ok := local[name]; ok && name != current && oid == br.OID {
				continue // the local branch stands for it, at the same commit
			}
		}
		files, err := m.repo.AddedFiles(ctx, base.oid, b, store.Dir)
		if err != nil {
			continue // a branch with no merge base (an orphan) is not in flight against this base
		}
		if len(files) == 0 {
			continue
		}
		blobs, err := m.repo.Files(ctx, b, files)
		if err != nil {
			return nil, err
		}
		src := store.MapSource{}
		for p, blob := range blobs {
			src[strings.TrimPrefix(p, store.Dir+"/")] = blob
		}
		set, err := store.Load(ctx, src)
		if err != nil {
			return nil, err
		}
		if len(set.Records) > 0 {
			out[b] = set
		}
	}
	return out, nil
}

func (m *memory) actor(ctx context.Context, flag string, env Env) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if a := env.Getenv("MEMORY_ACTOR"); a != "" {
		return a, nil
	}
	if m.repo != nil {
		if a := m.repo.UserEmail(ctx); a != "" {
			return a, nil
		}
	}
	return "", errors.New("no actor: pass --actor, set MEMORY_ACTOR, or configure git user.email")
}

func (m *memory) newID(kind string, env Env) (string, error) {
	e, err := env.Entropy()
	if err != nil {
		return "", err
	}
	return record.NewID(kind, env.Now(), e), nil
}

// resolveRef accepts a record id or subject#family and returns the record.
func (m *memory) resolveRef(ref string) (*record.Record, error) {
	if r := m.set.Get(ref); r != nil {
		return r, nil
	}
	if i := strings.IndexByte(ref, '#'); i > 0 {
		if h := m.proj.Head(ref[:i], ref[i+1:]); h != nil {
			return h, nil
		}
		return nil, fmt.Errorf("%s has no active head", ref)
	}
	return nil, fmt.Errorf("no record %s", ref)
}

func (m *memory) docs(inflight map[string]*store.Set) []*search.Doc {
	var docs []*search.Doc
	for _, id := range m.set.IDs() {
		docs = append(docs, m.doc(m.set, m.set.Records[id], string(m.proj.Status(id)), ""))
	}
	branches := make([]string, 0, len(inflight))
	for b := range inflight {
		branches = append(branches, b)
	}
	sort.Strings(branches)
	for _, b := range branches {
		other := resolve.New(inflight[b])
		for _, id := range inflight[b].IDs() {
			if m.set.Get(id) != nil {
				continue // a retained branch cannot resurrect or duplicate a known identity
			}
			docs = append(docs, m.doc(inflight[b], inflight[b].Records[id], string(other.Status(id)), b)) // the body lives in the set the record came from
		}
	}
	return docs
}

func (m *memory) doc(from *store.Set, r *record.Record, status, branch string) *search.Doc {
	d := &search.Doc{ID: r.ID, Kind: r.Kind, Subject: r.Subject, Family: r.Family, Status: status, Statement: r.Statement, Intent: r.Intent,
		Rationale: r.Rationale, Tags: r.Tags, Code: association.Scopes(r, from, m.set), InFlight: branch, Body: string(from.Bodies[r.ID]), Historical: sessionlog.Document(r) || capture.Derived(r)}
	return d
}

// ---------------------------------------------------------------- init

func runInit(ctx context.Context, env Env) error {
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	if m.set.Policy != nil {
		if _, err := fmt.Fprintf(env.Stdout, "memory already initialised at %s\n", m.dir); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		return nil
	}
	b, err := policy.Default().Marshal()
	if err != nil {
		return err
	}
	if err := store.WritePolicy(m.dir, b); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(env.Stdout, "initialised %s with the default policy; records go under %s/<namespace>/<name>/\n", filepath.Join(m.dir, store.PolicyFile), m.dir); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------- add

type addInput struct {
	Kind         string            `json:"kind"`
	Subject      string            `json:"subject"`
	Family       string            `json:"family,omitempty"`
	Statement    string            `json:"statement"`
	Intent       string            `json:"intent,omitempty"`
	Rationale    string            `json:"rationale,omitempty"`
	FitCriterion string            `json:"fit_criterion,omitempty"`
	LearnedFrom  []string          `json:"learned_from,omitempty"`
	Alternatives []string          `json:"alternatives_rejected,omitempty"`
	Tags         []string          `json:"tags,omitempty"`
	Consequence  string            `json:"consequence,omitempty"`
	Expires      string            `json:"expires,omitempty"`
	Supersedes   []string          `json:"supersedes,omitempty"`
	Serves       []string          `json:"serves,omitempty"`
	DependsOn    []string          `json:"depends_on,omitempty"`
	About        []string          `json:"about,omitempty"`
	Refines      []string          `json:"refines,omitempty"`
	Source       string            `json:"source,omitempty"`
	PR           int               `json:"pr,omitempty"`
	Commit       string            `json:"commit,omitempty"`
	Note         string            `json:"note,omitempty"`
	Relations    []record.Relation `json:"relations,omitempty"`
	Body         string            `json:"body,omitempty"`      // Document: the narrative itself
	BodyFile     string            `json:"body_file,omitempty"` // Document: or the file holding it
	LiteralBody  bool              `json:"literal_body,omitempty"`
}

func runAdd(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("add", env)
	var in addInput
	var learned, alts, tags, sup, serves, deps, about, refines multi
	fs.StringVar(&in.Kind, "kind", "", "record kind")
	fs.StringVar(&in.Subject, "subject", "", "subject (namespace/name)")
	fs.StringVar(&in.Family, "family", "", "family (exclusivity key)")
	fs.StringVar(&in.Statement, "statement", "", "the statement")
	fs.StringVar(&in.Intent, "intent", "", "the future decision or action this knowledge informs")
	fs.StringVar(&in.Rationale, "rationale", "", "the reason or constraint that cannot be reconstructed from code")
	fs.StringVar(&in.FitCriterion, "fit", "", "how it is verified")
	fs.Var(&learned, "learned-from", "what it was learned from (repeatable)")
	fs.Var(&alts, "alternative", "an alternative rejected (repeatable)")
	fs.Var(&tags, "tag", "a tag (repeatable; code:<path> marks code impact)")
	fs.StringVar(&in.Consequence, "consequence", "", "declared consequence: low, medium, high")
	fs.StringVar(&in.Expires, "expires", "", "expiry (assumptions)")
	fs.Var(&sup, "supersedes", "record this replaces (repeatable)")
	fs.Var(&serves, "serves", "claim this serves: id or subject#family (repeatable)")
	fs.Var(&deps, "depends-on", "record this rests on (repeatable)")
	fs.Var(&about, "about", "anchor this is about (repeatable)")
	fs.Var(&refines, "refines", "record this refines (repeatable)")
	fs.StringVar(&in.Source, "source", "", "provenance: the document or log it came from")
	fs.IntVar(&in.PR, "pr", 0, "provenance: the pull request")
	fs.StringVar(&in.Commit, "commit", "", "provenance: the source commit")
	fs.StringVar(&in.Note, "note", "", "provenance: an attribution note")
	fs.StringVar(&in.BodyFile, "body-file", "", "Document: the markdown file holding the narrative")
	fs.BoolVar(&in.LiteralBody, "literal-body", false, "Document: preserve the body without interpreting {{...}} embeds")
	actorFlag := fs.String("actor", "", "who issues the record")
	fromJSON := fs.Bool("json", false, "read the fields as JSON from stdin")
	if _, err := parse(fs, args); err != nil {
		return exitUsage, err
	}
	if *fromJSON {
		dec := json.NewDecoder(env.Stdin)
		dec.DisallowUnknownFields()
		if err := dec.Decode(&in); err != nil {
			return exitUsage, usage("stdin JSON: %v", err)
		}
	} else {
		in.LearnedFrom, in.Alternatives, in.Tags, in.Supersedes, in.Serves, in.DependsOn, in.About, in.Refines = learned, alts, tags, sup, serves, deps, about, refines
	}
	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	actor, err := m.actor(ctx, *actorFlag, env)
	if err != nil {
		return exitUsage, err
	}
	body, err := readBody(in, env)
	if err != nil {
		return exitUsage, err
	}
	r, err := m.build(in, actor, env, body)
	if err != nil {
		return exitOK, err // Run maps a usage error to 2 and anything else to 1
	}
	return m.store(ctx, r, body, env)
}

// readBody returns a Document's narrative from --body-file, the JSON body
// field, or the JSON body_file; a Document without one is a usage error.
func readBody(in addInput, env Env) ([]byte, error) {
	if in.Kind != record.Document {
		if in.Body != "" || in.BodyFile != "" {
			return nil, usage("only a Document takes a body")
		}
		return nil, nil
	}
	switch {
	case in.Body != "":
		return []byte(in.Body), nil
	case in.BodyFile != "":
		path := in.BodyFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(env.Dir, path)
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, usage("--body-file: %v", err)
		}
		return b, nil
	}
	return nil, usage("a Document needs its narrative: --body-file doc.md (or body / body_file in the JSON)")
}

// store admits and writes a record, with its body when it is a Document.
func (m *memory) store(ctx context.Context, r *record.Record, body []byte, env Env) (int, error) {
	if err := m.admit(r, body); err != nil {
		return exitRefused, err
	}
	if err := m.captureEvidence(ctx, r); err != nil {
		return exitRefused, err
	}
	var path string
	var err error
	if r.Kind == record.Document {
		path, err = store.WriteDocument(m.dir, r, body)
	} else {
		path, err = store.Write(m.dir, r)
	}
	if err != nil {
		return exitRefused, err
	}
	if _, err := fmt.Fprintf(env.Stdout, "%s\n%s\n", r.ID, path); err != nil {
		return exitRefused, fmt.Errorf("write command output: %w", err)
	}
	if r.Kind == record.Document {
		if _, err := fmt.Fprintf(env.Stdout, "%s\n", filepath.ToSlash(filepath.Join(store.Dir, r.BodyPath()))); err != nil {
			return exitRefused, fmt.Errorf("write command output: %w", err)
		}
	}
	return exitOK, nil
}

func (m *memory) build(in addInput, actor string, env Env, body []byte) (*record.Record, error) {
	if !record.IsKind(in.Kind) {
		return nil, usage("--kind %q is not one of %s", in.Kind, strings.Join(record.Kinds(), ", "))
	}
	id, err := m.newID(in.Kind, env)
	if err != nil {
		return nil, err
	}
	r := &record.Record{Schema: record.Schema, ID: id, Kind: in.Kind, Subject: in.Subject, Family: in.Family, Statement: in.Statement, Intent: in.Intent,
		Rationale: in.Rationale, FitCriterion: in.FitCriterion, LearnedFrom: in.LearnedFrom, Alternatives: in.Alternatives, Tags: in.Tags,
		Consequence: in.Consequence, Expires: in.Expires, Issuer: actor, CreatedAt: env.Now().UTC().Format(time.RFC3339),
		Provenance: record.Provenance{Source: in.Source, PR: in.PR, Commit: in.Commit, Note: in.Note}, Relations: in.Relations, LiteralBody: in.LiteralBody}
	add := func(typ string, refs []string) error {
		for _, ref := range refs {
			target, err := m.resolveRef(ref)
			if err != nil {
				return usage("--%s %s: %v", strings.ReplaceAll(typ, "_", "-"), ref, err)
			}
			r.Relations = append(r.Relations, record.Relation{Type: typ, Target: target.ID})
		}
		return nil
	}
	for _, x := range []struct {
		typ  string
		refs []string
	}{{record.Supersedes, in.Supersedes}, {record.Serves, in.Serves}, {record.DependsOn, in.DependsOn}, {record.About, in.About}, {record.Refines, in.Refines}} {
		if err := add(x.typ, x.refs); err != nil {
			return nil, err
		}
	}
	if r.Kind == record.Document {
		r.BodySHA256 = record.HashBody(body)
		if !r.LiteralBody {
			r.Embeds = record.ParseEmbeds(body, r.Subject)
		}
	}
	if err := r.Validate(); err != nil {
		return nil, usage("%v", err)
	}
	for _, rel := range r.Relations {
		if m.set.Get(rel.Target) == nil {
			return nil, usage("relation %s target %s: no record %s", rel.Type, rel.Target, rel.Target)
		}
	}
	return r, nil
}

// admit refuses at write time what the merge check would refuse later: a
// claim in a family that already has an active head it does not name, or a
// supersedes target that is no longer the head.
func (m *memory) admit(r *record.Record, body []byte) error {
	if err := r.Validate(); err != nil {
		return err
	}
	if err := capture.CheckNew(r); err != nil {
		return err
	}
	if m.set.Get(r.ID) != nil {
		return fmt.Errorf("%w: %s", store.ErrExists, r.ID)
	}
	for _, edge := range r.Relations {
		if m.set.Get(edge.Target) == nil {
			return fmt.Errorf("dangling-target: %s %s %s, which does not exist", r.ID, edge.Type, edge.Target)
		}
	}
	// every supersession, whatever the kind, must be valid and point at a
	// target that is still the current one
	for _, t := range r.Rel(record.Supersedes) {
		target := m.set.Get(t)
		if target == nil {
			return fmt.Errorf("dangling-target: %s does not exist", t)
		}
		if v := resolve.SupersedesViolation(r, target); v != nil {
			return fmt.Errorf("%s: %s", v.Code, v.What)
		}
		if st := m.proj.Status(t); st != resolve.Active {
			msg := "nothing active stands in its place"
			if record.IsClaim(target.Kind) {
				msg = "the family has no active head"
				if h := m.proj.Head(target.Subject, target.Family); h != nil {
					msg = "the head is now " + h.ID
				}
			}
			return fmt.Errorf("effective-set-moved: %s is %s; %s — rebase the claim on the current head", t, st, msg)
		}
	}
	if record.IsClaim(r.Kind) {
		named := map[string]bool{}
		for _, t := range r.Rel(record.Supersedes) {
			named[t] = true
		}
		var unnamed []*record.Record
		for _, current := range m.proj.Active(r.Subject) {
			if record.IsClaim(current.Kind) && current.Family == r.Family && !named[current.ID] {
				unnamed = append(unnamed, current)
			}
		}
		if len(unnamed) > 0 {
			ids := make([]string, 0, len(unnamed))
			for _, current := range unnamed {
				ids = append(ids, current.ID)
			}
			sort.Strings(ids)
			return fmt.Errorf("conflict-at-admission: %s#%s already has active claim(s) %s; a successor must name every active claim with --supersedes, or choose another family",
				r.Subject, r.Family, strings.Join(ids, ", "))
		}
	}
	if r.Kind == record.Answer {
		q := r.Rel(record.Answers)[0]
		question := m.set.Get(q)
		if question == nil {
			return fmt.Errorf("dangling-target: %s does not exist", q)
		}
		if r.Subject != question.Subject {
			return fmt.Errorf("answers-across-subjects: answer on %s targets question %s on %s", r.Subject, q, question.Subject)
		}
		if m.proj.Status(q) != resolve.Active {
			return fmt.Errorf("answers-inactive-question: %s is %s; an answer needs an open question", q, m.proj.Status(q))
		}
		named := map[string]bool{}
		for _, id := range r.Rel(record.Supersedes) {
			named[id] = true
		}
		for _, current := range m.proj.ActiveAnswers(q) {
			if !named[current.ID] {
				return fmt.Errorf("answer-conflict-at-admission: %s already has active answer %s; a correction supersedes every active answer it replaces", q, current.ID)
			}
		}
	}
	if r.Kind == record.Revocation {
		targetID := r.Rel(record.Revokes)[0]
		target := m.set.Get(targetID)
		if target == nil {
			return fmt.Errorf("dangling-target: %s does not exist", targetID)
		}
		if r.Subject != target.Subject {
			return fmt.Errorf("revokes-across-subjects: revocation on %s targets %s on %s", r.Subject, targetID, target.Subject)
		}
		if status := m.proj.Status(targetID); status != resolve.Active {
			return fmt.Errorf("revokes-inactive-record: %s is already %s", targetID, status)
		}
	}
	var unresolved []string
	for _, ref := range r.Embeds {
		i := strings.IndexByte(ref, '#')
		if m.proj.Head(ref[:i], ref[i+1:]) == nil {
			unresolved = append(unresolved, "{{"+ref+"}}")
		}
	}
	if len(unresolved) > 0 {
		return fmt.Errorf("embed-unresolved: %s name no active claim; a document embeds claims, it does not introduce them — add the claim first", strings.Join(unresolved, ", "))
	}
	trial, err := m.projectionWith(r, body)
	if err != nil {
		return fmt.Errorf("project candidate: %w", err)
	}
	var defects []error
	if defect := firstNewProjectionDefect(m.proj, trial); defect != "" {
		defects = append(defects, errors.New(defect))
	}
	for _, violation := range resolve.HistoricalTransitionViolations(m.proj, trial) {
		defects = append(defects, fmt.Errorf("%s: %s", violation.Code, violation.What))
	}
	return errors.Join(defects...)
}

func (m *memory) projectionWith(r *record.Record, body []byte) (*resolve.Projection, error) {
	files := make(store.MapSource, len(m.set.Bytes)+1)
	maps.Copy(files, m.set.Bytes)
	b, err := record.Marshal(r)
	if err != nil {
		return nil, err
	}
	files[r.Path()] = b
	if r.Kind == record.Document && body != nil {
		if _, exists := files[r.BodyPath()]; exists {
			return nil, fmt.Errorf("%w: %s", store.ErrExists, r.BodyPath())
		}
		files[r.BodyPath()] = body
	}
	set, err := store.Load(context.Background(), files)
	if err != nil {
		return nil, err
	}
	priorProblems := map[string]bool{}
	for _, problem := range m.set.Problems {
		priorProblems[problem.Path+"\x00"+problem.Err.Error()] = true
	}
	for _, problem := range set.Problems {
		if !priorProblems[problem.Path+"\x00"+problem.Err.Error()] {
			return nil, fmt.Errorf("record-invalid: %s: %w", problem.Path, problem.Err)
		}
	}
	if set.Get(r.ID) == nil {
		return nil, fmt.Errorf("record-invalid: candidate %s is absent from its trial projection", r.ID)
	}
	return resolve.New(set), nil
}

// firstNewProjectionDefect compares structural identities, not aggregate
// counts: repairing one old defect cannot hide a different defect introduced
// by the new act.
func firstNewProjectionDefect(before, after *resolve.Projection) string {
	old := projectionDefects(before)
	current := projectionDefects(after)
	var keys []string
	for key, defect := range current {
		if !projectionDefectCovered(old, defect) {
			keys = append(keys, key)
		}
	}
	if len(keys) == 0 {
		return ""
	}
	sort.Strings(keys)
	return current[keys[0]].what
}

type projectionDefect struct {
	key     string
	kind    string
	anchor  string
	members []string
	what    string
}

func projectionDefects(proj *resolve.Projection) map[string]projectionDefect {
	out := map[string]projectionDefect{}
	for _, dangling := range proj.Set.Dangling {
		key := strings.Join([]string{"dangling-target", dangling.From, dangling.Type, dangling.Target}, "\x00")
		out[key] = projectionDefect{key: key, kind: "dangling-target", anchor: key,
			what: fmt.Sprintf("dangling-target: %s %s %s, which does not exist", dangling.From, dangling.Type, dangling.Target)}
	}
	for _, id := range proj.Set.IDs() {
		r := proj.Set.Records[id]
		if r.Kind != record.Document || proj.Status(id) != resolve.Active {
			continue
		}
		for _, ref := range r.Embeds {
			subject, family, _ := strings.Cut(ref, "#")
			if proj.Head(subject, family) == nil {
				key := "embed-unresolved\x00" + id + "\x00" + ref
				out[key] = projectionDefect{key: key, kind: "embed-unresolved", anchor: key,
					what: fmt.Sprintf("embed-unresolved: %s embeds {{%s}}, which has no active head", id, ref)}
			}
		}
	}
	for _, v := range proj.Violations {
		members := append([]string(nil), v.Related...)
		sort.Strings(members)
		anchor := strings.Join([]string{v.Code, v.Record, v.Target}, "\x00")
		key := strings.Join([]string{"violation", anchor, strings.Join(members, ",")}, "\x00")
		out[key] = projectionDefect{key: key, kind: "violation", anchor: anchor, members: members, what: v.Code + ": " + v.What}
	}
	for _, c := range proj.Conflicts() {
		key := strings.Join([]string{"family-conflict", c.Subject, c.Family, strings.Join(c.IDs, ",")}, "\x00")
		out[key] = projectionDefect{key: key, kind: "family-conflict", anchor: c.Subject + "\x00" + c.Family, members: append([]string(nil), c.IDs...), what: fmt.Sprintf("conflict-at-admission: %s#%s would have %d active claims (%s)", c.Subject, c.Family, len(c.IDs), strings.Join(c.IDs, ", "))}
	}
	for _, c := range proj.AnswerConflicts() {
		key := strings.Join([]string{"answer-conflict", c.Question, strings.Join(c.IDs, ",")}, "\x00")
		out[key] = projectionDefect{key: key, kind: "answer-conflict", anchor: c.Question, members: append([]string(nil), c.IDs...), what: fmt.Sprintf("answer-conflict-at-admission: %s would have %d active answers (%s)", c.Question, len(c.IDs), strings.Join(c.IDs, ", "))}
	}
	for _, c := range proj.RevocationConflicts() {
		key := strings.Join([]string{"revocation-conflict", c.Target, strings.Join(c.IDs, ",")}, "\x00")
		out[key] = projectionDefect{key: key, kind: "revocation-conflict", anchor: c.Target, members: append([]string(nil), c.IDs...), what: fmt.Sprintf("revocation-conflict-at-admission: %s would have %d active revocations (%s)", c.Target, len(c.IDs), strings.Join(c.IDs, ", "))}
	}
	return out
}

func projectionDefectCovered(before map[string]projectionDefect, after projectionDefect) bool {
	if _, existed := before[after.key]; existed {
		return true
	}
	if len(after.members) == 0 {
		return false
	}
	for _, prior := range before {
		if prior.kind == after.kind && prior.anchor == after.anchor && strictProjectionSubset(after.members, prior.members) {
			return true
		}
	}
	return false
}

func strictProjectionSubset(candidate, prior []string) bool {
	if len(candidate) >= len(prior) {
		return false
	}
	want := make(map[string]bool, len(prior))
	for _, value := range prior {
		want[value] = true
	}
	for _, value := range candidate {
		if !want[value] {
			return false
		}
	}
	return true
}

// ---------------------------------------------------------------- revise / render

func runRevise(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("revise", env)
	bodyFile := fs.String("body-file", "", "the markdown file holding the next narrative")
	title := fs.String("statement", "", "a new title (default: the current one)")
	intent := fs.String("intent", "", "what this revision is for")
	rationale := fs.String("rationale", "", "why")
	actorFlag := fs.String("actor", "", "who revises")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 1 || *bodyFile == "" || !record.ValidRef(pos[0]) {
		return exitUsage, usage("revise <subject#slot> --body-file doc.md [--statement title] [--intent …] [--rationale …]")
	}
	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	head, err := m.resolveRef(pos[0])
	if err != nil {
		return exitUsage, usage("%v", err)
	}
	if head.Kind != record.Document {
		return exitUsage, usage("%s is a %s, not a Document", pos[0], head.Kind)
	}
	actor, err := m.actor(ctx, *actorFlag, env)
	if err != nil {
		return exitUsage, err
	}
	if *title == "" {
		*title = head.Statement
	}
	in := addInput{Kind: record.Document, Subject: head.Subject, Family: head.Family, Statement: *title, Intent: *intent, Rationale: *rationale,
		Supersedes: []string{head.ID}, BodyFile: *bodyFile, Tags: head.Tags, LiteralBody: head.LiteralBody}
	if head.LiteralBody {
		// A literal Document remains the lossless view of the same imported
		// source path. PR/commit/note describe the old record and must not be
		// misattributed to its new revision.
		in.Source = head.Provenance.Source
	}
	body, err := readBody(in, env)
	if err != nil {
		return exitUsage, err
	}
	r, err := m.build(in, actor, env, body)
	if err != nil {
		return exitOK, err
	}
	return m.store(ctx, r, body, env)
}

func runRender(ctx context.Context, args []string, env Env) error {
	fs := newFlags("render", env)
	slot := fs.String("slot", "", "the document slot (default: prd for prd/…, design for design/…)")
	at := fs.String("at", "", "a git ref: render the document as the records stood there")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || !record.ValidSubject(pos[0]) {
		return usage("render <subject> [--slot prd|design] [--at ref]")
	}
	subject := pos[0]
	if *slot == "" {
		*slot = record.Namespace(subject)
	}
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	proj, source := m.proj, "the working tree"
	if *at != "" {
		if err := m.needGit(); err != nil {
			return err
		}
		if !m.repo.RefExists(ctx, *at) {
			return fmt.Errorf("ref %q does not exist", *at)
		}
		then, err := m.baseSet(ctx, *at)
		if err != nil {
			return err
		}
		proj, source = resolve.New(then), *at
	}
	rendered, err := view.Render(proj, subject, *slot, source)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(env.Stdout, rendered); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

func clip(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

// ---------------------------------------------------------------- revoke

func runRevoke(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("revoke", env)
	reason := fs.String("reason", "", "why the record is withdrawn")
	actorFlag := fs.String("actor", "", "who revokes")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 1 || *reason == "" {
		return exitUsage, usage("revoke <id> --reason \"…\"")
	}
	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	target, err := m.resolveRef(pos[0])
	if err != nil {
		return exitUsage, usage("%v", err)
	}
	if st := m.proj.Status(target.ID); st != resolve.Active {
		return exitRefused, fmt.Errorf("%s is already %s", target.ID, st)
	}
	actor, err := m.actor(ctx, *actorFlag, env)
	if err != nil {
		return exitUsage, err
	}
	id, err := m.newID(record.Revocation, env)
	if err != nil {
		return exitRefused, err
	}
	r := &record.Record{Schema: record.Schema, ID: id, Kind: record.Revocation, Subject: target.Subject, Statement: *reason, Issuer: actor,
		CreatedAt: env.Now().UTC().Format(time.RFC3339), Relations: []record.Relation{{Type: record.Revokes, Target: target.ID}}}
	if err := m.admit(r, nil); err != nil {
		return exitRefused, err
	}
	path, err := store.Write(m.dir, r)
	if err != nil {
		return exitRefused, err
	}
	if _, err := fmt.Fprintf(env.Stdout, "%s\n%s\n", r.ID, path); err != nil {
		return exitRefused, fmt.Errorf("write command output: %w", err)
	}
	if deps := m.proj.Dependents(target.ID); len(deps) > 0 {
		if _, err := fmt.Fprintf(env.Stdout, "reconsider — these rest on %s:\n", target.ID); err != nil {
			return exitRefused, fmt.Errorf("write command output: %w", err)
		}
		for _, d := range deps {
			if _, err := fmt.Fprintf(env.Stdout, "  %s %s#%s (%s): supersede it with a new basis, or revoke it\n", d.Type, d.Record.Subject, d.Record.Family, d.Record.ID); err != nil {
				return exitRefused, fmt.Errorf("write command output: %w", err)
			}
		}
	}
	return exitOK, nil
}

// ---------------------------------------------------------------- ask / answer

func runAsk(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("ask", env)
	subject := fs.String("subject", "", "the subject the question is about")
	text := fs.String("text", "", "the question")
	anyway := fs.Bool("anyway", false, "record the question even if a similar one exists")
	actorFlag := fs.String("actor", "", "who asks")
	if _, err := parse(fs, args); err != nil {
		return exitUsage, err
	}
	if *subject == "" || *text == "" {
		return exitUsage, usage("ask --subject S --text \"…\"")
	}
	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	if match := ask.Find(m.proj, *subject, *text, m.pol.Ask.Similarity); match != nil && !*anyway {
		if match.Answer != nil {
			if _, err := fmt.Fprintf(env.Stdout, "answered already (%s, %s, similarity %.2f): %s\nanswer (%s, %s): %s\n", match.Question.ID, match.Question.CreatedAt[:10], match.Similarity,
				match.Question.Statement, match.Answer.Issuer, match.Answer.CreatedAt[:10], match.Answer.Statement); err != nil {
				return exitRefused, fmt.Errorf("write command output: %w", err)
			}
			return exitOK, nil
		}
		if _, err := fmt.Fprintf(env.Stdout, "asked already and still open (%s, %s, similarity %.2f): %s\nwait for its answer, or pass --anyway\n", match.Question.ID, match.Question.CreatedAt[:10],
			match.Similarity, match.Question.Statement); err != nil {
			return exitRefused, fmt.Errorf("write command output: %w", err)
		}
		return exitOK, nil
	}
	actor, err := m.actor(ctx, *actorFlag, env)
	if err != nil {
		return exitUsage, err
	}
	id, err := m.newID(record.Question, env)
	if err != nil {
		return exitRefused, err
	}
	r := &record.Record{Schema: record.Schema, ID: id, Kind: record.Question, Subject: *subject, Statement: *text, Issuer: actor, CreatedAt: env.Now().UTC().Format(time.RFC3339)}
	if err := r.Validate(); err != nil {
		return exitUsage, usage("%v", err)
	}
	if err := m.admit(r, nil); err != nil {
		return exitRefused, err
	}
	path, err := store.Write(m.dir, r)
	if err != nil {
		return exitRefused, err
	}
	if _, err := fmt.Fprintf(env.Stdout, "new question %s\n%s\n", r.ID, path); err != nil {
		return exitRefused, fmt.Errorf("write command output: %w", err)
	}
	return exitOK, nil
}

func runAnswer(ctx context.Context, args []string, env Env) (int, error) {
	fs := newFlags("answer", env)
	text := fs.String("text", "", "the answer")
	replace := fs.Bool("replace", false, "correct the current answer: the new one supersedes it")
	actorFlag := fs.String("actor", "", "who answers")
	pos, err := parse(fs, args)
	if err != nil {
		return exitUsage, err
	}
	if len(pos) != 1 || *text == "" {
		return exitUsage, usage("answer <question-id> --text \"…\" [--replace]")
	}
	m, release, err := openLocked(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	defer release()
	q := m.set.Get(pos[0])
	if q == nil || q.Kind != record.Question {
		return exitUsage, usage("%s is not a question", pos[0])
	}
	if st := m.proj.Status(q.ID); st != resolve.Active {
		return exitRefused, fmt.Errorf("%s is %s; an answer needs an open question", q.ID, st)
	}
	current := m.proj.ActiveAnswers(q.ID)
	if len(current) > 0 && !*replace {
		return exitRefused, fmt.Errorf("%s already has an answer, %s (%s, %s): %s — pass --replace to correct it, which supersedes it", q.ID, current[len(current)-1].ID,
			current[len(current)-1].Issuer, current[len(current)-1].CreatedAt[:10], clip(current[len(current)-1].Statement, 80))
	}
	if len(current) == 0 && *replace {
		return exitRefused, fmt.Errorf("%s has no active answer to replace", q.ID)
	}
	actor, err := m.actor(ctx, *actorFlag, env)
	if err != nil {
		return exitUsage, err
	}
	id, err := m.newID(record.Answer, env)
	if err != nil {
		return exitRefused, err
	}
	rels := []record.Relation{{Type: record.Answers, Target: q.ID}}
	for _, a := range current {
		rels = append(rels, record.Relation{Type: record.Supersedes, Target: a.ID})
	}
	r := &record.Record{Schema: record.Schema, ID: id, Kind: record.Answer, Subject: q.Subject, Statement: *text, Issuer: actor,
		CreatedAt: env.Now().UTC().Format(time.RFC3339), Relations: rels}
	if err := r.Validate(); err != nil {
		return exitUsage, usage("%v", err)
	}
	return m.store(ctx, r, nil, env)
}

// ---------------------------------------------------------------- views

func runView(ctx context.Context, args []string, env Env) error {
	fs := newFlags("view", env)
	since := fs.String("since", "", "a git ref: mark what is new relative to it")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usage("view <subject> [--since ref]")
	}
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	subject := pos[0]
	if err := view.ValidateDocumentFiles(m.proj, subject, ""); err != nil {
		return fmt.Errorf("cannot view %s: %w", subject, err)
	}
	if len(m.set.BySubject[subject]) == 0 {
		return fmt.Errorf("no records on %s", subject)
	}
	var newIDs map[string]bool
	if *since != "" {
		if err := m.needGit(); err != nil {
			return err
		}
		if !m.repo.RefExists(ctx, *since) {
			return fmt.Errorf("ref %q does not exist", *since)
		}
		then, err := m.baseSet(ctx, *since)
		if err != nil {
			return err
		}
		newIDs = map[string]bool{}
		for id := range m.set.Records {
			if _, ok := then.Records[id]; !ok {
				newIDs[id] = true
			}
		}
	}
	if record.Namespace(subject) == "design" {
		if _, err := fmt.Fprint(env.Stdout, view.Design(m.proj, subject)); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		return nil
	}
	if _, err := fmt.Fprint(env.Stdout, view.PRD(m.proj, subject, newIDs)); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

func runWhy(ctx context.Context, args []string, env Env) error {
	if len(args) != 1 {
		return usage("why <id | subject#family>")
	}
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	r, err := m.resolveRef(args[0])
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(env.Stdout, view.Why(m.proj, r.ID)); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

func runDelivery(ctx context.Context, args []string, env Env) error {
	if len(args) != 1 {
		return usage("delivery <subject>")
	}
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(env.Stdout, view.Delivery(m.proj, args[0])); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

func runStatus(ctx context.Context, args []string, env Env) error {
	if len(args) != 0 {
		return usage("status takes no arguments")
	}
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprint(env.Stdout, view.Status(m.proj, env.Now())); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------- search / collide

func runSearch(ctx context.Context, args []string, env Env) error {
	fs := newFlags("search", env)
	var tags, code multi
	subject := fs.String("subject", "", "exact subject, or a namespace prefix ending in /")
	kind := fs.String("kind", "", "record kind")
	fs.Var(&tags, "tag", "tag to boost (repeatable)")
	fs.Var(&code, "code", "code path to boost (repeatable)")
	k := fs.Int("k", 10, "how many")
	all := fs.Bool("all", false, "include superseded, revoked, and historical records")
	pos, err := parse(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		return usage("search \"text\" [flags]")
	}
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	var inflight map[string]*store.Set
	if base := m.mainline(ctx); base.oid != "" {
		inflight, err = m.inFlight(ctx, base)
		if err != nil {
			return err
		}
	}
	ix := search.Build(m.docs(inflight))
	hits := ix.Search(search.Query{Text: pos[0], Subject: *subject, Kind: *kind, Tags: tags, Code: code, K: *k, IncludeInactive: *all})
	if len(hits) == 0 {
		if _, err := fmt.Fprintln(env.Stdout, "nothing matches"); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		return nil
	}
	for _, h := range hits {
		where := h.Doc.Subject
		if h.Doc.Family != "" {
			where += "#" + h.Doc.Family
		}
		status := h.Doc.Status
		if h.Doc.InFlight != "" {
			status = "in flight on " + h.Doc.InFlight
		}
		if _, err := fmt.Fprintf(env.Stdout, "%2d %5.2f %s %s [%s] %s\n", h.Rank, h.Score, h.Doc.ID, where, status, clip(h.Doc.Statement, 110)); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		if len(h.Boosts) > 0 {
			if _, err := fmt.Fprintf(env.Stdout, "         boosts: %s\n", strings.Join(h.Boosts, ", ")); err != nil {
				return fmt.Errorf("write command output: %w", err)
			}
		}
	}
	return nil
}

type collideOutput struct {
	Draft      search.Draft       `json:"draft"`
	Judge      string             `json:"judge"`
	Candidates []search.Candidate `json:"candidates"`
	Verdicts   []search.Verdict   `json:"verdicts"`
	Supersedes []string           `json:"supersedes"`
}

func runCollide(ctx context.Context, args []string, env Env) error {
	fs := newFlags("collide", env)
	var d search.Draft
	var tags, code multi
	fs.StringVar(&d.Statement, "statement", "", "the draft statement")
	fs.StringVar(&d.Subject, "subject", "", "its subject")
	fs.StringVar(&d.Family, "family", "", "its family")
	fs.StringVar(&d.Kind, "kind", "", "its kind")
	fs.StringVar(&d.Intent, "intent", "", "its intent")
	fs.StringVar(&d.Rationale, "rationale", "", "its rationale")
	fs.Var(&tags, "tag", "tag (repeatable)")
	fs.Var(&code, "code", "code path it touches (repeatable)")
	k := fs.Int("k", 10, "how many candidates")
	judge := fs.String("judge-cmd", "", "command that judges the candidates (reads JSON on stdin, writes JSON verdicts)")
	asJSON := fs.Bool("json", false, "machine output")
	noInflight := fs.Bool("no-inflight", false, "ignore other branches")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if d.Statement == "" {
		return usage("collide --statement \"…\" [flags]")
	}
	d.Tags, d.Code = tags, code
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	var inflight map[string]*store.Set
	if base := m.mainline(ctx); base.oid != "" && !*noInflight {
		inflight, err = m.inFlight(ctx, base)
		if err != nil {
			return err
		}
	}
	ix := search.Build(m.docs(inflight))
	out := collideOutput{Draft: d, Judge: "heuristic", Candidates: search.Candidates(ix, d, *k)}
	if *judge != "" && len(out.Candidates) > 0 {
		res, err := search.RunJudge(ctx, *judge, search.JudgeInput{Draft: d, Candidates: out.Candidates})
		if err != nil {
			return err
		}
		out.Judge, out.Verdicts = *judge, res.Verdicts
	} else {
		out.Verdicts = search.Heuristic(d, out.Candidates)
	}
	for _, v := range out.Verdicts {
		if v.Relation == search.RelSupersedes {
			out.Supersedes = append(out.Supersedes, v.ID)
		}
	}
	if *asJSON {
		enc := json.NewEncoder(env.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	if len(out.Candidates) == 0 {
		if _, err := fmt.Fprintln(env.Stdout, "no candidates: nothing in the memory shares words, subject, tags or code with the draft"); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		return nil
	}
	verdict := map[string]search.Verdict{}
	for _, v := range out.Verdicts {
		verdict[v.ID] = v
	}
	if _, err := fmt.Fprintf(env.Stdout, "candidates (judge: %s)\n", out.Judge); err != nil {
		return fmt.Errorf("write command output: %w", err)
	}
	for _, c := range out.Candidates {
		where := c.Subject
		if c.Family != "" {
			where += "#" + c.Family
		}
		status := c.Status
		if c.InFlight != "" {
			status = "in flight on " + c.InFlight
		}
		v := verdict[c.ID]
		rel := v.Relation
		if rel == "" {
			rel = "-"
		}
		if _, err := fmt.Fprintf(env.Stdout, "%2d %5.2f %-10s %s %s [%s] %s\n", c.Rank, c.Score, rel, c.ID, where, status, clip(c.Statement, 90)); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
		if v.Why != "" {
			if _, err := fmt.Fprintf(env.Stdout, "         %s\n", v.Why); err != nil {
				return fmt.Errorf("write command output: %w", err)
			}
		}
	}
	if len(out.Supersedes) > 0 {
		if _, err := fmt.Fprintf(env.Stdout, "suggested: --supersedes %s\n", strings.Join(out.Supersedes, " --supersedes ")); err != nil {
			return fmt.Errorf("write command output: %w", err)
		}
	}
	return nil
}

// ---------------------------------------------------------------- pack

func runPack(ctx context.Context, args []string, env Env) error {
	fs := newFlags("pack", env)
	var subjects, families, code multi
	fs.Var(&subjects, "subject", "subject in scope (repeatable)")
	fs.Var(&families, "family", "family in scope (repeatable; F or subject#F)")
	fs.Var(&code, "code", "explicit repository-relative task path (repeatable)")
	budget := fs.Int("budget", 0, "token budget (default from policy)")
	asJSON := fs.Bool("json", false, "machine output")
	if _, err := parse(fs, args); err != nil {
		return err
	}
	if len(subjects) == 0 && len(code) == 0 {
		return usage("pack --subject S [--family F] [--code path] [--budget N], or pack --code path")
	}
	m, err := open(ctx, env)
	if err != nil {
		return err
	}
	for _, s := range subjects {
		if !record.ValidSubject(s) {
			return usage("subject %q is not namespace/name", s)
		}
	}
	var inflight map[string]*store.Set
	if base := m.mainline(ctx); base.oid != "" {
		inflight, err = m.inFlight(ctx, base)
		if err != nil {
			return err
		}
	}
	if *budget < 0 {
		return usage("--budget %d: a pack needs a positive token budget (0 means the policy default)", *budget)
	}
	if *budget == 0 {
		*budget = m.pol.Pack.DefaultBudgetTokens
	}
	// the budget bounds what is delivered, so the bytes measured are the
	// bytes written: one serializer for both, newline included
	serialize := func(pk *pack.Pack) []byte { return []byte(pk.Render()) }
	if *asJSON {
		serialize = func(pk *pack.Pack) []byte {
			b, err := json.MarshalIndent(pk, "", "  ")
			if err != nil {
				return nil
			}
			return append(b, '\n')
		}
	}
	pk, err := pack.Compile(m.proj, inflight, pack.Scope{Subjects: subjects, Families: families, Code: code}, *budget, func(pk *pack.Pack) int { return record.Tokens(string(serialize(pk))) })
	if err != nil {
		return err
	}
	if _, err := env.Stdout.Write(serialize(pk)); err != nil {
		return fmt.Errorf("write the pack: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------- packet / check

func runPacket(ctx context.Context, args []string, env Env, check bool) (int, error) {
	verb := "packet"
	if check {
		verb = "check"
	}
	fs := newFlags(verb, env)
	baseFlag := fs.String("base", "", "base ref; required for check (in CI the pull request's base), defaults to the configured mainline for packet")
	body := fs.Bool("body", false, "print the pull request body")
	description := fs.String("description", "", "title for the pull request body")
	if _, err := parse(fs, args); err != nil {
		return exitUsage, err
	}
	m, err := open(ctx, env)
	if err != nil {
		return exitRefused, err
	}
	base, err := m.base(ctx, *baseFlag, check)
	if err != nil {
		return exitRefused, err
	}
	baseSet, err := m.baseSet(ctx, base.oid)
	if err != nil {
		return exitRefused, err
	}
	basePolicy, err := policy.Parse(baseSet.Policy)
	if err != nil {
		return exitRefused, fmt.Errorf("the base's %w", err)
	}
	var ancestor *store.Set
	if mb, err := m.repo.MergeBase(ctx, base.oid, "HEAD"); err == nil {
		if ancestor, err = m.baseSet(ctx, mb); err != nil {
			return exitRefused, err
		}
	}
	inflight, err := m.inFlight(ctx, base)
	if err != nil {
		return exitRefused, err
	}
	branch, err := m.repo.CurrentBranch(ctx)
	if err != nil {
		return exitRefused, err
	}
	p := packet.Compute(packet.Input{Base: baseSet, Ancestor: ancestor, Head: m.set, InFlight: inflight, Policy: m.pol, BasePolicy: basePolicy, Branch: branch,
		BaseRef: base.String(), Description: *description})
	if *body {
		if _, err := fmt.Fprint(env.Stdout, p.Render()); err != nil {
			return exitRefused, fmt.Errorf("write command output: %w", err)
		}
	} else {
		if _, err := fmt.Fprint(env.Stdout, p.Summary()); err != nil {
			return exitRefused, fmt.Errorf("write command output: %w", err)
		}
	}
	if check && !p.OK() {
		return exitRefused, fmt.Errorf("%d blocker(s)", len(p.Blockers))
	}
	if check {
		report := m.evidenceCheck(ctx, base.oid)
		return emitEvidence(env, report, false)
	}
	return exitOK, nil
}
