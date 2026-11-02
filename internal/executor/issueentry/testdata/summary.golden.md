Authored explanation

<!-- pyxis-task-proposal: preserved -->

<!-- pyxis:issue-execution-summary/v1 -->
## Pyxis execution

Status: review_pending. Execution: 123 ms (command start to terminal; excludes intake and final metadata write).

Contract: sha256:contract

| Role | Harness | Model | Effort | Tokens | Tool calls |
|---|---|---|---|---|---|
| implementation | codex | actual-model | high | 0 | unavailable |
| review | claude | actual-review | deep | unavailable | unavailable |

Usage scope: current_invocation. Only retained native launch observations are counted; missing role data is unavailable. Duration covers the latest command invocation. Tool counts and intake usage are unavailable unless observed by an adapter.

```json
{
  "schema_version": "issue-execution-summary/v1",
  "contract_id": "sha256:contract",
  "source": "carina:opaque#17",
  "proposal_number": 77,
  "revision": "revision",
  "terminal": "review_pending",
  "execution_elapsed_ms": 123,
  "timing_boundary": "command_start_to_terminal_excludes_final_summary_write_and_intake",
  "usage_scope": "current_invocation",
  "attempts": [
    {
      "attempt_id": "run-a/invocation-1",
      "role": "implementation",
      "harness": "codex",
      "model": "actual-model",
      "effort": "high",
      "status": "completed",
      "tokens": {
        "state": "observed",
        "value": 0
      },
      "turns": {
        "state": "observed",
        "value": 0
      },
      "tool_calls": {
        "state": "unavailable",
        "reason": "adapter does not report tool counts"
      }
    },
    {
      "attempt_id": "run-a/invocation-2",
      "role": "review",
      "harness": "claude",
      "model": "actual-review",
      "effort": "deep",
      "status": "failed",
      "tokens": {
        "state": "unavailable",
        "reason": "adapter did not report token usage"
      },
      "turns": {
        "state": "unavailable",
        "reason": "adapter did not report turn count"
      },
      "tool_calls": {
        "state": "unavailable",
        "reason": "adapter does not report tool counts"
      }
    }
  ],
  "total_tokens": {
    "state": "partial",
    "value": 0,
    "reason": "some invocation usage is unavailable"
  },
  "intake_tokens": {
    "state": "unavailable",
    "reason": "invoking session is not a bounded executor process"
  },
  "intake_elapsed_ms": {
    "state": "unavailable",
    "reason": "intake occurs before command entry"
  }
}
```
<!-- /pyxis:issue-execution-summary/v1 -->
