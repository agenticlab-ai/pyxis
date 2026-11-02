package localfs

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

// The signal vocabulary is the store's governed control-state dictionary
// (docs/tech-design/local-work-tracker.md §3.5, §3.14). The canonical seed set
// ships in code (model.SeedSignals — the human-authored governed input, never
// rebuilt from the store, AP-24); the store file `signals.md` holds only the
// operand's classification-axis extensions. The vocabulary a caller reads is the
// seed merged with those extensions, so a fresh store already carries the full
// governed set without a write-on-read.
//
// Signal mutation on an item is a header change, not a log entry: a signal name
// is a closed-schema header field (§3.1). Mutations are idempotent — a
// present-add or an absent-remove is a recorded no-op that writes nothing
// (AP-22, DINV — no torn or spurious write).

const vocabFile = "signals.md"

func (s *Store) vocabPath() string { return filepath.Join(s.root, vocabFile) }

// Signals returns the full vocabulary: the governed seed merged with the
// operand's classification extensions, each reporting its role.
func (s *Store) Signals(ctx context.Context) ([]model.Signal, []schema.Defect) {
	exts, defects := s.readExtensions()
	if len(defects) > 0 {
		return nil, defects
	}
	out := append([]model.Signal{}, model.SeedSignals()...)
	return append(out, exts...), nil
}

// AddSignal puts a defined signal on the item. An undefined name is refused
// naming it (prd:local-work-tracker/UB-2); an already-present signal is a
// recorded no-op returning the item unchanged (AP-22).
func (s *Store) AddSignal(ctx context.Context, id int, name string) (model.WorkItem, []schema.Defect) {
	const verb = "signal"
	name = strings.TrimSpace(name)
	return s.mutate(verb, id, func(w *model.WorkItem) (bool, []schema.Defect) {
		defined, defects := s.definedSignal(ctx, name)
		if len(defects) > 0 {
			return false, defects
		}
		if !defined {
			return false, []schema.Defect{def(verb, itemPath(s.itemsDir(), id), "add-signal",
				fmt.Sprintf("signal %q is not defined in the vocabulary", name),
				"define it first (carina signal def) or use a seed signal (carina signal ls)")}
		}
		if contains(w.Signals, name) {
			return false, nil // recorded no-op
		}
		w.Signals = append(w.Signals, name)
		return true, nil
	})
}

// RemoveSignal takes a signal off the item. Removing an absent signal is a
// recorded no-op returning the item unchanged (AP-22).
func (s *Store) RemoveSignal(ctx context.Context, id int, name string) (model.WorkItem, []schema.Defect) {
	name = strings.TrimSpace(name)
	return s.mutate("signal", id, func(w *model.WorkItem) (bool, []schema.Defect) {
		if !contains(w.Signals, name) {
			return false, nil // recorded no-op
		}
		w.Signals = removeStr(w.Signals, name)
		return true, nil
	})
}

// DefineSignal extends the vocabulary along the classification axis. The order
// of refusals names the specific offense: an engine-recognized name cannot be
// redefined; a non-classification role cannot be operand-defined; a
// classification name must be namespaced kind:/area:.
func (s *Store) DefineSignal(ctx context.Context, sig model.Signal) []schema.Defect {
	const verb = "signal"
	fail := func(reason, remedy string) []schema.Defect {
		return []schema.Defect{def(verb, s.vocabPath(), "define", reason, remedy)}
	}
	if strings.TrimSpace(sig.Name) == "" {
		return fail("signal name is empty", "supply a signal name")
	}
	if model.EngineRecognizedSignal(sig.Name) {
		return fail(fmt.Sprintf("engine-recognized signal %q cannot be redefined", sig.Name),
			"operands extend only the classification axis (kind:/area:); the seed vocabulary is governed")
	}
	if !model.ValidRole(sig.Role) {
		return fail(fmt.Sprintf("role %q is out of set", sig.Role), "role is one of "+join(model.Roles))
	}
	if sig.Role != model.RoleClassification {
		return fail(fmt.Sprintf("cannot define a %s signal; only classification signals are operand-extensible", sig.Role),
			"define a kind:/area: signal with role "+string(model.RoleClassification))
	}
	if !model.IsClassificationName(sig.Name) {
		return fail(fmt.Sprintf("classification signal %q is not namespaced kind:… or area:…", sig.Name),
			"name a classification signal kind:<x> or area:<x>")
	}
	exts, defects := s.readExtensions()
	if len(defects) > 0 {
		return defects
	}
	for i, e := range exts {
		if e.Name == sig.Name {
			if e == sig {
				return nil // identical repeat: recorded no-op (AP-22)
			}
			exts[i] = sig
			return s.writeVocab(verb, exts)
		}
	}
	return s.writeVocab(verb, append(exts, sig))
}

