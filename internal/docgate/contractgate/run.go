package contractgate

import (
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Run resolves feature's assembled contract read-only from root's committed
// machine faces and returns one closed Verdict for the DEFAULT trigger
// (planning-entry — every class blocking, T5's behavior). It is the back-compat
// entry; RunTrigger takes an explicit trigger for the continuous surface.
func Run(root, feature string) Verdict {
	return RunTrigger(root, feature, TriggerPlanningEntry)
}

// RunTrigger resolves feature's assembled contract read-only from root's
// committed machine faces and returns one closed Verdict judged under the given
// trigger's severity split (design §3.1, §3.10). It runs the contract-scope check
// set and records the identity of every input it evaluated (AP-13). Each failed
// check's defects are dispositioned blocking or advisory by the per-trigger
// severity map (FR-13): a blocking defect fails the run, an advisory one is
// surfaced but does not (FR-9). On any fail-closed edge — a missing/unreadable
// input or an unmerged contract (configuration-error), a malformed machine face
// (error), a malformed severity-map policy (configuration-error, UB-6) — it
// returns that class with zero checks run and Blocking set, never `clean` (design
// §3.11, DINV-1/DINV-5). It loads its own config + severity map from root; makes
// zero mutations on every path. A MISSING severity map uses the built-in default
// split (UB-6).
func RunTrigger(root, feature string, trig Trigger) Verdict {
	cfg, cdef := config.Load(root)
	if len(cdef) > 0 {
		// A malformed .pyxis/config.yaml means the gate cannot resolve where the
		// contract lives — a configuration error, fail closed with the config
		// defects named (design §3.11).
		v := newVerdict(feature)
		v.Trigger = trig
		v.Class = ClassConfigurationError
		v.Blocking = true // a fail-closed edge always stops the run
		v.Reason = "the pyxis config does not parse: " + cdef[0].Reason
		v.CheckResults = resultsForDefects(cdef)
		return v
	}
	// The severity-map policy: absent → the built-in default split (UB-6);
	// malformed → a configuration-error verdict naming the policy defect with zero
	// checks run (UB-6, the fail-closed edge). Load it BEFORE resolving so a bad
	// policy fails closed regardless of the contract's state.
	sev, sdef := LoadSeverityMap(root)
	if len(sdef) > 0 {
		v := newVerdict(feature)
		v.Trigger = trig
		v.Class = ClassConfigurationError
		v.Blocking = true
		v.Reason = "the severity-map policy does not parse: " + sdef[0].Reason
		v.CheckResults = resultsForDefects(sdef)
		return v
	}
	return runResolved(root, feature, trig, cfg, sev)
}

// RunWithConfig is Run with an explicit config for the DEFAULT trigger
// (planning-entry) and the built-in default severity split — the in-process test
// seam (design §3.11: the machine faces are local-substitutable; a fixture repo
// supplies a Defaults()-shaped config). Preserved as the T2–T5 signature.
func RunWithConfig(root, feature string, cfg config.Config) Verdict {
	return runResolved(root, feature, TriggerPlanningEntry, cfg, defaultSeverityMap())
}

// RunWithConfigTrigger is RunWithConfig with an explicit trigger and severity map
// — the in-process seam for the split tests (a fixture repo + a chosen trigger +
// a chosen policy, no file read needed).
func RunWithConfigTrigger(root, feature string, trig Trigger, cfg config.Config, sev SeverityMap) Verdict {
	return runResolved(root, feature, trig, cfg, sev)
}

// runResolved is the shared body: resolve the contract, run the check subset, and
// disposition each failed check by the severity map for the active trigger. A
// fail-closed resolution edge returns its verdict unchanged (Blocking already set
// on those paths). The overall Class is the most-severe FAILING class (blocking
// or advisory alike — it names what is wrong); Blocking is true iff any failed
// check is blocking at this trigger (it drives the exit code).
func runResolved(root, feature string, trig Trigger, cfg config.Config, sev SeverityMap) Verdict {
	c, verr := resolve(root, feature, cfg)
	if verr != nil {
		verr.Trigger = trig
		verr.Blocking = true // a fail-closed verdict always stops the run
		return *verr
	}

	v := newVerdict(feature)
	v.Trigger = trig
	v.Evaluated = evaluatedIdentities(c)

	// Assemble and run the contract-scope check subset — the self-registered
	// checks in name order (design §3.9; the DCR registry), feature-scoped over
	// the one resolved contract. Each contributes a CheckResult; a failing check
	// contributes its declared verdict class and its per-trigger severity, and the
	// overall class is the most severe FAILING class (classRank), or clean when
	// every check passes. Blocking accrues only from checks the severity map marks
	// blocking at this trigger (FR-9/FR-13/§3.10).
	worst := ClassClean
	var blockReasons, advisoryReasons []string
	for _, chk := range contractScopeChecks() {
		defects := chk.Run(c)
		passed := len(defects) == 0
		res := CheckResult{Check: chk.Name, Passed: passed}
		if passed {
			res.Class = ClassClean
		} else {
			res.Class = chk.Class
			res.Severity = sev.Severity(trig, chk.SeverityClass)
			res.Defects = defects
			if classRank[chk.Class] > classRank[worst] {
				worst = chk.Class
			}
			if res.Severity == SeverityBlocking {
				v.Blocking = true
				blockReasons = append(blockReasons, chk.Name)
			} else {
				advisoryReasons = append(advisoryReasons, chk.Name)
			}
		}
		v.CheckResults = append(v.CheckResults, res)
	}
	v.Class = worst
	if worst != ClassClean {
		sort.Strings(blockReasons)
		sort.Strings(advisoryReasons)
		v.Reason = severitySplitReason(blockReasons, advisoryReasons)
	}
	return v
}

// severitySplitReason phrases the verdict's reason line, distinguishing the
// checks that BLOCK the run from those merely surfaced as advisory (FR-9). A
// reader sees at a glance which defects stop planning and which are informational.
func severitySplitReason(blocking, advisory []string) string {
	var parts []string
	if len(blocking) > 0 {
		parts = append(parts, "blocking contract-scope checks failed: "+joinComma(blocking))
	}
	if len(advisory) > 0 {
		parts = append(parts, "advisory contract-scope checks failed (non-blocking): "+joinComma(advisory))
	}
	return strings.Join(parts, "; ")
}

// evaluatedIdentities records the content identity of every resolved input —
// the source map's fingerprint and each live serving design's — so the verdict
// is never true about the wrong contract state (design §3.7, AP-13). Sorted for
// determinism (DINV-5).
func evaluatedIdentities(c *contract) []InputIdentity {
	out := []InputIdentity{
		{Kind: "prd-source-map", Ref: c.SourcePath, Fingerprint: c.Source.Fingerprint},
	}
	for _, d := range c.Serving {
		out = append(out, InputIdentity{Kind: "serving-design", Ref: d.Slug, Fingerprint: d.Fingerprint})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Ref < out[j].Ref
	})
	return out
}

