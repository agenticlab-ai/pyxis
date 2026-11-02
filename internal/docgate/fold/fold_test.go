package fold_test

import (
	"bytes"
	"flag"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/fold"
	"github.com/agenticlab-ai/pyxis/internal/docgate/ledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

var update = flag.Bool("update", false, "rewrite golden expect/ files")

// fixtureZeroFingerprint is the provisional fingerprint recorded in the real
// ledger's projection at PRD merge time — the DOD-5 bootstrap target.
const fixtureZeroFingerprint = "sha256:f80cb5311106a0194797052491aa820a150f50479384cf4543f6713c666f111e"

func readEntries(t *testing.T, dir string) []ledger.Entry {
	t.Helper()
	items, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read %s: %v", dir, err)
	}
	var entries []ledger.Entry
	for _, it := range items {
		name := it.Name()
		if it.IsDir() || !strings.HasSuffix(name, ".md") || strings.HasPrefix(name, "effective") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatal(err)
		}
		entries = append(entries, ledger.Entry{Name: name, Bytes: b})
	}
	return entries
}

func TestFixtureZero(t *testing.T) {
	entries := readEntries(t, "testdata/fixture-zero")
	res := fold.Fold(entries)
	if len(res.Defects) != 0 {
		t.Fatalf("fixture zero has defects: %v", res.Defects)
	}
	if res.Fingerprint != fixtureZeroFingerprint {
		t.Fatalf("fingerprint mismatch:\n got %s\nwant %s", res.Fingerprint, fixtureZeroFingerprint)
	}
	again := fold.Fold(entries)
	if !bytes.Equal(res.EffectiveMD, again.EffectiveMD) || !bytes.Equal(res.SourceJSON, again.SourceJSON) {
		t.Fatal("fold is not deterministic across runs")
	}
}

func TestOpsSuiteGolden(t *testing.T) {
	dir := "testdata/ops-suite"
	entries := readEntries(t, dir)
	res := fold.Fold(entries)
	if len(res.Defects) != 0 {
		t.Fatalf("ops-suite has defects: %v", res.Defects)
	}
	goldens := map[string][]byte{
		"expect/effective.md":          res.EffectiveMD,
		"expect/effective.source.json": res.SourceJSON,
	}
	for name, got := range goldens {
		path := filepath.Join(dir, name)
		if *update {
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, got, 0o644); err != nil {
				t.Fatal(err)
			}
			continue
		}
		want, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("golden missing (run with -update): %v", err)
		}
		if !bytes.Equal(got, want) {
			t.Errorf("%s differs from golden (run with -update after intentional changes)", name)
		}
	}
	// The one amendment computes contract-changing and touches all op targets.
	if len(res.Amendments) != 1 || res.Amendments[0].ComputedTier != "contract-changing" {
		t.Fatalf("amendment meta wrong: %+v", res.Amendments)
	}
	joined := strings.Join(res.Amendments[0].Affected, " ")
	for _, want := range []string{"FR-1", "FR-2", "FR-4", "NG-1", "Q1", "§4"} {
		if !strings.Contains(joined, want) {
			t.Errorf("affected IDs missing %s: %s", want, joined)
		}
	}
	md := string(res.EffectiveMD)
	for _, want := range []string{
		"FR-3 *(001 ← FR-1)*",        // supersede lineage
		"FR-4 *(001)*",               // add lineage
		"FR-5 *(001)*",               // rescope absorbing row
		"resolved (001) — Per-shard", // answered question
		"## Superseded & retired",    // appendix
		"**FR-2** *(retired (001))*", // retire recorded verbatim
		"other shards and",           // §4 replaced body
	} {
		if !strings.Contains(md, want) {
			t.Errorf("effective.md missing %q", want)
		}
	}
	if strings.Contains(md, "\n| FR-1 |") {
		t.Error("superseded FR-1 row still present as a live table row")
	}
	byID := map[string]fold.ResolvedUnit{}
	for _, unit := range res.Resolved {
		byID[unit.ID] = unit
	}
	if got := byID["FR-1"]; got.Status != "superseded" || got.SupersededBy == nil || *got.SupersededBy != "FR-3" {
		t.Fatalf("resolved FR-1 lineage wrong: %+v", got)
	}
	if got := byID["FR-2"]; got.Status != "retired" {
		t.Fatalf("resolved FR-2 status wrong: %+v", got)
	}
	if got := byID["FR-3"]; got.Status != "live" || got.Supersedes == nil || *got.Supersedes != "FR-1" || len(got.Columns) != len(got.Cells) {
		t.Fatalf("resolved FR-3 row wrong: %+v", got)
	}
	var gotOrder []string
	for _, unit := range res.Resolved {
		gotOrder = append(gotOrder, unit.ID)
	}
	wantOrder := append([]string(nil), gotOrder...)
	schema.SortIDs(wantOrder)
	if strings.Join(gotOrder, "\x00") != strings.Join(wantOrder, "\x00") {
		t.Fatalf("resolved units are not in canonical ID order: %v", gotOrder)
	}
}

