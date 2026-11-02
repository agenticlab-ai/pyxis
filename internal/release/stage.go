package release

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/payload"
)

// The payload staging contract (installer-v1 §3.1, §3.3; the T11 charter):
// the staged tree is the OPERAND-SIDE layout, containing exactly
//
//   - every git-TRACKED file under .pyxis/** — tracked-only staging is what
//     keeps the gitignored caches (.pyxis/bin/, .pyxis/feedback-queue/,
//     .pyxis/contract-gate-reports/, vendor node_modules/, .pyxis/scratch/ scratch)
//     out of the payload, exactly as the class policy's header assumes;
//   - the tracked docs/tech-design/index.* design indexes — the generated-
//     class rows the policy and the shape's manifest example enumerate
//     (relmanifest's real-tree test stages the same set);
//   - the T9 overlay mapping: every file under .pyxis/dist/github-workflows/
//     EXCEPT README.md copies per file, by name, to .github/workflows/<name>
//     (the README beside the overlay is packer documentation, never overlay
//     content). The overlay SOURCE also ships at its own .pyxis path — the
//     payload carries both, and the class policy classes both.
//
// Staging reads the WORKING TREE's bytes for each tracked path; the release
// workflow runs on a clean tag checkout, so tree == commit == artifacts
// (§3.1's provenance hop). Enumeration is `git ls-files` — the index, no
// commit required — so fixture repos need only init + add.
const (
	overlaySrcDir = ".pyxis/dist/github-workflows"
	overlayDstDir = ".github/workflows"
	overlayDoc    = "README.md" // packer documentation, never mapped
)

// stagePathspecs is the tracked-payload enumeration handed to git ls-files.
var stagePathspecs = []string{".pyxis", "docs/tech-design/index.*"}

// StagePayload stages the payload tree from repoRoot into dst (which must
// exist and be empty of prior staging concerns — callers hand a fresh temp
// dir) and returns the staged payload-relative slash paths, sorted by git.
func StagePayload(ctx context.Context, repoRoot, dst string) ([]string, error) {
	tracked, err := listTracked(ctx, repoRoot)
	if err != nil {
		return nil, err
	}
	if len(tracked) == 0 {
		return nil, fmt.Errorf("release: no tracked payload files under %v in %s — not a pyxis source tree?", stagePathspecs, repoRoot)
	}
	staged := make([]string, 0, len(tracked)+2)
	for _, rel := range tracked {
		if err := copyStaged(repoRoot, dst, rel, rel); err != nil {
			return nil, err
		}
		staged = append(staged, rel)
	}
	// Authoring guides cite Pyxis's own frozen designs and effective PRDs.
	// Installed operands have their own docs/ tree, so ship source snapshots
	// under the engine-owned reference tree instead of writing Pyxis contracts
	// into the operand's documents. A missing cited source refuses the pack.
	references, err := stageAuthoringReferences(repoRoot, dst)
	if err != nil {
		return nil, err
	}
	staged = append(staged, references...)
	// The overlay mapping (T9's README): per file, by name, README excluded.
	mapped, err := overlayMap(dst)
	if err != nil {
		return nil, err
	}
	for _, name := range mapped {
		src := path.Join(overlaySrcDir, name)
		dstRel := path.Join(overlayDstDir, name)
		if dstRel == payload.InstalledWorkflowTarget {
			// The installed CI workflow is a GENERATED template carrying
			// {{branch}}/{{remote}} and bootstrap-pin tokens
			// (config-driven-portability-v1 §3.1, FR-4). The overlay must NOT
			// stage a token-bearing file into an operand's .github/workflows/:
			// it renders the DEFAULT (config.Defaults() → origin/main) with
			// empty pins. converge re-renders the operand's copy per-config and
			// per-release from the same template (the overlay SOURCE ships
			// engine-verbatim under .pyxis/**, the single source — DINV-4).
			if err := renderStagedWorkflow(dst, src, dstRel); err != nil {
				return nil, err
			}
		} else if err := copyStaged(dst, dst, src, dstRel); err != nil {
			return nil, err
		}
		staged = append(staged, dstRel)
	}
	if len(mapped) == 0 {
		return nil, fmt.Errorf("release: overlay home %s stages no workflow files — the installed-repo gate workflow variant (T9) is a pack prerequisite (installer-v1 §3.10)", overlaySrcDir)
	}
	return staged, nil
}

