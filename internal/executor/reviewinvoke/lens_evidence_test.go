package reviewinvoke

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/reviewobligation"
	"github.com/agenticlab-ai/pyxis/internal/executor/worker"
	"github.com/agenticlab-ai/pyxis/internal/implreview"
	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/implreview/persist"
	implreport "github.com/agenticlab-ai/pyxis/internal/implreview/report"
	implruntime "github.com/agenticlab-ai/pyxis/internal/implreview/runtime"
)

const fixtureNativeReviewParent = "native-parent"

func fixtureNativeLenses(state, aggregate string) (*port.NativeReviewChildren, error) {
	data, err := os.ReadFile(filepath.Join(state, "review-evidence", "pack.txt"))
	if err != nil {
		return nil, err
	}
	digest, err := reviewPackDigest(data)
	if err != nil {
		return nil, err
	}
	report, err := persist.ParseHandoff(aggregate)
	if err != nil {
		return nil, err
	}
	evidence := lensEvidence{Schema: lensEvidenceSchema, PackDigest: digest, ReviewerRoot: fixtureNativeReviewParent, ContextID: report.Rounds[len(report.Rounds)-1].ContextID}
	children := &port.NativeReviewChildren{}
	for _, name := range []string{implreport.LensSpecFidelity, implreport.LensDesignConformance, implreport.LensSecurityAbuse, implreport.LensCorrectnessCritic} {
		result := lensResult{Schema: lensResultSchema, Lens: name, PackDigest: digest, Status: "completed", Candidates: []implreport.Finding{}}
		wire, err := json.Marshal(result)
		if err != nil {
			return nil, err
		}
		id := "native-child-" + name
		children.Children = append(children.Children, port.NativeReviewChild{ContextID: id, Completed: true, Output: lensFence + string(wire) + "\n```\n"})
		evidence.Lenses = append(evidence.Lenses, retainedLens{Lens: name, ContextID: id, PackDigest: digest, Status: "completed", Candidates: []implreport.Finding{}})
	}
	path := filepath.Join(state, "review-evidence", "lenses.json")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		wire, err := json.Marshal(evidence)
		if err != nil {
			return nil, err
		}
		if err := os.WriteFile(path, wire, 0o600); err != nil {
			return nil, err
		}
	}
	return children, nil
}

func nativeEvidenceFixture(t *testing.T) (*preparedReviewInputs, string, port.RunResult) {
	t.Helper()
	state := t.TempDir()
	if err := os.Mkdir(filepath.Join(state, "review-evidence"), 0o700); err != nil {
		t.Fatal(err)
	}
	inputs := pack.ReviewInputs{Subject: "github:o/r#548", Proposal: []byte("diff"), CodegateVerdict: []byte("green"), Charter: []byte("charter"), DesignBindings: []byte("design"), Calibration: []byte("calibration")}
	data, err := pack.Build(inputs)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(state, "review-evidence", "pack.txt"), string(data))
	aggregate := artifactFromBrokerBrief("/impl-review github:o/r#548 abc123\nReview process: review-process-test\n" + reviewModelBrief("review-model"))
	children, err := fixtureNativeLenses(state, aggregate)
	if err != nil {
		t.Fatal(err)
	}
	return &preparedReviewInputs{inputs: inputs}, state, port.RunResult{Output: aggregate, SessionID: fixtureNativeReviewParent, ReviewChildren: children}
}

