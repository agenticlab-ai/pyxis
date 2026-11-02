package policy

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Parse validates the closed risk-policy grammar over normalized bytes and
// returns the parsed shape together with every named defect. The grammar is
// a fixed YAML subset — exactly the pinned schema (merge-lane-v1 §3.6): the
// seven top-level keys, floor's two lists, auto_classes as a list of
// data-defined predicates, and the scalar threshold/activation/kill blocks.
// Unknown keys, unknown structure, wrong types, a wrong schema version, or a
// missing required block are all defects — never a lenient parse (DINV-8).
// Parse is pure (no git, no clock); Load layers the file read and the
// ratified-lineage check over it.
func Parse(b []byte) (*Policy, []schema.Defect) {
	p := &Policy{}
	var defects []schema.Defect
	bad := func(n int, op, reason, remedy string) {
		defects = append(defects, def(op, fmt.Sprintf("line %d: %s", n, reason), remedy))
	}

	// presence tracking — closed schema requires every block declared.
	var seenSchema, seenRatified, seenFloorPaths, seenFloorClasses bool
	var seenAuto, seenThresholds, seenActivation, seenKill bool
	var seenEnforce, seenBackstop, seenWindow, seenBar bool

	top := ""           // current top-level block: floor|auto_classes|thresholds|activation|""
	floorSub := ""      // within floor: hard_gated_paths|hard_gated_classes|""
	var cur *AutoClass  // the auto class currently being built
	pathsBlock := false // a `paths:` with no inline value opened a block list

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
			top, floorSub, cur, pathsBlock = "", "", nil, false
			key, val, hasVal := cut(t)
			switch key {
			case "schema_version":
				seenSchema = true
				p.SchemaVersion = val
			case "ratified_by":
				seenRatified = true
				p.RatifiedBy = val
			case "kill_switch":
				seenKill = true
				setBool(&p.KillSwitch, val, n, "parse", bad)
			case "floor":
				if hasVal {
					bad(n, "parse", "floor is a block, not a scalar", "give floor its hard_gated_paths and hard_gated_classes lists")
				}
				top = "floor"
			case "auto_classes":
				seenAuto = true
				top = "auto_classes"
				if hasVal {
					// only the empty inline list is legal here.
					if items, ok := inlineList(val); ok && len(items) == 0 {
						p.AutoClasses = []AutoClass{}
					} else {
						bad(n, "parse", "auto_classes is a block or the empty list []", "list each class as `- name:` items, or `auto_classes: []` when none are ratified")
					}
				}
			case "thresholds":
				seenThresholds = true
				top = "thresholds"
			case "activation":
				seenActivation = true
				top = "activation"
			default:
				bad(n, "parse", fmt.Sprintf("unknown top-level key %q", key), "the schema is closed (risk-policy/v1): see merge-lane-v1 §3.6")
			}

		case 2:
			switch top {
			case "floor":
				key, _, _ := cut(t)
				switch key {
				case "hard_gated_paths":
					seenFloorPaths = true
					floorSub = "hard_gated_paths"
				case "hard_gated_classes":
					seenFloorClasses = true
					floorSub = "hard_gated_classes"
				default:
					bad(n, "parse", fmt.Sprintf("unknown floor key %q", key), "floor carries only hard_gated_paths and hard_gated_classes")
				}
			case "auto_classes":
				if !strings.HasPrefix(t, "- ") {
					bad(n, "parse", "auto_classes items are `- name:` list entries", "start each class with `- name: <name>`")
					continue
				}
				p.AutoClasses = append(p.AutoClasses, AutoClass{})
				cur = &p.AutoClasses[len(p.AutoClasses)-1]
				pathsBlock = false
				setClassField(cur, strings.TrimPrefix(t, "- "), n, bad)
			case "thresholds":
				key, val, _ := cut(t)
				switch key {
				case "shadow_window_min_proposals":
					seenWindow = true
					setInt(&p.Thresholds.ShadowWindowMinProposals, val, n, bad)
				case "agreement_bar":
					seenBar = true
					setBar(&p.Thresholds.AgreementBar, val, n, bad)
				default:
					bad(n, "parse", fmt.Sprintf("unknown thresholds key %q", key), "thresholds carries only shadow_window_min_proposals and agreement_bar")
				}
			case "activation":
				key, val, _ := cut(t)
				switch key {
				case "enforce":
					seenEnforce = true
					setBool(&p.Activation.Enforce, val, n, "parse", bad)
				case "backstop_change_ref":
					seenBackstop = true
					p.Activation.BackstopChangeRef = val
				default:
					bad(n, "parse", fmt.Sprintf("unknown activation key %q", key), "activation carries only enforce and backstop_change_ref")
				}
			default:
				bad(n, "parse", "unexpected indented line", "indent 2 belongs to floor, auto_classes, thresholds, or activation")
			}

		case 4:
			switch {
			case top == "floor" && floorSub != "":
				if !strings.HasPrefix(t, "- ") {
					bad(n, "parse", "floor list expects `- <glob>` items", "list each glob as a `- ` entry")
					continue
				}
				item := unquote(strings.TrimPrefix(t, "- "))
				if floorSub == "hard_gated_paths" {
					p.Floor.HardGatedPaths = append(p.Floor.HardGatedPaths, item)
				} else {
					p.Floor.HardGatedClasses = append(p.Floor.HardGatedClasses, item)
				}
			case top == "auto_classes" && cur != nil:
				key, _, _ := cut(t)
				if key == "paths" && !strings.Contains(t, "[") && strings.TrimSpace(strings.TrimPrefix(t, "paths:")) == "" {
					pathsBlock = true // block form: following indent-6 `- ` items
					continue
				}
				pathsBlock = false
				setClassField(cur, t, n, bad)
			default:
				bad(n, "parse", "unexpected indentation", "indent 4 belongs to a floor list or an auto_classes field")
			}

		case 6:
			if top == "auto_classes" && cur != nil && pathsBlock && strings.HasPrefix(t, "- ") {
				cur.Paths = append(cur.Paths, unquote(strings.TrimPrefix(t, "- ")))
				continue
			}
			bad(n, "parse", "unexpected indentation", "indent 6 is only an auto_classes paths block list")

		default:
			bad(n, "parse", "unrecognized structure", "the risk-policy grammar is a closed YAML subset (merge-lane-v1 §3.6)")
		}
	}

	// required-block presence — the closed schema declares all of them.
	requirePresent(&defects, seenSchema, "schema_version", "declare `schema_version: "+SchemaVersion+"`")
	requirePresent(&defects, seenRatified, "ratified_by", "name the ratifying operator")
	requirePresent(&defects, seenFloorPaths, "floor.hard_gated_paths", "declare the floor's hard_gated_paths list")
	requirePresent(&defects, seenFloorClasses, "floor.hard_gated_classes", "declare the floor's hard_gated_classes list")
	requirePresent(&defects, seenAuto, "auto_classes", "declare auto_classes (use `[]` until a class is ratified)")
	requirePresent(&defects, seenThresholds && seenWindow && seenBar, "thresholds", "declare thresholds with shadow_window_min_proposals and agreement_bar")
	requirePresent(&defects, seenActivation && seenEnforce && seenBackstop, "activation", "declare activation with enforce and backstop_change_ref")
	requirePresent(&defects, seenKill, "kill_switch", "declare kill_switch (true flips the lane HUMAN-only)")

	if seenSchema && p.SchemaVersion != SchemaVersion {
		defects = append(defects, def("parse", fmt.Sprintf("unknown schema version %q", p.SchemaVersion),
			"this build parses "+SchemaVersion+" only; a different version is never parsed leniently (DINV-8)"))
	}

	return p, defects
}

