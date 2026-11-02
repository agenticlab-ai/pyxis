package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	localcodehost "github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit"
	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func draftReviewCLIFixture(t *testing.T) (string, string, *localcodehost.ProposalStore, codehost.ProposalGeneration) {
	t.Helper()
	root, origin := localGitCLIFixture(t, true)
	options := localcodehost.Options{Root: root, Origin: origin, Remote: "upstream", Mainline: "trunk"}
	proposals := localcodehost.NewProposals(localcodehost.ProposalOptions{Git: options, Kind: "draft"})
	target := localGitCLI(t, origin, "rev-parse", "refs/heads/trunk")
	localGitCLI(t, root, "commit", "--allow-empty", "-m", "prepared contract")
	request, err := codehost.NewDraftReviewRequest("sha256:" + strings.Repeat("a", 64))
	if err != nil {
		t.Fatal(err)
	}
	prepared, err := proposals.Publish(t.Context(), codehost.ProposalChange{ID: "draft-widget", Feature: "widget", Branch: "refs/heads/draft-widget", Head: localGitCLI(t, root, "rev-parse", "HEAD"), State: "prepared", ExpectedTargetOID: target, ReviewRequest: &request})
	if err != nil {
		t.Fatal(err)
	}
	swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
	return root, origin, proposals, prepared
}

func savedDraftReviewRequest(t *testing.T, generation string) (string, codehost.ReviewBinding) {
	t.Helper()
	var out, diagnostic bytes.Buffer
	if exit := draftVerb([]string{"review-request", "draft-widget", "--generation", generation}, &out, &diagnostic); exit != 0 || !strings.Contains(diagnostic.String(), "unauthenticated") || !strings.Contains(diagnostic.String(), "no host-level protection") {
		t.Fatalf("render request: exit=%d out=%s err=%s", exit, &out, &diagnostic)
	}
	var binding codehost.ReviewBinding
	if err := json.Unmarshal(out.Bytes(), &binding); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "review-request.json")
	if err := os.WriteFile(path, out.Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	return path, binding
}

func TestDraftReviewCLIExactRequestPublicationAndReadOnlyCloneVerification(t *testing.T) {
	localgittest.Skip(t)
	root, origin, _, prepared := draftReviewCLIFixture(t)
	request, binding := savedDraftReviewRequest(t, prepared.OID)
	_, again := savedDraftReviewRequest(t, prepared.OID)
	if again != binding || binding.SchemaVersion != codehost.ReviewBindingSchema || binding.ProposalOID != prepared.OID || binding.TargetRef != "refs/heads/trunk" {
		t.Fatalf("render minted another request or target: %+v %+v", binding, again)
	}
	var out, diagnostic bytes.Buffer
	if exit := draftVerb([]string{"verify-review", "--request", request}, &out, &diagnostic); exit != 1 || !strings.Contains(diagnostic.String(), "receipt is absent") {
		t.Fatalf("missing receipt accepted: exit=%d err=%s", exit, &diagnostic)
	}
	beforeMain := localGitCLI(t, origin, "rev-parse", "refs/heads/trunk")
	args := []string{"record-review", "--request", request, "--reviewer", "operator-label", "--capture", "reviewed the saved request"}
	out.Reset()
	diagnostic.Reset()
	if exit := draftVerb(args, &out, &diagnostic); exit != 0 {
		t.Fatalf("record: exit=%d err=%s", exit, &diagnostic)
	}
	var recorded codehost.ReceiptRecord
	if err := json.Unmarshal(out.Bytes(), &recorded); err != nil || recorded.Receipt.Binding != binding || recorded.Receipt.Reviewer != "operator-label" || recorded.Trust != (codehost.Trust{SharedAccount: true}) {
		t.Fatalf("wrong output: %+v, %v", recorded, err)
	}
	if localGitCLI(t, origin, "rev-parse", "refs/heads/trunk") != beforeMain || localGitCLI(t, origin, "rev-parse", binding.Branch) != binding.Head {
		t.Fatal("receipt advanced mainline or reviewed branch")
	}
	out.Reset()
	if exit := draftVerb(args, &out, &diagnostic); exit != 0 || !strings.Contains(out.String(), recorded.OID) {
		t.Fatalf("retry: exit=%d err=%s", exit, &diagnostic)
	}
	before := localGitCLI(t, origin, "show-ref")
	clone := filepath.Join(t.TempDir(), "clone")
	localGitCLI(t, filepath.Dir(clone), "clone", "--origin", "upstream", "--branch", "trunk", origin, clone)
	swap(t, &draftRepoRoot, func() (string, error) { return clone, nil })
	out.Reset()
	if exit := draftVerb([]string{"verify-review", "--request", request}, &out, &diagnostic); exit != 0 || !strings.Contains(out.String(), recorded.OID) {
		t.Fatalf("clone verify: exit=%d err=%s", exit, &diagnostic)
	}
	if localGitCLI(t, origin, "show-ref") != before || localGitCLI(t, root, "status", "--porcelain") != "" {
		t.Fatal("verification changed reviewed refs or source")
	}
	out.Reset()
	if exit := draftVerb([]string{"record-review", "--request", request, "--reviewer", "another-label"}, &out, &diagnostic); exit != 1 || localGitCLI(t, origin, "show-ref") != before {
		t.Fatalf("overwrite: exit=%d err=%s", exit, &diagnostic)
	}
}

