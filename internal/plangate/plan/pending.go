package plan

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// PendingBasis names the binding lock a lock proposal is about to create. Its
// tag does not exist until the proposal merges, so a fold reads the lock's
// records from the prepared commit the operator is asked to approve.
type PendingBasis struct {
	Basis  string // lock/<feature>/vN
	Commit string // the prepared commit holding the compiled records
}

// regularFaces refuses plan faces that are not plain files a write stays
// within: a face committed as a symbolic link, gitlink, or other non-regular
// entry, or a plan directory that resolves outside root.
func regularFaces(root, rel string) []schema.Defect {
	refuse := func(reason string) []schema.Defect {
		return []schema.Defect{def(rel, "plan files", reason, "commit the plan's two files as regular files in the plans directory")}
	}
	realRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return refuse(fmt.Sprintf("resolve the repository: %v", err))
	}
	dir, err := filepath.EvalSymlinks(filepath.Join(root, filepath.FromSlash(path.Dir(rel))))
	if err != nil {
		return refuse(fmt.Sprintf("resolve the plan directory: %v", err))
	}
	if inside, err := filepath.Rel(realRoot, dir); err != nil || inside == ".." || strings.HasPrefix(inside, ".."+string(filepath.Separator)) {
		return refuse("the plan directory resolves outside the repository")
	}
	for _, face := range []string{rel, JSONPath(rel)} {
		info, err := os.Lstat(filepath.Join(dir, path.Base(face)))
		switch {
		case err == nil && info.Mode()&fs.ModeSymlink != 0:
			return refuse(fmt.Sprintf("%s is a symbolic link", face))
		case err == nil && !info.Mode().IsRegular():
			return refuse(fmt.Sprintf("%s is not a regular file", face))
		}
	}
	return nil
}

// draftBorn reports a draft-born plan by its PRD row, read with the plan
// parser's own header-row grammar, even when the rest of its header no
// longer parses.
func draftBorn(source []byte) bool {
	for line := range strings.SplitSeq(string(source), "\n") {
		if strings.HasPrefix(line, "## ") {
			break
		}
		if m := tableRowRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil && m[1] == "PRD" && strings.HasPrefix(m[2], "prd/") {
			return true
		}
	}
	return false
}

// FoldFileAt folds like FoldFile. A plan naming pending.Basis reads that lock
// from pending.Commit: the lock is being prepared, so its tag is not
// published yet, and a local tag of that name is stale, never the lock.
func FoldFileAt(root string, cfg config.Config, rel string, pending PendingBasis) Result {
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return Result{Defects: []schema.Defect{def(rel, "read", err.Error(), "point fold at an existing NNN-plan.md")}}
	}
	doc, defects := Parse(rel, b)
	if doc == nil || len(defects) > 0 {
		return Result{Defects: defects}
	}
	doc.pending = &pending
	if vdef := Validate(root, cfg, doc); len(vdef) > 0 {
		return Result{Doc: doc, Defects: vdef}
	}
	js, err := doc.JSON()
	if err != nil {
		return Result{Doc: doc, Defects: []schema.Defect{def(rel, "render", err.Error(), "report this — the model must always marshal")}}
	}
	return Result{Doc: doc, JSON: js}
}

// RebaseLive moves a feature's live plan — its highest-numbered plan — onto
// the pending lock: it rewrites the Basis row, refolds the plan against the
// pending lock's records, and writes both faces. A plan that no longer fits the
// contract returns its defects and leaves both files unchanged. A feature with
// no plan, or whose live plan names no lock Basis, is not a defect.
func RebaseLive(root string, cfg config.Config, feature string, pending PendingBasis) (changed []string, defects []schema.Defect, err error) {
	dir := path.Join(filepath.ToSlash(filepath.Clean(cfg.Plans)), feature)
	files, err := Files(root, dir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, err
	}
	if len(files) == 0 {
		return nil, nil, nil
	}
	rel := files[len(files)-1]
	if defects := regularFaces(root, rel); len(defects) > 0 {
		return nil, defects, nil
	}
	mdPath := filepath.Join(root, filepath.FromSlash(rel))
	source, err := os.ReadFile(mdPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read live plan %s: %w", rel, err)
	}
	rebased, ok := withBasis(source, pending.Basis)
	if !ok {
		// A plan with no Basis row is not moved. A draft-born plan must
		// name its lock, so the fold refuses one whose row was removed or
		// broken; a document-pipeline plan has none and stays as it is.
		if draftBorn(source) {
			if res := FoldFileAt(root, cfg, rel, pending); len(res.Defects) > 0 {
				return nil, res.Defects, nil
			}
		}
		return nil, nil, nil
	}
	if err := os.WriteFile(mdPath, rebased, 0o644); err != nil {
		return nil, nil, fmt.Errorf("write live plan %s: %w", rel, err)
	}
	res := FoldFileAt(root, cfg, rel, pending)
	if len(res.Defects) > 0 {
		if restoreErr := os.WriteFile(mdPath, source, 0o644); restoreErr != nil {
			return nil, res.Defects, fmt.Errorf("restore live plan %s: %w", rel, restoreErr)
		}
		return nil, res.Defects, nil
	}
	if !bytes.Equal(source, rebased) {
		changed = append(changed, rel)
	}
	jsRel := JSONPath(rel)
	jsPath := filepath.Join(root, filepath.FromSlash(jsRel))
	committed, err := os.ReadFile(jsPath)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, nil, fmt.Errorf("read live plan face %s: %w", jsRel, err)
	}
	if !bytes.Equal(committed, res.JSON) {
		if err := os.WriteFile(jsPath, res.JSON, 0o644); err != nil {
			return nil, nil, fmt.Errorf("write live plan face %s: %w", jsRel, err)
		}
		changed = append(changed, jsRel)
	}
	return changed, nil, nil
}

// withBasis rewrites the header's Basis row. A plan without one is a
// document-pipeline or intake plan and is left alone.
func withBasis(source []byte, basis string) ([]byte, bool) {
	lines := strings.Split(string(source), "\n")
	for i, line := range lines {
		if strings.HasPrefix(line, "## ") {
			break
		}
		// The header row the parser reads as Basis, in any spacing it accepts.
		if m := tableRowRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil && m[1] == "Basis" {
			lines[i] = "| **Basis** | " + basis + " |"
			return []byte(strings.Join(lines, "\n")), true
		}
	}
	return source, false
}
