{
  "_comment": "Argus write-hook adapter for Claude Code — a POINTER, not a content copy. It wires Claude Code's own PreToolUse hook (Write|Edit|MultiEdit) to the canonical deny decision (cmd/argus-writehook), which holds the deny logic and the protected-path knowledge once (DINV-5, AP-25). This file contains NO protected-path knowledge. Merge this fragment into .claude/settings.json to enable the write-time layer; the argus-writehook binary must be on PATH. Fail-closed: Claude's PreToolUse default is fail-open, so the shim maps every internal error to a deny — this adapter IS the explicit fail-closed setting.",
  "hooks": {
    "PreToolUse": [
      {
        "matcher": "Write|Edit|MultiEdit",
        "hooks": [
          {
            "type": "command",
            "command": "argus-writehook --cli=claude",
            "timeout": 5
          }
        ]
      }
    ]
  }
}
