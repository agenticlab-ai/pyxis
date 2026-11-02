// Package seed implements `codegate seed` — the one-shot authoring aid of
// prd:implementation-verification/FR-20 (amendment 001): where recognized
// stack marker files exist and no verification manifest does, write a
// closed-schema-valid template manifest declaring the detected stack, for
// the operator to own and edit. The harness's runtime reading is unchanged
// (FR-5) — this package writes the declared artifact once; it never detects
// anything at verification time.
//
// Detection reads the ecosystem defaults' marker set (the same single
// source doctor's code-without-manifest advisory and adopt's stack
// detection read), never a second, hand-kept list. Both edges fail closed:
// an existing manifest is operator-owned data seed refuses to touch
// (UB-10), and a tree with no recognized markers — or markers matching
// multiple stacks — gets the hand-authoring remedy, never a guess (UB-11).
package seed

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/verify/ecosystem"
	"github.com/agenticlab-ai/pyxis/internal/verify/invoke"
	"github.com/agenticlab-ai/pyxis/internal/verify/schema"
)

// manifestVersion is the schema version stamped into every seeded manifest,
// matching the hand-authored corpus (the in-tree fixtures).
const manifestVersion = "1.0.0"

// ladderVerbNames is the verb set a placeholder template declares — the four
// names design:codegate-v1 §3.7's rungs consume ("format-check" and "lint"
// on rung 0, "build" on rung 1, "test" on rungs 2–3). Stacks that ship real
// ecosystem defaults already bind exactly this set; placeholders mirror it
// so an edited template feeds rungs 0–3. exec-probe (rung 4) is deliberately
// absent: an unbound verb there is FR-6's skip-with-record, while a
// placeholder would turn that skip into a spawn error.
var ladderVerbNames = []string{"build", "format-check", "lint", "test"}

// Package seams keep Seed's race-sensitive exclusive-create boundaries
// deterministic under unit test. Production leaves both bound to the concrete
// implementations below.
var (
	seedMarshalTemplate = marshalTemplate
	seedWriteExclusive  = writeExclusive
)

// Result reports a successful seed for the caller to narrate.
type Result struct {
	Path        string // the manifest's workspace-relative path
	Stack       string // the detected stack
	Placeholder bool   // true when the verb commands are named placeholders
}

// Refusal is a named seed refusal or failure: Message for the human, Code
// the process exit code — invoke.ExitConfigErr for the contract refusals
// (UB-10, UB-11) and invoke.ExitRungErr for operational failures. Seed
// emits no verdict artifact: it verifies nothing, so the run/capture
// refusal protocol (ConfigError's verdict emission) does not apply.
type Refusal struct {
	Message string
	Code    int
}

// detection is one stack whose marker files are present in the workspace.
type detection struct {
	stack   string
	markers []string
}

// Seed writes the template manifest for the workspace's detected stack, or
// refuses by name. On a contract refusal (UB-10, UB-11) nothing is written —
// not even the manifest's parent directory; only an operational write
// failure can leave the freshly created (empty) parent behind.
func Seed(workspace string, defs *ecosystem.Defaults) (*Result, *Refusal) {
	manifestAbs := filepath.Join(workspace, filepath.FromSlash(invoke.ManifestRelPath))

	// UB-10: an existing manifest is operator-owned — refuse, change
	// nothing. Lstat, not Stat: a symlink at the manifest path — even a
	// dangling one — is an existing operator-placed entry, never a hole to
	// write through (the create below is O_EXCL for the same reason).
	if _, err := os.Lstat(manifestAbs); err == nil {
		return nil, &Refusal{
			Message: fmt.Sprintf("a verification manifest already exists at %s — it is operator-owned; edit it directly, seed never overwrites it", invoke.ManifestRelPath),
			Code:    invoke.ExitConfigErr,
		}
	}

	found := detectStacks(workspace, defs)

	// UB-11: no recognized markers, or markers matching multiple stacks —
	// write nothing and name the hand-authoring remedy. Never guess.
	switch {
	case len(found) == 0:
		return nil, &Refusal{
			Message: fmt.Sprintf("no recognized stack marker files (%s) — hand-author %s yourself, declaring the stack this repository verifies as and its verb commands", allMarkers(defs), invoke.ManifestRelPath),
			Code:    invoke.ExitConfigErr,
		}
	case len(found) > 1:
		return nil, &Refusal{
			Message: fmt.Sprintf("markers for multiple stacks are present (%s) — seed never guesses; hand-author %s yourself, declaring the one stack this repository verifies as", describeDetections(found), invoke.ManifestRelPath),
			Code:    invoke.ExitConfigErr,
		}
	}

	stack := found[0].stack

	// FR-20: verb commands filled from the ecosystem defaults where real
	// defaults exist, named to-be-edited placeholders otherwise.
	// ResolveVerbs already returns fresh copies, never the shared defaults.
	verbs := defs.ResolveVerbs(stack, nil)
	placeholder := len(verbs) == 0
	if placeholder {
		verbs = placeholderVerbs()
	}

	data, ref := seedMarshalTemplate(&schema.Manifest{
		Version: manifestVersion,
		Stack:   stack,
		Verbs:   verbs,
	})
	if ref != nil {
		return nil, ref
	}

	if err := os.MkdirAll(filepath.Dir(manifestAbs), 0o755); err != nil {
		return nil, &Refusal{
			Message: fmt.Sprintf("cannot create %s: %v", filepath.Dir(invoke.ManifestRelPath), err),
			Code:    invoke.ExitRungErr,
		}
	}
	if err := seedWriteExclusive(manifestAbs, data); err != nil {
		if os.IsExist(err) {
			// The Lstat→create window closed on us (or a symlink sits at the
			// path, which O_EXCL refuses): same UB-10 posture either way.
			return nil, &Refusal{
				Message: fmt.Sprintf("a verification manifest already exists at %s — it is operator-owned; edit it directly, seed never overwrites it", invoke.ManifestRelPath),
				Code:    invoke.ExitConfigErr,
			}
		}
		return nil, &Refusal{
			Message: fmt.Sprintf("cannot write %s: %v", invoke.ManifestRelPath, err),
			Code:    invoke.ExitRungErr,
		}
	}

	return &Result{Path: invoke.ManifestRelPath, Stack: stack, Placeholder: placeholder}, nil
}

