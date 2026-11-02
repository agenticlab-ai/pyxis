package reconcile

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
	"testing"
)

func TestFeatureContextSourcesMultiOwnerBacktickedBasis(t *testing.T) {
	snapshot := ownershipTestSnapshot("alpha", "beta")
	shared := "docs/brainstorms/shared-tech-design.md"
	ownershipAddFile(&snapshot, shared, ownershipDesignBrainstorm(
		"shared",
		"`prd:beta` (merged) · `prd:alpha` (merged)",
	))

	owners, err := ContextSourceOwners(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := owners[shared], []string{"alpha", "beta"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("shared Basis owners = %v, want %v", got, want)
	}
	for _, feature := range []string{"alpha", "beta"} {
		sources, err := FeatureContextSources(snapshot, feature)
		if err != nil {
			t.Fatalf("FeatureContextSources(%q): %v", feature, err)
		}
		if !ownershipHasPath(sources, shared) {
			t.Fatalf("FeatureContextSources(%q) omitted shared backticked Basis %s: %v", feature, shared, sources)
		}
	}
}

func TestFeatureContextSourcesCrossSlugBridge(t *testing.T) {
	snapshot := ownershipTestSnapshot("argus-boundary-enforcement")
	paths := ownershipAddCrossSlugContext(&snapshot, "problem-ledger/v1")

	sources, err := FeatureContextSources(snapshot, "argus-boundary-enforcement")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range paths {
		if !ownershipHasPath(sources, want) {
			t.Errorf("cross-slug context omitted %s: %v", want, sources)
		}
	}
}

func TestFeatureContextSourcesRejectsMalformedCrossSlugStageOne(t *testing.T) {
	snapshot := ownershipTestSnapshot("alpha")
	context := "shared-context"
	design := "docs/brainstorms/" + context + "-tech-design.md"
	stageOne := "docs/brainstorms/" + context + ".md"
	ownershipAddFile(&snapshot, design, ownershipDesignBrainstorm(context, "prd:alpha"))
	ownershipAddFile(&snapshot, stageOne, []byte("# Broken stage-one record\n\nnot a header table\n\n## Idea\n"))
	// Deliberately put the stage-one record first. Ownership must not depend on
	// the pinned inventory's iteration order.
	ordered := []string{stageOne}
	for _, source := range snapshot.AuthoredSources {
		if source != stageOne {
			ordered = append(ordered, source)
		}
	}
	snapshot.AuthoredSources = ordered

	_, err := FeatureContextSources(snapshot, "alpha")
	if err == nil || !strings.Contains(err.Error(), "brainstorm "+stageOne+" has invalid ownership metadata") {
		t.Fatalf("malformed bridged stage-one error = %v", err)
	}
}

func TestFeatureContextSourcesRejectsNonClosedHeaders(t *testing.T) {
	tests := []struct {
		name string
		body []byte
		want string
	}{
		{
			name: "unknown field",
			body: ownershipDoc("# Alpha — design-brainstorm record", [][2]string{
				{"In one line", "alpha"}, {"Relevant-when", "testing alpha"}, {"Basis", "prd:alpha"},
				{"Authority", "operator"}, {"Date(s)", "2026-09-08"}, {"Status", "gated: proceed"},
				{"Tracking", "—"}, {"Feeds", "—"}, {"Prior records", "—"}, {"Unexpected", "must reject"},
				{"Template version", "design-brainstorm-record/v1"},
			}, "## Basis\n"),
			want: "unknown header field",
		},
		{
			name: "duplicate field",
			body: ownershipDoc("# Alpha — design-brainstorm record", [][2]string{
				{"In one line", "alpha"}, {"Relevant-when", "testing alpha"}, {"Basis", "prd:alpha"},
				{"Basis", "prd:alpha"}, {"Authority", "operator"}, {"Date(s)", "2026-09-08"},
				{"Status", "gated: proceed"}, {"Tracking", "—"}, {"Feeds", "—"}, {"Prior records", "—"},
				{"Template version", "design-brainstorm-record/v1"},
			}, "## Basis\n"),
			want: "duplicate header field",
		},
		{
			name: "unclosed inline code",
			body: ownershipDesignBrainstorm("alpha", "`prd:alpha"),
			want: "Basis",
		},
		{
			name: "row outside first table",
			body: []byte(strings.Replace(
				string(ownershipDesignBrainstorm("alpha", "prd:alpha")),
				"\n## Basis", "\n| **Basis** | prd:alpha |\n\n## Basis", 1,
			)),
			want: "outside the first header table",
		},
		{
			name: "duplicate table before section",
			body: []byte(strings.Replace(
				string(ownershipDesignBrainstorm("alpha", "prd:alpha")),
				"\n## Basis", "\n| Field | Value |\n|---|---|\n| **Basis** | prd:alpha |\n\n## Basis", 1,
			)),
			want: "outside the first header table",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := ownershipTestSnapshot("alpha")
			ownershipAddFile(&snapshot, "docs/brainstorms/alpha-tech-design.md", tt.body)
			if _, err := FeatureContextSources(snapshot, "alpha"); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("malformed ownership header error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestFeatureContextSourcesRejectsInexactProblemTemplateVersion(t *testing.T) {
	snapshot := ownershipTestSnapshot("argus-boundary-enforcement")
	ownershipAddCrossSlugContext(&snapshot, "problem-ledger/v1-forged")

	_, err := FeatureContextSources(snapshot, "argus-boundary-enforcement")
	if err == nil || !strings.Contains(err.Error(), "exact template version") {
		t.Fatalf("forged problem Template version error = %v", err)
	}
}

func TestFeatureContextSourcesScopesMalformedReviewFailure(t *testing.T) {
	malformed := []byte("# Broken review\n\nThis is not a typed review header.\n\n## Findings\n")

	unrelated := ownershipTestSnapshot("alpha", "beta")
	ownershipAddFile(&unrelated, "docs/reviews/beta-000.md", malformed)
	if _, err := FeatureContextSources(unrelated, "alpha"); err != nil {
		t.Fatalf("unrelated malformed review blocked alpha closeout: %v", err)
	}

	relevant := ownershipTestSnapshot("alpha", "beta")
	ownershipAddFile(&relevant, "docs/reviews/alpha-000.md", malformed)
	if _, err := FeatureContextSources(relevant, "alpha"); err == nil || !strings.Contains(err.Error(), "review") {
		t.Fatalf("target-relevant malformed review error = %v", err)
	}
}

func TestFeatureContextSourcesScopesUnsupportedProblemFiles(t *testing.T) {
	snapshot := ownershipTestSnapshot("alpha", "beta")
	root := "docs/problems/beta"
	ownershipAddFile(&snapshot, root+"/problem.md", ownershipProblemLedger("beta", "problem-ledger/v1"))
	ownershipAddFile(&snapshot, root+"/delivery.md", ownershipDeliveryLedger("beta"))
	ownershipAddFile(&snapshot, root+"/notes.md", []byte("unsupported\n"))

	if _, err := FeatureContextSources(snapshot, "alpha"); err != nil {
		t.Fatalf("unrelated unsupported problem file blocked alpha closeout: %v", err)
	}
	if _, err := FeatureContextSources(snapshot, "beta"); err == nil || !strings.Contains(err.Error(), "unsupported authored context "+root+"/notes.md") {
		t.Fatalf("target unsupported problem file error = %v", err)
	}
	owners, err := ContextSourceOwners(snapshot)
	if err != nil {
		t.Fatalf("complete owner projection: %v", err)
	}
	if _, ok := owners[root+"/problem.md"]; ok {
		t.Fatalf("invalid problem directory was partially projected: %v", owners)
	}
}

func TestTypedPRDFeaturesRejectsUnicodeTokenContinuation(t *testing.T) {
	if owners, err := typedPRDFeatures("prd:alphaβ"); err == nil {
		t.Fatalf("Unicode continuation parsed as owners %v", owners)
	}
	if owners, err := typedPRDFeatures("prd:alpha · prd:beta"); err != nil || !reflect.DeepEqual(owners, []string{"alpha", "beta"}) {
		t.Fatalf("explicit Unicode separator owners = %v, err = %v", owners, err)
	}
}

func TestProblemSourceAddressesAreDistinct(t *testing.T) {
	opts := optionDefaults(Options{})
	paths := []string{
		"docs/problems/widget/problem.md",
		"docs/problems/widget/delivery.md",
		"docs/problems/gadget/problem.md",
	}
	want := []string{
		"problem/widget#problem",
		"problem/widget#delivery",
		"problem/gadget#problem",
	}
	seen := map[string]bool{}
	for i, source := range paths {
		subject, family := sourceAddress(source, opts)
		got := subject + "#" + family
		if got != want[i] {
			t.Errorf("sourceAddress(%q) = %q, want %q", source, got, want[i])
		}
		if seen[got] {
			t.Errorf("sourceAddress(%q) reused %q", source, got)
		}
		seen[got] = true
	}
}

func ownershipTestSnapshot(features ...string) Snapshot {
	opts := optionDefaults(Options{})
	snapshot := Snapshot{Files: map[string]File{}, SourceOptions: opts}
	for i, feature := range features {
		digestByte := byte('a' + i)
		base := fmt.Sprintf("%s/%s/000-base.md", opts.PRDs, feature)
		ownershipAddFile(&snapshot, base, []byte("# "+feature+" PRD\n"))
		snapshot.PRDs = append(snapshot.PRDs, PRD{
			Feature:         feature,
			Source:          fmt.Sprintf("%s/%s/effective.md", opts.PRDs, feature),
			AsOf:            "000",
			Fingerprint:     "sha256:" + strings.Repeat(string(digestByte), 64),
			SourceMapSHA256: strings.Repeat(string(digestByte), 64),
		})
	}
	sort.Strings(snapshot.AuthoredSources)
	return snapshot
}

func ownershipAddFile(snapshot *Snapshot, source string, body []byte) {
	if snapshot.Files == nil {
		snapshot.Files = map[string]File{}
	}
	snapshot.Files[source] = File{Path: source, Mode: "100644", SHA256: sourceDigest(body), Body: body}
	snapshot.AuthoredSources = append(snapshot.AuthoredSources, source)
	sort.Strings(snapshot.AuthoredSources)
}

func ownershipAddCrossSlugContext(snapshot *Snapshot, problemTemplate string) []string {
	stageThree := "docs/brainstorms/argus-attribution-hardening-tech-design.md"
	stageOne := "docs/brainstorms/argus-attribution-hardening.md"
	problem := "docs/problems/argus-attribution-hardening/problem.md"
	delivery := "docs/problems/argus-attribution-hardening/delivery.md"
	ownershipAddFile(snapshot, stageThree, ownershipDesignBrainstorm("argus-attribution-hardening", "prd:argus-boundary-enforcement (merged)"))
	ownershipAddFile(snapshot, stageOne, ownershipStageOneBrainstorm("argus-attribution-hardening"))
	ownershipAddFile(snapshot, problem, ownershipProblemLedger("argus-attribution-hardening", problemTemplate))
	ownershipAddFile(snapshot, delivery, ownershipDeliveryLedger("argus-attribution-hardening"))
	return []string{stageThree, stageOne, problem, delivery}
}

func ownershipDesignBrainstorm(name, basis string) []byte {
	return ownershipDoc("# "+name+" — design-brainstorm record", [][2]string{
		{"In one line", name}, {"Relevant-when", "designing " + name}, {"Basis", basis},
		{"Authority", "operator"}, {"Date(s)", "2026-09-08"}, {"Status", "gated: proceed"},
		{"Tracking", "—"}, {"Feeds", "—"}, {"Prior records", "—"},
		{"Template version", "design-brainstorm-record/v1"},
	}, "## Basis\n")
}

func ownershipStageOneBrainstorm(name string) []byte {
	return ownershipDoc("# "+name+" — brainstorm record", [][2]string{
		{"In one line", name}, {"Relevant-when", "shaping " + name}, {"Idea", name},
		{"Operator", "operator"}, {"Date(s)", "2026-09-08"}, {"Status", "gated: proceed"},
		{"Tracking", "—"}, {"Feeds", "—"}, {"Prior records", "—"},
		{"Template version", "brainstorm-record/v1"},
	}, "## Idea\n")
}

func ownershipProblemLedger(slug, template string) []byte {
	return ownershipDoc("# "+slug+" — problem ledger", [][2]string{
		{"Slug", slug}, {"Principal", "operator"}, {"Authority-contract version", "authority-contract/v1"},
		{"Envelope override", "—"}, {"Template version", template},
	}, "## Idea\n\nverbatim\n\n## Authority entries\n")
}

func ownershipDeliveryLedger(slug string) []byte {
	return ownershipDoc("# "+slug+" — delivery ledger", [][2]string{
		{"Slug", slug}, {"Template version", "delivery-ledger/v1"},
	}, "## Derived state\n\n## Journal\n")
}

func ownershipDoc(title string, fields [][2]string, tail string) []byte {
	var body strings.Builder
	body.WriteString(title)
	body.WriteString("\n\n| Field | Value |\n|---|---|\n")
	for _, field := range fields {
		fmt.Fprintf(&body, "| **%s** | %s |\n", field[0], field[1])
	}
	body.WriteString("\n")
	body.WriteString(tail)
	return []byte(body.String())
}

func ownershipHasPath(sources []ContextSource, want string) bool {
	for _, source := range sources {
		if source.Path == want {
			return true
		}
	}
	return false
}
