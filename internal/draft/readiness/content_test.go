package readiness

import (
	"os"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
)

func readyFiles(t *testing.T) map[string][]byte {
	t.Helper()
	prd, err := os.ReadFile("../compile/testdata/prd/working.md")
	if err != nil {
		t.Fatal(err)
	}
	prd = []byte(strings.Replace(string(prd), "| Q-1 | Who owns retries? | PM | lock | open | |", "| Q-1 | Who owns retries? | PM | lock | resolved | The PM owns retries. |", 1))
	prd = []byte(strings.Replace(string(prd), "## 5. System requirements", "## 1. Summary\n\n## 2. Problem space\n\n## 3. Goals, metrics, and guardrails\n\n## 4. Solution space\n\n## 5. System requirements", 1))
	prd = []byte(strings.Replace(string(prd), "| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", "### 5.8 Definition of Done\n\n| ID | Done criterion | Source | Verify | Required evidence | Blocking? |", 1))
	prd = []byte(strings.Replace(string(prd), "## 7. Learning", "## 6. Delivery\n\n## 7. Learning", 1))
	prd = append(prd, []byte("\n## 8. Contract readiness\n")...)
	files := map[string][]byte{
		"prd.md":       prd,
		"tech-spec.md": []byte("# Widget — tech spec\n\n### D-1 — Open widget\n\n**Decision:** Open and queue the widget.\n\n**Serves:** FR-1 · UB-1\n"),
	}
	compiled, err := compile.Compile("widget", "main-commit", files)
	if err != nil {
		t.Fatal(err)
	}
	files["reviews/001.md"] = []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** " + compiled.Digest + "\n")
	files["reviews/002.md"] = []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** " + compiled.Digest + "\n")
	return files
}

func judgeFiles(t *testing.T, files map[string][]byte) Verdict {
	t.Helper()
	result, err := compile.Compile("widget", "main-commit", files)
	if err != nil {
		t.Fatal(err)
	}
	return Judge(result, Sources{})
}

func refreshReviews(t *testing.T, files map[string][]byte) {
	t.Helper()
	result, err := compile.Compile("widget", "main-commit", files)
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"reviews/001.md", "reviews/002.md"} {
		body, ok := files[path]
		if !ok {
			continue
		}
		lines := strings.Split(string(body), "\n")
		for i, line := range lines {
			if strings.HasPrefix(line, "**Draft digest:** ") {
				lines[i] = "**Draft digest:** " + result.Digest
			}
		}
		files[path] = []byte(strings.Join(lines, "\n"))
	}
}

func TestContentReadinessConditions(t *testing.T) {
	base := readyFiles(t)
	if verdict := judgeFiles(t, base); !verdict.Ready || len(verdict.Unmet) != 0 {
		t.Fatalf("ready fixture: %+v", verdict)
	}
	cases := []struct {
		name string
		edit func(map[string][]byte)
		want Unmet
	}{
		{"open question", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| resolved | The PM owns retries. |", "| open | |", 1))
		}, Unmet{"FR-12", "prd/widget#Q-1", "answer the question or record a product ruling"}},
		{"open problem", func(f map[string][]byte) {
			f["problems.md"] = []byte("### P-1 — FR-1 — Retry ownership\n\n**Problem:** Who owns retry?\n")
		}, Unmet{"FR-12", "prd/widget#P-1", "answer the question or record a product ruling"}},
		{"tech-spec question", func(f map[string][]byte) {
			f["tech-spec.md"] = append(f["tech-spec.md"], []byte("\n## 7. Open questions\n\n| ID | Question | Owner | Blocks | Status | Resolution |\n|---|---|---|---|---|---|\n| Q1 | Who owns the queue? | engineer | implementation | open | proposed answer |\n")...)
		}, Unmet{"FR-12", "design/widget#Q1", "answer the question or record a product ruling"}},
		{"unserved Must", func(f map[string][]byte) {
			f["tech-spec.md"] = []byte(strings.Replace(string(f["tech-spec.md"]), "FR-1 · UB-1", "UB-1", 1))
		}, Unmet{"FR-13", "prd/widget#FR-1", "add a tech-spec decision serving FR-1"}},
		{"Must fit", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| It opens. | Test |", "| TBD | Test |", 1))
		}, Unmet{"FR-14", "prd/widget#FR-1", "add a pass/fail fit criterion"}},
		{"Must verify", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| It opens. | Test |", "| It opens. | TBD |", 1))
		}, Unmet{"FR-14", "prd/widget#FR-1", "add a verification method"}},
		{"non-Must fit", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| Under 1 ms. | Test |", "| TBD | Test |", 1))
		}, Unmet{"FR-40", "prd/widget#NFR-1", "add a pass/fail fit criterion"}},
		{"non-Must verify", func(f map[string][]byte) {
			f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| Under 1 ms. | Test |", "| Under 1 ms. | TBD |", 1))
		}, Unmet{"FR-40", "prd/widget#NFR-1", "add a verification method"}},
		{"missing review", func(f map[string][]byte) { delete(f, "reviews/002.md") }, Unmet{"FR-16", "reviews/tech-spec", "run the tech-spec review on the current draft"}},
		{"blocking finding", func(f map[string][]byte) {
			f["reviews/001.md"] = append(f["reviews/001.md"], []byte("\n### FND-1 — Missing test\n\n- **Severity**: blocking\n- **Disposition**: open\n")...)
		}, Unmet{"FR-16", "reviews/001.md#FND-1", "apply the finding or record a declined disposition with the operator's ruling"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			files := readyFiles(t)
			tc.edit(files)
			refreshReviews(t, files)
			verdict := judgeFiles(t, files)
			if verdict.Ready || len(verdict.Unmet) != 1 || verdict.Unmet[0] != tc.want {
				t.Fatalf("want only %+v, got %+v", tc.want, verdict.Unmet)
			}
		})
	}
}