func TestNativeLensEvidenceRequiresNativeParentAndDerivedContext(t *testing.T) {
	for _, kind := range []string{"missing-native-parent", "missing-retained-parent", "blank-native-parent", "forged-round-context"} {
		t.Run(kind, func(t *testing.T) {
			p, state, run := nativeEvidenceFixture(t)
			path := filepath.Join(state, "review-evidence", "lenses.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var evidence lensEvidence
			if err := decodeLensJSON(data, &evidence); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "missing-native-parent":
				run.SessionID, evidence.ReviewerRoot = "", ""
			case "missing-retained-parent":
				evidence.ReviewerRoot = ""
			case "blank-native-parent":
				run.SessionID, evidence.ReviewerRoot = " \t", " \t"
			case "forged-round-context":
				run.SessionID, evidence.ReviewerRoot = fixtureNativeReviewParent, fixtureNativeReviewParent
				evidence.ContextID = "forged-round-context"
				report, err := persist.ParseHandoff(run.Output)
				if err != nil {
					t.Fatal(err)
				}
				report.Rounds[len(report.Rounds)-1].ContextID = evidence.ContextID
				data, err := report.Marshal()
				if err != nil {
					t.Fatal(err)
				}
				run.Output = "```impl-review\n" + string(data) + "\n```\n"
			}
			data, err = json.Marshal(evidence)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(data))
			if err := p.verifyLenses(t.Context(), state, run); err == nil || p.verifiedLenses != nil {
				t.Fatalf("unproven native parent/context was accepted: err=%v proof=%+v", err, p.verifiedLenses)
			}
		})
	}
}

func TestNativeLensEvidenceRejectsWithheldOrForgedCompletion(t *testing.T) {
	for _, kind := range []string{"valid", "audit-digest-mismatch", "unavailable", "failed-child", "missing-child", "missing-native", "duplicate-context", "parent-context", "duplicate-lens", "unknown-lens", "wrong-pack", "forged-retained-status", "dropped-candidate", "missing-candidates", "unknown-native-field", "native-surplus-json", "native-incomplete-block", "native-oversize", "missing-state", "missing-pack", "bad-pack", "missing-file", "bad-file", "wrong-root", "unknown-file-field", "bad-aggregate", "no-round", "wrong-round-context", "cancelled"} {
		t.Run(kind, func(t *testing.T) {
			p, state, run := nativeEvidenceFixture(t)
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			child := &run.ReviewChildren.Children[3]
			switch kind {
			case "audit-digest-mismatch":
				child.Output = `{"lens":"correctness-critic","pack_digest":"sha256:body","candidates":[],"digest_mismatch":"expected body; observed whole-file"}`
			case "unavailable":
				child.Output = strings.Replace(child.Output, `"status":"completed"`, `"status":"unavailable","reason":"digest mismatch"`, 1)
			case "failed-child":
				child.Completed = false
			case "missing-child":
				run.ReviewChildren.Children = run.ReviewChildren.Children[:3]
			case "missing-native":
				run.ReviewChildren = nil
			case "duplicate-context":
				child.ContextID = run.ReviewChildren.Children[0].ContextID
			case "parent-context":
				child.ContextID = run.SessionID
			case "duplicate-lens":
				child.Output = strings.Replace(child.Output, "correctness-critic", "spec-fidelity", 1)
			case "unknown-lens":
				child.Output = strings.Replace(child.Output, "correctness-critic", "unknown-lens", 1)
			case "wrong-pack":
				child.Output = strings.Replace(child.Output, `"pack_digest":"sha256:`, `"pack_digest":"sha256:wrong-`, 1)
			case "dropped-candidate":
				child.Output = strings.Replace(child.Output, `"candidates":[]`, `"candidates":[{"id":"FND-1","message":"must preserve this candidate"}]`, 1)
			case "missing-candidates":
				child.Output = strings.Replace(child.Output, `"candidates":[]`, `"candidates":null`, 1)
			case "unknown-native-field":
				child.Output = strings.Replace(child.Output, `"status":"completed"`, `"status":"completed","digest_mismatch":"withheld"`, 1)
			case "native-surplus-json":
				child.Output = strings.Replace(child.Output, "\n```", " {}\n```", 1)
			case "native-incomplete-block":
				child.Output = strings.TrimSuffix(child.Output, "\n```\n")
			case "native-oversize":
				child.Output = strings.Repeat("x", (8<<20)+1)
			case "missing-state":
				state = filepath.Join(state, "missing")
			case "missing-pack":
				if err := os.Remove(filepath.Join(state, "review-evidence", "pack.txt")); err != nil {
					t.Fatal(err)
				}
			case "bad-pack":
				writeFile(t, filepath.Join(state, "review-evidence", "pack.txt"), "not a canonical pack")
			case "bad-aggregate":
				run.Output = "parent says approved without a report"
			case "no-round":
				report, err := persist.ParseHandoff(run.Output)
				if err != nil {
					t.Fatal(err)
				}
				report.Rounds = nil
				data, err := json.Marshal(report)
				if err != nil {
					t.Fatal(err)
				}
				run.Output = "```impl-review\n" + string(data) + "\n```\n"
			case "wrong-round-context":
				contextID, err := implruntime.ContextID(fixtureNativeReviewParent)
				if err != nil {
					t.Fatal(err)
				}
				run.Output = strings.ReplaceAll(run.Output, contextID, "another-context")
			case "wrong-root":
				run.SessionID = "another-native-parent"
			case "cancelled":
				cancel()
			case "missing-file":
				if err := os.Remove(filepath.Join(state, "review-evidence", "lenses.json")); err != nil {
					t.Fatal(err)
				}
			case "bad-file":
				writeFile(t, filepath.Join(state, "review-evidence", "lenses.json"), "not JSON")
			case "forged-retained-status", "unknown-file-field":
				path := filepath.Join(state, "review-evidence", "lenses.json")
				data, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if kind == "forged-retained-status" {
					data = []byte(strings.Replace(string(data), `"status":"completed"`, `"status":"unavailable"`, 1))
				} else {
					data = []byte(strings.Replace(string(data), `"schema_version":`, `"invented_field":true,"schema_version":`, 1))
				}
				writeFile(t, path, string(data))
			}
			err := p.verifyLenses(ctx, state, run)
			if kind == "valid" && err != nil {
				t.Fatal(err)
			}
			if kind != "valid" && err == nil {
				t.Fatalf("%s became a completed four-lens review", kind)
			}
		})
	}
}