func requirePresent(defects *[]schema.Defect, ok bool, name, remedy string) {
	if !ok {
		*defects = append(*defects, def("parse", fmt.Sprintf("required key %q is absent", name), remedy))
	}
}

// setClassField applies one `key: value` field to the auto class being built.
func setClassField(c *AutoClass, field string, n int, bad func(int, string, string, string)) {
	key, val, hasVal := cut(field)
	if !hasVal {
		bad(n, "parse", fmt.Sprintf("auto_classes field %q needs a value", key), "each class field is `key: value`")
		return
	}
	switch key {
	case "name":
		c.Name = unquote(val)
	case "requires_reviewer_verdict":
		setBool(&c.RequiresReviewerVerdict, val, n, "parse", bad)
	case "paths":
		items, ok := inlineList(val)
		if !ok {
			bad(n, "parse", "auto_classes paths must be an inline list like [\"cmd/**\"] or a block of `- ` items", "give the class a whole-diff path perimeter")
			return
		}
		c.Paths = append(c.Paths, items...)
	default:
		bad(n, "parse", fmt.Sprintf("unknown auto_classes field %q", key), "a class carries name, paths, and requires_reviewer_verdict")
	}
}

// cut splits `key: value`; hasVal is false for a bare `key:` block opener.
func cut(t string) (key, val string, hasVal bool) {
	k, v, found := strings.Cut(t, ":")
	if !found {
		return strings.TrimSpace(t), "", false
	}
	v = unquote(v)
	return strings.TrimSpace(k), v, v != ""
}

// inlineList parses `["a", "b"]` (and the empty `[]`) into its items.
func inlineList(v string) ([]string, bool) {
	v = strings.TrimSpace(v)
	if !strings.HasPrefix(v, "[") || !strings.HasSuffix(v, "]") {
		return nil, false
	}
	inner := strings.TrimSpace(v[1 : len(v)-1])
	if inner == "" {
		return []string{}, true
	}
	var out []string
	for _, part := range strings.Split(inner, ",") {
		out = append(out, strings.Trim(strings.TrimSpace(part), `"`))
	}
	return out, true
}

func setBool(dst *bool, v string, n int, op string, bad func(int, string, string, string)) {
	switch v {
	case "true":
		*dst = true
	case "false":
		*dst = false
	default:
		bad(n, op, fmt.Sprintf("%q is not a boolean", v), "use true or false")
	}
}

func setInt(dst *int, v string, n int, bad func(int, string, string, string)) {
	x, err := strconv.Atoi(v)
	if err != nil || x <= 0 {
		bad(n, "parse", fmt.Sprintf("%q is not a positive integer", v), "the shadow window is a positive proposal count")
		return
	}
	*dst = x
}

func setBar(dst *float64, v string, n int, bad func(int, string, string, string)) {
	x, err := strconv.ParseFloat(v, 64)
	if err != nil || x < 0 || x > 1 {
		bad(n, "parse", fmt.Sprintf("%q is not an agreement bar in [0,1]", v), "agreement_bar is a fraction between 0 and 1")
		return
	}
	*dst = x
}

// unquote trims an optional trailing comment, surrounding quotes, and space.
func unquote(v string) string {
	v = strings.TrimSpace(v)
	if i := strings.Index(v, " #"); i >= 0 {
		v = strings.TrimSpace(v[:i])
	}
	return strings.Trim(v, `"`)
}