// definedSignal reports whether name is in the vocabulary (seed ∪ extensions).
func (s *Store) definedSignal(ctx context.Context, name string) (bool, []schema.Defect) {
	sigs, defects := s.Signals(ctx)
	if len(defects) > 0 {
		return false, defects
	}
	for _, sig := range sigs {
		if sig.Name == name {
			return true, nil
		}
	}
	return false, nil
}

// readExtensions parses the operand's classification-axis extensions from the
// vocabulary file; an absent file means no extensions (the seed still applies).
func (s *Store) readExtensions() ([]model.Signal, []schema.Defect) {
	raw, err := os.ReadFile(s.vocabPath())
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, []schema.Defect{def("signal", s.vocabPath(), "read", err.Error(), "ensure the vocabulary file is readable")}
	}
	return parseVocab(s.vocabPath(), raw)
}

// writeVocab renders the extensions and writes the vocabulary file atomically.
func (s *Store) writeVocab(verb string, exts []model.Signal) []schema.Defect {
	if err := os.MkdirAll(s.root, 0o755); err != nil {
		return []schema.Defect{def(verb, s.root, "mkdir", err.Error(), "ensure the store root is writable")}
	}
	if err := s.atomicReplace(s.vocabPath(), renderVocab(exts), nil); err != nil {
		return []schema.Defect{def(verb, s.vocabPath(), "write", err.Error(), "ensure the store has free space and is writable")}
	}
	return nil
}

// The vocabulary file grammar: a closed pipe-delimited table of the operand's
// classification extensions, human-readable and git-diffable (§3.5). The seed is
// never written here; only kind:/area: extensions are.

func renderVocab(exts []model.Signal) []byte {
	var b strings.Builder
	b.WriteString(fence + "\nschema: " + model.SignalVocabularySchema + "\n" + fence + "\n\n")
	b.WriteString("# Signal vocabulary — operand extensions\n\n")
	b.WriteString("The canonical seed vocabulary ships with pyxis; this file records only the\n")
	b.WriteString("classification-axis signals this operand has added (kind:… / area:…).\n\n")
	b.WriteString("| name | role | color | description |\n")
	b.WriteString("|---|---|---|---|\n")
	for _, sig := range exts {
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n", sig.Name, sig.Role, sig.Color, sig.Description)
	}
	return schema.Normalize([]byte(b.String()))
}

// parseVocab reads the extension table, failing closed on a malformed row or an
// out-of-set/non-classification signal (§3.13, DINV-11). The header and
// separator rows are skipped; only classification-axis extensions are legal.
func parseVocab(file string, raw []byte) ([]model.Signal, []schema.Defect) {
	var out []model.Signal
	var defects []schema.Defect
	bad := func(op, reason, remedy string) {
		defects = append(defects, schema.Defect{Verb: "signal", File: file, Op: op, Reason: reason, Remedy: remedy})
	}
	for _, line := range strings.Split(string(schema.Normalize(raw)), "\n") {
		t := strings.TrimSpace(line)
		if !strings.HasPrefix(t, "|") || !strings.HasSuffix(t, "|") {
			continue
		}
		cells := splitRow(t)
		if len(cells) == 4 && cells[0] == "name" && cells[1] == "role" {
			continue // header row
		}
		if isSeparatorRow(cells) {
			continue
		}
		if len(cells) != 4 {
			bad("parse-vocab", fmt.Sprintf("vocabulary row %q has %d columns, want 4", t, len(cells)),
				"each row is | name | role | color | description |")
			continue
		}
		sig := model.Signal{Name: cells[0], Role: model.Role(cells[1]), Color: cells[2], Description: cells[3]}
		if reason := extensionDefect(sig); reason != "" {
			bad("parse-vocab", reason, "an operand signal is a kind:/area: name with role "+string(model.RoleClassification))
			continue
		}
		out = append(out, sig)
	}
	return out, defects
}

// extensionDefect returns why sig is not a legal operand extension, or "".
func extensionDefect(sig model.Signal) string {
	if !model.ValidRole(sig.Role) {
		return fmt.Sprintf("signal %q has role %q out of set", sig.Name, sig.Role)
	}
	if sig.Role != model.RoleClassification {
		return fmt.Sprintf("signal %q has non-classification role %q; only classification signals are operand-defined", sig.Name, sig.Role)
	}
	if !model.IsClassificationName(sig.Name) {
		return fmt.Sprintf("classification signal %q is not namespaced kind:… or area:…", sig.Name)
	}
	return ""
}

func splitRow(t string) []string {
	t = strings.TrimPrefix(t, "|")
	t = strings.TrimSuffix(t, "|")
	parts := strings.Split(t, "|")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func isSeparatorRow(cells []string) bool {
	if len(cells) == 0 {
		return false
	}
	for _, c := range cells {
		if strings.Trim(c, "-: ") != "" {
			return false
		}
	}
	return true
}

func contains(xs []string, v string) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func removeStr(xs []string, v string) []string {
	out := make([]string, 0, len(xs))
	for _, x := range xs {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
