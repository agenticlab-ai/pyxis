package checks

import (
	"context"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/verify/diffset"
	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/registry"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

func TestMachineryProtection_RegisteredOnRung0(t *testing.T) {
	check := registry.GetCheck(MachineryProtectionCheckID)
	if check == nil {
		t.Fatal("machinery-protection is not registered")
	}
	if check.Rung != "0" {
		t.Errorf("rung = %q, want 0", check.Rung)
	}
	if !check.AuthorIndependent {
		t.Error("machinery-protection must be author-independent: it runs no repo-supplied verb")
	}
	if len(check.VerbDeps) != 0 {
		t.Errorf("VerbDeps = %v, want none (verbless check)", check.VerbDeps)
	}
}

func TestMachineryPaths_DerivedConstantSet(t *testing.T) {
	paths := MachineryPaths(nil)
	for _, want := range []string{invoke.ManifestRelPath, AcquisitionPinRelPath, invoke.VerdictRelPath} {
		if !paths[want] {
			t.Errorf("MachineryPaths(nil) missing %q: %v", want, paths)
		}
	}
	if len(paths) != 3 {
		t.Errorf("MachineryPaths(nil) = %v, want exactly the 3 constant-derived paths", paths)
	}
}

func TestMachineryPaths_IncludesDeclaredBaseline(t *testing.T) {
	m := &schema.Manifest{Version: "1", Stack: "go", BaselinePath: "docs/verify-baseline.json"}
	paths := MachineryPaths(m)
	if !paths["docs/verify-baseline.json"] {
		t.Errorf("MachineryPaths did not include the declared baseline path: %v", paths)
	}
	if len(paths) != 4 {
		t.Errorf("MachineryPaths(with baseline) = %v, want exactly 4 paths", paths)
	}
}

func TestMachineryPaths_NoDeclaredBaselineAddsNothing(t *testing.T) {
	m := &schema.Manifest{Version: "1", Stack: "go"} // BaselinePath left unset
	paths := MachineryPaths(m)
	if len(paths) != 3 {
		t.Errorf("MachineryPaths(no baseline) = %v, want exactly the 3 constant-derived paths", paths)
	}
}

// TestMachineryPaths_BaselinePathNormalized proves a "./"-prefixed baseline
// spelling — a natural, common way to write a relative path, and one
// resolveBaselineHash (invoke.go) already reads correctly via
// filepath.Join's implicit cleaning — still exact-matches the canonical form
// git reports in a real diff. Without normalizing, this spelling would read
// fine yet silently escape machinery protection.
func TestMachineryPaths_BaselinePathNormalized(t *testing.T) {
	m := &schema.Manifest{Version: "1", Stack: "go", BaselinePath: "./docs/verify-baseline.json"}
	paths := MachineryPaths(m)
	if !paths["docs/verify-baseline.json"] {
		t.Errorf("MachineryPaths did not normalize the './'-prefixed baseline path: %v", paths)
	}
	if len(paths) != 4 {
		t.Errorf("MachineryPaths(with baseline) = %v, want exactly 4 paths", paths)
	}
}

// TestMachineryProtection_NormalizedBaselinePathReds is the end-to-end leg:
// a diff modifying the baseline reds even when the manifest spells its
// declared path with a "./" prefix.
func TestMachineryProtection_NormalizedBaselinePathReds(t *testing.T) {
	manifest := &schema.Manifest{Version: "1", Stack: "go", BaselinePath: "./docs/verify-baseline.json"}
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusModified, Path: "docs/verify-baseline.json"},
	}}
	env := &Env{Manifest: manifest, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runMachineryProtection(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail (a './'-prefixed manifest spelling must not escape protection)", result.State)
	}
	if len(findings) != 1 || findings[0].Path != "docs/verify-baseline.json" {
		t.Errorf("findings = %+v, want one machinery finding naming docs/verify-baseline.json", findings)
	}
}

// TestMachineryProtection_BaselineAdditionReds is catch seed 3's first leg:
// a baseline addition riding a proposal diff.
func TestMachineryPaths_AbsoluteAndBackslashBaselineNormalized(t *testing.T) {
	// However the manifest spells the baseline path, it must normalize to the
	// git workspace-relative form so it can't read correctly yet escape
	// protection (an absolute or backslash spelling previously did).
	for _, spelling := range []string{
		"docs/verify-baseline.json",
		"./docs/verify-baseline.json",
		"/docs/verify-baseline.json",
		`docs\verify-baseline.json`,
	} {
		m := &schema.Manifest{Version: "1", Stack: "go", BaselinePath: spelling}
		if !MachineryPaths(m)["docs/verify-baseline.json"] {
			t.Errorf("baseline spelled %q not normalized into the protected set: %v", spelling, MachineryPaths(m))
		}
	}
}