func TestDraftReviewCLIRejectsUnversionedOrUnsupportedSavedRequests(t *testing.T) {
	localgittest.Skip(t)
	_, origin, _, prepared := draftReviewCLIFixture(t)
	path, binding := savedDraftReviewRequest(t, prepared.OID)
	before := localGitCLI(t, origin, "show-ref")
	for _, schema := range []string{"", "local-review-binding/v2", codehost.ReviewRequestSchema} {
		binding.SchemaVersion = schema
		body, err := json.Marshal(binding)
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0600); err != nil {
			t.Fatal(err)
		}
		for _, verb := range []string{"record-review", "verify-review"} {
			args := []string{verb, "--request", path}
			if verb == "record-review" {
				args = append(args, "--reviewer", "operator")
			}
			var out, diagnostic bytes.Buffer
			if exit := draftVerb(args, &out, &diagnostic); exit != 1 || !strings.Contains(diagnostic.String(), "schema") || localGitCLI(t, origin, "show-ref") != before {
				t.Fatalf("schema %q accepted by %s: exit=%d %s", schema, verb, exit, &diagnostic)
			}
		}
	}
}

func TestDraftReviewCLIReservesReceiptPublicationForOperator(t *testing.T) {
	localgittest.Skip(t)
	_, origin, _, prepared := draftReviewCLIFixture(t)
	request, _ := savedDraftReviewRequest(t, prepared.OID)
	args := []string{"record-review", "--request", request, "--reviewer", "operator-label"}
	before := localGitCLI(t, origin, "show-ref")
	t.Setenv("ARGUS_DRIVE_SLUG", "widget")
	var out, diagnostic bytes.Buffer
	if exit := draftVerb(args, &out, &diagnostic); exit != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "operator procedure") || localGitCLI(t, origin, "show-ref") != before {
		t.Fatalf("drive recorded an operator review: exit=%d err=%s", exit, &diagnostic)
	}
	out.Reset()
	diagnostic.Reset()
	if exit := draftVerb([]string{"verify-review", "--request", request}, &out, &diagnostic); exit != 1 || !strings.Contains(diagnostic.String(), "receipt is absent") {
		t.Fatal("drive refusal left a review receipt")
	}
	t.Setenv("ARGUS_DRIVE_SLUG", "")
	out.Reset()
	diagnostic.Reset()
	if exit := draftVerb(args, &out, &diagnostic); exit != 0 {
		t.Fatalf("operator recording refused: exit=%d err=%s", exit, &diagnostic)
	}
	before = localGitCLI(t, origin, "show-ref")
	t.Setenv("ARGUS_DRIVE_SLUG", "widget")
	out.Reset()
	diagnostic.Reset()
	if exit := draftVerb([]string{"verify-review", "--request", request}, &out, &diagnostic); exit != 0 || localGitCLI(t, origin, "show-ref") != before {
		t.Fatalf("drive could not observe the operator review: exit=%d err=%s", exit, &diagnostic)
	}
}

func TestDraftReviewCLIRefusesSavedRequestAfterNewPreparationAtSameHead(t *testing.T) {
	localgittest.Skip(t)
	_, origin, proposals, prepared := draftReviewCLIFixture(t)
	path, binding := savedDraftReviewRequest(t, prepared.OID)
	next, err := codehost.NewDraftReviewRequest(binding.Digest)
	if err != nil {
		t.Fatal(err)
	}
	p := prepared.Proposal
	if _, err := proposals.Publish(t.Context(), codehost.ProposalChange{ID: p.ID, Feature: p.Feature, Branch: p.Branch, Head: p.Head, State: "prepared", ExpectedProposalOID: prepared.OID, ExpectedBranchOID: p.Head, ExpectedTargetOID: p.TargetOID, ReviewRequest: &next}); err != nil {
		t.Fatal(err)
	}
	before := localGitCLI(t, origin, "show-ref")
	var out, diagnostic bytes.Buffer
	if exit := draftVerb([]string{"review-request", p.ID, "--generation", prepared.OID}, &out, &diagnostic); exit != 1 || out.Len() != 0 || !strings.Contains(diagnostic.String(), "displayed proposal generation no longer matches") || localGitCLI(t, origin, "show-ref") != before {
		t.Fatalf("displayed packet silently obtained a newer request: exit=%d out=%s err=%s", exit, &out, &diagnostic)
	}
	out.Reset()
	diagnostic.Reset()
	if exit := draftVerb([]string{"record-review", "--request", path, "--reviewer", "operator"}, &out, &diagnostic); exit != 1 || !strings.Contains(diagnostic.String(), "no longer matches") || localGitCLI(t, origin, "show-ref") != before {
		t.Fatalf("stale saved request: exit=%d err=%s", exit, &diagnostic)
	}
}

