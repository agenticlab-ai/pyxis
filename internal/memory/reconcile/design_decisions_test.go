package reconcile

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestExtractLiveDesignDecisionsPreservesExactEvidenceAndStableAddresses(t *testing.T) {
	body := []byte("" +
		"# Widget — technical design\n" +
		"\n" +
		"## 0. Read to rule\n" +
		"\n" +
		"This human-only route says **Decision:** do not extract this summary.\n" +
		"\n" +
		"## 2. Decision\n" +
		"\n" +
		"**Decision:** Keep the first source line\n" +
		"and preserve this exact continuation.\n" +
		"\n" +
		"Explanatory rationale is deliberately outside the extracted block.\n" +
		"\n" +
		"### 3.1 Persistence\n" +
		"\n" +
		"The durable boundary is explicit. **Decision:** Store one immutable record.\n" +
		"\n" +
		"`**Decision:** inline code is not a binding callout`\n" +
		"\n" +
		"<!-- **Decision:** hidden source is not a binding callout -->\n" +
		"\n" +
		"```markdown\n" +
		"**Decision:** fenced examples are not binding callouts.\n" +
		"```\n" +
		"\n" +
		"### 3.2 Ordering\n" +
		"\n" +
		"**Decision:** Emit in source order.\n" +
		"\n" +
		"**Decision:** Keep a section-local ordinal.\n")
	design, file := liveDecisionFixture("widget-v1", body)

	got, err := ExtractLiveDesignDecisions(design, file)
	if err != nil {
		t.Fatalf("ExtractLiveDesignDecisions: %v", err)
	}
	if len(got) != 4 {
		t.Fatalf("blocks = %d, want 4: %+v", len(got), got)
	}

	want := []struct {
		section   string
		heading   string
		family    string
		ordinal   int
		statement string
		startLine int
		endLine   int
	}{
		{"2", "2. Decision", "decision.2.1", 1, "Keep the first source line\nand preserve this exact continuation.", 9, 10},
		{"3.1", "3.1 Persistence", "decision.3.1.1", 1, "Store one immutable record.", 16, 16},
		{"3.2", "3.2 Ordering", "decision.3.2.1", 1, "Emit in source order.", 28, 28},
		{"3.2", "3.2 Ordering", "decision.3.2.2", 2, "Keep a section-local ordinal.", 30, 30},
	}
	for i, block := range got {
		want := want[i]
		if block.Subject != "design/widget-v1" || block.Source != file.Path || block.SourceSHA256 != file.SHA256 {
			t.Errorf("block %d identity = %+v", i, block)
		}
		if block.Section != want.section || block.SectionHeading != want.heading || block.Family != want.family || block.Ordinal != want.ordinal {
			t.Errorf("block %d address = section %q heading %q family %q ordinal %d; want %+v", i, block.Section, block.SectionHeading, block.Family, block.Ordinal, want)
		}
		if block.Statement != want.statement || block.StartLine != want.startLine || block.EndLine != want.endLine {
			t.Errorf("block %d source projection = statement %q lines %d-%d; want %+v", i, block.Statement, block.StartLine, block.EndLine, want)
		}
		if block.StartByte < 0 || block.StartByte >= block.EndByte || block.EndByte > len(body) || !bytes.Equal(body[block.StartByte:block.EndByte], []byte(block.Evidence)) {
			t.Errorf("block %d evidence span does not reproduce exact bytes", i)
		}
		if block.StatementStartByte < block.StartByte || block.StatementStartByte >= block.StatementEndByte || block.StatementEndByte > block.EndByte || string(body[block.StatementStartByte:block.StatementEndByte]) != block.Statement {
			t.Errorf("block %d statement span does not reproduce exact bytes", i)
		}
		if block.MarkerByte < block.StartByte || block.MarkerByte+len(designDecisionMarker) > block.EndByte || string(body[block.MarkerByte:block.MarkerByte+len(designDecisionMarker)]) != designDecisionMarker || block.MarkerLine < block.StartLine || block.MarkerLine > block.EndLine || block.SectionLine <= 0 {
			t.Errorf("block %d marker/section location is not exact: %+v", i, block)
		}
		if block.EvidenceSHA256 != decisionDigest([]byte(block.Evidence)) {
			t.Errorf("block %d evidence digest = %q", i, block.EvidenceSHA256)
		}
	}
	if strings.Contains(strings.Join([]string{got[0].Statement, got[1].Statement, got[2].Statement, got[3].Statement}, "\n"), "Explanatory rationale") {
		t.Fatal("adjacent rationale was inferred into a Decision")
	}
}

