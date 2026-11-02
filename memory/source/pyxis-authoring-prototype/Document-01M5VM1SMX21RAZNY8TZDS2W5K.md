# prototype — build a requested idea before its contract

## Draft-born features

Run this guide only when the idea's provider asks for a prototype, invokes
`/prototype` on an open feature draft, or (for an Argus drive) a current
authority entry explicitly directs the run. It runs before the first PRD
content is authored. A draft without a prototype record gets no prototype
step; do not start one because the idea merely seems uncertain or technically
interesting.

1. **Confirm the draft and capture the idea.** Use the open feature draft and
   its configured `<draft-root>`. If none exists, do the read-only overlap
   check from `/feature-brainstorm`; for a distinct idea, choose its slug and
   run `go run ./cmd/pyxis draft open <feature>` from a clean worktree. Stop
   before stage 1 authors any brainstorm or PRD content. For an Argus-driven
   idea, use the provider's words in the problem ledger and include the
   applicable authority entry when it carries the request or its constraints;
   otherwise use the provider's words. Write those words and the smallest
   end-to-end behavior to exercise, plus what to leave out, to a local scratch
   file such as `.pyxis/scratch/prototype-<feature>-idea.md`. Keep the exercise narrow; the
   prototype is a disposable probe, not a second implementation.

2. **Launch the separate run.** Pass the asking session's own harness and model
   as the ambient values, even when the repository may select its configured
   route:

   ```sh
   go run ./cmd/pyxis draft prototype <feature> \
     --idea <idea-file> \
     --ambient-backend <this session's harness> \
     --ambient-model <this session's model>
   ```

   Wait for the command. It launches a separate headless process in the
   prototype worktree. The asking session does not build the prototype.

3. **Read and check the result.** Read every report line and the findings file
   it names in the prototype worktree. Check each reported finding against the
   worktree and its command or output evidence. Record the reported route,
   prototype branch, base commit, and head commit. Do not copy prototype files
   into the draft.

4. **Write the draft record.** Instantiate
   `.pyxis/templates/prototype-record-template.md` as the next unused
   `<draft-root>/<feature>/brainstorm/NNN.md` file on the existing draft
   branch. Use `prototype-record/v1`; preserve the record's `L-n` anchors
   across prototype records and never reuse one. Transfer the verified
   findings, including what was tried and observed. If the run completed but
   could not reach a working state, record the obstacle as a finding and set
   `Outcome` to `did not run`. If it learned nothing, write `No findings:` and
   explain why. Commit and push the record with the draft working files.

   A command that exits non-zero or reports no findings file is a failed run,
   not an empty findings result. Do not invent findings or a record for that
   run; report its cause and stop this guide. The user can decide whether to
   ask for another run.

5. **Leave the prototype local.** Keep the prototype worktree available for
   the draft's authoring stages, then report the route, branch, head commit,
   and record path. The session that ends the draft releases the prototype as
   described below.

## Never

- Push the prototype branch or open a pull request from it.
- Build the prototype in the asking session or continue building after the
  separate run ends.
- Copy prototype code into the feature draft or an implementation branch.
- Treat prototype behavior as a product ruling or settle a product question
  without the responsible principal.
- Start a prototype unless the provider explicitly asked for one.
- Continue after a failed run or missing findings file; report its cause and
  stop.

## Release

The session that ends the draft on this machine removes the local prototype.
After verifying that the feature lock landed on mainline, run
`go run ./cmd/pyxis draft prototype <feature> --remove`. When abandoning the
draft, run `go run ./cmd/pyxis draft abandon <feature>` and then remove the
prototype immediately. Argus journals the removal. If removal refuses because
the prototype worktree is dirty, commit or discard those prototype changes
and retry; never publish them as part of the feature.