func TestNativeLensEvidenceAcceptsAndCountsRetainedCandidates(t *testing.T) {
	p, state, run := nativeEvidenceFixture(t)
	path := filepath.Join(state, "review-evidence", "lenses.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var evidence lensEvidence
	if err := decodeLensJSON(data, &evidence); err != nil {
		t.Fatal(err)
	}
	evidence.ReviewerRoot, run.SessionID = "native-parent", "native-parent"
	child, err := parseNativeLens(run.ReviewChildren.Children[3].Output)
	if err != nil {
		t.Fatal(err)
	}
	child.Candidates = []implreport.Finding{{ID: "FND-1", Lens: child.Lens, Message: "candidate to screen", CalRow: "CAL-1"}}
	data, err = json.Marshal(child)
	if err != nil {
		t.Fatal(err)
	}
	run.ReviewChildren.Children[3].Output = lensFence + string(data) + "\n```\n"
	evidence.Lenses[3].Candidates = child.Candidates
	data, err = json.Marshal(evidence)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, path, string(data))
	report, err := persist.ParseHandoff(run.Output)
	if err != nil {
		t.Fatal(err)
	}
	report.Rounds[0].Candidates, report.Rounds[0].Killed = 1, 1
	data, err = json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	run.Output = "```impl-review\n" + string(data) + "\n```\n"
	if err := p.verifyLenses(t.Context(), state, run); err != nil {
		t.Fatal(err)
	}
	run.ReviewChildren.Children[3].ContextID = run.SessionID
	if err := p.verifyLenses(t.Context(), state, run); err == nil {
		t.Fatal("parent context became a fresh child")
	}
}

