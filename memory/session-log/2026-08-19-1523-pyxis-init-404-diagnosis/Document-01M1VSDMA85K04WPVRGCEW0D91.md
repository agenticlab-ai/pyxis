# Session log — diagnosing a `pyxis init` artifact-source 404 on v0.1.46

Started: 2026-08-19 15:23 · CLI: Claude Code · Stage or skill: ad-hoc diagnosis

Nothing escalated this session.

**Correction — 2026-08-19 17:07:** the line above was written at the session's
first step, before the problem below came up. It is wrong. It stays where it is
because this log is append-only: one problem was escalated this session, and it
is recorded as P-1.

## P-1 — a consumer could not install because the default artifact source is a repo they cannot read

- **Found:** 2026-08-19 15:23
- **Problem:** A person outside the organisation installed the CLI and then
  could not set up a repository with it. The tool reaches for its own
  artifacts on the private source repository by default, and nobody outside
  the three paid seats can read that repository. The distribution mirror and
  the download door both exist and both carry the same files, but nothing in
  the install path tells the tool to use either of them.
- **Blocked:** Two ways out, and only the operator could pick. Granting the
  person access to the mirror means buying another paid seat, and another for
  every future consumer. Switching the default to the download door costs no
  seats but changes a decision that a merged requirements document and a
  merged design both record.
- **Asked:** 2026-08-19 15:40 — whether to buy the seat, or make the download
  door the default channel for everyone.
- **Decided:** 2026-08-19 15:52 — make the download door the default. Keep the
  GitHub channel working as an override; no consumer should have to pass any
  option anywhere, beyond putting the door credential in their environment.
- **How:** Opened the next typed amendment on the installer requirements
  ledger to carry the decision, since the current wording names the code host
  as the artifact host. The implementation follows the amendment.
- **Refs:** docs/prds/installer/003-default-distribution-channel.md ·
  docs/prds/installer/effective.md §5.5, §6.2 · docs/tech-design/installer-v1.md §3.1 ·
  internal/resolve/resolve.go:168 · .pyxis/scripts/pyxis-bootstrap.sh:50 ·
  install.sh:32 · https://dist.pyxis.agenticlab.ai
