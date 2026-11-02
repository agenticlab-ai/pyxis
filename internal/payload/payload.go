// Package payload declares the shipped-instantiable contract of the release
// payload's template set (installer plan T9; installer-v1 §3.3, §3.5 step 4):
// where each instantiable lives in the payload tree, the operand path init
// instantiates it at, and — for the two knob-substituted templates — the
// closed substitution-variable set. The declaration lives in code (AP-10) so
// the seeding engine (T8) consumes it and the conformance tests hold it:
// every `{{name}}` a substituted template carries is declared here, and
// substitution refuses unresolved variables by name (fail closed, the I3
// discipline). The package writes nothing.
package payload

import (
	"fmt"
	"regexp"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Payload-relative homes of the shipped instantiables (the solution shape's
// "New shipped instantiables" set) and of the installed-repo workflow
// variant with its operand-side mapping target (installer-v1 §3.10; the
// overlay contract is documented in the README beside the variant).
const (
	TemplatesDir = ".pyxis/templates"

	ProductVisionSkeleton          = TemplatesDir + "/product-vision-skeleton.md"
	ArchitecturePrinciplesSkeleton = TemplatesDir + "/architecture-principles-skeleton.md"
	OperandAgentsManual            = TemplatesDir + "/operand-agents-manual.md"
	WelcomeItem                    = TemplatesDir + "/welcome-item.md"
	HarnessImplementNowSkill       = TemplatesDir + "/harness-implement-now-skill.md"
	AuthorityContractTemplate      = TemplatesDir + "/authority-contract-template.md"
	AuthorityContractTarget        = schema.AuthorityContractPath

	InstalledWorkflow       = ".pyxis/dist/github-workflows/docs-gate.yml"
	InstalledWorkflowTarget = ".github/workflows/docs-gate.yml"
	ForgejoWorkflow         = ".pyxis/dist/forgejo-workflows/docs-gate.yml"
	ForgejoWorkflowTarget   = ".forgejo/workflows/docs-gate.yml"

	// OperandAgentsManualTarget is where the knob-substituted manual lands —
	// instantiated once, then repo-owned (installer-v1 §3.5 step 4, DINV-3).
	OperandAgentsManualTarget = "AGENTS.md"
)

// SkeletonTargets maps each auto-seeded governance-doc skeleton to the operand
// path init instantiates it at, byte-verbatim: the two skeletons carry no
// substitution variables — their fill points are the human author's
// (prd:installer/NG-1). Only the two governing documents are seeded; every
// other top-level document is opt-in — a repo adds one if it wants it, and
// nothing is mandated to exist. The living maps (docs/domain-model.md,
// docs/system-design.md) are not seeded here; a repo that authors one is still
// first-class — docgate's check-maps fences its registry anchors whenever it is
// present (config's system_maps).
//
// It is a PURE FUNCTION of the governing-doc path config (config-driven-
// portability-v1 §3.2, FR-3; DINV-1) — each target is the configured location
// of that governing document, recomputed at every call with no cached or
// materialized derived state (DINV-1's no-second-source rule in miniature: a
// cache would be a second thing that can drift from config). On a default
// config the two targets are byte-identical to the former literals (DINV-2).
func SkeletonTargets(cfg config.Config) map[string]string {
	return map[string]string{
		ProductVisionSkeleton:          cfg.ProductVision,
		ArchitecturePrinciplesSkeleton: cfg.ArchitecturePrinciples,
	}
}

// Vars is the closed substitution-variable set per knob-substituted template
// (sorted). A variable outside a template's declared set — in the template or
// in the values handed to Substitute — is a defect, not an extension point:
// the set grows only with the knob schema it mirrors (installer-v1 §3.7).
var Vars = map[string][]string{
	OperandAgentsManual:      {"argus_enabled", "branch", "ci_mode", "tracker"},
	WelcomeItem:              {"gates_active", "prds_path", "tech_designs_path", "tracker"},
	HarnessImplementNowSkill: {"backend_id", "harness_name"},
	// The installed CI workflow — a GENERATED surface converge renders from the
	// repo config (config-driven-portability-v1 §3.1, FR-4): its push trigger,
	// fetch, and gate merge-base carry branch+remote as substitution points
	// because a CI host evaluates the trigger with no config read. The rest are
	// the bootstrap shim's inputs, rendered from the verified release manifest
	// (WorkflowPins). There is no path key, since the installed variant references
	// no target-facing doc path.
	InstalledWorkflow: {"artifact_source", "branch", "linux_amd_checksum", "linux_arm_checksum", "remote", "version"},
	ForgejoWorkflow:   {"artifact_source", "artifact_token_env", "branch", "linux_amd_checksum", "linux_arm_checksum", "remote", "runner_label", "version"},
}

// varRe is the one substitution-token grammar: `{{name}}`, lowercase
// snake-case names — the SCAF-3 substitution-variable convention.
var varRe = regexp.MustCompile(`\{\{([a-z_]+)\}\}`)

// VarsIn returns the sorted, de-duplicated substitution variables a template
// carries.
func VarsIn(doc []byte) []string {
	seen := map[string]bool{}
	var out []string
	for _, m := range varRe.FindAllSubmatch(doc, -1) {
		name := string(m[1])
		if !seen[name] {
			seen[name] = true
			out = append(out, name)
		}
	}
	slices.Sort(out)
	return out
}

// Substitute resolves every `{{name}}` token against vals and fails closed:
// a token whose variable is missing from vals — or any `{{` remnant the
// grammar did not admit — is a named error and nothing partial is returned.
func Substitute(doc []byte, vals map[string]string) ([]byte, error) {
	var missing []string
	out := varRe.ReplaceAllFunc(doc, func(m []byte) []byte {
		name := string(varRe.FindSubmatch(m)[1])
		v, ok := vals[name]
		if !ok {
			missing = append(missing, name)
			return m
		}
		return []byte(v)
	})
	if len(missing) > 0 {
		slices.Sort(missing)
		return nil, fmt.Errorf("unresolved substitution variable(s): %s", strings.Join(slices.Compact(missing), ", "))
	}
	if i := strings.Index(string(out), "{{"); i >= 0 {
		line := 1 + strings.Count(string(out[:i]), "\n")
		return nil, fmt.Errorf("line %d: substitution remnant %q — tokens are {{lower_snake_case}}", line, snippet(out[i:]))
	}
	return out, nil
}

// WorkflowPins are the bootstrap shim's inputs in the installed CI workflow: the
// pyxis release and its Linux binary hashes, taken from the release manifest the
// caller verified against system.manifest_sha256, plus the source a hosted runner
// fetches from. They name a built release, so only a consumer of that release can
// supply them; RenderStagedInstalledWorkflow is the pack-time form without them.
type WorkflowPins struct {
	Version        string
	LinuxARM64     string
	LinuxAMD64     string
	ArtifactSource string
}

// RenderInstalledWorkflow renders the installed CI workflow from its shipped
// template bytes, substituting the repo's branch and remote and the bootstrap
// pins — the ONE place the installed-workflow substitution set (matching Vars[
// InstalledWorkflow]) is spelled for a consumer (AP-11). It fails closed: an
// empty pin stays unresolved and Substitute names it, so a release that does not
// pin a binary refuses here rather than rendering a workflow the shim would
// refuse on every run.
func RenderInstalledWorkflow(template []byte, branch, remote string, pins WorkflowPins) ([]byte, error) {
	values := map[string]string{"branch": branch, "remote": remote}
	for key, v := range map[string]string{
		"version":            pins.Version,
		"linux_arm_checksum": pins.LinuxARM64,
		"linux_amd_checksum": pins.LinuxAMD64,
		"artifact_source":    pins.ArtifactSource,
	} {
		if v == "" {
			continue
		}
		// Every pin sits inside a single-quoted YAML scalar.
		if unsafeWorkflowValue(v) {
			return nil, fmt.Errorf("workflow: unsafe value for %s", key)
		}
		values[key] = v
	}
	return Substitute(template, values)
}

// RenderStagedInstalledWorkflow is the pack-time render of the same template,
// for the payload row at InstalledWorkflowTarget. The manifest records the payload
// archive's hash, so the archive is sealed before a manifest exists, and the row
// is generated-class: every consumer renders its own copy through
// RenderInstalledWorkflow and never reads these bytes. The pins therefore render
// empty, which the bootstrap shim refuses by name if the copy is ever run as is.
func RenderStagedInstalledWorkflow(template []byte, branch, remote string) ([]byte, error) {
	return Substitute(template, map[string]string{
		"branch": branch, "remote": remote,
		"version": "", "linux_arm_checksum": "", "linux_amd_checksum": "", "artifact_source": "",
	})
}

// snippet bounds an error excerpt to one short, single-line fragment.
func snippet(b []byte) string {
	s := string(b)
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 24 {
		s = s[:24] + "…"
	}
	return s
}