// A harness whose reviewer never sees a child's context reports the name the
// reviewer does see. The retained outcome then cites that name, and the host
// joins it to the child's real context from the harness's own record.
func TestNativeLensEvidenceJoinsRetainedLensesByInvocationName(t *testing.T) {
	for _, kind := range []string{"by-name", "context-wins", "unknown-name", "reused-name", "no-identity"} {
		t.Run(kind, func(t *testing.T) {
			p, state, run := nativeEvidenceFixture(t)
			path := filepath.Join(state, "review-evidence", "lenses.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var evidence lensEvidence
			if err := json.Unmarshal(data, &evidence); err != nil {
				t.Fatal(err)
			}
			for i := range run.ReviewChildren.Children {
				name := "/root/" + strings.ReplaceAll(evidence.Lenses[i].Lens, "-", "_")
				run.ReviewChildren.Children[i].InvocationID = name
				evidence.Lenses[i].ContextID, evidence.Lenses[i].InvocationID = "", name
			}
			wantErr := ""
			switch kind {
			case "context-wins":
				evidence.Lenses[0].ContextID, evidence.Lenses[0].InvocationID = run.ReviewChildren.Children[0].ContextID, "/root/not-a-child"
			case "unknown-name":
				evidence.Lenses[0].InvocationID, wantErr = "/root/not-a-child", "retained outcome differs from its native child"
			case "reused-name":
				run.ReviewChildren.Children[1].InvocationID, wantErr = run.ReviewChildren.Children[0].InvocationID, "reused native child invocation"
			case "no-identity":
				evidence.Lenses[0].InvocationID, wantErr = "", "retained outcome differs from its native child"
			}
			wire, err := json.Marshal(evidence)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, path, string(wire))
			err = p.verifyLenses(t.Context(), state, run)
			if wantErr == "" {
				if err != nil || p.verifiedLenses == nil {
					t.Fatalf("verifyLenses = %v with evidence %+v", err, p.verifiedLenses)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), wantErr) || p.verifiedLenses != nil {
				t.Fatalf("verifyLenses = %v, want %q and no verified evidence", err, wantErr)
			}
		})
	}
}

func TestReviewPackDigestVerifiesBodyRatherThanWholeFile(t *testing.T) {
	_, state, _ := nativeEvidenceFixture(t)
	data, err := os.ReadFile(filepath.Join(state, "review-evidence", "pack.txt"))
	if err != nil {
		t.Fatal(err)
	}
	digest, err := reviewPackDigest(data)
	if err != nil || digest == "" {
		t.Fatal(digest, err)
	}
	for _, bad := range [][]byte{[]byte("no manifest"), []byte(strings.Replace(string(data), "# digest: sha256:", "# digest: wrong-", 1)), []byte(strings.Replace(string(data), "# digest:", "# digest: duplicate\n# digest:", 1))} {
		if _, err := reviewPackDigest(bad); err == nil {
			t.Fatal("noncanonical digest accepted")
		}
	}
}

func TestWithheldNativeLensCannotReachPublication(t *testing.T) {
	for _, local := range []bool{false, true} {
		t.Run(map[bool]string{false: "hosted", true: "local"}[local], func(t *testing.T) {
			world := newLocalWorld(t)
			rt := &fakeRuntime{}
			publisher := &fakeVerifier{}
			i := lifecycleInvoker(t, rt, publisher, time.Minute)
			if local {
				i = localInvoker(t, rt, world.repo)
			} else {
				i.repoDir = world.repo
				i.prepareInputs = func(ctx context.Context, in reviewInputPreparation) (*preparedReviewInputs, error) {
					in.workdir, in.base, in.revision, in.charter = world.worktree, world.base, world.tip, "frozen charter"
					return (&Invoker{repoDir: world.repo}).materializeReviewInputs(ctx, in)
				}
			}
			rt.launchCheck = func(spec port.LaunchSpec) error {
				root := filepath.Join(spec.RuntimeStateDir, "review-evidence", "input-root")
				writeFile(t, filepath.Join(root, "review", "design-bindings.md"), "governing design\n")
				aggregate := artifactFromBrokerBrief(spec.Brief)
				report, err := persist.ParseHandoff(aggregate)
				if err != nil {
					return err
				}
				body, err := pack.BuildFromRoot(t.Context(), root, report.ProposalRef)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(spec.RuntimeStateDir, "review-evidence", "pack.txt"), string(body))
				children, err := fixtureNativeLenses(spec.RuntimeStateDir, aggregate)
				if err != nil {
					return err
				}
				children.Children[3].Output = `{"lens":"correctness-critic","candidates":[],"digest_mismatch":"expected body; observed whole-file"}`
				rt.result.ReviewChildren = children
				rt.result.SessionID = fixtureNativeReviewParent
				return nil
			}
			if local {
				result := i.ReviewLocal(t.Context(), port.Task{ID: "T5", Brief: "frozen charter"}, localWorkdir{world.worktree, world.branch}, 1)
				if result.Err == nil || result.Verdict == "approve" {
					t.Fatalf("withheld local lens accepted: %+v", result)
				}
			} else {
				result := i.Invoke(t.Context(), port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"})
				if result.Err == nil || publisher.publishes != 0 || result.ObligationStatus != "pending" {
					t.Fatalf("withheld lens published: %+v publications=%d", result, publisher.publishes)
				}
			}
		})
	}
}