var (
	authoringDesignReference = regexp.MustCompile(`docs/tech-design/[a-z][a-z0-9-]*\.md`)
	authoringPRDReference    = regexp.MustCompile(`docs/prds/[a-z][a-z0-9-]*/`)
	authoringTemplatePath    = regexp.MustCompile(`docs/templates/[a-z][a-z0-9-]*\.md`)
	authoringSourceCommand   = regexp.MustCompile(`go run \./cmd/([a-z][a-z0-9-]*)`)
	// A few guides also give bare engine-tool commands in inline code or
	// fenced command blocks. Match only those command positions and the verbs
	// used by the shipped guides, so prose such as "the docgate gate suite"
	// remains prose. Source checkout commands are rendered before this pass.
	authoringBareCommand = regexp.MustCompile("(?m)(^[ \\t]*|`)(docgate|plangate|carina|feature-drive) (add-incident|context-pack|contract-gate|create|diff|dimensions|dogfood|fold|gate|graduate|lint-structure|precheck|project|render|report|routes|signal|validate-amendments|verify-generated)\\b")
)

// stageAuthoringReferences renders the source checkout's runnable guides and
// templates into installed copies, then stages the Pyxis contracts they cite.
// The root policy documents and templates are read transitively from direct
// skills, so their source-only commands must be rendered too. Source files
// keep their go run commands and source paths; only staged copies use the
// installed CLI and release-pinned reference paths. Literal Pyxis contracts
// are mirrored, while operand paths with placeholders stay operand-owned.
// The release manifest pins every rendered file and copied contract byte.
func stageAuthoringReferences(repoRoot, dst string) ([]string, error) {
	var guides []string
	for _, pattern := range []string{
		filepath.Join(dst, ".pyxis", "*.md"),
		filepath.Join(dst, ".pyxis", "authoring", "*.md"),
		filepath.Join(dst, ".pyxis", "skills", "*", "SKILL.md"),
		filepath.Join(dst, ".pyxis", "templates", "*.md"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, fmt.Errorf("release: scan authoring references: %w", err)
		}
		guides = append(guides, matches...)
	}
	refs := map[string]bool{}
	templates := map[string]bool{}
	for _, guide := range guides {
		body, err := os.ReadFile(guide)
		if err != nil {
			return nil, fmt.Errorf("release: read authoring guide %s: %w", guide, err)
		}
		rendered := authoringSourceCommand.ReplaceAllStringFunc(string(body), func(command string) string {
			tool := strings.TrimPrefix(command, "go run ./cmd/")
			if tool == "pyxis" {
				return "pyxis"
			}
			return "pyxis exec " + tool + " --"
		})
		rendered = authoringBareCommand.ReplaceAllString(rendered, "${1}pyxis exec ${2} -- ${3}")
		rendered = authoringTemplatePath.ReplaceAllStringFunc(rendered, func(ref string) string {
			staged := path.Join(".pyxis/templates", path.Base(ref))
			templates[staged] = true
			return staged
		})
		rendered = authoringDesignReference.ReplaceAllStringFunc(rendered, func(ref string) string {
			refs[ref] = true
			return path.Join(".pyxis/reference", ref)
		})
		rendered = authoringPRDReference.ReplaceAllStringFunc(rendered, func(ref string) string {
			contract := path.Join(ref, "effective.md")
			refs[contract] = true
			return path.Join(".pyxis/reference", contract)
		})
		if rendered != string(body) {
			if err := os.WriteFile(guide, []byte(rendered), 0o644); err != nil {
				return nil, fmt.Errorf("release: render authoring guide %s: %w", guide, err)
			}
		}
	}
	for template := range templates {
		info, err := os.Stat(filepath.Join(dst, filepath.FromSlash(template)))
		if err != nil || !info.Mode().IsRegular() {
			return nil, fmt.Errorf("release: authoring template %s is missing from the staged payload", template)
		}
	}
	paths := make([]string, 0, len(refs))
	for ref := range refs {
		paths = append(paths, path.Join(".pyxis/reference", ref))
	}
	sort.Strings(paths)
	for _, staged := range paths {
		ref := strings.TrimPrefix(staged, ".pyxis/reference/")
		if err := copyStaged(repoRoot, dst, ref, staged); err != nil {
			return nil, fmt.Errorf("release: authoring reference %s: %w", ref, err)
		}
	}
	return paths, nil
}