func TestExtractLiveDesignDecisionsPreservesCRLFSourceBytes(t *testing.T) {
	body := []byte("# Widget\r\n\r\n## 2. Decision\r\n\r\n**Decision:** line one\r\nline two\r\n")
	design, file := liveDecisionFixture("widget", body)

	got, err := ExtractLiveDesignDecisions(design, file)
	if err != nil {
		t.Fatalf("ExtractLiveDesignDecisions: %v", err)
	}
	if len(got) != 1 || got[0].Statement != "line one\r\nline two" || got[0].Evidence != "**Decision:** line one\r\nline two\r\n" {
		t.Fatalf("CRLF evidence changed: %+v", got)
	}
	if !bytes.Equal(body[got[0].StartByte:got[0].EndByte], []byte(got[0].Evidence)) {
		t.Fatal("CRLF evidence span is not byte-exact")
	}
}

func TestExtractLiveDesignDecisionsFailsClosed(t *testing.T) {
	validBody := []byte("# Widget\n\n## 2. Decision\n\n**Decision:** choose one.\n")
	tests := []struct {
		name   string
		body   []byte
		mutate func(*Design, *File)
		want   string
	}{
		{name: "not live", body: validBody, mutate: func(d *Design, _ *File) { d.Status = "superseded" }, want: "not live"},
		{name: "source mismatch", body: validBody, mutate: func(d *Design, _ *File) { d.Source = "docs/tech-design/other.md" }, want: "does not match pinned file"},
		{name: "slug path mismatch", body: validBody, mutate: func(_ *Design, f *File) { f.Path = "docs/tech-design/other.md" }, want: "does not match pinned file"},
		{name: "invalid subject", body: validBody, mutate: func(d *Design, f *File) { d.Slug = "Bad"; d.Source = "docs/tech-design/Bad.md"; f.Path = d.Source }, want: "cannot form a memory subject"},
		{name: "file digest mismatch", body: validBody, mutate: func(_ *Design, f *File) { f.SHA256 = strings.Repeat("0", 64) }, want: "pinned file digest"},
		{name: "index digest mismatch", body: validBody, mutate: func(d *Design, _ *File) { d.SHA256 = strings.Repeat("0", 64) }, want: "design-index digest"},
		{name: "no binding marker", body: []byte("# Widget\n\n## 2. Decision\n\nNo callout.\n"), want: "no explicit binding"},
		{name: "marker before section", body: []byte("# Widget\n\n**Decision:** too early.\n"), want: "no section heading"},
		{name: "empty marker line", body: []byte("# Widget\n\n## 2. Decision\n\n**Decision:**\ncontinued guess\n"), want: "followed on the same line"},
		{name: "marker without separator", body: []byte("# Widget\n\n## 2. Decision\n\n**Decision:**guess\n"), want: "followed on the same line"},
		{name: "two markers in paragraph", body: []byte("# Widget\n\n## 2. Decision\n\n**Decision:** first; **Decision:** second.\n"), want: "contains 2 visible"},
		{name: "list marker", body: []byte("# Widget\n\n## 2. Decision\n\n- **Decision:** list item.\n"), want: "instead of a decision paragraph"},
		{name: "list marker after prose", body: []byte("# Widget\n\n## 2. Decision\n\nIntro.\n- **Decision:** list item.\n"), want: "instead of a decision paragraph"},
		{name: "list continuation marker", body: []byte("# Widget\n\n## 2. Decision\n\n- explanatory list item\n  **Decision:** illustrative continuation.\n"), want: "instead of a decision paragraph"},
		{name: "list continuation after blank", body: []byte("# Widget\n\n## 2. Decision\n\n- explanatory list item\n\n  **Decision:** illustrative continuation.\n"), want: "instead of a decision paragraph"},
		{name: "numbered list marker", body: []byte("# Widget\n\n## 2. Decision\n\n1. **Decision:** list item.\n"), want: "instead of a decision paragraph"},
		{name: "quote marker after prose", body: []byte("# Widget\n\n## 2. Decision\n\nIntro.\n> **Decision:** quoted item.\n"), want: "instead of a decision paragraph"},
		{name: "indented marker after prose", body: []byte("# Widget\n\n## 2. Decision\n\nIntro.\n    **Decision:** code block item.\n"), want: "instead of a decision paragraph"},
		{name: "table marker", body: []byte("# Widget\n\n## 2. Decision\n\n| **Decision:** | table cell |\n"), want: "instead of a decision paragraph"},
		{name: "duplicate section", body: []byte("# Widget\n\n## 2. First\n\n**Decision:** one.\n\n## 2. Second\n\n**Decision:** two.\n"), want: "section identity \"2\" occurs 2 times"},
		{name: "heading has no visible identity", body: []byte("# Widget\n\n## `hidden`\n\n**Decision:** no stable section.\n"), want: "no stable textual identity"},
		{name: "only escaped marker", body: []byte("# Widget\n\n## 2. Decision\n\n\\**Decision:** illustrative.\n"), want: "no explicit binding"},
		{name: "only section zero marker", body: []byte("# Widget\n\n## 0. Read to rule\n\n**Decision:** summary only.\n"), want: "no explicit binding"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			design, file := liveDecisionFixture("widget", tt.body)
			if tt.mutate != nil {
				tt.mutate(&design, &file)
			}
			_, err := ExtractLiveDesignDecisions(design, file)
			if !errors.Is(err, ErrDesignDecisionExtraction) || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want ErrDesignDecisionExtraction containing %q", err, tt.want)
			}
			var detail *DesignDecisionExtractionError
			if !errors.As(err, &detail) || detail.Reason == "" {
				t.Fatalf("error has no typed detail: %T %v", err, err)
			}
		})
	}
}

