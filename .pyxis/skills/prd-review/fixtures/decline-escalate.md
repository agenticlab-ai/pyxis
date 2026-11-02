# Fixture: decline-escalate — a reasoned decline of a blocking finding

## Subject

`subjects/queue-widget-000.md`.

## Scripted author

Applies three blocking findings; **declines the incident-class finding**
with the reason "the cache is invalidated synchronously on mutation — the
stale-read class cannot occur"; leaves the advisory open.

## Expected report

- The decline is recorded verbatim per-finding (an unreasoned decline
  would be invalid — FR-8).
- The verdict flips to `escalate`; the **embedded brief** names the single
  conflict (the finding + the decline reason), the options with risks, and
  the one decision needed — citing `FND-n`.
- The subject is parked: the review is complete, nothing retries, no
  tracker item is minted by the reviewer (the surrounding flow's job).