func TestDraftReviewCLIRejectsMalformedInputWithoutPublication(t *testing.T) {
	localgittest.Skip(t)
	_, origin, _, _ := draftReviewCLIFixture(t)
	before := localGitCLI(t, origin, "show-ref")
	for _, body := range []string{"{", "{}\n{}", "{\"unknown\":true}", strings.Repeat(" ", (64<<10)+1), "{}"} {
		path := filepath.Join(t.TempDir(), "request.json")
		if err := os.WriteFile(path, []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		var out, diagnostic bytes.Buffer
		if exit := draftVerb([]string{"record-review", "--request", path, "--reviewer", "operator"}, &out, &diagnostic); exit != 1 || localGitCLI(t, origin, "show-ref") != before {
			t.Fatalf("malformed request %q: exit=%d err=%s", body[:min(len(body), 25)], exit, &diagnostic)
		}
	}
	for _, path := range []string{filepath.Join(t.TempDir(), "missing"), t.TempDir()} {
		var out, diagnostic bytes.Buffer
		if exit := draftVerb([]string{"verify-review", "--request", path}, &out, &diagnostic); exit != 1 {
			t.Fatalf("bad request path: exit=%d err=%s", exit, &diagnostic)
		}
	}
	for _, args := range [][]string{{"review-request"}, {"review-request", ""}, {"record-review"}, {"record-review", "--unknown"}, {"record-review", "--request", "file"}, {"verify-review"}, {"verify-review", "--request", "file", "extra"}, {"verify-review", "--request", "file", "--reviewer", "ignored"}} {
		var out, diagnostic bytes.Buffer
		if exit := draftVerb(args, &out, &diagnostic); exit != 2 {
			t.Fatalf("invalid usage %v: exit=%d err=%s", args, exit, &diagnostic)
		}
	}
	for _, args := range [][]string{
		{"review-request", "draft-widget"},
		{"review-request", "draft-widget", "--generation", "short"},
		{"review-request", "draft-widget", "--generation", strings.Repeat("A", 40)},
		{"review-request", "draft-widget", "--other", strings.Repeat("a", 64)},
		{"review-request", "", "--generation", strings.Repeat("a", 40)},
	} {
		var out, diagnostic bytes.Buffer
		if exit := draftVerb(args, &out, &diagnostic); exit != 2 || out.Len() != 0 || localGitCLI(t, origin, "show-ref") != before {
			t.Fatalf("unbound request usage %v: exit=%d err=%s", args, exit, &diagnostic)
		}
	}
}

type draftReviewFailWriter struct{}

func (draftReviewFailWriter) Write([]byte) (int, error) { return 0, errors.New("closed output") }

func TestDraftReviewCLIRequiresExplicitReadyLocalOrigin(t *testing.T) {
	localgittest.Skip(t)
	for _, state := range []string{"no local binding", "hosted binding", "invalid binding", "invalid YAML", "missing mainline"} {
		t.Run(state, func(t *testing.T) {
			root, origin := localGitCLIFixture(t, false)
			var replacement string
			switch state {
			case "no local binding":
				replacement = ""
			case "hosted binding":
				replacement = "integrations:\n  schema_version: pyxis-integrations/v1\n  bindings:\n    code_host:\n      provider: github\n      repository: o/r\n"
			case "invalid binding":
				replacement = "integrations:\n  schema_version: unknown\n"
			case "invalid YAML":
				replacement = "[invalid YAML"
			}
			if state != "missing mainline" {
				if err := os.WriteFile(filepath.Join(root, ".pyxis/config.yaml"), []byte(replacement), 0644); err != nil {
					t.Fatal(err)
				}
			}
			swap(t, &draftRepoRoot, func() (string, error) { return root, nil })
			var out, diagnostic bytes.Buffer
			if exit := draftVerb([]string{"review-request", "draft-widget", "--generation", strings.Repeat("a", 40)}, &out, &diagnostic); exit != 1 || out.Len() != 0 || localGitCLI(t, origin, "for-each-ref", "--format=%(refname)") != "" {
				t.Fatalf("%s: exit=%d out=%s err=%s", state, exit, &out, &diagnostic)
			}
			if state == "missing mainline" && !strings.Contains(diagnostic.String(), "first configured-mainline push") {
				t.Fatalf("missing first-push remedy: %s", &diagnostic)
			}
		})
	}
	root, _, _, prepared := draftReviewCLIFixture(t)
	var diagnostic bytes.Buffer
	if exit := draftVerb([]string{"review-request", "draft-widget", "--generation", prepared.OID}, draftReviewFailWriter{}, &diagnostic); exit != 1 || !strings.Contains(diagnostic.String(), "closed output") {
		t.Fatalf("output failure: exit=%d err=%s", exit, &diagnostic)
	}
	swap(t, &draftRepoRoot, func() (string, error) { return root, errors.New("outside repository") })
	if exit := draftVerb([]string{"review-request", "draft-widget", "--generation", prepared.OID}, &bytes.Buffer{}, &diagnostic); exit != 1 || !strings.Contains(diagnostic.String(), "outside repository") {
		t.Fatalf("missing repository: exit=%d err=%s", exit, &diagnostic)
	}
}
