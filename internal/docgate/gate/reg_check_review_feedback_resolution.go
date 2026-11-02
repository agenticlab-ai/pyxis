package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The enforced-resolution arm of review-feedback capture (docs/tech-design/
// review-feedback-capture.md §3.5, DINV-5): a base-free, repo-wide
// self-registered check that scans the tracked corpus for `review-feedback:`
// citations and reds any that does not resolve to a real ledger entry — moving
// the FR-4/M4 provenance guarantee from convention to gate (AP-10).
//
// A `review-feedback:<subject-stem>#<entry-anchor>` occurrence is recognized as
// a citation only outside code context (a fenced block or an inline-code span
// is a displayed example, the `check-links`/`check-ledger-citations` corpus
// convention). The angle-bracketed `<subject-stem>`/`<entry-anchor>`
// metavariables never match the concrete-token grammar, so a grammar
// illustration written with the placeholder is not even an occurrence. Unlike
// the delivery arm — where target resolvability *decides* scope — resolution is
// exactly what this check enforces: every recognized citation MUST resolve, so
// a dead ref is a defect rather than a silent drop.
//
// The same recognition covers the amendment model's free ref slot without
// touching it (F-6): a `Learned-from: review-thread:review-feedback:<stem>#<a>`
// line embeds a `review-feedback:` citation, so the uniform scan resolves it —
// a real anchor passes, a dangling one reds — while `validate-amendments` still
// sees only the `review-thread` *class* it already accepts (no model change).
//
// Report-only; the only subprocess is `git ls-files`, exactly as the sibling
// base-free corpus checks read the tracked tree.
func init() {
	Register(Check{
		Verb:           reviewFeedbackResolutionVerb,
		Help:           "every review-feedback citation resolves to a real ledger entry",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckReviewFeedbackResolution(c.Root, c.Cfg) },
	})
}

const (
	reviewFeedbackResolutionVerb = "check-review-feedback-resolution"
	// opUnresolvedFeedbackRef is the single DINV-5 discriminator: a recognized
	// `review-feedback:` citation whose subject/anchor names no ledger entry.
	opUnresolvedFeedbackRef = "unresolved-review-feedback"
)

// feedbackCiteRe matches one `review-feedback:<subject-stem>#<entry-anchor>`
// occurrence: a kebab-case subject stem and a dense three-digit entry sequence
// — the entry's anchor, equal to its filename's NNN (§3.1). The angle-bracketed
// metavariables carry a `<`, which this concrete grammar never matches.
var feedbackCiteRe = regexp.MustCompile(`review-feedback:([a-z0-9]+(?:-[a-z0-9]+)*)#(\d{3})`)

// feedbackEntryFileRe matches a ledger entry's basename: NNN-<source>.md — the
// per-subject directory's citable content (the generated projection is not an
// entry and never resolves a citation).
var feedbackEntryFileRe = regexp.MustCompile(`^(\d{3})-[a-z0-9][a-z0-9-]*\.md$`)

// CheckReviewFeedbackResolution enumerates the tracked corpus once
// (`git ls-files`), builds the set of real ledger entry anchors per subject
// from the files under the configured feedback root, then reports a defect for
// every recognized `review-feedback:` citation whose subject/anchor names no
// such entry. It is silent on every non-citation — an occurrence inside a code
// span or fenced block, or an angle-bracketed metavariable — so the design/plan
// docs' grammar illustrations stay green (DINV-5, §3.5).
func CheckReviewFeedbackResolution(root string, cfg config.Config) []schema.Defect {
	tracked, err := gitLsFiles(root)
	if err != nil {
		return []schema.Defect{vdef(reviewFeedbackResolutionVerb, "", "corpus",
			"could not enumerate the tracked corpus: "+err.Error(),
			"the check runs `git ls-files` against the local clone; ensure it is a git repository")}
	}

	// The resolvable index: subject stem -> the set of entry anchors (NNN) that
	// exist as tracked files under the feedback root. "Resolves by grep" is this
	// membership test — no separate filesystem read, the tracked list is the
	// ground truth.
	feedbackRoot := filepath.ToSlash(filepath.Clean(cfg.Feedback)) + "/"
	index := map[string]map[string]bool{}
	for _, rel := range tracked {
		if !strings.HasPrefix(rel, feedbackRoot) {
			continue
		}
		// docs/feedback/<stem>/<NNN>-<source>.md — exactly one directory below
		// the root; anything deeper or shallower is not a citable entry.
		sub := strings.TrimPrefix(rel, feedbackRoot)
		parts := strings.Split(sub, "/")
		if len(parts) != 2 {
			continue
		}
		stem, base := parts[0], parts[1]
		m := feedbackEntryFileRe.FindStringSubmatch(base)
		if m == nil {
			continue // the projection or any non-entry file
		}
		if index[stem] == nil {
			index[stem] = map[string]bool{}
		}
		index[stem][m[1]] = true
	}

	var defects []schema.Defect
	for _, rel := range tracked {
		if !strings.HasSuffix(rel, ".md") {
			continue // citations live in markdown documents
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		fenced := false
		for i, line := range strings.Split(string(schema.Normalize(b)), "\n") {
			// A citation inside a fenced code block or an inline-code span is a
			// displayed example, not a citation (the shared corpus convention).
			if strings.HasPrefix(strings.TrimSpace(line), "```") {
				fenced = !fenced
				continue
			}
			if fenced {
				continue
			}
			for _, m := range feedbackCiteRe.FindAllStringSubmatch(stripCodeSpans(line), -1) {
				stem, anchor := m[1], m[2]
				if index[stem][anchor] {
					continue // resolves — a real ledger entry (silent)
				}
				cite := "review-feedback:" + stem + "#" + anchor
				defects = append(defects, vdef(reviewFeedbackResolutionVerb, rel, opUnresolvedFeedbackRef,
					fmt.Sprintf("line %d: %s resolves to no ledger entry under %s%s/", i+1, cite, feedbackRoot, stem),
					"cite an existing entry: the anchor is its dense NNN sequence and the subject its docs/feedback/<stem>/ directory"))
			}
		}
	}
	return defects
}