// classRank orders the mechanical failure classes so the overall verdict class
// is the most severe failing check's class. Clean is the floor; error and
// configuration-error sit above the content-defect classes (a fail-closed
// infrastructure failure dominates a content gap).
var classRank = map[Class]int{
	ClassClean:              0,
	ClassStale:              1,
	ClassUnresolved:         2,
	ClassIncomplete:         3,
	ClassMalformed:          4,
	ClassError:              5,
	ClassConfigurationError: 6,
}

// configErr builds a fail-closed configuration-error verdict naming the missing
// input, with zero checks run (design §3.11). The reserved semantic slot stays
// not-evaluated (DINV-4).
func configErr(feature, file, reason, remedy string) Verdict {
	v := newVerdict(feature)
	v.Class = ClassConfigurationError
	v.Reason = reason
	v.CheckResults = resultsForDefects([]schema.Defect{{
		Verb: "contract-gate", File: file, Op: "resolve", Reason: reason, Remedy: remedy,
	}})
	return v
}

// malformedErr builds a fail-closed error verdict for a present-but-unparseable
// machine face, with zero checks run (design §3.11).
func malformedErr(feature, file, reason, remedy string) Verdict {
	v := newVerdict(feature)
	v.Class = ClassError
	v.Reason = reason
	v.CheckResults = resultsForDefects([]schema.Defect{{
		Verb: "contract-gate", File: file, Op: "resolve", Reason: reason, Remedy: remedy,
	}})
	return v
}

// resultsForDefects wraps resolution defects as a single failed pseudo-check
// result so the human summary and any future report can enumerate them without
// a separate field. These are resolution failures, not check runs — the check
// set itself is untouched (design §3.11: zero checks run on a fail-closed edge).
func resultsForDefects(defects []schema.Defect) []CheckResult {
	if len(defects) == 0 {
		return []CheckResult{}
	}
	return []CheckResult{{Check: "resolve", Class: ClassConfigurationError, Passed: false, Defects: defects}}
}

func joinComma(xs []string) string {
	out := ""
	for i, x := range xs {
		if i > 0 {
			out += ", "
		}
		out += x
	}
	return out
}
