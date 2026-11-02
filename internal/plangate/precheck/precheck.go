// Package precheck selects the planning-entry gate for one feature contract.
package precheck

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/draft/lifecycle"
	"github.com/agenticlab-ai/pyxis/internal/draft/readiness"
	"github.com/agenticlab-ai/pyxis/internal/plangate/plan"
)

var featureName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// RunDocument returns handled when a feature uses the document contract gate
// or its inputs are invalid. Draft-born features return false, allowing the
// composition root to inspect the selected code host only when needed.
func RunDocument(root, feature string) (handled bool, defects []schema.Defect) {
	if !featureName.MatchString(feature) {
		return true, []schema.Defect{def(feature, "precheck", "invalid feature slug", "use a lowercase feature slug")}
	}
	cfg, defects := config.Load(root)
	if len(defects) != 0 {
		return true, defects
	}
	return document(root, feature, cfg)
}

// Run returns the locked Basis for a draft-born feature and all blocking
// defects. A document-pipeline feature delegates to the contract gate; the
// Git process port is never called on that branch.
func Run(ctx context.Context, root, feature string, run lifecycle.RunCommand) (string, []schema.Defect) {
	return entry(ctx, root, feature, run, "", judgeVersion)
}

type versionJudge func(context.Context, string, config.Config, string, int, string, lifecycle.RunCommand) (readiness.Verdict, error)

func entry(ctx context.Context, root, feature string, run lifecycle.RunCommand, origin string, judge versionJudge) (string, []schema.Defect) {
	if !featureName.MatchString(feature) {
		return "", []schema.Defect{def(feature, "precheck", "invalid feature slug", "use a lowercase feature slug")}
	}
	cfg, configDefects := config.Load(root)
	if len(configDefects) != 0 {
		return "", configDefects
	}
	if origin != "" {
		cfg.Remote = origin
	}
	if handled, defects := document(root, feature, cfg); handled {
		return "", defects
	}
	return locked(ctx, root, feature, cfg, run, judge)
}

func document(root, feature string, cfg config.Config) (bool, []schema.Defect) {
	for _, name := range []string{"000-base.md", "effective.source.json"} {
		ledger := filepath.Join(root, cfg.PRDs, feature, name)
		if _, err := os.Stat(ledger); err == nil {
			return true, plan.Precheck(root, feature)
		} else if !os.IsNotExist(err) {
			return true, []schema.Defect{def(ledger, "precheck", fmt.Sprintf("inspect PRD ledger: %v", err), "repair the ledger path and retry")}
		}
	}
	return false, nil
}

func locked(ctx context.Context, root, feature string, cfg config.Config, run lifecycle.RunCommand, judge versionJudge) (string, []schema.Defect) {
	path := "lock/" + feature
	refuse := func(reason, remedy string) (string, []schema.Defect) {
		return "", []schema.Defect{def(path, "precheck/locked-readiness", reason, remedy)}
	}
	if run == nil {
		return refuse("binding-lock check has no Git process adapter", "run plangate precheck through the CLI")
	}
	mainline, version, err := currentLock(ctx, root, feature, cfg, run)
	if err != nil {
		return refuse(err.Error(), "restore access to the current mainline and binding-lock history, then retry")
	}
	if version == 0 {
		return refuse("feature has no binding locked version", "approve and merge the draft, then record its lock before planning")
	}
	basis := path + "/v" + strconv.Itoa(version)
	verdict, err := judge(ctx, root, cfg, feature, version, mainline, run)
	if err != nil {
		return refuse(err.Error(), "repair the binding lock or current mainline and retry")
	}
	if verdict.Ready {
		// Verification and readiness perform further origin reads. Accept their
		// result only while the mainline and selected version still match the
		// view against which readiness was computed.
		currentMainline, currentVersion, err := currentLock(ctx, root, feature, cfg, run)
		if err != nil {
			return refuse(err.Error(), "restore access to the current mainline and binding-lock history, then retry")
		}
		if currentMainline != mainline || currentVersion != version {
			return refuse("mainline or latest binding lock changed during readiness verification", "retry planning against the current mainline and latest binding lock")
		}
		return basis, nil
	}
	defects := []schema.Defect{def(basis, "precheck/locked-readiness", "binding locked version is not ready against current mainline", "resolve the named readiness defects before drafting or projecting a plan")}
	for _, unmet := range verdict.Unmet {
		defects = append(defects, def(unmet.Item, "precheck/"+unmet.Condition, unmet.Condition+": "+unmet.Item, unmet.Remedy))
	}
	return "", defects
}

func currentLock(ctx context.Context, root, feature string, cfg config.Config, run lifecycle.RunCommand) (string, int, error) {
	ref := "refs/heads/" + cfg.Branch
	base, err := run(ctx, root, nil, "git", "ls-remote", "--refs", cfg.Remote, ref)
	if err != nil {
		return "", 0, fmt.Errorf("resolve current mainline: %w", err)
	}
	fields := strings.Fields(string(base))
	if len(fields) != 2 || fields[1] != ref || (len(fields[0]) != 40 && len(fields[0]) != 64) || strings.Trim(fields[0], "0123456789abcdef") != "" {
		return "", 0, fmt.Errorf("resolve current mainline: %s must advertise exactly one full commit OID", ref)
	}
	mainline := fields[0]
	// FETCH_HEAD belongs to the shared checkout. Import the advertised object
	// by OID without reading or replacing that mutable fetch result.
	if _, err := run(ctx, root, nil, "git", "fetch", "--no-tags", "--no-write-fetch-head", "--no-recurse-submodules", cfg.Remote, mainline); err != nil {
		return "", 0, fmt.Errorf("fetch current mainline: %w", err)
	}
	version, err := latestVersion(ctx, root, cfg.Remote, feature, run)
	return mainline, version, err
}

func def(file, op, reason, remedy string) schema.Defect {
	return schema.Defect{Verb: "plangate", File: file, Op: op, Reason: reason, Remedy: remedy}
}