// listTracked enumerates the tracked payload paths via git ls-files -z,
// excluding engine-internal test material (isDevOnlyPayloadPath).
func listTracked(ctx context.Context, repoRoot string) ([]string, error) {
	args := append([]string{"-C", repoRoot, "ls-files", "-z", "--full-name", "--"}, stagePathspecs...)
	cmd := exec.CommandContext(ctx, "git", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("release: git ls-files in %s: %w (%s)", repoRoot, err, strings.TrimSpace(stderr.String()))
	}
	var out []string
	for p := range strings.SplitSeq(stdout.String(), "\x00") {
		if p == "" || isDevOnlyPayloadPath(p) {
			continue
		}
		out = append(out, p)
	}
	return out, nil
}

// devOnlyPayloadSegments are the path segments marking engine-internal test
// material the payload never ships: the bash shim tests (.pyxis/scripts/tests),
// the render/skill fixtures (.pyxis/ui/fixtures, .pyxis/skills/*/fixtures), and
// any Go testdata. An operand runs the skills, never the engine's tests, and
// no skill reads its fixtures/ at runtime — the runtime inputs (calibration.md,
// SKILL.md) live outside these segments. Excluding them
// keeps the payload lean and drops the resolver-referencing shim tests the
// retired-resolver precondition flags.
var devOnlyPayloadSegments = map[string]bool{"tests": true, "fixtures": true, "testdata": true}

// devOnlyPayloadPrefixes mark source-repo-only operational surface the operand
// payload never ships. .pyxis/hooks/ holds the SOURCE repo's desk git hooks —
// source-mode delegates that run `go run ./cmd/pyxis` through the resolver
// scripts (they need the engine source and a Go toolchain). An installed
// operand carries neither: at init it materializes the .pyxis/dist/operand-hooks/
// variant (which calls the pinned `pyxis` binary directly) into .githooks and
// points core.hooksPath there. Shipping the source hooks would bloat the operand
// with files it never runs AND trip the retired-resolver release precondition
// (they name the resolver scripts) — the same "don't ship dev surface" rule the
// dev-only segments above apply to tests and fixtures.
// Forgejo source CI templates invoke source-only resolver/build tooling. They
// remain available to pyxis-release workflows in the source checkout, while
// installed repositories receive the separate binary-only gate template.
// Retained evidence belongs to the source repository's history; it is not an
// installed runtime input and may describe retired source tooling.
var devOnlyPayloadPrefixes = []string{".pyxis/hooks/", ".pyxis/logs/", ".pyxis/evidence/", ".pyxis/dist/forgejo-source-workflows/"}

// isDevOnlyPayloadPath reports whether a tracked payload path is engine-internal
// (a dev-only segment) or source-repo-only surface (a dev-only prefix), and is
// therefore excluded from the operand payload.
func isDevOnlyPayloadPath(p string) bool {
	// This is the Source repository's live, operator-owned policy. Shipping
	// it would install Source's grants into a consumer and cause converge to
	// overwrite the consumer's own policy. Init seeds the shipped TEMPLATE
	// once at floor; existing policies remain outside engine ownership.
	if p == payload.AuthorityContractTarget {
		return true
	}
	for seg := range strings.SplitSeq(p, "/") {
		if devOnlyPayloadSegments[seg] {
			return true
		}
	}
	for _, pre := range devOnlyPayloadPrefixes {
		if strings.HasPrefix(p, pre) {
			return true
		}
	}
	return false
}

