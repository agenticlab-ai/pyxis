// Package sweep is the maintenance sweep's one focused package: the closed-parse
// policy every detector shares (waivers + staleness bound), the deterministic
// report/finding schema every pass emits, the three detectors, and the pass
// that composes them (maintenance-sweep-v1 §2, §3). This file owns the policy
// surface (§3.4): a single closed-parse file — schema version, the staleness
// bound, package waivers, and plan waivers — where every waiver entry requires
// a recorded reason and every uncertain input fails closed with a named defect
// (DINV-3, DINV-5). An absent policy file is the legal empty policy carrying the
// default bound (the sweep is advisory — §3.1). Stdlib only, per the repo Go
// rules; the parse is closed, never a lenient guess.
package sweep

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// PolicySchemaVersion is the one policy shape this build parses against. A
// present policy declaring any other version is a defect, never parsed
// leniently (DINV-3).
const PolicySchemaVersion = "maintenance-policy/v1"

// DefaultStalenessBound is the rollup-staleness threshold when the policy does
// not name one — fourteen days (maintenance-sweep-v1 §3.4). The absent-file
// empty policy carries it too, so the sweep runs with a sane default.
const DefaultStalenessBound = 14 * 24 * time.Hour

// policyVerb names this parser in every defect it emits (schema.Defect.Verb).
const policyVerb = "maintenance-policy"

// policyFile is the defect File placeholder — overwritten by Load/LoadFile with
// the resolved policy path; the pure Parse path names the schema generically.
const policyFile = "maintenance-policy"

// PackageWaiver suppresses one unwired-packages finding. Both fields are
// required: a reasonless entry refuses to load, so every suppression is
// attributable in history (DINV-5, prd FR-10).
type PackageWaiver struct {
	Package string
	Reason  string
}

// PlanWaiver suppresses one plan's touches-underdelivery findings — the
// in-flight-plan control (a freshly frozen plan mid-execution). Reason-required
// exactly as the package waiver.
type PlanWaiver struct {
	Plan   string
	Reason string
}

// Policy is the parsed, validated sweep policy — the closed shape every detector
// reads (maintenance-sweep-v1 §3.4). Every field is data: retuning the bound or
// a waiver is a policy-file PR, never a code change.
type Policy struct {
	SchemaVersion  string
	StalenessBound time.Duration
	PackageWaivers []PackageWaiver
	PlanWaivers    []PlanWaiver
}

// Empty is the legal empty policy: the current schema version, the default
// staleness bound, and no waivers. It is what an absent policy file resolves to
// (maintenance-sweep-v1 §3.1) — the sweep is advisory, so no policy is a legal
// state, not a fault.
func Empty() Policy {
	return Policy{
		SchemaVersion:  PolicySchemaVersion,
		StalenessBound: DefaultStalenessBound,
	}
}

func policyDef(reason, remedy string) schema.Defect {
	return schema.Defect{Verb: policyVerb, File: policyFile, Op: "parse", Reason: reason, Remedy: remedy}
}

// LoadPolicy resolves the policy path from the config surface and loads it. An
// absent file is the empty policy with no defects; a malformed file, an unknown
// key, or a reasonless waiver returns the empty policy and a named defect
// (DINV-3).
func LoadPolicy(root string) (Policy, []schema.Defect) {
	cfg, _ := config.Load(root)
	return LoadPolicyFile(root, cfg.MaintenancePolicy)
}

// LoadPolicyFile reads root/rel, parses it closed, and stamps every defect with
// the resolved path. It is LoadPolicy's core and the substitution point for
// hermetic tests. An absent file is the legal empty policy (no defects); an
// unreadable file or any parse defect returns the empty policy with the defects
// named — the caller then treats the pass as faulted.
func LoadPolicyFile(root, rel string) (Policy, []schema.Defect) {
	stamp := func(ds []schema.Defect) []schema.Defect {
		for i := range ds {
			ds[i].File = rel
		}
		return ds
	}

	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		if os.IsNotExist(err) {
			return Empty(), nil
		}
		return Empty(), stamp([]schema.Defect{policyDef(
			fmt.Sprintf("unreadable: %v", err),
			"fix permissions, or remove the file to run with the empty policy and default bound")})
	}

	p, defects := Parse(b)
	if len(defects) > 0 {
		return Empty(), stamp(defects)
	}
	return p, nil
}

