package reviewinvoke

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/agenticlab-ai/pyxis/internal/implreview/pack"
	"github.com/agenticlab-ai/pyxis/internal/runlogs"
)

func reviewDebugBrief(state, assembler string) string {
	inputRoot := filepath.Join(state, "review-evidence", "input-root")
	build := fmt.Sprintf("\n\nThe trusted Go host has already materialized review/proposal.diff, review/codegate.txt, review/charter.md and review/calibration.md under %q. Read those exact files; do not search other workspaces, transcripts or old review logs for inputs. The one remaining input is review/design-bindings.md: use native Write/Edit to write the relevant binding elements from the governing design identified by the frozen charter. If that design is missing or ambiguous, report the missing review input instead of reconstructing it from code or earlier reviews. Then build the shared pack with this exact trusted host executable: %q --internal-review-pack --root %q --subject <proposal-ref> --out %q. The assembler and fixed five-source mapping are compiled into trusted host Go code and work without Pyxis source, PATH lookup or the operand's ignored binary cache. Do not compile or execute proposal code to construct review inputs. Do not reimplement the assembler or fabricate its manifest; if the supplied assembler is unavailable, report review absence.\n", inputRoot, assembler, inputRoot, filepath.Join(state, "review-evidence", "pack.txt"))
	build += fmt.Sprintf("\nDerive the round context_id with the existing Go provenance helper: %q --internal-review-context --reviewer-root <your-native-reviewer-session-id>. Supply your actual fresh native root identity; this command computes the identifier and does not prove freshness or create a session. No custom hashing helper is needed.\n", assembler)
	build += fmt.Sprintf("\nBefore assembling the final machine block, read the exact compiled report schema with this same trusted executable: %q --internal-review-schema. Use its field names and hierarchy exactly: schema_version is the envelope key, context_id belongs directly to each round, and provenance contains model, prompts_version and cal_version. Do not infer the wire format from Source-only Go paths, invent aliases or put diagnostic lens/screen data into the report. The schema describes serialization only; obtain all review facts and identities from the actual native run. Existing semantic validation, independent-review proof and exact host bindings still apply. If the helper is unavailable, report review absence instead of guessing a payload.\n", assembler)
	build += "\nThe four files are staging copies, not authorities: the host retains their original bytes outside your runtime. After you finish, it independently rebuilds the canonical pack from those retained bytes, your design-bindings.md and the host's proposal identity. A missing or altered pack is rejected before any report is accepted or published. Write only the remaining design bindings; do not change the four host-prepared sources.\n"
	build += "\nThe Go helper rule applies throughout input materialization, diagnostics, screening and report assembly. Use native Write/Edit tools for data files. Do not create or execute Python, JavaScript or shell helpers, including inline helpers or interpreter availability probes. If a heredoc is blocked, use Write/Edit or a Go helper in this writable private runtime.\n"
	build += "\nAuthoritative input path/kind/order mapping (host data):\n\n```yaml\n" + pack.DeclarationText() + "```\n"
	build += "\nThe pack's # digest hashes only the bytes after the first blank line separating the manifest from the body. Never compare that digest to a hash of the complete pack file. Supply this exact body-digest rule to each lens with its own opaque instruction. Each child must emit one final ```impl-review-lens machine block with closed JSON fields schema_version=impl-review-lens-result/v1, lens, pack_digest, status=completed|failed|unavailable, reason (required for failed/unavailable, absent for completed), and candidates (an array of full finding payloads, including an observed empty array). Preserve a digest mismatch or inability to judge as failed/unavailable; never interpret it as a completed empty finding set. Do not emit a block in intermediate commentary. In lenses.json use schema_version=impl-review-lens-evidence/v1 with shared_pack_digest, context_id (the aggregate round context), reviewer_root (native parent session ID), optional pack_path, and exactly four lenses. Each lens has lens, native_context_id (the actual returned child identity), optional native_task_invocation_id, shared_pack_digest, status, optional reason, and its full candidates. The host cross-checks those values against native Task results. If any lens is missing, unavailable, failed, or disagrees about the pack, report review absence and do not emit an approving aggregate.\n"
	build += "\nExecute the native subagent tools; never substitute authored lens results for actual child responses. Retain what each spawn actually returned and invent nothing: a native context ID where the tool returns one, as native_context_id; otherwise the task name it returned, as native_task_invocation_id, with native_context_id left empty. For Codex, call spawn_agent once per lens with fork_turns set to none, so the child starts without your history, and task_name set to the lens name with underscores (spec_fidelity, design_conformance, security_abuse, correctness_critic); then call wait_agent until list_agents reports all four completed. Codex returns a task name such as /root/spec_fidelity and no thread ID: that is expected, and that name is what you retain. The host does not take your word for the children. It reads each child's own identity, its parent, and its final response from the harness's records of this run, and requires all four to have completed under your session. Check this capability before doing the panel's work: if the first spawn returns neither a context ID nor a task name, stop immediately and report review absence. Do not run the rest of the panel or construct aggregate evidence from that unsupported interface.\n"
	return build + fmt.Sprintf("\n\nRetain review execution evidence in %q (this private runtime path is writable). Before any lens runs, write pack.txt containing the one complete shared pack, then use four genuinely fresh native Task/subagent contexts, named exactly spec-fidelity, design-conformance, security-abuse, and correctness-critic. Each child receives only that identical pack and its own opaque lens instruction; never inline all four lenses in your own session or give a child sibling findings. In lenses.json record each lens's native child/context identity, shared-pack digest, full candidates, and native task invocation identity when available. In screen.json retain every killed candidate with its evidence-based rejection reason, and the survivors. Keep these files separate from the final impl-review handoff; the host retains them before deleting your disposable runtime. If fresh subagents are unavailable, report the review absence rather than claiming four fresh lens runs.", filepath.Join(state, "review-evidence")) +
		"\n\nFinish all tool and lens work before delivering the handoff; wait for any background command to finish before your final response. Keep the machine block out of intermediate commentary. Emit the complete artifact only in your final response, with both Markdown fences on their own lines and a newline after the closing fence. End the response there: do not follow it with an acknowledgement, a completion summary, or a statement that the handoff was already emitted. A native CLI may coalesce later assistant text without a separator even within one message; text glued to the closing fence makes the machine block unterminated and the host rejects it.\n"
}

