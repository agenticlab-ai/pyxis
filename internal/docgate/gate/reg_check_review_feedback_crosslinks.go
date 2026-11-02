package gate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/feedback"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// The cross-link arm of review-feedback capture (docs/tech-design/
// review-feedback-capture.md §3.5, FR-8): a base-free, repo-wide self-registered
// check that enforces every ONE-DIRECTIONAL cross-link a ledger entry carries —
// `review:<report-stem>#<anchor>` to a reviewer M2 residual and
// `delivery:<slug>#<anchor>` to an Argus E-n disposition — resolves to its
// target, reddening a dangling one ("broken cross-link → flagged", FR-8's fit
// criterion; AP-10, convention → gate).
//
// One-directional by construction: the check reads only the entry's own
// `## Cross-links` section (via the shared fold parse) and the cited target
// files — it never writes, so the targets (`review-report/v1`, the Argus
// ledgers) are provably unmodified (NG-5). The sibling `check-review-feedback-
// resolution` enforces the *inbound* edge (documents citing an entry); this arm
// enforces the *outbound* edge (an entry citing an existing capture), and the
// two never overlap. A `delivery:` cross-link is resolution-enforced here even
// when its slug names no ledger — the inverse of `check-ledger-citations`, which
// scope-decides on resolvability and so silently drops an unknown slug; a
// cross-link must resolve, so a dead one is a defect, never a silent drop.
//
// Report-only; the only subprocess is `git ls-files`, exactly as the sibling
// base-free corpus checks read the tracked tree.
func init() {
	Register(Check{
		Verb:           reviewFeedbackCrosslinkVerb,
		Help:           "every review-feedback entry cross-link resolves to its reviewer/Argus target",
		NeedsMergeBase: false,
		Run:            func(c Ctx) Defects { return CheckReviewFeedbackCrosslinks(c.Root, c.Cfg) },
	})
}

const (
	reviewFeedbackCrosslinkVerb = "check-review-feedback-crosslinks"
	// opUnresolvedCrosslink is the single FR-8 discriminator: an entry cross-link
	// whose target file, finding anchor, or journal entry does not exist.
	opUnresolvedCrosslink = "unresolved-crosslink"
)

// crossLinkParseRe splits one recognized cross-link into its kind, stem, and
// anchor. It mirrors feedback.crossLinkRe (the fold's collector), so this arm
// resolves exactly the citations the projection carries — one grammar, two
// readers, never divergent.
var crossLinkParseRe = regexp.MustCompile(`^(review|delivery):([a-z0-9]+(?:-[a-z0-9]+)*)#([A-Za-z0-9][A-Za-z0-9_-]*)$`)

// parsedDeliveryLedger caches one Argus delivery ledger's parse — reused across
// every cross-link that cites it, and its parse defect surfaced once.
type parsedDeliveryLedger struct {
	ledger  *argusledger.Ledger
	defects []schema.Defect
}

