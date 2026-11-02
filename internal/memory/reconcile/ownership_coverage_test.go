package reconcile

import (
	"maps"
	"reflect"
	"strings"
	"testing"
)

func TestOwnershipFeatureAndPlanBoundaries(t *testing.T) {
	if _, err := FeatureContextSources(ownershipTestSnapshot("alpha"), "not/a-slug"); err == nil || !strings.Contains(err.Error(), "canonical feature slug") {
		t.Fatalf("invalid feature error = %v", err)
	}

	duplicate := ownershipTestSnapshot("alpha")
	duplicate.PRDs = append(duplicate.PRDs, duplicate.PRDs[0])
	if _, err := ContextSourceOwners(duplicate); err == nil || !strings.Contains(err.Error(), "more than one compiled PRD ledger") {
		t.Fatalf("duplicate PRD error = %v", err)
	}

	missingLedger := ownershipTestSnapshot("alpha")
	missingLedger.AuthoredSources = nil
	missingLedger.Files = map[string]File{}
	if _, err := FeatureContextSources(missingLedger, "alpha"); err == nil || !strings.Contains(err.Error(), "no pinned authored PRD ledger sources") {
		t.Fatalf("missing PRD ledger error = %v", err)
	}

	valid := ownershipTestSnapshot("alpha")
	planPath := "docs/plans/alpha/001-plan.md"
	proofPath := "docs/plans/alpha/proof.md"
	ownershipAddFile(&valid, planPath, ownershipPlan("alpha", "docs/prds/alpha", "plan-template/v1"))
	ownershipAddFile(&valid, proofPath, []byte("# Proof\n"))
	sources, err := FeatureContextSources(valid, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{planPath, proofPath} {
		if !ownershipHasPath(sources, want) {
			t.Errorf("valid plan directory omitted %s: %v", want, sources)
		}
	}

	invalid := ownershipTestSnapshot("alpha", "beta")
	ownershipAddFile(&invalid, "docs/plans/alpha/001-plan.md", []byte("# Broken plan\n"))
	if _, err := FeatureContextSources(invalid, "alpha"); err == nil || !strings.Contains(err.Error(), "invalid ownership metadata") {
		t.Fatalf("invalid target plan error = %v", err)
	}
	if _, err := FeatureContextSources(invalid, "beta"); err != nil {
		t.Fatalf("invalid unrelated plan blocked beta: %v", err)
	}

	mismatch := ownershipTestSnapshot("alpha")
	ownershipAddFile(&mismatch, "docs/plans/alpha/001-plan.md", ownershipPlan("beta", "docs/prds/beta", "plan-template/v1"))
	if _, err := FeatureContextSources(mismatch, "alpha"); err == nil || !strings.Contains(err.Error(), "does not bind directory") {
		t.Fatalf("mismatched target plan error = %v", err)
	}
}

func TestOwnershipDesignReviewAndStageOneProjection(t *testing.T) {
	snapshot := ownershipTestSnapshot("alpha")
	designPath := "docs/tech-design/alpha-v1.md"
	reviewPath := "docs/reviews/alpha-v1.md"
	brainstormPath := "docs/brainstorms/alpha.md"
	ownershipAddFile(&snapshot, designPath, []byte("# Alpha design\n"))
	snapshot.Designs = append(snapshot.Designs, Design{
		Slug:   "alpha-v1",
		Source: designPath,
		Serves: []string{"not-a-prd-ref", "prd:alpha/FR-1"},
	})
	ownershipAddFile(&snapshot, reviewPath, ownershipReview("design-review-report/v1", "design:alpha-v1 (merged)"))
	ownershipAddFile(&snapshot, brainstormPath, ownershipStageOneWithFeed("alpha", "`docs/prds/alpha/`"))

	sources, err := FeatureContextSources(snapshot, "alpha")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{designPath, reviewPath, brainstormPath} {
		if !ownershipHasPath(sources, want) {
			t.Errorf("projected feature context omitted %s: %v", want, sources)
		}
	}

	unknown := ownershipTestSnapshot("alpha")
	ownershipAddFile(&unknown, "docs/brainstorms/alpha.md", ownershipDoc("# Alpha brainstorm", [][2]string{
		{"In one line", "alpha"}, {"Template version", "brainstorm-record/v9"},
	}, "## Idea\n"))
	if _, err := FeatureContextSources(unknown, "alpha"); err == nil || !strings.Contains(err.Error(), "unknown template version") {
		t.Fatalf("unknown relevant brainstorm template error = %v", err)
	}
}

func TestStageOneBrainstormOwnerGrammar(t *testing.T) {
	opts := optionDefaults(Options{})
	valid := map[string]string{
		"In one line": "alpha", "Relevant-when": "shaping alpha", "Idea": "alpha",
		"Operator": "operator", "Date(s)": "2026-09-08", "Status": "gated: proceed",
		"Tracking": "—", "Feeds": "docs/prds/alpha/", "Prior records": "—",
		"Template version": "brainstorm-record/v1",
	}
	owners, err := stageOneBrainstormOwners("docs/brainstorms/alpha.md", valid, opts)
	if err != nil || !reflect.DeepEqual(owners, []string{"alpha"}) {
		t.Fatalf("bare feed owners = %v, err = %v", owners, err)
	}

	for _, dash := range []string{"-", "—"} {
		header := cloneOwnershipHeader(valid)
		header["Feeds"] = dash
		if owners, err := stageOneBrainstormOwners("docs/brainstorms/alpha.md", header, opts); err != nil || owners != nil {
			t.Errorf("Feeds %q owners = %v, err = %v", dash, owners, err)
		}
	}

	tests := []struct {
		name string
		feed string
		edit func(map[string]string)
		want string
	}{
		{name: "missing field", feed: "docs/prds/alpha/", edit: func(h map[string]string) { delete(h, "Idea") }, want: "required header field"},
		{name: "multiple tokens", feed: "docs/prds/alpha/ extra", want: "not one exact path"},
		{name: "wrong root", feed: "spec/prds/alpha/", want: "not an exact PRD directory"},
		{name: "missing trailing slash", feed: "docs/prds/alpha", want: "not an exact PRD directory"},
		{name: "nested feature", feed: "docs/prds/alpha/beta/", want: "canonical feature"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			header := cloneOwnershipHeader(valid)
			header["Feeds"] = tt.feed
			if tt.edit != nil {
				tt.edit(header)
			}
			if _, err := stageOneBrainstormOwners("docs/brainstorms/alpha.md", header, opts); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestReviewOwnerGrammar(t *testing.T) {
	designOwners := map[string][]string{"alpha-v1": {"alpha", "shared"}}
	tests := []struct {
		name    string
		file    string
		body    []byte
		want    []string
		wantErr string
	}{
		{name: "prd bare", file: "docs/reviews/alpha-000.md", body: ownershipReview("review-report/v1", "prd:alpha/000"), want: []string{"alpha"}},
		{name: "prd code", file: "docs/reviews/alpha-000.md", body: ownershipReview("review-report/v1", "`prd:alpha/000`"), want: []string{"alpha"}},
		{name: "prd ambiguous", file: "docs/reviews/alpha-000.md", body: ownershipReview("review-report/v1", "prd:alpha/000 extra"), wantErr: "not one exact typed reference"},
		{name: "prd filename mismatch", file: "docs/reviews/beta-000.md", body: ownershipReview("review-report/v1", "prd:alpha/000"), wantErr: "filename disagree"},
		{name: "design bare", file: "docs/reviews/alpha-v1.md", body: ownershipReview("design-review-report/v1", "design:alpha-v1 (merged)"), want: []string{"alpha", "shared"}},
		{name: "design code", file: "docs/reviews/alpha-v1.md", body: ownershipReview("design-review-report/v1", "`design:alpha-v1` · merged"), want: []string{"alpha", "shared"}},
		{name: "design missing token", file: "docs/reviews/alpha-v1.md", body: ownershipReview("design-review-report/v1", "alpha-v1"), wantErr: "no exact leading design reference"},
		{name: "design unresolved", file: "docs/reviews/missing.md", body: ownershipReview("design-review-report/v1", "design:missing"), wantErr: "does not resolve"},
		{name: "unknown version", file: "docs/reviews/alpha-000.md", body: ownershipReview("review-report/v9", "prd:alpha/000"), wantErr: "unknown review template"},
		{name: "malformed header", file: "docs/reviews/alpha-000.md", body: []byte("# Review\n\nnot a table\n"), wantErr: "header table"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := reviewOwners(tt.file, tt.body, designOwners)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("error = %v, want substring %q", err, tt.wantErr)
				}
				return
			}
			if err != nil || !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("owners = %v, err = %v, want %v", got, err, tt.want)
			}
		})
	}

	missing := ownershipDoc("# Review", [][2]string{{"Template version", "review-report/v1"}}, "## Round 1\n")
	if _, err := reviewOwners("docs/reviews/alpha-000.md", missing, designOwners); err == nil || !strings.Contains(err.Error(), "required header field") {
		t.Fatalf("missing review field error = %v", err)
	}
}