// retainReviewDebug copies only named review diagnostics after the runtime has
// quiesced. Authentication databases and the rest of the disposable HOME are
// never imported. OpenRoot refuses links escaping the owned runtime directory.
func (i *Invoker) retainReviewDebug(ctx context.Context, state, process string) error {
	root, err := os.OpenRoot(state)
	if err != nil {
		return fmt.Errorf("open review runtime evidence: %w", err)
	}
	defer func() {
		if err := root.Close(); err != nil {
			i.log.Warn("close review evidence root", "error", err)
		}
	}()
	for _, name := range []string{"pack.txt", "lenses.json", "screen.json"} {
		source, err := root.Open(filepath.Join("review-evidence", name))
		if os.IsNotExist(err) {
			continue
		} // old/native-unavailable runs retain their native transcript only
		if err != nil {
			return fmt.Errorf("open review debug %s: %w", name, err)
		}
		info, statErr := source.Stat()
		if statErr != nil || !info.Mode().IsRegular() || info.Size() > 8<<20 {
			return errors.Join(fmt.Errorf("review debug %s must be a regular file of at most 8 MiB: %v", name, statErr), source.Close())
		}
		dest, err := runlogs.Create(ctx, i.repoDir, runlogs.Reviews, process+"-"+name+"-*")
		if err != nil {
			return errors.Join(err, source.Close())
		}
		_, copyErr := io.Copy(dest, io.LimitReader(source, 8<<20))
		sourceErr, closeErr := source.Close(), dest.Close()
		if err := errors.Join(copyErr, sourceErr, closeErr); err != nil {
			return fmt.Errorf("retain review debug %s: %w", name, err)
		}
		i.log.Info("impl-review debug evidence retained", "process", process, "kind", name, "path", dest.Name())
	}
	return nil
}