func TestResolvedUnitsFollowSectionSupersede(t *testing.T) {
	b := base("| FR-2 | When old, the s shall disappear. | old rationale | Should | low | old fit | Test | G1 |\n")
	replace := amend(`### op: supersede §5.1 F

Reason: replace the complete requirement section.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When x, the s shall use the replacement. | replacement rationale | Must | high | replacement fit | Test | G1 |
| FR-3 | When new, the s shall include the section-added row. | new rationale | Should | med | new fit | Inspection | G1 |
`, "Blast radius", "tier: contract-changing; touches: §5.1; reason: replace the section")
	res := fold.Fold([]ledger.Entry{b, replace})
	if len(res.Defects) != 0 {
		t.Fatalf("section supersede has defects: %v", res.Defects)
	}
	byID := make(map[string]fold.ResolvedUnit, len(res.Resolved))
	for _, unit := range res.Resolved {
		byID[unit.ID] = unit
	}
	if got := byID["FR-1"]; got.Status != "live" || got.Cells[1] != "When x, the s shall use the replacement." || got.Cells[2] != "replacement rationale" {
		t.Fatalf("retained row did not use current projected cells: %+v", got)
	}
	if got := byID["FR-2"]; got.Status == "live" {
		t.Fatalf("row omitted by section replacement remained live: %+v", got)
	}
	if got := byID["FR-3"]; got.Status != "live" || got.Cells[1] != "When new, the s shall include the section-added row." || got.IntroducedBy != "001-x.md" {
		t.Fatalf("section-introduced row was not included with current truth and origin: %+v", got)
	}
}

func TestResolvedUnitsCanonicalizeFrozenHeaderColumns(t *testing.T) {
	b := ledger.Entry{Name: "000-base.md", Bytes: []byte(`# t — PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v1 |

## 2. Problem

| # | When… | I want to… | So I can… | Today's workaround | Evidence |
|---|---|---|---|---|---|
| J1 | a legacy PRD is folded | retain its current cells | reconcile the same semantic unit | none | merged corpus |
`)}
	res := fold.Fold([]ledger.Entry{b})
	if len(res.Defects) != 0 {
		t.Fatalf("legacy-header fold has defects: %v", res.Defects)
	}
	if len(res.Resolved) != 1 {
		t.Fatalf("resolved units = %d, want 1: %+v", len(res.Resolved), res.Resolved)
	}
	got := res.Resolved[0]
	if got.ID != "J1" || got.Status != "live" || got.Cells[1] != "a legacy PRD is folded" {
		t.Fatalf("legacy row did not retain current cells: %+v", got)
	}
	if !slices.Equal(got.Columns, schema.TableSchemas["J"]) {
		t.Fatalf("compiler columns = %v, want semantic schema %v", got.Columns, schema.TableSchemas["J"])
	}
}

func TestResolvedUnitsRejectNonLiveIDReintroduction(t *testing.T) {
	retire := amend("### op: retire FR-1\n\nReason: remove it.\n")
	reintroduce := amend(`### op: supersede §5.1 F

Reason: illegally reuse the retired ID.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When reused, the s shall fail closed. | r | Must | med | f | Test | G1 |
`, "Blast radius", "tier: contract-changing; touches: §5.1; reason: r")
	reintroduce.Name = "002-reintroduce.md"
	red(t, "section-reintroduces-retired-id", "non-live unit FR-1 is reintroduced", base(""), retire, reintroduce)
}