// writeExclusive creates the manifest with O_EXCL so seed can never follow a
// pre-existing entry (a stale symlink would otherwise route the write outside
// the declared path) and never clobbers a file that appeared after the UB-10
// probe.
func writeExclusive(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return fmt.Errorf("write: %w", err)
	}
	return f.Close()
}

// detectStacks returns every stack with at least one marker file present,
// in StackNames order (deterministic). Presence is checked by filename
// only; content is never parsed — matching the registry's own contract.
func detectStacks(workspace string, defs *ecosystem.Defaults) []detection {
	var found []detection
	for _, name := range defs.StackNames() {
		var present []string
		for _, marker := range defs.ForStack(name).MarkerFiles {
			if _, err := os.Stat(filepath.Join(workspace, marker)); err == nil {
				present = append(present, marker)
			}
		}
		if len(present) > 0 {
			found = append(found, detection{stack: name, markers: present})
		}
	}
	return found
}

// placeholderVerbs builds the named to-be-edited placeholder set. Each
// command is a single self-naming token: running it unedited spawn-fails
// with `"codegate-placeholder--declare-your-test-command": executable file
// not found` — a loud, named failure, never a silent green. The env
// allowlist is pre-filled (the pass-through pair every real command needs)
// so editing the command string alone yields a working verb.
func placeholderVerbs() map[string]*schema.Verb {
	verbs := make(map[string]*schema.Verb, len(ladderVerbNames))
	for _, name := range ladderVerbNames {
		verbs[name] = &schema.Verb{
			Command: "codegate-placeholder--declare-your-" + name + "-command",
			Env: map[string]schema.EnvEntry{
				"HOME": {Mode: schema.EnvPass},
				"PATH": {Mode: schema.EnvPass},
			},
		}
	}
	return verbs
}

// marshalTemplate serializes the manifest and proves it closed-parses with
// the harness's own parser (FR-20's fit criterion: the harness consumes a
// seeded manifest identically to a hand-authored one). A parse failure here
// is a programmer error surfaced loudly, never a written-but-unusable file.
func marshalTemplate(m *schema.Manifest) ([]byte, *Refusal) {
	data, err := schema.MarshalManifestIndent(m)
	if err != nil {
		return nil, &Refusal{
			Message: fmt.Sprintf("cannot serialize the template manifest: %v", err),
			Code:    invoke.ExitRungErr,
		}
	}
	if _, err := schema.UnmarshalManifest(data); err != nil {
		return nil, &Refusal{
			Message: fmt.Sprintf("seed built a manifest that does not closed-parse — a programmer error, not a defect in your tree: %v", err),
			Code:    invoke.ExitRungErr,
		}
	}
	return data, nil
}

// allMarkers renders the registry's full marker set for the no-markers
// refusal, derived from the defaults so the message can never drift from
// what detection actually reads.
func allMarkers(defs *ecosystem.Defaults) string {
	var names []string
	for _, stack := range defs.StackNames() {
		names = append(names, defs.ForStack(stack).MarkerFiles...)
	}
	return strings.Join(names, ", ")
}

// describeDetections renders the ambiguity refusal's evidence: each detected
// stack with the marker files that matched it.
func describeDetections(found []detection) string {
	parts := make([]string, 0, len(found))
	for _, d := range found {
		parts = append(parts, fmt.Sprintf("%s: %s", d.stack, strings.Join(d.markers, " ")))
	}
	return strings.Join(parts, "; ")
}