// Parse validates the closed maintenance-policy grammar over normalized bytes
// and returns the parsed shape together with every named defect. The grammar is
// a fixed YAML subset: schema_version and staleness_bound scalars, and a waivers
// block whose packages and plans lists carry reason-required entries. Unknown
// keys, unknown structure, a wrong schema version, a malformed bound, or a
// reasonless/incomplete waiver are all defects — never a lenient parse (DINV-3).
// Parse is pure (no filesystem, no clock).
func Parse(b []byte) (Policy, []schema.Defect) {
	p := Empty()
	var defects []schema.Defect
	bad := func(n int, reason, remedy string) {
		defects = append(defects, policyDef(fmt.Sprintf("line %d: %s", n, reason), remedy))
	}

	seenSchema := false
	top := "" // "" | "waivers"
	sub := "" // "" | "packages" | "plans"
	var curPkg *PackageWaiver
	var curPlan *PlanWaiver

	for i, raw := range strings.Split(string(schema.Normalize(b)), "\n") {
		n := i + 1
		line := strings.TrimRight(raw, " ")
		if line == "" || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		indent := len(line) - len(strings.TrimLeft(line, " "))
		t := strings.TrimSpace(line)

		switch indent {
		case 0:
			// A top-level line always resets the block context.
			top, sub, curPkg, curPlan = "", "", nil, nil
			key, val, hasVal := policyCut(t)
			switch key {
			case "schema_version":
				seenSchema = true
				p.SchemaVersion = val
			case "staleness_bound":
				if !hasVal {
					bad(n, "staleness_bound needs a value", "give a positive Go duration like \"336h\"")
					continue
				}
				setPolicyDuration(&p.StalenessBound, val, n, bad)
			case "waivers":
				if hasVal {
					bad(n, "waivers is a block, not a scalar", "give waivers its packages and plans lists")
					continue
				}
				top = "waivers"
			default:
				bad(n, fmt.Sprintf("unknown top-level key %q", key), "the schema is closed (maintenance-policy/v1): see maintenance-sweep-v1 §3.4")
			}

		case 2:
			if top != "waivers" {
				bad(n, "unexpected indented line", "indent 2 belongs to the waivers block")
				continue
			}
			key, _, hasVal := policyCut(t)
			if hasVal {
				bad(n, fmt.Sprintf("waivers key %q is a list block, not a scalar", key), "list waiver entries as `- ` items under packages or plans")
				continue
			}
			switch key {
			case "packages":
				sub = "packages"
			case "plans":
				sub = "plans"
			default:
				bad(n, fmt.Sprintf("unknown waivers key %q", key), "waivers carries only packages and plans")
			}
			curPkg, curPlan = nil, nil

		case 4:
			if top != "waivers" || sub == "" {
				bad(n, "unexpected indentation", "indent 4 belongs to a waivers list entry")
				continue
			}
			if !strings.HasPrefix(t, "- ") {
				bad(n, "waiver entries are `- ` list items", "start each entry with `- package:` (packages) or `- plan:` (plans)")
				continue
			}
			field := strings.TrimPrefix(t, "- ")
			if sub == "packages" {
				p.PackageWaivers = append(p.PackageWaivers, PackageWaiver{})
				curPkg = &p.PackageWaivers[len(p.PackageWaivers)-1]
				curPlan = nil
				setPkgField(curPkg, field, n, bad)
			} else {
				p.PlanWaivers = append(p.PlanWaivers, PlanWaiver{})
				curPlan = &p.PlanWaivers[len(p.PlanWaivers)-1]
				curPkg = nil
				setPlanField(curPlan, field, n, bad)
			}

		case 6:
			if top != "waivers" || sub == "" {
				bad(n, "unexpected indentation", "indent 6 belongs to a waiver entry's fields")
				continue
			}
			if strings.HasPrefix(t, "- ") {
				bad(n, "unexpected list item", "a waiver entry's continuation fields are `key: value`, not `- ` items")
				continue
			}
			if sub == "packages" {
				if curPkg == nil {
					bad(n, "waiver field precedes any `- ` entry", "each field follows its `- package:` entry line")
					continue
				}
				setPkgField(curPkg, t, n, bad)
			} else {
				if curPlan == nil {
					bad(n, "waiver field precedes any `- ` entry", "each field follows its `- plan:` entry line")
					continue
				}
				setPlanField(curPlan, t, n, bad)
			}

		default:
			bad(n, "unrecognized structure", "the maintenance-policy grammar is a closed YAML subset (maintenance-sweep-v1 §3.4)")
		}
	}

	// A present policy MUST declare its schema version and match this build's.
	if !seenSchema {
		defects = append(defects, policyDef("required key \"schema_version\" is absent",
			"declare `schema_version: "+PolicySchemaVersion+"`"))
	} else if p.SchemaVersion != PolicySchemaVersion {
		defects = append(defects, policyDef(fmt.Sprintf("unknown schema version %q", p.SchemaVersion),
			"this build parses "+PolicySchemaVersion+" only; a different version is never parsed leniently (DINV-3)"))
	}

	// Every waiver entry is complete and reason-carrying — the visibility
	// contract's ground (DINV-5, prd FR-10): a reasonless entry refuses to load.
	for _, w := range p.PackageWaivers {
		if w.Package == "" {
			defects = append(defects, policyDef("a package waiver names no package", "each package waiver is `- package: <import path>` with a reason"))
			continue
		}
		if w.Reason == "" {
			defects = append(defects, policyDef(fmt.Sprintf("the package waiver for %q has no reason", w.Package),
				"every waiver entry requires a recorded reason — suppression is always attributable (prd FR-10)"))
		}
	}
	for _, w := range p.PlanWaivers {
		if w.Plan == "" {
			defects = append(defects, policyDef("a plan waiver names no plan", "each plan waiver is `- plan: <plan id>` with a reason"))
			continue
		}
		if w.Reason == "" {
			defects = append(defects, policyDef(fmt.Sprintf("the plan waiver for %q has no reason", w.Plan),
				"every waiver entry requires a recorded reason — suppression is always attributable (prd FR-10)"))
		}
	}

	return p, defects
}