// TestResolvedUnitsMatchCurrentCorpus guards the compiler face against the
// repository's complete current PRD corpus. It also byte-checks both public
// artifacts so a Resolved-only repair cannot mutate committed projections.
func TestResolvedUnitsMatchCurrentCorpus(t *testing.T) {
	root := filepath.Join("..", "..", "..", "docs", "prds")
	features, err := os.ReadDir(root)
	if err != nil {
		t.Fatalf("read current PRD corpus: %v", err)
	}
	checked := 0
	for _, feature := range features {
		if !feature.IsDir() {
			continue
		}
		dir := filepath.Join(root, feature.Name())
		if _, err := os.Stat(filepath.Join(dir, "000-base.md")); err != nil {
			continue
		}
		checked++
		t.Run(feature.Name(), func(t *testing.T) {
			res := fold.Fold(readEntries(t, dir))
			if len(res.Defects) != 0 {
				t.Fatalf("fold current ledger: %v", res.Defects)
			}
			for name, got := range map[string][]byte{
				"effective.md":          res.EffectiveMD,
				"effective.source.json": res.SourceJSON,
			} {
				want, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil {
					t.Fatalf("read committed %s: %v", name, err)
				}
				if !bytes.Equal(got, want) {
					t.Fatalf("%s changed while deriving Resolved", name)
				}
			}

			current, defects := ledger.ParseProjection("effective.md", res.EffectiveMD)
			if current == nil || len(defects) != 0 {
				t.Fatalf("parse current effective projection: %v", defects)
			}
			live := map[string]fold.ResolvedUnit{}
			for _, unit := range res.Resolved {
				if unit.Status == "live" {
					live[unit.ID] = unit
				}
			}
			for id, projected := range current.Units {
				resolved, ok := live[id]
				if !ok {
					t.Errorf("effective live unit %s is absent from Resolved", id)
					continue
				}
				columns := schema.TableSchemas[projected.Prefix]
				if resolved.Prefix != projected.Prefix || !slices.Equal(resolved.Columns, columns) || !slices.Equal(resolved.Cells, projected.Cells) {
					t.Errorf("Resolved live unit %s differs from effective.md:\n got %+v\nwant %+v", id, resolved.Cells, projected.Cells)
				}
			}
			for id := range live {
				if current.Units[id] == nil {
					t.Errorf("Resolved unit %s is live but absent from effective.md", id)
				}
			}
		})
	}
	if checked == 0 {
		t.Fatal("current PRD corpus contained no ledgers")
	}
}

// red builds a ledger from literals and asserts one defect reason substring.
func red(t *testing.T, name, wantReason string, entries ...ledger.Entry) {
	t.Helper()
	res := fold.Fold(entries)
	for _, d := range res.Defects {
		if strings.Contains(d.Reason, wantReason) {
			if d.Remedy == "" {
				t.Errorf("%s: defect has no remedy", name)
			}
			return
		}
	}
	t.Errorf("%s: no defect containing %q (got %v)", name, wantReason, res.Defects)
}

func base(extra string) ledger.Entry {
	return ledger.Entry{Name: "000-base.md", Bytes: []byte(`# t — PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v1 |

## 5. R

### 5.1 F

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When x, the s shall y. | r | Must | med | f | Test | G1 |
` + extra)}
}

func amend(ops string, headerOverride ...string) ledger.Entry {
	h := map[string]string{
		"Amends": "t", "Status": "Approved", "Owner": "@o", "Date": "2026-07-06",
		"Learned-from": "build-result:x", "Blast radius": "tier: contract-changing; touches: FR-1; reason: r",
		"Template version": "amendment-template/v1",
	}
	for i := 0; i+1 < len(headerOverride); i += 2 {
		h[headerOverride[i]] = headerOverride[i+1]
	}
	var b strings.Builder
	b.WriteString("# Amendment 001 — x\n\n| Field | Value |\n|---|---|\n")
	for _, f := range []string{"Amends", "Status", "Owner", "Date", "Learned-from", "Blast radius", "Template version"} {
		b.WriteString("| **" + f + "** | " + h[f] + " |\n")
	}
	b.WriteString("\n## Operations\n\n" + ops + "\n")
	return ledger.Entry{Name: "001-x.md", Bytes: []byte(b.String())}
}

