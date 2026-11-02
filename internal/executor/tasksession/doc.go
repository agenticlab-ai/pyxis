// Package tasksession is task session continuity: the machine-local ledger
// that remembers which harness conversation a task's launches continue, so the
// second and every later round of the SAME task — a local-gate fix, a review
// remediation, a remote-CI heal, a respawn, a human-review feedback round —
// resumes the context the model already built instead of rebuilding it from
// a brief.
//
// Shape and scope:
//
//   - One `task-session/v1` record per (repository, task), latest wins, in an
//     append-only JSONL file under the repository's git common dir
//     (`<common>/pyxis/task-sessions.jsonl`), guarded by a file lock. Harness
//     sessions are files the CLI keeps on THIS machine, so the ledger is
//     honestly machine-scoped: it never claims a session another machine
//     could resume.
//   - A session is per task and per harness. It is never resumed across tasks
//     (the charter-bleed guard), never after the harness changes, never for a
//     reviewer (always a fresh, blind sibling), and never into a worktree
//     other than the one it was built in — a recreated worktree (the branch was
//     discarded) starts a fresh session, which is also how a task whose
//     proposal landed and was reaped ends: its record goes unused and is
//     superseded the next time the task is dispatched. Retire is the explicit
//     end for a driver that observes landing or abandonment.
//   - The same record carries the human-feedback cursor: the newest proposal
//     comment a feedback round has serviced, so a comment wakes exactly one
//     round and human rounds are bounded by human actions, not by a counter.
//
// The Worker, the drive-to-green fixer, and the Dispatcher's feedback detector
// consume this through the consumer-side interfaces they each declare
// (worker.SessionContinuity, dispatch.FeedbackLedger); Continuity implements
// both over a Store.
package tasksession
