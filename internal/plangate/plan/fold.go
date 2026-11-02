// Fold and verify — the plan class's projection verbs (plan-new-v1 §3.6):
// fold parses + gates a plan source and emits its canonical machine face;
// verify byte-compares the committed face against a recomputation, the
// doc system's verify-generated pattern applied to this class (A-8).
package plan

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Result is one plan file's fold outcome.
type Result struct {
	Doc     *Doc
	JSON    []byte
	Defects []schema.Defect
}

// JSONPath is the machine face beside a plan source.
func JSONPath(source string) string {
	return strings.TrimSuffix(source, ".md") + ".json"
}

// Files lists a plan directory's NNN-plan.md sources, ascending.
func Files(root, dir string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return nil, fmt.Errorf("read plan dir %s: %w", dir, err)
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() && fileNameRe.MatchString(e.Name()) {
			out = append(out, dir+"/"+e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// FoldFile parses, gates, and renders one plan source (repo-relative path).
func FoldFile(root string, cfg config.Config, rel string) Result {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return Result{Defects: []schema.Defect{def(rel, "read", err.Error(), "point fold at an existing NNN-plan.md")}}
	}
	doc, defects := Parse(rel, b)
	if doc == nil || len(defects) > 0 {
		return Result{Defects: defects}
	}
	if vdef := Validate(root, cfg, doc); len(vdef) > 0 {
		return Result{Doc: doc, Defects: vdef}
	}
	js, err := doc.JSON()
	if err != nil {
		return Result{Doc: doc, Defects: []schema.Defect{def(rel, "render", err.Error(), "report this — the model must always marshal")}}
	}
	return Result{Doc: doc, JSON: js}
}

// Verify recomputes a source's machine face and byte-compares the committed
// one; a missing or divergent NNN-plan.json is a defect (never hand-edit it).
func Verify(root string, cfg config.Config, rel string) []schema.Defect {
	res := FoldFile(root, cfg, rel)
	if len(res.Defects) > 0 {
		return res.Defects
	}
	jsRel := JSONPath(rel)
	committed, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(jsRel)))
	if err != nil {
		return []schema.Defect{def(jsRel, "verify", "committed machine face missing", "run `plangate fold` and commit both faces together (design §3.1)")}
	}
	if !bytes.Equal(committed, res.JSON) {
		return []schema.Defect{def(jsRel, "verify", "committed machine face differs from the recomputed fold", "never hand-edit the .json: re-run `plangate fold` and recommit")}
	}
	return nil
}
