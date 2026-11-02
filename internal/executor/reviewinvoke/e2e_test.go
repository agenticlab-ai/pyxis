package reviewinvoke

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/adapters/harness/codex"
	"github.com/agenticlab-ai/pyxis/internal/executiondemand"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview/calibration"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	githubpersist "github.com/agenticlab-ai/pyxis/internal/implreview/persist/github"
	"github.com/agenticlab-ai/pyxis/internal/implreview/process"
	"github.com/agenticlab-ai/pyxis/internal/implreview/publication"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

// TestEndToEnd_LiveProposalReview drives one real green proposal through the
// production Invoker and a fresh Codex process, then reads the newly-created
// GitHub artifact back through the production persistence adapter. It is gated
// because it makes a model call and one proposal-medium write. The proposal and
// exact reviewed revision are mandatory inputs so an accidental invocation can
// never guess a write target or certify moving bytes. The concrete launch is
// resolved from the adapter's production role=review catalog, and production
// preflight authenticates both Codex and the code-host writer before launch.
func TestEndToEnd_LiveProposalReview(t *testing.T) {
	if os.Getenv("PYXIS_E2E_IMPL_REVIEW") == "" {
		t.Skip("set PYXIS_E2E_IMPL_REVIEW=1 plus proposal and revision inputs to run the live review")
	}
	rawProposalRef := strings.TrimSpace(os.Getenv("PYXIS_E2E_PROPOSAL_REF"))
	revision := strings.TrimSpace(os.Getenv("PYXIS_E2E_REVIEWED_REVISION"))
	if rawProposalRef == "" || revision == "" {
		t.Fatal("PYXIS_E2E_PROPOSAL_REF and PYXIS_E2E_REVIEWED_REVISION are required")
	}
	repo, proposal := splitProposalRef(t, rawProposalRef)
	proposalRef, err := persist.GitHubProposalRef(repo, proposal)
	if err != nil {
		t.Fatalf("canonical proposal ref: %v", err)
	}

	repoDir, err := filepath.Abs("../../..")
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}
	if _, err := os.Stat(filepath.Join(repoDir, ".pyxis", "skills", "impl-review", "SKILL.md")); err != nil {
		t.Fatalf("resolved workdir %q is not the pyxis source root: %v", repoDir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	before := artifactCommentIDs(t, ctx, repo, proposal)

	codexRuntime, err := codex.New(codex.WithProfiles(fixtureHarnessConfig(t).Profiles("codex")))
	if err != nil {
		t.Fatalf("build Codex runtime: %v", err)
	}
	if err := codexRuntime.AuthCheck(ctx); err != nil {
		t.Skipf("Codex runtime not ready: %v", err)
	}
	obligationStore, err := reviewobligation.NewGitStore(repoDir, "origin", reviewobligation.DefaultEvidenceRef)
	if err != nil {
		t.Fatalf("build review obligation store: %v", err)
	}
	processStore, err := process.NewGitStore(repoDir, "origin", process.DefaultEvidenceRef)
	if err != nil {
		t.Fatalf("build review process store: %v", err)
	}
	publicationStore, err := publication.NewGitStore(repoDir, "origin", publication.DefaultEvidenceRef)
	if err != nil {
		t.Fatalf("build publication store: %v", err)
	}
	table, err := calibration.LoadFile(filepath.Join(repoDir, ".pyxis", "skills", "impl-review", "calibration.md"))
	if err != nil {
		t.Fatalf("load review calibration: %v", err)
	}
	coordinator, err := process.New(processStore, table)
	if err != nil {
		t.Fatalf("build process coordinator: %v", err)
	}
	host, err := githubpersist.NewHostAdapter(repo, publicationStore)
	if err != nil {
		t.Fatalf("build host persistence: %v", err)
	}
	terminalVerifier, err := NewStoreTerminalVerifier(obligationStore, publicationStore)
	if err != nil {
		t.Fatalf("build terminal verifier: %v", err)
	}
	inv, err := New(Dependencies{Runtime: codexRuntime, Harness: "codex", RepositoryDir: repoDir, RepositorySlug: repo, Remote: "origin", Budget: 25 * time.Minute,
		Processes: coordinator, Persistence: host, TerminalVerifier: terminalVerifier, Log: slog.New(slog.NewTextHandler(os.Stderr, nil))})
	if err != nil {
		t.Fatalf("build review invoker: %v", err)
	}
	inv.WithObligationStore(obligationStore).WithProposalInspector(e2eProposalInspector{revision: revision})
	if err := inv.Preflight(ctx); err != nil {
		t.Fatalf("preflight live reviewer and artifact broker: %v", err)
	}
	task := port.Task{ID: "E2E-IMPL-REVIEW", DemandID: executiondemand.DemandID(stableDigest("e2e-impl-review-demand")), DemandProfile: executiondemand.ProfileMinimum{Capability: executiondemand.CapabilityFrontier, Deliberation: executiondemand.DeliberationMaximum, Context: executiondemand.ContextExtended, Assurance: executiondemand.AssuranceCritical}}
	proposalValue := port.Proposal{Number: proposal, Branch: "e2e-reviewed-revision"}
	capture := worker.ReviewCapture{
		ProposalRef: proposalRef, Revision: revision,
		ObligationID: stableID("review-obligation", proposalRef+"\x00"+revision), Harness: "codex",
	}
	if err := inv.store.Append(ctx, reviewobligation.Obligation{SchemaVersion: reviewobligation.Schema, ID: capture.ObligationID, Repository: repo, TaskID: task.ID, ProposalRef: proposalRef, Revision: revision, Harness: "codex", DemandID: task.DemandID, DemandProfile: task.DemandProfile}); err != nil {
		t.Fatalf("persist live review obligation: %v", err)
	}
	if err := inv.store.CompleteHandoff(ctx, capture.ObligationID); err != nil {
		t.Fatalf("persist live implementation-terminal handoff: %v", err)
	}
	inv.captures[captureKey(task, proposalValue)] = capture
	result := inv.Invoke(ctx, task, proposalValue)
	if result.Err != nil {
		t.Fatalf("live review invoke: %v", result.Err)
	}

	after := artifactCommentIDs(t, ctx, repo, proposal)
	var created []string
	for id := range after {
		if !before[id] {
			created = append(created, id)
		}
	}
	if len(created) != 1 {
		t.Fatalf("new impl-review artifact comments = %v, want exactly one (before=%v after=%v)", created, before, after)
	}

	payload, err := host.ExactRead(ctx, result.NativeReceipt)
	if err != nil {
		t.Fatalf("read persisted review artifact %s: %v", result.NativeReceipt, err)
	}
	if payload.SchemaVersion != implreport.SchemaVersion || payload.ProposalRef != proposalRef {
		t.Fatalf("persisted payload identity = schema %q proposal %q, want %q / %q", payload.SchemaVersion, payload.ProposalRef, implreport.SchemaVersion, proposalRef)
	}
	if len(payload.Rounds) == 0 || payload.Rounds[len(payload.Rounds)-1].ReviewedRevision != revision {
		t.Fatalf("persisted payload rounds = %+v, want latest revision %s", payload.Rounds, revision)
	}
	if model := payload.Rounds[len(payload.Rounds)-1].Provenance.Model; model != result.Model {
		t.Fatalf("persisted current-round model = %q, want host-resolved native model %q", model, result.Model)
	}
	t.Logf("native reviewer profile=%s model=%s effort=%s reported_run_tokens=%d", result.NativeProfileID, result.Model, result.Effort, result.Tokens)
	t.Logf("persisted and read back impl-review artifact github:%s for %s@%s", created[0], proposalRef, revision)
}

type e2eProposalInspector struct{ revision string }

func (i e2eProposalInspector) InspectProposal(context.Context, port.Proposal) (port.Landing, string, port.CheckState, error) {
	return port.LandingOpen, i.revision, port.CheckGreen, nil
}

func splitProposalRef(t *testing.T, ref string) (string, int) {
	t.Helper()
	ref = strings.TrimPrefix(ref, "github:")
	mark := strings.LastIndexByte(ref, '#')
	if mark <= 0 || mark == len(ref)-1 {
		t.Fatalf("proposal ref %q must be owner/repo#number", ref)
	}
	n, err := strconv.Atoi(ref[mark+1:])
	if err != nil || n <= 0 {
		t.Fatalf("proposal ref %q has an invalid number", ref)
	}
	return ref[:mark], n
}

func artifactCommentIDs(t *testing.T, ctx context.Context, repo string, proposal int) map[string]bool {
	t.Helper()
	endpoint := fmt.Sprintf("repos/%s/issues/%d/comments", repo, proposal)
	query := ".[] | select(.body | contains(\"```impl-review\\n\")) | .id"
	cmd := exec.CommandContext(ctx, "gh", "api", endpoint, "--paginate", "--jq", query)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("list impl-review artifacts on %s#%d: %v: %s", repo, proposal, err, strings.TrimSpace(string(out)))
	}
	ids := make(map[string]bool)
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if id := strings.TrimSpace(line); id != "" {
			ids[id] = true
		}
	}
	return ids
}