func TestExtractLiveDesignDecisionsCurrentCorpus(t *testing.T) {
	root, err := filepath.Abs(filepath.Join("..", "..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	indexBody, err := os.ReadFile(filepath.Join(root, "docs", "tech-design", "index.json"))
	if err != nil {
		t.Fatal(err)
	}
	var index struct {
		Designs map[string]struct {
			Status string   `json:"status"`
			Serves []string `json:"serves"`
		} `json:"designs"`
	}
	if err := json.Unmarshal(indexBody, &index); err != nil {
		t.Fatal(err)
	}
	var slugs []string
	serving := map[string]bool{}
	for slug, entry := range index.Designs {
		if entry.Status == "live" {
			slugs = append(slugs, slug)
			serving[slug] = len(entry.Serves) > 0
		}
	}
	sort.Strings(slugs)
	if len(slugs) == 0 {
		t.Fatal("generated index has no live technical designs")
	}

	total := 0
	servingTotal := 0
	servingDesigns := 0
	for _, slug := range slugs {
		source := filepath.ToSlash(filepath.Join("docs", "tech-design", slug+".md"))
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(source)))
		if err != nil {
			t.Errorf("%s: %v", source, err)
			continue
		}
		digest := decisionDigest(body)
		blocks, err := ExtractLiveDesignDecisions(
			Design{Slug: slug, Source: source, SHA256: digest, Status: "live"},
			File{Path: source, SHA256: digest, Body: body},
		)
		if err != nil {
			t.Errorf("%s: %v", source, err)
			continue
		}
		for _, block := range blocks {
			if block.StartByte < 0 || block.EndByte > len(body) || block.StartByte >= block.EndByte || !bytes.Equal(body[block.StartByte:block.EndByte], []byte(block.Evidence)) {
				t.Errorf("%s %s: evidence span is not exact", source, block.Family)
			}
		}
		total += len(blocks)
		if serving[slug] {
			servingDesigns++
			servingTotal += len(blocks)
		}
	}
	if total < len(slugs) {
		t.Fatalf("extracted %d decisions from %d live designs; each live design needs at least one", total, len(slugs))
	}
	t.Logf("extracted %d exact Decision blocks from %d live technical designs (%d blocks from %d live serving designs)", total, len(slugs), servingTotal, servingDesigns)
}

func liveDecisionFixture(slug string, body []byte) (Design, File) {
	source := "docs/tech-design/" + slug + ".md"
	digest := decisionDigest(body)
	return Design{Slug: slug, Source: source, SHA256: digest, Status: "live"}, File{Path: source, SHA256: digest, Body: body}
}
