# plangate's Carina tracker adapter — solution shape

| Field | Value |
|---|---|
| **Design** | plangate-carina-adapter |
| **Status** | superseded-by-code |

*Concrete forms for the design's decisions — authoritative until the code exists, then `superseded-by-code`. Names/flags shown are the intended shape; final spellings fix at implementation. Each element is captioned with the `DINV-n`/`A-n` it realizes.*

## Selector at the projection boundary

`cmd/plangate/main.go` picks the adapter from `--tracker` instead of hardcoding `tracker.GH`; everything upstream of the `Tracker` interface is untouched (**DINV-1**, **A-3**):

```go
// cmd/plangate/main.go — the ONLY change outside the new adapter package.
var tr tracker.Tracker
switch trackerFlag { // --tracker gh|carina (default "gh")
case "carina":
    tr = tracker.NewCarina(carinaRoot) // --carina-root <dir> (default the repo work-items root)
default:
    tr = &tracker.GH{Repo: repo}       // unchanged
}
// … the rest of `project` — decomposition, fold, create→spec→link→wire→route — is identical.
```

## The Carina adapter implements the same `Tracker` interface

One struct drives the `carina` CLI behind the existing interface — behaviorally identical to `gh` (additive, idempotent, never mutates a pre-existing item), so it passes the same contract test (**DINV-2**, **A-1**):

```go
// internal/plangate/tracker/carina.go
type Carina struct {
    Root string        // carina --root <dir>
    bin  string        // resolved carina binary (Q1); "" → `go run ./cmd/carina` in dev/CI
}
func NewCarina(root string) *Carina { /* resolve bin, pre-flight (R1) */ }

// run shells one carina verb with --json and decodes the typed result (AP-6).
func (c *Carina) run(args ...string) (carinaJSON, error) {
    // exec: <bin> <verb> --root <c.Root> --json <args...>; non-zero → typed Defect naming verb+stderr
}
```

## Write half — item identity is Carina's own id

`CreateIssue` returns the **Carina id** (not a GitHub number); links, edges, and comments address items by it (**DINV-4**, **A-5**), and `issueType` maps to the Carina `--type`:

```go
func (c *Carina) CreateIssue(title, body string, labels []string, issueType string) (int, error) {
    j, err := c.run("create", "--type", carinaType(issueType), "--title", title, "--body", body)
    // carinaType: "epic"→"epic", "task"→"task", ""→"task" (untyped fallback, legibility-additive)
    id := j.ID
    c.applyLabels(id, labels) // § mapping
    return id, err            // this int IS the Carina id — DINV-4
}
func (c *Carina) LinkSubIssue(parent, child int) error { _, e := c.run("parent", "set", itoa(child), itoa(parent)); return e }
func (c *Carina) AddBlockedBy(issue, blocker int) error { _, e := c.run("dep", "add", itoa(issue), itoa(blocker)); return e }
func (c *Carina) Comment(issue int, body string) error  { _, e := c.run("comment", itoa(issue), "--body", body); return e }
```

## Label → signal / type mapping

The operator-ruled mapping: known labels → Carina signals, `epic` elided (the type carries it), and **any unmapped label auto-defined as a classification signal** — so `EnsureLabel`/`AddLabels` never fail on an unknown label (**DINV-3**, **A-4**):

```go
// labelMap — engine-recognized targets; everything else → classification axis.
var labelMap = map[string]string{
    "local-agent": "agent",     // authorization analog
    "operator":    "operator",  // routing (exists)
    // "kind:*" passes through unchanged; "epic" is ELIDED (the --type carries the hierarchy role)
}
func (c *Carina) EnsureLabel(name, color, description string) error {
    if name == "epic" { return nil }                 // type carries it (DINV-3)
    sig := labelMap[name]
    if sig == "" && !strings.HasPrefix(name, "kind:") {
        // unmapped → define on the operand-extensible classification axis, idempotently
        _, _ = c.run("signal", "def", name, "classification", "--color", color, "--description", description)
    }
    return nil // engine-recognized signals already exist in the seed vocab → no-op
}
func (c *Carina) applyLabels(id int, labels []string) {
    for _, l := range labels {
        if l == "epic" { continue }
        sig := l; if m, ok := labelMap[l]; ok { sig = m }
        _, _ = c.run("signal", "add", itoa(id), sig)  // idempotent (AP-22)
    }
}
```

## Read half — idempotency by plan-marker probe

A re-run finds an already-projected item by its `pyxis:plan-*` marker via `carina search` (exact substring over title/body/comments), so projection converges with zero duplicates and zero mutation (**DINV-5**, **A-2**):

```go
func (c *Carina) OpenIssues() ([]tracker.Issue, error) {
    j, err := c.run("list", "--state", "open")   // header-shaped; bodies via search/show
    return j.Issues(), err
}
// The convergent probe (in project's create step) locates a task by its marker:
//   found := c.run("search", "pyxis:plan-task local-work-tracker/001/T3")  // exact marker → 0/1 hit
func (c *Carina) SubIssues(parent int) ([]tracker.Issue, error) {
    // interim: parse `show <parent>` rollup; switch to `list --parent <parent>` when FR-19 ships (Q2)
    j, err := c.run("show", itoa(parent))
    return j.Children(), err
}
func (c *Carina) BlockedBy(issue int) ([]int, error) { j, e := c.run("show", itoa(issue)); return j.BlockedBy(), e }
func (c *Carina) Comments(issue int) ([]string, error) { j, e := c.run("show", itoa(issue)); return j.Comments(), e }
```

## CLI surface

Selecting Carina is one flag; the projection is otherwise identical to the GitHub path (**A-2**):

```
plangate project --tracker carina [--carina-root <dir>] docs/plans/<feature>
                                  # create → spec → link → wire → route into a local Carina store
plangate project --dry-run --tracker carina docs/plans/<feature>
                                  # the same write-set review, no writes
```