// overlayMap lists the overlay files to map, from the STAGED tree (so only
// tracked overlay content maps), README excluded, sorted by ReadDir.
func overlayMap(stagedRoot string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(stagedRoot, filepath.FromSlash(overlaySrcDir)))
	if os.IsNotExist(err) {
		return nil, nil // reported by StagePayload as the missing-variant prerequisite
	}
	if err != nil {
		return nil, fmt.Errorf("release: read overlay home %s: %w", overlaySrcDir, err)
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			return nil, fmt.Errorf("release: overlay home %s contains a directory %q — the mapping is per file, by name (T9's README)", overlaySrcDir, e.Name())
		}
		if e.Name() == overlayDoc {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}

// copyStaged copies one payload file srcRel (under srcRoot) to dstRel (under
// dstRoot), creating directories, refusing non-regular sources, and
// normalizing the mode to 0755/0644 by the source's owner-exec bit — the
// same normalization the deterministic archive applies.
func copyStaged(srcRoot, dstRoot, srcRel, dstRel string) error {
	src := filepath.Join(srcRoot, filepath.FromSlash(srcRel))
	info, err := os.Lstat(src)
	if err != nil {
		return fmt.Errorf("release: stage %s: %w", srcRel, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("release: tracked payload entry %s is not a regular file (%s) — the packed payload carries regular files only", srcRel, info.Mode())
	}
	b, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("release: stage %s: %w", srcRel, err)
	}
	dst := filepath.Join(dstRoot, filepath.FromSlash(dstRel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("release: stage %s: %w", dstRel, err)
	}
	mode := os.FileMode(0o644)
	if info.Mode()&0o100 != 0 {
		mode = 0o755
	}
	if err := os.WriteFile(dst, b, mode); err != nil {
		return fmt.Errorf("release: stage %s: %w", dstRel, err)
	}
	return nil
}

// renderStagedWorkflow stages the installed CI workflow by RENDERING the
// overlay-source template (srcRel, under stagedRoot) with the DEFAULT config
// (config-driven-portability-v1 §3.1, FR-4): the shipped template carries
// substitution tokens, so a verbatim copy would land a broken token-bearing
// workflow in the operand's .github/workflows/. The binary pins cannot be
// rendered here (payload.RenderStagedInstalledWorkflow says why); converge
// renders the operand's real copy from the SAME template (the overlay source,
// shipped engine-verbatim under .pyxis/** — the single source, DINV-4/AP-11),
// and this one exists to carry the manifest's generated row. The mode is 0644 (a
// workflow is never executable), matching the deterministic archive's
// normalization for a non-exec source.
func renderStagedWorkflow(stagedRoot, srcRel, dstRel string) error {
	src := filepath.Join(stagedRoot, filepath.FromSlash(srcRel))
	tmpl, err := os.ReadFile(src)
	if err != nil {
		return fmt.Errorf("release: stage %s: %w", srcRel, err)
	}
	d := config.Defaults()
	rendered, err := payload.RenderStagedInstalledWorkflow(tmpl, d.Branch, d.Remote)
	if err != nil {
		return fmt.Errorf("release: render installed workflow %s from %s: %w — the shipped template's substitution set is payload.Vars[InstalledWorkflow]", dstRel, srcRel, err)
	}
	dst := filepath.Join(stagedRoot, filepath.FromSlash(dstRel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return fmt.Errorf("release: stage %s: %w", dstRel, err)
	}
	if err := os.WriteFile(dst, rendered, 0o644); err != nil {
		return fmt.Errorf("release: stage %s: %w", dstRel, err)
	}
	return nil
}

// sourceCommit resolves the staged tree's provenance commit (tag == commit
// == artifacts, §3.1).
func sourceCommit(ctx context.Context, repoRoot string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "-C", repoRoot, "rev-parse", "HEAD")
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("release: git rev-parse HEAD in %s: %w (%s)", repoRoot, err, strings.TrimSpace(stderr.String()))
	}
	return strings.TrimSpace(stdout.String()), nil
}
