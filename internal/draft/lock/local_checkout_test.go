package lock

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/adapters/codehost/localgit/localgittest"
	draftshell "github.com/agenticlab-ai/pyxis/internal/adapters/draft/shell"
	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestLocalPreparationPreservesCheckoutChangesBeforeAlignment(t *testing.T) {
	localgittest.Skip(t)
	for _, change := range []string{"working file", "new commit", "different branch", "head unreadable", "branch unreadable", "status unreadable"} {
		t.Run(change, func(t *testing.T) {
			root, cfg, options, local := localPreparationFixture(t, true)
			prdPath := filepath.Join(root, "drafts/widget/prd.md")
			wantPRD, err := os.ReadFile(prdPath)
			if err != nil {
				t.Fatal(err)
			}
			changed := false
			wantHead, wantBranch := "", ""
			local.Memory = func(context.Context, string, string) error {
				changed = true
				switch change {
				case "working file":
					wantPRD = append(wantPRD, []byte("\nConcurrent source edit.\n")...)
					fixtureFile(t, root, "drafts/widget/prd.md", wantPRD)
				case "new commit":
					gitFixture(t, root, "commit", "--allow-empty", "-m", "concurrent source commit")
				case "different branch":
					gitFixture(t, root, "switch", "-c", "concurrent-work")
				}
				wantHead = gitFixture(t, root, "rev-parse", "HEAD")
				wantBranch = gitFixture(t, root, "branch", "--show-current")
				return nil
			}
			reset := false
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if dir == root && changed && name == "git" {
					if len(args) >= 2 && args[0] == "reset" && args[1] == "--hard" {
						reset = true
					}
					if (change == "head unreadable" && slices.Equal(args, []string{"rev-parse", "HEAD"})) || (change == "branch unreadable" && slices.Equal(args, []string{"branch", "--show-current"})) || (change == "status unreadable" && slices.Equal(args, []string{"status", "--porcelain"})) {
						return nil, errors.New("checkout cannot be read")
					}
				}
				return draftshell.RunLocalGit(ctx, dir, env, name, args...)
			}
			out, err := PrepareLocal(t.Context(), root, cfg, "widget", run, local)
			if err == nil || !strings.Contains(err.Error(), "checkout changed") || reset || out.Proposal == nil {
				t.Fatalf("concurrent work was not preserved: %+v %v", out, err)
			}
			if gitFixture(t, root, "rev-parse", "HEAD") != wantHead || gitFixture(t, root, "branch", "--show-current") != wantBranch {
				t.Fatal("failed alignment moved the local checkout")
			}
			gotPRD, err := os.ReadFile(prdPath)
			if err != nil || string(gotPRD) != string(wantPRD) {
				t.Fatal("failed alignment overwrote source bytes")
			}
			if gitFixture(t, options.Origin, "rev-parse", out.Proposal.Proposal.Branch) != out.Proposal.Proposal.Head {
				t.Fatal("prepared generation was not retained for recovery")
			}
		})
	}
}

func TestLocalResumePreservesSourceCheckoutWhenOriginChangesDuringInspection(t *testing.T) {
	localgittest.Skip(t)
	for _, change := range []string{"proposal round", "mainline"} {
		t.Run(change, func(t *testing.T) {
			root, cfg, options, local := localPreparationFixture(t, true)
			first, err := PrepareLocal(t.Context(), root, cfg, "widget", refuseLocalCommand([]string{"reset"}, ""), local)
			if err == nil || first.Proposal == nil {
				t.Fatal("fixture did not retain preparation before its failed reset")
			}
			wantHead, wantBranch := gitFixture(t, root, "rev-parse", "HEAD"), gitFixture(t, root, "branch", "--show-current")
			wantPRD, err := os.ReadFile(filepath.Join(root, "drafts/widget/prd.md"))
			if err != nil {
				t.Fatal(err)
			}
			changed, reset := false, false
			wantRefs := ""
			run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
				if name == "git" && len(args) > 0 && args[0] == "reset" {
					reset = true
				}
				if !changed && name == "git" && len(args) > 0 && args[0] == "ls-remote" && slices.Contains(args, "--tags") {
					changed = true
					g, p := *first.Proposal, first.Proposal.Proposal
					if change == "proposal round" {
						_, err := local.Proposals.Publish(ctx, codehost.ProposalChange{ID: p.ID, Feature: p.Feature, Branch: p.Branch, Head: p.Head, State: "open", ExpectedProposalOID: g.OID, ExpectedBranchOID: p.Head, ExpectedTargetOID: p.TargetOID})
						if err != nil {
							t.Fatal(err)
						}
					} else {
						next := gitFixture(t, root, "commit-tree", p.TargetOID+"^{tree}", "-p", p.TargetOID, "-m", "concurrent mainline advance")
						gitFixture(t, root, "push", cfg.Remote, next+":"+p.TargetRef)
					}
					wantRefs = localLandingRefs(t, options.Origin)
				}
				return draftshell.RunLocalGit(ctx, dir, env, name, args...)
			}
			if _, err := PrepareLocal(t.Context(), root, cfg, "widget", run, local); !errors.Is(err, codehost.ErrProposalConflict) || !changed || reset {
				t.Fatalf("changed origin reached checkout alignment: changed=%v reset=%v err=%v", changed, reset, err)
			}
			gotPRD, err := os.ReadFile(filepath.Join(root, "drafts/widget/prd.md"))
			if err != nil || string(gotPRD) != string(wantPRD) || gitFixture(t, root, "rev-parse", "HEAD") != wantHead || gitFixture(t, root, "branch", "--show-current") != wantBranch || localLandingRefs(t, options.Origin) != wantRefs {
				t.Fatal("refused recovery changed source bytes, checkout, or origin refs")
			}
		})
	}
}