func TestProblemOwnershipMissingAndInvalidPairs(t *testing.T) {
	missing := ownershipTestSnapshot("alpha")
	ownershipAddFile(&missing, "docs/problems/alpha/problem.md", ownershipProblemLedger("alpha", "problem-ledger/v1"))
	if _, err := FeatureContextSources(missing, "alpha"); err == nil || !strings.Contains(err.Error(), "both problem.md and delivery.md") {
		t.Fatalf("missing problem pair error = %v", err)
	}
	if owners, err := ContextSourceOwners(missing); err != nil {
		t.Fatalf("non-strict missing pair error = %v", err)
	} else if _, ok := owners["docs/problems/alpha/problem.md"]; ok {
		t.Fatalf("non-strict missing pair was projected: %v", owners)
	}

	invalid := ownershipTestSnapshot("alpha")
	ownershipAddFile(&invalid, "docs/problems/alpha/problem.md", ownershipProblemLedger("alpha", "problem-ledger/v1"))
	ownershipAddFile(&invalid, "docs/problems/alpha/delivery.md", []byte("# Broken delivery\n"))
	if _, err := FeatureContextSources(invalid, "alpha"); err == nil || !strings.Contains(err.Error(), "is invalid") {
		t.Fatalf("invalid problem pair error = %v", err)
	}
	if owners, err := ContextSourceOwners(invalid); err != nil {
		t.Fatalf("non-strict invalid pair error = %v", err)
	} else if _, ok := owners["docs/problems/alpha/problem.md"]; ok {
		t.Fatalf("non-strict invalid pair was partially projected: %v", owners)
	}

	mismatch := ownershipTestSnapshot("alpha")
	ownershipAddFile(&mismatch, "docs/problems/alpha/problem.md", ownershipProblemLedger("beta", "problem-ledger/v1"))
	ownershipAddFile(&mismatch, "docs/problems/alpha/delivery.md", ownershipDeliveryLedger("alpha"))
	if _, err := FeatureContextSources(mismatch, "alpha"); err == nil || !strings.Contains(err.Error(), "does not bind Slug") {
		t.Fatalf("mismatched problem slug error = %v", err)
	}
}