func TestNativeLensCompletionRequiresProtectedRetention(t *testing.T) {
	p, state, run := nativeEvidenceFixture(t)
	processes := &fakeProcessCoordinator{}
	if err := p.recordLenses(t.Context(), processes, implreview.AdmittedProcess{}); err == nil {
		t.Fatal("unverified panel retained")
	}
	if err := p.verifyLenses(t.Context(), state, run); err != nil {
		t.Fatal(err)
	}
	if err := p.recordLenses(t.Context(), processes, implreview.AdmittedProcess{}); err != nil {
		t.Fatal(err)
	}
	processes.nativeErr = errors.New("protected store unavailable")
	err := p.recordLenses(t.Context(), processes, implreview.AdmittedProcess{})
	if err == nil || lensEvidenceFailureClass(err) != worker.ReviewFailureInfrastructure {
		t.Fatal("retention failure became completed review", err)
	}
	withoutNative := struct{ implreview.ProcessCoordinator }{processes}
	if err := p.recordLenses(t.Context(), withoutNative, implreview.AdmittedProcess{}); err == nil {
		t.Fatal("unavailable protected writer accepted")
	}
}

func TestLegacyReceiptCannotBecomeVerifiedReviewWithoutNativeLensProof(t *testing.T) {
	rt := &fakeRuntime{}
	publisher := &fakeVerifier{recoveryState: implreview.PublicationRecoveryReceiptRecorded}
	i := lifecycleInvoker(t, rt, publisher, time.Minute)
	i.processes = &fakeProcessCoordinator{nativeErr: errors.New("legacy process has no native lens proof")}
	task, proposal := port.Task{ID: "T5"}, port.Proposal{Number: 548, Branch: "feat/x"}
	captureForInvoke(i, task, proposal)
	lease, err := i.store.Begin(t.Context(), reviewobligation.Attempt{
		SchemaVersion: reviewobligation.AttemptSchema, ObligationID: "review-obligation-test",
		AttemptID: "legacy-process", RunID: "old-run", AttemptProofRef: "proof-original",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := lease.Close(); err != nil {
		t.Fatal(err)
	}
	publisher.recovery = &implreview.PublicationTerminal{ReviewProcessID: "legacy-process", ReviewObligationID: "review-obligation-test",
		ProposalID: "github:o/r#548", ReviewedRevision: "abc123", Status: implreview.PublicationVerified, AttemptProofRef: "proof-original"}
	result := i.Invoke(t.Context(), task, proposal)
	if result.Err == nil || result.Status == "verified" || result.FailureClass != worker.ReviewFailureArtifactUntrusted || publisher.recoverCalls != 0 || publisher.publishes != 0 || rt.launches != 0 {
		t.Fatalf("legacy parent claim authenticated: %+v mutations=%d publishes=%d launches=%d", result, publisher.recoverCalls, publisher.publishes, rt.launches)
	}
}