func TestLocalFirstPreparationRechecksOriginAroundCheckoutAlignment(t *testing.T) {
	localgittest.Skip(t)
	for _, moment := range []string{"before reset", "after reset", "dirty after reset", "restore unavailable"} {
		for _, change := range []string{"proposal round", "mainline"} {
			t.Run(moment+"/"+change, func(t *testing.T) {
				root, cfg, options, local := localPreparationFixture(t, true)
				oldHead := gitFixture(t, root, "rev-parse", "HEAD")
				oldBranch := gitFixture(t, root, "branch", "--show-current")
				prdPath := filepath.Join(root, "drafts/widget/prd.md")
				oldPRD, err := os.ReadFile(prdPath)
				if err != nil {
					t.Fatal(err)
				}
				compiled, changed, reset := false, false, false
				wantRefs := ""
				cause := errors.New("rollback reset unavailable")
				local.Memory = func(context.Context, string, string) error { compiled = true; return nil }
				advance := func(ctx context.Context) {
					changed = true
					g, err := local.Proposals.Read(ctx, "draft-widget-v1")
					if err != nil || g.Proposal.State != "prepared" {
						t.Fatalf("race missed prepared publication: %+v %v", g, err)
					}
					p := g.Proposal
					if change == "proposal round" {
						_, err = local.Proposals.Publish(ctx, codehost.ProposalChange{ID: p.ID, Feature: p.Feature, Branch: p.Branch, Head: p.Head, State: "open", ExpectedProposalOID: g.OID, ExpectedBranchOID: p.Head, ExpectedTargetOID: p.TargetOID})
						if err != nil {
							t.Fatal(err)
						}
					} else {
						next := gitFixture(t, root, "commit-tree", p.TargetOID+"^{tree}", "-p", p.TargetOID, "-m", "origin moved after preparation")
						gitFixture(t, root, "push", cfg.Remote, next+":"+p.TargetRef)
					}
					wantRefs = localLandingRefs(t, options.Origin)
					if moment == "dirty after reset" {
						fixtureFile(t, root, "concurrent.txt", []byte("preserve this edit\n"))
					}
				}
				run := func(ctx context.Context, dir string, env []string, name string, args ...string) ([]byte, error) {
					align := dir == root && name == "git" && len(args) == 3 && args[0] == "reset" && args[1] == "--hard"
					if align && changed && moment == "restore unavailable" && args[2] == oldHead {
						return nil, cause
					}
					output, err := draftshell.RunLocalGit(ctx, dir, env, name, args...)
					if align {
						reset = true
					}
					if err == nil && compiled && !changed && dir == root && name == "git" && (moment == "before reset" && slices.Equal(args, []string{"status", "--porcelain"}) || moment != "before reset" && align) {
						advance(ctx)
					}
					return output, err
				}
				out, err := PrepareLocal(t.Context(), root, cfg, "widget", run, local)
				if !errors.Is(err, codehost.ErrProposalConflict) || !changed || out.Proposal == nil || moment == "before reset" && reset {
					t.Fatalf("stale preparation was accepted: %+v changed=%v reset=%v err=%v", out, changed, reset, err)
				}
				if localLandingRefs(t, options.Origin) != wantRefs || gitFixture(t, root, "branch", "--show-current") != oldBranch {
					t.Fatal("refusal replaced concurrent origin work or switched checkout")
				}
				switch moment {
				case "dirty after reset":
					got, readErr := os.ReadFile(filepath.Join(root, "concurrent.txt"))
					if readErr != nil || string(got) != "preserve this edit\n" || gitFixture(t, root, "rev-parse", "HEAD") != out.Head || !strings.Contains(err.Error(), "checkout changed") {
						t.Fatal("rollback overwrote concurrent local edits")
					}
				case "restore unavailable":
					if !errors.Is(err, cause) || gitFixture(t, root, "rev-parse", "HEAD") != out.Head {
						t.Fatal("failed source restoration was hidden")
					}
				default:
					got, readErr := os.ReadFile(prdPath)
					if readErr != nil || string(got) != string(oldPRD) || gitFixture(t, root, "rev-parse", "HEAD") != oldHead {
						t.Fatal("origin conflict removed the original draft checkout")
					}
				}
			})
		}
	}
}
