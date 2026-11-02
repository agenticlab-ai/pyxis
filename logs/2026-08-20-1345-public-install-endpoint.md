# Session log — a public one-liner install endpoint (pyxis.agenticlab.ai/install)

Started: 2026-08-20 13:45 · CLI: Claude Code · Stage or skill: chore (distribution channel)

## P-1 — what the public endpoint should serve, and who stands it up

- **Found:** 2026-08-20 13:52
- **Problem:** The ask was to replace handing people an install script with a
  single line they can run. Two things were genuinely undecided. First, what
  the public address should return: a small wrapper that asks the private
  channel which release it currently pins, or the real pinned installer
  served openly — which would put the version number and every checksum on
  the public internet. Second, the address does not exist yet: there is no
  name record for it, and the program that guards the private channel is not
  kept in this repository, so nothing here could create or change it.
- **Blocked:** Either answer to the first question produces a different
  script, and guessing wrong would either leak release details or build the
  wrong thing. The second is simply outside what this session can reach — the
  hosting account is not available from here.
- **Asked:** 2026-08-20 13:55 — put both to the operator, with the leak
  spelled out and a recommendation for the wrapper.
- **Decided:** 2026-08-20 13:58 — the operator chose the wrapper, and took
  the hosting and name-record work themselves, saying to assume the file will
  be published at the address and the name set up accordingly.
- **How:** Rewrote the operator script as the bytes that address serves —
  public, carrying no version or checksum of its own, safe to pipe into a
  shell — and covered it with an offline test that drives it against a fake
  channel. Left the hosting untouched.
- **Refs:** `scripts/install_pyxis.sh` · `internal/release/frontdoor_test.go`
  · `README.md` · `docs/techspec.md` · `docs/prds/installer/` FR-2, FR-17,
  ABUSE-4 · `https://pyxis.agenticlab.ai/install`
