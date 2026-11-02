// Distributed check registration for the plan gate — plangate's half of the
// parallel-authorship fix (docs/tech-design/distributed-check-registration.md).
// Validate()'s list of checkX(...) calls was the merge magnet every new plan
// rule appended to; here each rule contributes ITSELF from its own file via
// init() -> RegisterTask/RegisterDoc, and Validate ASSEMBLES them, sorted by
// name — never init() order. The graph/governing/corpus setup and the
// header/bucket frame stay inline in Validate; only the rule enumeration is
// distributed. Adding a rule is one new file, zero shared lines. plangate
// carries its OWN registry with a gate-specific context (Env), per the design's
// Q3 — the pattern is identical to docgate's even though the context differs.
package plan

import (
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
)

// Bad is the defect sink every check writes through — Validate owns the closure
// (it captures the source file and appends), the checks just call it.
type Bad = func(op, reason, remedy string)

// Env bundles everything a plan check may read behind one type: the resolved
// governing surface, the transitive reach graph, the loaded design body and
// corpus, plus root/cfg/doc. A check reads only the fields it needs.
type Env struct {
	Root       string
	Cfg        config.Config
	Doc        *Doc
	Reach      map[string]map[string]bool
	Gov        governing
	DesignBody []byte
	Corpus     map[string]bool
	Locked     *lockedInputs
}

// TaskCheck is one per-task rule, contributed by its own file; Validate runs it
// once per task. DocCheck is one whole-doc rule, run once. Both dispatch through
// the sorted registries below, so neither carries a line in Validate to conflict
// on, and two rules in two files never interleave.
type TaskCheck struct {
	Name string // unique; the deterministic sort key
	Run  func(e *Env, t *Task, bad Bad)
}

type DocCheck struct {
	Name string
	Run  func(e *Env, bad Bad)
}

// The registries are written only from package init() (one Register per check
// file), so they are effectively sealed once running; reads go through the
// sorted accessors, so the verdict order never depends on init() order.
var (
	taskChecks []TaskCheck
	docChecks  []DocCheck
)

// RegisterTask / RegisterDoc contribute a check from its file's init().
func RegisterTask(c TaskCheck) { taskChecks = append(taskChecks, c) }
func RegisterDoc(c DocCheck)   { docChecks = append(docChecks, c) }

// TaskChecks / DocChecks return the registered checks sorted by name — the
// stable order that makes the gate verdict deterministic regardless of init()
// evaluation order (design DINV-1).
func TaskChecks() []TaskCheck {
	out := append([]TaskCheck(nil), taskChecks...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func DocChecks() []DocCheck {
	out := append([]DocCheck(nil), docChecks...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}
