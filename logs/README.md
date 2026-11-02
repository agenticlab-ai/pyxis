# Historical session evidence

Session logs are no longer required. A session with nothing durable to add
creates no documentation. New operational notes belong under ignored
`.pyxis/logs/sessions/`; they are not imported into memory or checked for
memory coverage.

Existing entries with decisions or other substantive context remain as
historical evidence. Preserve their original bytes and references. Their
old instructions do not govern new sessions. Record new durable decisions,
constraints, and lessons once in the requirement, design, issue, pull request,
incident, or typed memory record they belong to, following AGENTS.md.

The session-log cleanup removes only mechanically identified empty entries
and exact, unreferenced empty memory imports. Other imported log Documents
remain available through explicit history access (memory search --all,
memory view, or memory render), but are excluded from ordinary search,
collision suggestions, and context packs.
