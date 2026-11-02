package lock

import (
	"strings"
	"testing"

	localgit "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
	"github.com/agenticlab-ai/pyxis/internal/draft/compile"
)

func TestLocalSecondVersionRequiresItsVerifiedSeedAndFreshReview(t *testing.T) {
	localgittest.Skip(t)
	for _, format := range []string{"sha1", "sha256"} {
		t.Run(format, func(t *testing.T) {
			t.Setenv("GIT_DEFAULT_HASH", format)
			testLocalSecondVersionRequiresItsVerifiedSeedAndFreshReview(t, format)
		})
	}
}

func testLocalSecondVersionRequiresItsVerifiedSeedAndFreshReview(t *testing.T, format string) {
	t.Helper()
	root, cfg, options, local := localPreparationFixture(t, true)
	if actual := gitFixture(t, root, "rev-parse", "--show-object-format"); actual != format {
		t.Fatalf("fixture object format = %s, want %s", actual, format)
	}
	if _, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local); err != nil {
		t.Fatal(err)
	}
	receipts, landings := localgit.NewReceipts(options), localgit.NewDraftLandings(options)
	firstBinding, err := receipts.Binding(t.Context(), "draft-widget-v1")
	if err != nil {
		t.Fatal(err)
	}
	firstReceipt, err := receipts.Record(t.Context(), firstBinding, "operator", "")
	if err != nil {
		t.Fatal(err)
	}
	first, err := LockLocal(t.Context(), root, cfg, firstBinding, draftshell.RunLocalGit, local, receipts, landings)
	if err != nil {
		t.Fatal(err)
	}
	// Reopening must use the verified origin revision even when a checkout's
	// cached tag points at unrelated older memory.
	gitFixture(t, root, "update-ref", "refs/tags/lock/widget/v1", firstBinding.TargetOID)
	seed, err := LoadLocalContract(t.Context(), root, "widget", 1, landings)
	if err != nil {
		t.Fatal(err)
	}
	gitFixture(t, root, "fetch", "origin", "main")
	gitFixture(t, root, "switch", "-c", "draft/widget-v2", "origin/main")
	fixtureFile(t, root, "drafts/widget/prd.md", seed.PRD)
	fixtureFile(t, root, "drafts/widget/tech-spec.md", seed.TechSpec)
	gitFixture(t, root, "add", "--", "drafts/widget/prd.md", "drafts/widget/tech-spec.md")
	gitFixture(t, root, "commit", "-m", "seed next version from exact verified lock")
	prd := []byte(strings.Replace(string(seed.PRD), "The widget shall open.", "The widget shall open durably.", 1))
	if string(prd) == string(seed.PRD) {
		t.Fatal("fixture did not revise the contract")
	}
	revised, err := compile.Compile("widget", first.Lock.MergeCommit, map[string][]byte{"prd.md": prd, "tech-spec.md": seed.TechSpec})
	if err != nil {
		t.Fatal(err)
	}
	fixtureFile(t, root, "drafts/widget/prd.md", prd)
	fixtureFile(t, root, "drafts/widget/reviews/003.md", []byte("# PRD review\n\n**Review subject:** prd\n**Draft digest:** "+revised.Digest+"\n"))
	fixtureFile(t, root, "drafts/widget/reviews/004.md", []byte("# Tech-spec review\n\n**Review subject:** tech-spec\n**Draft digest:** "+revised.Digest+"\n"))
	gitFixture(t, root, "add", "--", "drafts/widget/prd.md", "drafts/widget/reviews/003.md", "drafts/widget/reviews/004.md")
	gitFixture(t, root, "commit", "-m", "revise and review the second contract")
	if _, err := local.Proposals.Publish(t.Context(), codehost.ProposalChange{ID: "draft-widget-v2", Feature: "widget", Branch: "refs/heads/draft/widget-v2", Head: gitFixture(t, root, "rev-parse", "HEAD"), State: "open", ExpectedTargetOID: first.Lock.MergeCommit}); err != nil {
		t.Fatal(err)
	}
	if _, err := PrepareLocal(t.Context(), root, cfg, "widget", draftshell.RunLocalGit, local); err != nil {
		t.Fatal(err)
	}
	binding, err := receipts.Binding(t.Context(), "draft-widget-v2")
	if err != nil {
		t.Fatal(err)
	}
	if binding.Nonce == firstBinding.Nonce || binding.Digest == firstBinding.Digest {
		t.Fatal("second preparation reused its prior review or contract")
	}
	if _, err := receipts.Record(t.Context(), binding, "operator", ""); err != nil {
		t.Fatal(err)
	}
	// Preparation cannot make a lost prior lock trustworthy. Locking rechecks
	// the retained seed evidence, source history, and current version again.
	firstReceiptRef := "refs/pyxis/receipts/" + firstBinding.ProposalID + "/" + firstBinding.Nonce
	gitFixture(t, options.Origin, "update-ref", "-d", firstReceiptRef)
	before := localLandingRefs(t, options.Origin)
	if result, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, landings); err == nil || result.Created {
		t.Fatalf("missing seed evidence accepted: %+v %v", result, err)
	}
	if localLandingRefs(t, options.Origin) != before {
		t.Fatal("unverified seed partially landed the next version")
	}
	gitFixture(t, options.Origin, "update-ref", firstReceiptRef, firstReceipt.OID)
	before = localLandingRefs(t, options.Origin)
	if result, err := LockLocal(t.Context(), root, cfg, binding, refuseLocalCommand([]string{"rev-list", "--reverse", "--first-parent"}, ""), local, receipts, landings); err == nil || result.Created || !strings.Contains(err.Error(), "reopened draft history") {
		t.Fatalf("unreadable seed history accepted: %+v %v", result, err)
	}
	if localLandingRefs(t, options.Origin) != before {
		t.Fatal("unreadable seed partially landed the next version")
	}
	gitFixture(t, options.Origin, "update-ref", "refs/tags/lock/widget/v2", first.Lock.MergeCommit)
	before = localLandingRefs(t, options.Origin)
	if result, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, landings); err == nil || result.Created || !strings.Contains(err.Error(), "lock version changed") {
		t.Fatalf("changed version accepted: %+v %v", result, err)
	}
	if localLandingRefs(t, options.Origin) != before {
		t.Fatal("changed version partially landed")
	}
	gitFixture(t, options.Origin, "update-ref", "-d", "refs/tags/lock/widget/v2")
	second, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, landings)
	if err != nil || !second.Created || second.Lock.Version != 2 || second.Lock.ProposalsSinceOpened != 1 || first.Lock.ProposalsSinceOpened != 1 {
		t.Fatalf("second lock: %+v %v", second, err)
	}
	contract, err := LoadLocalContract(t.Context(), root, "widget", 2, landings)
	if err != nil || string(contract.PRD) != string(prd) || contract.Digest != revised.Digest {
		t.Fatalf("second locked text changed: %+v %v", contract, err)
	}
	if older, err := landings.VerifyDraftLock(t.Context(), "widget", 1); err != nil || older != first.Lock {
		t.Fatalf("second landing invalidated the first retained lock: %+v %v", older, err)
	}
	changed := binding
	changed.Nonce = strings.Repeat("f", 64)
	if _, err := LockLocal(t.Context(), root, cfg, changed, draftshell.RunLocalGit, local, receipts, landings); err == nil {
		t.Fatal("retry accepted a different saved review request")
	}
	secondReceiptRef := "refs/pyxis/receipts/" + binding.ProposalID + "/" + binding.Nonce
	gitFixture(t, options.Origin, "update-ref", "-d", secondReceiptRef)
	if _, err := LockLocal(t.Context(), root, cfg, binding, draftshell.RunLocalGit, local, receipts, landings); err == nil {
		t.Fatal("retry accepted missing retained review evidence")
	}
}