// setPkgField applies one `key: value` field to the package waiver being built.
func setPkgField(w *PackageWaiver, field string, n int, bad func(int, string, string)) {
	key, val, hasVal := policyCut(field)
	if !hasVal {
		bad(n, fmt.Sprintf("package-waiver field %q needs a value", key), "each field is `key: value`")
		return
	}
	switch key {
	case "package":
		w.Package = val
	case "reason":
		w.Reason = val
	default:
		bad(n, fmt.Sprintf("unknown package-waiver field %q", key), "a package waiver carries package and reason")
	}
}

// setPlanField applies one `key: value` field to the plan waiver being built.
func setPlanField(w *PlanWaiver, field string, n int, bad func(int, string, string)) {
	key, val, hasVal := policyCut(field)
	if !hasVal {
		bad(n, fmt.Sprintf("plan-waiver field %q needs a value", key), "each field is `key: value`")
		return
	}
	switch key {
	case "plan":
		w.Plan = val
	case "reason":
		w.Reason = val
	default:
		bad(n, fmt.Sprintf("unknown plan-waiver field %q", key), "a plan waiver carries plan and reason")
	}
}

// policyCut splits `key: value`; hasVal is false for a bare `key:` block opener.
func policyCut(t string) (key, val string, hasVal bool) {
	k, v, found := strings.Cut(t, ":")
	if !found {
		return strings.TrimSpace(t), "", false
	}
	v = policyUnquote(v)
	return strings.TrimSpace(k), v, v != ""
}

// setPolicyDuration parses a positive Go duration scalar (e.g. "336h"); a
// malformed or non-positive value is a named defect and the default stands.
func setPolicyDuration(dst *time.Duration, v string, n int, bad func(int, string, string)) {
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		bad(n, fmt.Sprintf("staleness_bound %q is not a positive duration", v), "the bound is a positive Go duration like \"336h\" (14 days)")
		return
	}
	*dst = d
}

// policyUnquote trims an optional trailing comment, surrounding quotes, and space.
func policyUnquote(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"`)
}
