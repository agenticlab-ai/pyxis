// Package incidents is the registry's coded writer (prd:incident-machinery
// FR-1/FR-2; docs/tech-design/incident-machinery.md): ID allocation is
// code, never prompted; graduation is the one sanctioned mutation. Writer
// verbs are a distinct family from gate verbs — gate verbs never mutate;
// these write exactly one file, atomically, and only after the would-be
// result passes the lint's own shape checks (DINV-1/DINV-4 there).
package incidents

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/gate"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

var incidentCreateTemp = os.CreateTemp

func def(verb, file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: op, Reason: reason, Remedy: remedy}
}

// Entry is one incident capture, fields as the author supplies them.
type Entry struct {
	Date     string // YYYY-MM-DD — an argument, never a clock read (hermeticity)
	Tags     []string
	Decision string
	Broke    string
	Lesson   string
	Links    string
}

// Render produces the entry block for an allocated ID — the one rendering
// of incident-entry-template/v1 (the template file documents this shape;
// the lint checks it; DINV-3: one schema, three consumers).
func Render(id int, e Entry) string {
	var b strings.Builder
	fmt.Fprintf(&b, "**I%d (%s) [%s]**\n\n", id, e.Date, strings.Join(e.Tags, ", "))
	fmt.Fprintf(&b, "- **Decision:** %s\n", e.Decision)
	fmt.Fprintf(&b, "- **Broke:** %s\n", e.Broke)
	fmt.Fprintf(&b, "- **Lesson:** %s\n", e.Lesson)
	fmt.Fprintf(&b, "- **Links:** %s\n", e.Links)
	return b.String()
}

// Add appends a new entry under the next dense I<n>, refusing (registry
// byte-unchanged) if the result would fail the shape lint (FR-1, UB-1).
func Add(root string, cfg config.Config, e Entry) (int, []schema.Defect) {
	rel := cfg.Incidents
	path := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, []schema.Defect{def("add-incident", rel, "read",
			fmt.Sprintf("registry unreadable: %v", err), "the writer appends to an existing registry (UB-1)")}
	}
	if d := gate.LintIncidentBytes(rel, b); len(d) > 0 {
		return 0, append(d, def("add-incident", rel, "precondition",
			"registry fails lint before the write", "fix the registry first; the writer never writes onto a red registry (UB-1)"))
	}
	next := 1
	for _, l := range strings.Split(string(schema.Normalize(b)), "\n") {
		if m := schema.IncidentIDRe.FindStringSubmatch(l); m != nil {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			if n >= next {
				next = n + 1
			}
		}
	}
	content := string(schema.Normalize(b))
	candidate := content + "\n" + Render(next, e)
	normalized := schema.Normalize([]byte(candidate))
	if d := gate.LintIncidentBytes(rel, normalized); len(d) > 0 {
		return 0, append(d, def("add-incident", rel, "self-check",
			fmt.Sprintf("the would-be entry I%d fails the shape lint — nothing written", next),
			"fix the fields/tags and retry; the registry is byte-unchanged (DINV-4)"))
	}
	if err := atomicWrite(path, normalized); err != nil {
		return 0, []schema.Defect{def("add-incident", rel, "write", err.Error(), "the write is atomic or nothing")}
	}
	return next, nil
}

// Graduate appends `Graduated: AP-<n>` to an entry's Links line — the one
// sanctioned mutation (FR-2; INV-35). Fails on unknown ID or double
// graduation; the append-only checker's Links-growth rule accepts exactly
// this shape.
func Graduate(root string, cfg config.Config, incidentID int, principle string) []schema.Defect {
	rel := cfg.Incidents
	path := filepath.Join(root, filepath.FromSlash(rel))
	b, err := os.ReadFile(path)
	if err != nil {
		return []schema.Defect{def("graduate", rel, "read", fmt.Sprintf("registry unreadable: %v", err), "nothing to graduate")}
	}
	lines := strings.Split(string(schema.Normalize(b)), "\n")
	inEntry, done := false, false
	for i, l := range lines {
		if m := schema.IncidentIDRe.FindStringSubmatch(l); m != nil {
			var n int
			fmt.Sscanf(m[1], "%d", &n)
			inEntry = n == incidentID
			continue
		}
		if !inEntry || !strings.HasPrefix(strings.TrimSpace(l), "- **Links:**") {
			continue
		}
		if strings.Contains(l, "Graduated:") {
			return []schema.Defect{def("graduate", rel, "entry",
				fmt.Sprintf("I%d already carries a graduation pointer", incidentID),
				"an entry graduates once; the pointer is permanent")}
		}
		lines[i] = l + " Graduated: " + principle + "."
		done = true
		break
	}
	if !done {
		return []schema.Defect{def("graduate", rel, "entry",
			fmt.Sprintf("no entry I%d with a Links line found", incidentID), "name an existing entry")}
	}
	out := schema.Normalize([]byte(strings.Join(lines, "\n")))
	if d := gate.LintIncidentBytes(rel, out); len(d) > 0 {
		return append(d, def("graduate", rel, "self-check",
			"the graduated registry fails the shape lint — nothing written", "report this as a writer bug (DINV-4)"))
	}
	if err := atomicWrite(path, out); err != nil {
		return []schema.Defect{def("graduate", rel, "write", err.Error(), "the write is atomic or nothing")}
	}
	return nil
}

// atomicWrite is temp-in-same-dir + rename: the registry is never torn.
func atomicWrite(path string, b []byte) error {
	tmp, err := incidentCreateTemp(filepath.Dir(path), ".incident-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil {
		os.Remove(tmpName)
		return err
	}
	return os.Rename(tmpName, path)
}