func TestMachineryProtection_BaselineAdditionReds(t *testing.T) {
	manifest := &schema.Manifest{Version: "1", Stack: "go", BaselinePath: "docs/verify-baseline.json"}
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "docs/verify-baseline.json"},
	}}
	env := &Env{Manifest: manifest, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runMachineryProtection(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) != 1 || findings[0].Class != "machinery" || findings[0].Path != "docs/verify-baseline.json" {
		t.Errorf("findings = %+v, want one machinery finding naming docs/verify-baseline.json", findings)
	}
}

// TestMachineryProtection_ManifestVerbUnbindingReds is catch seed 3's second
// leg: a manifest verb-unbinding riding a proposal diff — end to end through
// real git so the manifest's own path comes from an actual diff.
func TestMachineryProtection_ManifestVerbUnbindingReds(t *testing.T) {
	dir := initRepo(t)
	writeFile(t, dir, ".pyxis/codegate.json", `{"version":"1","stack":"go","verbs":{"lint":{"command":"go vet ./..."}}}`)
	base := commit(t, dir, "base with a bound lint verb")

	// Unbind the lint verb — exactly the ABUSE-3 gaming move FR-8 exists to catch.
	writeFile(t, dir, ".pyxis/codegate.json", `{"version":"1","stack":"go","verbs":{}}`)
	head := commit(t, dir, "unbind lint")

	ds, err := diffset.Extract(context.Background(), dir, base, head)
	if err != nil {
		t.Fatalf("diffset.Extract: %v", err)
	}

	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runMachineryProtection(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) != 1 || findings[0].Path != invoke.ManifestRelPath {
		t.Errorf("findings = %+v, want one finding naming %s", findings, invoke.ManifestRelPath)
	}
}

// TestMachineryProtection_PinDowngradeReds is catch seed 3's third leg.
func TestMachineryProtection_PinDowngradeReds(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusModified, Path: AcquisitionPinRelPath},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runMachineryProtection(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) != 1 || findings[0].Path != AcquisitionPinRelPath {
		t.Errorf("findings = %+v, want one finding naming %s", findings, AcquisitionPinRelPath)
	}
}

// TestMachineryProtection_VerdictPathReds proves design §3.6's DINV-2
// belt-and-braces claim ("the verdict path also joins the
// machinery-protection set"): a diff that touches the verdict artifact's own
// path — e.g. a `git add -f` past the scratch exclusion — reds here even if
// the primary exclusion/tracked-path refusal mechanisms were bypassed.
func TestMachineryProtection_VerdictPathReds(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: invoke.VerdictRelPath},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runMachineryProtection(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" {
		t.Fatalf("state = %q, want fail", result.State)
	}
	if len(findings) != 1 || findings[0].Path != invoke.VerdictRelPath {
		t.Errorf("findings = %+v, want one finding naming %s", findings, invoke.VerdictRelPath)
	}
}

// TestMachineryProtection_RenameOutOfProtectedPathReds proves the
// TouchedPaths() rename coverage: moving the manifest itself away is
// gate-gaming as much as editing it in place.
func TestMachineryProtection_RenameOutOfProtectedPathReds(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusRenamed, OldPath: invoke.ManifestRelPath, Path: "elsewhere.json"},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runMachineryProtection(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "fail" || len(findings) != 1 || findings[0].Path != invoke.ManifestRelPath {
		t.Errorf("renaming the manifest away was not caught: state=%q findings=%+v", result.State, findings)
	}
}

// TestMachineryProtection_NonMachineryDiffPasses is catch seed 3's negative
// control: an ordinary source diff never triggers the finding.
func TestMachineryProtection_NonMachineryDiffPasses(t *testing.T) {
	ds := &diffset.DiffSet{Files: []diffset.FileChange{
		{Status: diffset.StatusAdded, Path: "internal/widget/widget.go"},
		{Status: diffset.StatusModified, Path: "internal/widget/widget_test.go"},
	}}
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New(), Diff: ds}
	result, findings, err := runMachineryProtection(context.Background(), env)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if result.State != "pass" || len(findings) != 0 {
		t.Errorf("non-machinery diff produced findings: state=%q findings=%+v", result.State, findings)
	}
}

func TestMachineryProtection_NilDiffErrors(t *testing.T) {
	env := &Env{Manifest: &schema.Manifest{Version: "1", Stack: "go"}, Ecosystem: ecosystem.New()}
	if _, _, err := runMachineryProtection(context.Background(), env); err == nil {
		t.Error("expected an error when no diff set is provided")
	}
}
