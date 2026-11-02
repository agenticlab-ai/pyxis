// Distributed check registration (the parallel-authorship fix). A central
// append-list — the Gate() enumeration, the CLI switch, the help block — is a
// merge magnet: every "add a capability" task edits the same lines, so N
// parallel branches hit O(N^2) keep-both conflicts against a moving main. The
// cure is structural: each check contributes ITSELF from its own file via
// init() -> Register, and the umbrella / CLI / help are ASSEMBLED from the
// registry, sorted by verb. Adding a check touches exactly one new file and
// zero shared lines; two checks never share a line to conflict on (and, living
// in separate files, never interleave). Determinism is preserved by sorting on
// the verb, never by relying on init() order.
package gate

import (
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Defects is the check result type, aliased into the gate package so a check
// file needs nothing but `package gate` + its init() — no per-file imports to
// drift or conflict on.
type Defects = []schema.Defect

// Ctx bundles everything a repo-wide check may need behind ONE type, so every
// check shares a single Run signature — the uniformity that makes a registry
// (and thus conflict-free contribution) possible despite heterogeneous needs.
// A check reads only the fields it wants; MergeBase is "" at the desk.
// Head is the diff span's end: "" means the working tree (the desk and CI
// default), a commit sha means the committed range <MergeBase>..<Head> — the
// pre-push verb sets it to each pushed sha so history checks judge exactly
// the pushed content, never another session's uncommitted tree state (#357).
type Ctx struct {
	Root      string
	MergeBase string
	Head      string
	Cfg       config.Config
	// Scan is the install-epoch scan-set filter (existing-repo-onboarding-v2
	// §3.4): computed ONCE per gate run and shared by every tree-local check,
	// so an adopted repository's pre-existing corpus is grandfathered and gate
	// cost scales with the changed set, not the corpus (NFR-1). Its zero value
	// puts every path in scope — the un-adopted / self-host case, byte-for-byte
	// the behaviour before adoption existed.
	Scan ScanScope
}

// ScanScope is the tree-local scan-set filter the install epoch defines
// (existing-repo-onboarding-v2 §3.4, DINV-6). When an epoch is set and
// reachable, only content changed since it (`git diff --name-only <epoch>`,
// which spans the working tree too) is judged; a repository with no epoch — or
// whose gate run never set one — leaves every path in scope. The changed set is
// carried, not recomputed, so the one `git diff` a run makes is shared by every
// check that consults it.
type ScanScope struct {
	// epochActive is true only when an epoch is set AND reachable. It is
	// deliberately false on an unreachable epoch, so the run FAILS CLOSED —
	// judging the whole corpus rather than silently grandfathering it (ABUSE-2);
	// the caller that computed the scope names that cause separately.
	epochActive bool
	// changed is the set of repository-relative (slash) paths that differ from
	// the epoch commit, populated only when epochActive.
	changed map[string]bool
}

// InScope reports whether a repository-relative (slash) path is judged this
// run: always true with no active epoch, else true only for a path changed
// since the epoch.
func (s ScanScope) InScope(rel string) bool {
	return !s.epochActive || s.changed[rel]
}

// Check is one repo-wide gate verb, contributed by its own file. Gate() and the
// CLI both dispatch through the registry, so they can never drift and neither
// carries a per-check line to conflict on.
type Check struct {
	Verb             string // unique; the deterministic sort key
	Help             string // one-line usage, assembled into the help block
	NeedsMergeBase   bool   // true -> Gate() skips it with no base; the CLI demands the flag
	RunOnPyxisSource bool   // retain this Argus integrity check while legacy document rules move to memory
	Run              func(c Ctx) Defects
}

// registry is written only from package init() (one Register per check file),
// so it is effectively sealed once the program is running. Reads go through the
// sorted accessors below, never the map, so output never depends on map order.
var registry = map[string]Check{}

// Register contributes a check. Called from a check file's init(). Verbs are
// unique by construction (one check per file); a duplicate verb is a program
// bug the tests catch, not a silent overwrite in production.
func Register(c Check) { registry[c.Verb] = c }

// Checks returns every registered check sorted by verb — the stable order that
// makes gate output deterministic regardless of init() evaluation order.
func Checks() []Check {
	out := make([]Check, 0, len(registry))
	for _, c := range registry {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Verb < out[j].Verb })
	return out
}

// Lookup finds a check by verb for individual CLI dispatch (every check stays
// runnable on its own, exactly as before).
func Lookup(verb string) (Check, bool) { c, ok := registry[verb]; return c, ok }
