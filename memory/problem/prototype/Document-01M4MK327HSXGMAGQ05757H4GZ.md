# Prototype an idea before its contract — problem ledger

| Field | Value |
|---|---|
| **Slug** | prototype |
| **Principal** | Operator (@mdarabi) |
| **Authority-contract version** | authority-contract/v1, tier-2 at intake |
| **Envelope override** | — |
| **Template version** | problem-ledger/v1 |

## Idea

> Rhonda test version 1 which I guess just drives and I will review and merge just to make sure that there is no Gap at least for first accept and raise the pr and we can start by simple idea of having the concept of prototype meaning that when we give an idea we can also prototype it first the prototype idea works like this the comments so the system will go to pixies will go implement it in a work tree and then try to understand what are the challenges technical challenges where the product conflicts that we need to think through so it gives more robust input for the real implementation so the main idea behind work in pixie would be let's learn by doing and then you apply that learning when you're going out through the future draft and the actual implementation so let's drive the idea of having prototype using Argus now

## Authority entries

### A-1 — The operator reviews and merges; Argus only drives

- **Date**: 2026-10-10
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: —
- **Ruling**: "test version 1 which … just drives and I will review and merge just to make sure that there is no Gap at least for first". This drive is the first end-to-end test of an Argus drive. Argus drives every stage; the operator reviews and merges each pull request. Argus performs no approval and no merge on this problem.

### A-2 — A Pyxis command controls the harness and model that build a prototype

- **Date**: 2026-10-10
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: —
- **Ruling**: Given in review of the prepared lock on pull request 1547: "agree with your analysis on not using the implement-now; but i still want to control the harness/model execution of the prototyping. so it should be a cmd in the pyxis ecosystem that based on the config value (if it exists) decides on the harness and the model; if there is no configured value, then it falls back to whather the ambient (current running harness and model)." Prototypes do not run through the implement-now flow.

### A-3 — Always a headless run; feature drafts only for now

- **Date**: 2026-10-10
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: —
- **Ruling**: The operator described the flow: "there's a skill that will trigger the prototype flow which is the headless harness plus the model selection plus the system prompt that explains what the ask is for and then it waits for it to be done and then it stores their learning and then brings back brings back that learning to the feature draft or anything else that requires". Asked to choose on two points, the operator agreed with both of Argus's recommendations ("agree with both oof your answers. update the pr for the final approving"): with nothing configured the command still launches a headless run, on the current session's harness and model; and a prototype exists only inside a feature draft in this first version.

