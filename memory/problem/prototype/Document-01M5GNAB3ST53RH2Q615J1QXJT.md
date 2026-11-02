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

### A-4 — Keep the same reviewer and fix it

- **Date**: 2026-10-11
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: delivery:prototype#E-31
- **Ruling**: "let's keep the same reviewer and fix it." Asked afterwards whether to move the lenses out of the harness into four separate launches, the operator ruled: "let's stick with the existing approach. fix the pid issue" and "raise a pr". The configured review backend stays; the reviewer's subagent evidence path is repaired in the engine. After the repair was merged: "let's pull the latest man and rerun the reviews".

### A-5 — Raise the review time bound to thirty minutes

- **Date**: 2026-10-11
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: —
- **Ruling**: "okay increase the limit to 30 minutes". Given after two reviews of task T1 finished their four lenses and were cut off at the default fifteen-minute bound while screening their candidates. The bound is the operator's machine setting; it is raised on the machine that runs this drive, and no other review setting changes.


### A-6 — Reopen the contract as version 2; Argus cancels the open tasks

- **Date**: 2026-10-15
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: delivery:prototype#E-36
- **Ruling**: The operator chose option (a) of brief E-36, "Reopen contract v2". In version 2, design decision D-1 gains two sentences. The command supports the ordinary layout, with the Git directory inside the main checkout, and refuses any other by name. It relies on Git's own commands, locks and refusals, with no guarantee when another process changes the prototype's branch or directory while it runs. T1 then runs again from a fresh branch. A reopen is refused while any task of the current plan is unfinished, so plan 001's open tasks T1, T3, T4 and T5 are cancelled and planned again from version 2. Asked who cancels them, the operator chose "Argus closes them": Argus closes the four issues as not planned, each with a comment citing the brief and this answer.

### A-7 — Argus closes plan 001's epic

- **Date**: 2026-10-16
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: delivery:prototype#E-45
- **Ruling**: Asked whether Argus should close plan 001's epic, issue 1551, as not planned, with a pointer to plan 002's epic, issue 1578, the operator answered "yes". Argus closes issue 1551 as not planned with a comment naming epic 1578 and this answer.

### A-8 — Accept the layout D-1 cannot detect; one pull request carries the change

- **Date**: 2026-10-21
- **Author**: Operator (@mdarabi)
- **Authority-contract version**: authority-contract/v1
- **Answers**: delivery:prototype#E-51
- **Ruling**: Told that the layout of brief E-51 needs a separate Git directory named `.git` inside another directory and a run from a linked worktree, which Argus judged very unlikely, the operator ruled on 2026-10-16: "if it is unlikely, we do not need to solve for it". D-1 accepts that layout as ordinary and does not detect it, as option (a) of the brief proposed. Told what option (a)'s route costs, the operator rejected it: "this is too much of a machineray to just handle one pr conflict. just think about it: 3 prs for fixiing one conflict. this is stupid". On 2026-10-19 the operator set the requirement for any such change: "No matter if we have to update the contract or the plan or whatever, there should be only one PR that the operator has to sign off", with changes made in place because Git keeps the history. The operator then had that flow built outside this drive and directed: "we will use the T1 issue as a real test for the feature after it is successfully implemented". After it merged, on 2026-10-21: "start that". Version 3 changes D-1, plan 002's T1 charter changes in place, and one pull request carries both with these ledgers. Issues 1579 to 1582 and epic 1578 stay open, and no plan 003 is drafted.