const goodSupersede = `### op: supersede FR-1

Reason: r.

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-2 | When x, the s shall z. | r | Must | med | f | Test | G1 |
`

func TestRedCases(t *testing.T) {
	red(t, "dangling-target", "dangling target", base(""), amend("### op: retire FR-9\n\nReason: r.\n"))
	red(t, "double-supersede", "already superseded", base(""), amend(goodSupersede),
		ledger.Entry{Name: "002-y.md", Bytes: amend(strings.Replace(goodSupersede, "FR-2", "FR-7", 1)).Bytes})
	red(t, "id-reuse", "ID reuse", base(""), amend(strings.Replace(goodSupersede, "FR-2", "FR-1", 2)))
	red(t, "payload-schema", "payload columns do not match", base(""),
		amend("### op: supersede FR-1\n\nReason: r.\n\n| ID | Requirement (EARS) | Priority |\n|---|---|---|\n| FR-2 | w | Must |\n"))
	red(t, "unknown-verb", "unknown operation verb", base(""), amend("### op: tighten FR-1\n\nReason: r.\n"))
	red(t, "status-draft", "not Approved", base(""), amend(goodSupersede, "Status", "Draft"))
	red(t, "bad-evidence", "unknown evidence class", base(""), amend(goodSupersede, "Learned-from", "vibes:review"))
	red(t, "understated-blast", "understates computed", base(""), amend(goodSupersede, "Blast radius", "tier: additive; touches: FR-1; reason: r"))
	red(t, "missing-answer-field", "missing field", base("\n### 7.2 Q\n\n| # | Question | Owner | Needed by | Status |\n|---|---|---|---|---|\n| Q1 | q | @o | d | open |\n"),
		amend("### op: answer Q1\n\n| Field | Value |\n|---|---|\n| **Answer** | a |\n"))
	red(t, "unknown-add-kind", "unknown add target kind", base(""), amend("### op: add ZZZ\n\nReason: r.\n\n| ID |\n|---|\n| Z-1 |\n"))
	red(t, "unknown-envelope-field", "unknown to the declared template version",
		base(""), ledger.Entry{Name: "001-x.md", Bytes: append(amend(goodSupersede).Bytes[:0:0], []byte(strings.Replace(string(amend(goodSupersede).Bytes), "| **Owner** | @o |", "| **Owner** | @o |\n| **Mood** | fine |", 1))...)})
	red(t, "empty-ledger", "empty ledger")
	red(t, "duplicate-base-id", "duplicate ID", ledger.Entry{Name: "000-base.md",
		Bytes: append([]byte{}, []byte(strings.Replace(string(base("").Bytes), "| FR-1 | When x, the s shall y. | r | Must | med | f | Test | G1 |",
			"| FR-1 | When x, the s shall y. | r | Must | med | f | Test | G1 |\n| FR-1 | dup | r | Must | med | f | Test | G1 |", 1))...)})
}

// TestFoldPurity walks the non-test sources of the pure packages and rejects
// any import outside the allowlist (design A-1 / DINV-1).
func TestFoldPurity(t *testing.T) {
	allow := map[string]bool{
		"crypto/sha256": true, "encoding/hex": true, "encoding/json": true,
		"fmt": true, "regexp": true, "sort": true, "strconv": true, "strings": true,
		"github.com/agenticlab-ai/pyxis/internal/docgate/schema": true,
		"github.com/agenticlab-ai/pyxis/internal/docgate/ledger": true,
	}
	for _, dir := range []string{".", "../ledger", "../schema"} {
		fset := token.NewFileSet()
		//nolint:staticcheck // Import-only purity inspection intentionally does not need package loading or build-tag association.
		pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
			return !strings.HasSuffix(fi.Name(), "_test.go")
		}, parser.ImportsOnly)
		if err != nil {
			t.Fatal(err)
		}
		for _, pkg := range pkgs {
			for fname, f := range pkg.Files {
				for _, imp := range f.Imports {
					path := strings.Trim(imp.Path.Value, `"`)
					if !allow[path] {
						t.Errorf("%s imports %s — outside the purity allowlist (DINV-1)", fname, path)
					}
				}
			}
		}
	}
}