func TestReviewsBindOnlyContractDigest(t *testing.T) {
	files := readyFiles(t)
	files["brainstorm/001.md"] = []byte("edited brainstorm")
	if verdict := judgeFiles(t, files); !verdict.Ready {
		t.Fatalf("brainstorm should not stale reviews: %+v", verdict.Unmet)
	}
	files["prd.md"] = append(files["prd.md"], []byte("\nA clarified requirement.\n")...)
	verdict := judgeFiles(t, files)
	if len(verdict.Unmet) != 2 || verdict.Unmet[0].Item != "reviews/001.md" || verdict.Unmet[1].Item != "reviews/002.md" {
		t.Fatalf("both reviews should be stale: %+v", verdict.Unmet)
	}
	refreshReviews(t, files)
	if verdict := judgeFiles(t, files); !verdict.Ready {
		t.Fatalf("refreshed reviews should pass: %+v", verdict.Unmet)
	}
}

// The authoring template explicitly permits "Must (rank 2)". Ranking must
// not exempt a requirement from mandatory coverage or fit-criterion checks.
func TestRankedMustRequirementsHoldReadiness(t *testing.T) {
	for _, priority := range []string{"Must (rank 1)", "must (rank 12)"} {
		for _, tc := range []struct {
			name, condition string
			edit            func(map[string][]byte)
		}{
			{"coverage", "FR-13", func(f map[string][]byte) {
				f["tech-spec.md"] = []byte(strings.Replace(string(f["tech-spec.md"]), "FR-1 · UB-1", "UB-1", 1))
			}},
			{"fit", "FR-14", func(f map[string][]byte) {
				f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| It opens. | Test |", "| TBD | Test |", 1))
			}},
			{"verification", "FR-14", func(f map[string][]byte) {
				f["prd.md"] = []byte(strings.Replace(string(f["prd.md"]), "| It opens. | Test |", "| It opens. | TBD |", 1))
			}},
		} {
			t.Run(priority+"/"+tc.name, func(t *testing.T) {
				files := readyFiles(t)
				before := string(files["prd.md"])
				after := strings.Replace(before, "| Usability | Must | high |", "| Usability | "+priority+" | high |", 1)
				if before == after {
					t.Fatal("fixture has no FR-1 priority to rank")
				}
				files["prd.md"] = []byte(after)
				tc.edit(files)
				refreshReviews(t, files)
				verdict := judgeFiles(t, files)
				if verdict.Ready || len(verdict.Unmet) != 1 || verdict.Unmet[0].Condition != tc.condition || verdict.Unmet[0].Item != "prd/widget#FR-1" {
					t.Fatalf("ranked Must escaped its condition: %+v", verdict)
				}
			})
		}
	}
}

func TestAllContentConditionsAppearTogether(t *testing.T) {
	files := readyFiles(t)
	files["prd.md"] = []byte(strings.Replace(string(files["prd.md"]), "| resolved | The PM owns retries. |", "| open | |", 1))
	files["prd.md"] = []byte(strings.Replace(string(files["prd.md"]), "| It opens. | Test |", "| TBD | TBD |", 1))
	files["prd.md"] = []byte(strings.Replace(string(files["prd.md"]), "| Under 1 ms. | Test |", "| TBD | Test |", 1))
	files["tech-spec.md"] = []byte(strings.Replace(string(files["tech-spec.md"]), "FR-1 · UB-1", "UB-1", 1))
	files["problems.md"] = []byte("### P-1 — FR-1 — Retry ownership\n\n**Problem:** Who owns retry?\n")
	files["reviews/001.md"] = append(files["reviews/001.md"], []byte("\n### FND-1 — Missing test\n\n- **Severity**: blocking\n- **Disposition**: open\n")...)
	verdict := judgeFiles(t, files)
	if verdict.Ready {
		t.Fatal("incomplete draft passed")
	}
	seen := map[string]bool{}
	for _, unmet := range verdict.Unmet {
		seen[unmet.Condition] = true
		if unmet.Item == "" || unmet.Remedy == "" {
			t.Fatalf("incomplete unmet item: %+v", unmet)
		}
	}
	for _, condition := range []string{"FR-12", "FR-13", "FR-14", "FR-16", "FR-40"} {
		if !seen[condition] {
			t.Errorf("missing %s: %+v", condition, verdict.Unmet)
		}
	}
}

func TestDispositionRequiresAnOutcome(t *testing.T) {
	files := readyFiles(t)
	files["reviews/001.md"] = append(files["reviews/001.md"], []byte("\n### FND-1 — Missing test\n\n- **Severity**: blocking\n- **Disposition**: declined — operator · accepts the risk\n")...)
	if verdict := judgeFiles(t, files); !verdict.Ready {
		t.Fatalf("ruled finding should pass: %+v", verdict.Unmet)
	}
}

func TestResolvedTechSpecQuestionClearsReadiness(t *testing.T) {
	files := readyFiles(t)
	files["tech-spec.md"] = append(files["tech-spec.md"], []byte("\n## 7. Open questions\n\n| ID | Question | Owner | Blocks | Status | Resolution |\n|---|---|---|---|---|---|\n| Q1 | Who owns the queue? | engineer | implementation | resolved | The PM owns the queue. |\n")...)
	refreshReviews(t, files)
	if verdict := judgeFiles(t, files); !verdict.Ready {
		t.Fatalf("resolved design question should pass: %+v", verdict.Unmet)
	}
}