// CheckReviewFeedbackCrosslinks enumerates the tracked corpus once
// (`git ls-files`), then for every review-feedback ledger entry resolves each
// one-directional cross-link it carries: a `review:` link to a review-report
// finding heading, a `delivery:` link to an Argus journal E-n entry. A cross-
// link whose target file is untracked, whose review-report finding anchor is
// absent, or whose delivery anchor resolves to no journal entry (or into the
// never-citable derived-state block) reds the check; a resolving one is silent.
// Malformed entries are the fold/verify-generated verbs' concern and are skipped
// here so a single defect is not reported twice.
func CheckReviewFeedbackCrosslinks(root string, cfg config.Config) []schema.Defect {
	tracked, err := gitLsFiles(root)
	if err != nil {
		return []schema.Defect{vdef(reviewFeedbackCrosslinkVerb, "", "corpus",
			"could not enumerate the tracked corpus: "+err.Error(),
			"the check runs `git ls-files` against the local clone; ensure it is a git repository")}
	}
	trackedSet := make(map[string]bool, len(tracked))
	for _, p := range tracked {
		trackedSet[p] = true
	}

	feedbackRoot := filepath.ToSlash(filepath.Clean(cfg.Feedback)) + "/"
	reviewsRoot := filepath.ToSlash(filepath.Clean(cfg.Reviews))
	problemsRoot := filepath.ToSlash(filepath.Clean(cfg.Problems))
	ledgerCache := map[string]parsedDeliveryLedger{}
	ledgerReported := map[string]bool{}

	var defects []schema.Defect
	emit := func(rel, reason, remedy string) {
		defects = append(defects, vdef(reviewFeedbackCrosslinkVerb, rel, opUnresolvedCrosslink, reason, remedy))
	}

	for _, rel := range tracked {
		// A cross-link is enforced only where it lives: an entry's own section.
		// docs/feedback/<stem>/<NNN>-<source>.md — exactly one directory deep.
		if !strings.HasPrefix(rel, feedbackRoot) {
			continue
		}
		sub := strings.TrimPrefix(rel, feedbackRoot)
		parts := strings.Split(sub, "/")
		if len(parts) != 2 || !feedbackEntryFileRe.MatchString(parts[1]) {
			continue
		}
		b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
		if err != nil {
			continue
		}
		entry, pdef := feedback.ParseEntry(parts[1], b)
		if len(pdef) > 0 || entry == nil {
			continue // a malformed entry is the fold/verify-generated verbs' job
		}
		for _, cite := range entry.CrossLinks {
			m := crossLinkParseRe.FindStringSubmatch(cite)
			if m == nil {
				continue // not a recognized grammar (the fold would not have carried it)
			}
			kind, stem, anchor := m[1], m[2], m[3]

			switch kind {
			case "review":
				target := reviewsRoot + "/" + stem + ".md"
				if !trackedSet[target] {
					emit(rel, fmt.Sprintf("%s does not resolve: no review report at %s", cite, target),
						fmt.Sprintf("cite an existing review report under %s/", reviewsRoot))
					continue
				}
				rb, rerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(target)))
				if rerr != nil {
					continue // tracked-but-unreadable is another verb's IO concern
				}
				if !reviewReportHasAnchor(rb, anchor) {
					emit(rel, fmt.Sprintf("%s does not resolve: %s carries no %s finding", cite, target, anchor),
						"the anchor is a review-report finding id (a `### FND-n` heading)")
				}

			case "delivery":
				target := problemsRoot + "/" + stem + "/delivery.md"
				if !trackedSet[target] {
					emit(rel, fmt.Sprintf("%s does not resolve: no delivery ledger at %s", cite, target),
						fmt.Sprintf("cite an existing Argus delivery ledger under %s/<slug>/", problemsRoot))
					continue
				}
				pc, seen := ledgerCache[stem]
				if !seen {
					lb, lerr := os.ReadFile(filepath.Join(root, filepath.FromSlash(target)))
					if lerr != nil {
						pc.defects = []schema.Defect{vdef(reviewFeedbackCrosslinkVerb, target, "read",
							lerr.Error(), "the cited delivery ledger must be readable")}
					} else {
						pc.ledger, pc.defects = argusledger.Parse(cfg.Problems, reviewFeedbackCrosslinkVerb, target, lb)
					}
					ledgerCache[stem] = pc
				}
				if len(pc.defects) > 0 {
					// A malformed cited ledger cannot be resolved against — surface
					// its named parse defect once, never a false pass.
					if !ledgerReported[stem] {
						ledgerReported[stem] = true
						defects = append(defects, pc.defects...)
					}
					continue
				}
				switch pc.ledger.ResolveAnchor(anchor) {
				case argusledger.AnchorJournal:
					// resolves — a journal E-n disposition (FR-8)
				case argusledger.AnchorDerived:
					emit(rel, fmt.Sprintf("%s resolves into %s's never-citable derived-state block", cite, target),
						"cite a journal E-n entry; the derived-state block is regenerated and never authoritative")
				default:
					emit(rel, fmt.Sprintf("%s does not resolve: %s carries no %s journal entry", cite, target, anchor),
						"the anchor is an Argus delivery-ledger journal entry (an `E-n`)")
				}
			}
		}
	}
	return defects
}

// reviewReportHasAnchor reports whether a review report contains a finding whose
// id equals anchor — a heading (`### FND-n — …`) whose first title token is the
// anchor. The finding heading is the review-report/v1 anchor a `review:` cross-
// link resolves to; the check never reads or writes past it (one-directional).
func reviewReportHasAnchor(b []byte, anchor string) bool {
	for _, l := range strings.Split(string(schema.Normalize(b)), "\n") {
		t := strings.TrimSpace(l)
		if !strings.HasPrefix(t, "#") {
			continue
		}
		title := strings.TrimSpace(strings.TrimLeft(t, "#"))
		if title == "" {
			continue
		}
		if strings.Fields(title)[0] == anchor {
			return true
		}
	}
	return false
}
