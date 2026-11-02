package contextpack

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Schema is the pack format version, recorded in the manifest so a consumer
// parses against a known shape.
const Schema = "context-pack/v1"

// bodyMarker delimits one source in the pack body — a fixed, greppable line
// naming the source. Serialization is fixed for determinism (DINV-1); the
// manifest's sources list is the authoritative content record (FR-2).
const bodyMarker = "===== context-pack source:"

// manifestSource records one resolved source in the manifest: its kind, path,
// and whether it was present (an optional source absent at build time is
// recorded, never emitted as an empty body section — FR-7).
type manifestSource struct {
	Kind    Kind
	Path    string
	Present bool
}

// Build assembles the pack for (stage, subject) from the declaration, reading
// current sources at the repo root — build-fresh, never stored, so a pack is
// never stale (DINV-2, FR-6). Output is a pure function of the declaration
// and the sources' content: declared order, verbatim bodies, a fixed manifest
// serialization, and NO wall-clock anywhere (built_from is a content
// revision), so two builds at one source state are byte-identical (DINV-1,
// FR-5). It reads only declared sources and writes none (DINV-4, DINV-5,
// NFR-5). A required source that does not resolve is a fail-closed defect
// with no artifact (UB-1, UB-2, DINV-6).
func Build(root, stage, subject string, d Declaration) ([]byte, []schema.Defect) {
	if strings.TrimSpace(subject) == "" {
		return nil, []schema.Defect{def("build", "empty subject", "pass a subject slug: context-pack --stage <s> --subject <slug>")}
	}
	sources, rdef := d.Resolve(stage, subject)
	if len(rdef) > 0 {
		return nil, rdef
	}

	var body bytes.Buffer
	// revInput accumulates a canonical encoding of every resolved source
	// (kind, path, present, content) so built_from is a content-addressed
	// revision: it changes when any source's content or presence changes
	// (freshness, DINV-2) and never depends on wall-clock (DINV-1).
	var revInput bytes.Buffer
	manifest := make([]manifestSource, 0, len(sources))

	for _, s := range sources {
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(s.Path)))
		if err != nil {
			if os.IsNotExist(err) {
				if s.Optional {
					manifest = append(manifest, manifestSource{Kind: s.Kind, Path: s.Path, Present: false})
					revInput.WriteString(string(s.Kind) + "\x00" + s.Path + "\x00absent\x00")
					continue
				}
				// A required source that does not resolve: fail closed, name
				// it, emit no artifact. For the subject kind this is UB-2 (the
				// subject does not exist); otherwise UB-1 (a dangling source).
				reason := fmt.Sprintf("required %s source %q does not resolve", s.Kind, s.Path)
				remedy := "fix the selector or the missing file; the compiler never emits a partial pack (DINV-6)"
				if s.Kind == KindSubject {
					reason = fmt.Sprintf("subject %q has no %s source at %q", subject, s.Kind, s.Path)
					remedy = "check the subject slug; the subject document must exist (UB-2)"
				}
				return nil, []schema.Defect{def("build", reason, remedy)}
			}
			return nil, []schema.Defect{def("build", fmt.Sprintf("reading %q: %v", s.Path, err), "fix file permissions")}
		}
		manifest = append(manifest, manifestSource{Kind: s.Kind, Path: s.Path, Present: true})
		revInput.WriteString(string(s.Kind) + "\x00" + s.Path + "\x00present\x00")
		revInput.Write(content)
		revInput.WriteString("\x00")
		// Verbatim body: the source content byte-for-byte, in declared order
		// (DINV-4). A trailing newline before the next marker keeps the
		// delimiters line-anchored without mutating the source bytes.
		fmt.Fprintf(&body, "%s %s %s =====\n", bodyMarker, s.Kind, s.Path)
		body.Write(content)
		if len(content) > 0 && content[len(content)-1] != '\n' {
			body.WriteByte('\n')
		}
	}

	builtFrom := "content:sha256:" + hexSum(revInput.Bytes())
	digest := "sha256:" + hexSum(body.Bytes())

	var out bytes.Buffer
	writeManifest(&out, stage, subject, builtFrom, digest, manifest)
	out.Write(body.Bytes())
	return out.Bytes(), nil
}

// writeManifest renders the fixed-order manifest header as a `#`-prefixed
// block, then a blank line. Field order is fixed and the sources are listed
// in declared (slice) order — never map iteration — so the header is
// deterministic (DINV-1).
func writeManifest(w *bytes.Buffer, stage, subject, builtFrom, digest string, sources []manifestSource) {
	fmt.Fprintf(w, "# %s\n", Schema)
	fmt.Fprintf(w, "# stage: %s\n", stage)
	fmt.Fprintf(w, "# subject: %s\n", subject)
	fmt.Fprintf(w, "# built_from: %s\n", builtFrom)
	fmt.Fprintf(w, "# digest: %s\n", digest)
	fmt.Fprintf(w, "# tool: %s\n", schema.ToolVersion)
	w.WriteString("# sources:\n")
	for _, s := range sources {
		state := "present"
		if !s.Present {
			state = "absent"
		}
		fmt.Fprintf(w, "#   %-11s %s (%s)\n", s.Kind, s.Path, state)
	}
	w.WriteString("\n")
}

func hexSum(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// Run loads the declaration from root/DeclarationFile and builds the pack for
// (stage, subject). It is the verb entry point: an absent or unparseable
// declaration is a named defect, never a lenient empty build.
func Run(root, stage, subject string) ([]byte, []schema.Defect) {
	b, err := os.ReadFile(filepath.Join(root, DeclarationFile))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, []schema.Defect{def("load", "no declaration file", "create "+DeclarationFile+" declaring each stage's stable-context classes")}
		}
		return nil, []schema.Defect{def("load", fmt.Sprintf("unreadable: %v", err), "fix file permissions")}
	}
	d, pdef := ParseDeclaration(b)
	if len(pdef) > 0 {
		return nil, pdef
	}
	return Build(root, stage, subject, d)
}

// Stages returns the declared stage names, sorted — a diagnostic helper for
// the verb's usage when an unknown stage is requested.
func Stages(d Declaration) []string {
	names := make([]string, 0, len(d))
	for s := range d {
		names = append(names, s)
	}
	sort.Strings(names)
	return names
}
