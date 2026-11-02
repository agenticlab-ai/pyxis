# Security policy

Pyxis runs AI agents with access to code and credentials, so we take reports
seriously.

## Report a vulnerability

Please don't open a public issue. Use GitHub's
[private vulnerability reporting](https://github.com/agenticlab-ai/pyxis/security/advisories/new)
and include what you found, how to reproduce it, and the impact you expect.

Reports are handled by the maintainers on a best-effort basis. We aim to
acknowledge a report within a week and will keep you updated until it is resolved.

## Supported versions

Pyxis is pre-1.0. Fixes go into the latest release only.

## Most relevant areas

- Escaping the agent process boundary (Bubblewrap on Linux, `sandbox-exec` on macOS).
- Installing or running an artifact that doesn't match its pinned checksum.
- Bypassing a merge gate, or the recorded authority of the AI engineer.
- Leaking a credential into logs, commits, or pull requests.