// TestPRDTemplateV2AndSectionZero is read-to-rule-v1's A-5 fixture: a
// prd-template/v2 base carrying §0 plus one wholesale §0 supersede declaring
// the computed contract-changing tier folds green; an unregistered template
// version still defects.
func TestPRDTemplateV2AndSectionZero(t *testing.T) {
	v2base := ledger.Entry{Name: "000-base.md", Bytes: []byte(`# t — PRD

| Field | Value |
|---|---|
| **Template version** | prd-template/v2 |

## 0. Read to rule — the human reviewer's section (~5 min)

*The body is the contract; this section only routes attention.*

**Judge these** — the original call.

## 5. R

### 5.1 F

| ID | Requirement (EARS) | Rationale | Priority | Risk | Fit criterion | Verify | Traces to |
|---|---|---|---|---|---|---|---|
| FR-1 | When x, the s shall y. | r | Must | med | f | Test | G1 |
`)}
	refresh := amend("### op: supersede §0\n\nReason: §0 refreshed after the contract moved.\n\n*The body is the contract; this section only routes attention.*\n\n**Judge these** — the refreshed call.\n",
		"Blast radius", "tier: contract-changing; touches: §0; reason: informative §0 refresh — rides the computed tier")
	res := fold.Fold([]ledger.Entry{v2base, refresh})
	if len(res.Defects) != 0 {
		t.Fatalf("v2 base + §0 supersede has defects: %v", res.Defects)
	}
	md := string(res.EffectiveMD)
	if !strings.Contains(md, "the refreshed call") {
		t.Error("effective.md missing the §0 replacement body")
	}
	if strings.Contains(md, "the original call") {
		t.Error("old §0 body still live after wholesale supersede")
	}

	red(t, "unregistered-template-version", "not in the supported set",
		ledger.Entry{Name: "000-base.md", Bytes: []byte("# t — PRD\n\n| Field | Value |\n|---|---|\n| **Template version** | prd-template/v9 |\n")})
}

// TestSectionSupersedeEmptyBodyRefused pins #523: a section-supersede whose
// payload self-truncated to nothing (the classic cause: the author included
// the section's own heading, which ends the op block at the parser's
// boundary) must RED the fold with a named defect — never project a hollowed
// section while every byte-compare check stays green (AP-17).
func TestSectionSupersedeEmptyBodyRefused(t *testing.T) {
	b := base("\n### 5.2 P\n\nold perimeter prose.\n")

	// The heading-in-payload form: everything after the heading line belongs
	// to a new block, so the op's effective body is empty.
	headingPayload := amend("### op: supersede §5.2 P\n\nReason: r.\n\n### 5.2 P\n\nnew perimeter prose that never attaches.\n",
		"Blast radius", "tier: contract-changing; touches: §5.2; reason: r")
	red(t, "empty-section-supersede", "empty replacement body", b, headingPayload)

	// The body-only form folds, replacing the section body.
	bodyPayload := amend("### op: supersede §5.2 P\n\nReason: r.\n\nnew perimeter prose.\n",
		"Blast radius", "tier: contract-changing; touches: §5.2; reason: r")
	res := fold.Fold([]ledger.Entry{b, bodyPayload})
	if len(res.Defects) != 0 {
		t.Fatalf("body-only section supersede has defects: %v", res.Defects)
	}
	md := string(res.EffectiveMD)
	if !strings.Contains(md, "new perimeter prose.") || strings.Contains(md, "old perimeter prose.") {
		t.Errorf("section body not replaced correctly:\n%s", md)
	}
}