func TestOwnershipHeaderAndTokenHelpers(t *testing.T) {
	headerCases := []struct {
		name string
		body string
		want string
	}{
		{name: "empty", want: "H1"},
		{name: "h2", body: "## Not H1\n", want: "H1"},
		{name: "no table", body: "# Title\n\nprose\n", want: "Field/Value"},
		{name: "no divider", body: "# Title\n\n| Field | Value |\n| **A** | b |\n", want: "divider"},
		{name: "no rows", body: "# Title\n\n| Field | Value |\n|---|---|\n\n## Body\n", want: "no bold"},
		{name: "malformed row", body: "# Title\n\n| Field | Value |\n|---|---|\n| A | b |\n\n## Body\n", want: "not one bold"},
		{name: "no h2", body: "# Title\n\n| Field | Value |\n|---|---|\n| **A** | b |\n", want: "no H2"},
	}
	for _, tt := range headerCases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := ownershipHeader([]byte(tt.body)); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}

	exactCases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", false}, {"alpha", "alpha", true}, {"alpha beta", "alpha beta", false},
		{"alpha`", "alpha`", false}, {"`a`", "a", true}, {"`a", "", false},
		{"``", "", false}, {"`a`b`", "", false},
	}
	for _, tt := range exactCases {
		if got, ok := exactCodeOrBare(tt.in); got != tt.want || ok != tt.ok {
			t.Errorf("exactCodeOrBare(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}

	leadingCases := []struct {
		in   string
		want string
		ok   bool
	}{
		{"", "", false}, {"design:alpha", "design:alpha", true},
		{"design:alpha detail", "design:alpha", true}, {"design:alpha·merged", "design:alpha", true},
		{"`design:alpha`", "design:alpha", true}, {"`design:alpha` (merged)", "design:alpha", true},
		{"`design:alpha` extra", "", false}, {"`x", "", false}, {"``", "", false},
	}
	for _, tt := range leadingCases {
		if got, ok := leadingTypedToken(tt.in); got != tt.want || ok != tt.ok {
			t.Errorf("leadingTypedToken(%q) = %q, %v; want %q, %v", tt.in, got, ok, tt.want, tt.ok)
		}
	}

	for _, value := range []string{"", "something prd:alpha", "prd:alpha!"} {
		if _, err := typedPRDFeatures(value); err == nil {
			t.Errorf("typedPRDFeatures(%q) unexpectedly succeeded", value)
		}
	}
	if !brainstormNameCouldOwn("alpha.md", "alpha") || !brainstormNameCouldOwn("alpha-tech-design-2.md", "alpha") || brainstormNameCouldOwn("beta.md", "alpha") || brainstormNameCouldOwn("alpha.md", "") {
		t.Error("brainstormNameCouldOwn boundary result incorrect")
	}
	if !reviewNameCouldOwn("alpha-000.md", "alpha", nil) || !reviewNameCouldOwn("shared.md", "alpha", map[string][]string{"shared": {"alpha"}}) || reviewNameCouldOwn("shared.md", "", nil) {
		t.Error("reviewNameCouldOwn boundary result incorrect")
	}

	if got := defectSummary([]ownershipCoverageDefect(nil)); got != "no parsed document" {
		t.Fatalf("empty defect summary = %q", got)
	}
	if got := defectSummary([]ownershipCoverageDefect{"first", "second"}); got != "first; second" {
		t.Fatalf("defect summary = %q", got)
	}
}

type ownershipCoverageDefect string

func (d ownershipCoverageDefect) String() string { return string(d) }

func ownershipPlan(feature, prd, version string) []byte {
	return ownershipDoc("# "+feature+" plan", [][2]string{
		{"Feature", feature}, {"PRD", prd}, {"Design", "docs/tech-design/" + feature + ".md"},
		{"Plan version", "001"}, {"Dialect", "harbormaster/fs-spec:v1"}, {"Epic title", feature},
		{"Status", "Draft"}, {"Template version", version},
	}, "## Tasks\n")
}

func ownershipReview(version, subject string) []byte {
	return ownershipDoc("# Review", [][2]string{
		{"In one line", "review"}, {"Relevant-when", "auditing"}, {"Subject", subject},
		{"Moment", "review"}, {"Reviewer", "reviewer"}, {"Date(s)", "2026-09-08"},
		{"Verdict", "approve"}, {"Tracking", "—"}, {"Template version", version},
	}, "## Round 1\n")
}

func ownershipStageOneWithFeed(name, feed string) []byte {
	return ownershipDoc("# "+name+" — brainstorm record", [][2]string{
		{"In one line", name}, {"Relevant-when", "shaping " + name}, {"Idea", name},
		{"Operator", "operator"}, {"Date(s)", "2026-09-08"}, {"Status", "gated: proceed"},
		{"Tracking", "—"}, {"Feeds", feed}, {"Prior records", "—"},
		{"Template version", "brainstorm-record/v1"},
	}, "## Idea\n")
}

func cloneOwnershipHeader(in map[string]string) map[string]string {
	out := make(map[string]string, len(in))
	maps.Copy(out, in)
	return out
}
