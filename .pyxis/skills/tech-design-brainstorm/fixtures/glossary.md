# Fixture: glossary — the authority overloads a term the glossary owns

## Basis + design questions

Basis: `corpus:KNOW-2` (corpus discovery & routing). Effort: "a generated
routing index for the doc corpus" — DQ-1: what does each row of the index
carry?

*(Seeded collision: the authority names the artifact a manifest, a word
`docs/wiki/glossary.md` already gives to three different things — the
release manifest, the verification manifest, and GOV-1's primitive
manifest — and rules out bare in all three entries.)*

## Scripted authority

Persona: quick to name things; takes the first word that fits.

Answer bank:

- on what each row carries → "one manifest — every doc, its Relevant-when,
  and its owning area." *(overloads a term the glossary already owns)*
- on the manifest clash with the glossary → "routing manifest, then — and
  never the bare word."
- default → adopt the recommendation.

## Expected record

- **Transcript, the turn that answers the collision** (FR-16): the session
  challenges the overloaded word *in the turn that answers the colliding
  reply* — naming what the word already means and offering a resolution. A
  challenge raised later, after further design questions have gone by, is
  the negative.
- **Candidate glossary terms**: one complete `T-n` carrying all three parts
  — the agreed term, what it means, and the synonyms ruled out (`· avoid:
  bare *manifest* …`). A `T-n` holding only the word records no resolution.
- `docs/wiki/glossary.md` is **byte-identical** afterwards: the session
  proposes vocabulary as a candidate, it never edits the shared glossary.
  This is FR-15's write perimeter — "modifying no other document" — reaching
  the one document a term collision most tempts a session to reach for.
- Gate verdict: any — this fixture asserts on the challenge and the
  candidate entry, not on the direction taken.
