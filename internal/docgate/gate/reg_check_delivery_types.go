package gate

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/argus/boundary"
	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
)

const deliveryTypesVerb = "check-delivery-types"

func init() {
	Register(Check{
		Verb: deliveryTypesVerb, Help: "changed delivery journals use canonical events and exact append-only historical corrections",
		NeedsMergeBase: true, RunOnPyxisSource: true, Run: CheckDeliveryTypes,
	})
}

// CheckDeliveryTypes judges only changed delivery ledgers. An untouched legacy
// error does not prevent unrelated implementation work. Pushed heads are read
// from Git, never from another session's worktree; desk checks include edits.
func CheckDeliveryTypes(c Ctx) Defects {
	bad := func(file string, err error) Defects {
		return Defects{def(deliveryTypesVerb, file, "git", fmt.Sprintf("cannot validate changed delivery journal: %v", err),
			"supply a fetched trusted merge base and readable candidate ledger")}
	}
	if strings.TrimSpace(c.MergeBase) == "" {
		return bad(c.Cfg.Problems, fmt.Errorf("no merge base supplied"))
	}
	args := []string{"-C", c.Root, "diff", "--name-only", "--no-renames", "-z", c.MergeBase}
	if c.Head != "" {
		args = append(args, c.Head)
	}
	args = append(args, "--", c.Cfg.Problems)
	out, err := exec.Command("git", args...).Output()
	if err != nil {
		return bad(c.Cfg.Problems, err)
	}
	basePaths, err := lsTreeLedgers(c.Root, c.MergeBase, c.Cfg.Problems)
	if err != nil {
		return bad(c.Cfg.Problems, err)
	}
	present := map[string]bool{}
	for _, p := range basePaths {
		present[p] = true
	}
	var defects Defects
	for file := range strings.SplitSeq(string(out), "\x00") {
		class, ok := argusledger.ClassForPath(c.Cfg.Problems, file)
		if !ok || class.DerivedRegion == "" {
			continue
		}
		var original, candidate []byte
		if present[file] {
			original, err = gitShow(c.Root, c.MergeBase, file)
			if err != nil {
				defects = append(defects, bad(file, err)...)
				continue
			}
		}
		if c.Head != "" {
			candidate, err = gitShow(c.Root, c.Head, file)
		} else {
			candidate, err = os.ReadFile(filepath.Join(c.Root, filepath.FromSlash(file)))
		}
		if err != nil {
			defects = append(defects, bad(file, err)...)
			continue
		}
		slug := path.Base(path.Dir(file))
		defects = append(defects, boundary.ValidateDeliveryChange(deliveryTypesVerb, c.Cfg, slug, original, candidate)...)
	}
	return defects
}
