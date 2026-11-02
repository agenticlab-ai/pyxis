// carina — the standalone work-item CLI over the localfs WorkTracker adapter
// (docs/tech-design/local-work-tracker.md §3.7, §3.9). Thin main: verb
// dispatch, human text or --json machine output, defects on stderr naming
// file/op/reason/remedy, exit codes 0 (clean) / 1 (defects) / 2 (usage). The
// binary holds no logic the port does not — it is argument parsing over one
// core, the same core the engine consumes through the port.
//
// Create and show landed first; T2 adds the lifecycle verbs edit, close, and
// reopen; T3 adds the append-log verbs comment and record; T4 adds signal
// {add,rm,def,ls}. Later tasks add the rest of the surface (list, ready…) the
// same way — dispatch cases over the same one core.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/localfs"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

func main() { os.Exit(run(os.Args[1:], os.Stdout, os.Stderr)) }

func usage(w io.Writer) int {
	if _, err := fmt.Fprintln(w, `usage: carina <verb> [flags]
verbs:
  create   create a work item (--type --title --body --priority), or create
           a task with typed planning requirements (--native-task <JSON file>)
  show     print a work item by id (positional <id>); an item with children
           reports its children rollup, grouped by open and closed
  edit     change a work item's title and/or body (positional <id> --title --body)
  close    close a work item (positional <id> --reason completed|rejected --author)
  reopen   reopen a closed work item (positional <id> --author)
  comment  append a comment to a work item's log (positional <id> --body --author)
  record   append a namespaced decision-record (positional <id> <namespace>
             --body <payload> --author); latest-wins per namespace, an
             identical repeat is a no-op
  signal   carry control state via the governed vocabulary
             signal add <id> <name>   put a defined signal on an item
             signal rm  <id> <name>   take a signal off an item
             signal def <name> <role> extend the classification axis (--color --description)
             signal ls                list the signal vocabulary
  parent   set or clear a work item's parent
             parent set <child-id> <parent-id>
             parent clear <child-id>
  dep      add or remove a blocked-by dependency edge
             dep add <id> <ref>   <ref> is an item id or the cross-repo
                                  form owner/repo#id (stored opaquely)
             dep rm  <id> <ref>
  list     list work items, filtered and stably ordered by creation
             (--state open|closed --signal <name> --role <role>
              --parent <id> — only that item's direct children, all states)
  search   find items whose title, body, or comments contain the text exactly
             (positional <text>)
  ready    print the ready set — open, authorized, unblocked, non-rollup
           leaves with no closing disposition, in creation order
  triage   print the triage (undecided) set — open, undecided, unblocked,
           unassigned non-rollup items, in creation order
  reindex  rebuild the reverse-parent index from the item files — the sole
           drift recovery; safe to run anytime (derived, non-authoritative)
flags:
  --root <dir>   operand root holding .pyxis/config.yaml (default ".")
  --json         emit schema-versioned machine output (and JSON-line defects)
  create: --type feature|milestone|epic|task (default task)
          --title <text> (required)
          --body <text>
          --priority none|urgent|high|medium|low
  edit:   --title <text> and/or --body <text> (only named fields change)
  close:  --reason completed|rejected (default completed)  --author <name>
  reopen: --author <name>
  comment: --body <text> (required)  --author <name>
  record: <namespace> triage-disposition|charter|foundation
          --body <payload> (required)  --author <name>
  signal def: --color <text> --description <text>
  list:   --state open|closed  --signal <name>  --role routing|authorization|status|classification  --parent <id>`); err != nil {
		return 1
	}
	return 2
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		return usage(stderr)
	}
	verb := args[0]
	fs := flag.NewFlagSet(verb, flag.ContinueOnError)
	fs.SetOutput(stderr)
	root := fs.String("root", ".", "operand root holding .pyxis/config.yaml")
	asJSON := fs.Bool("json", false, "emit schema-versioned machine output")
	typ := fs.String("type", "", "create: hierarchy type (default task)")
	title := fs.String("title", "", "create: title (required)")
	body := fs.String("body", "", "create/edit: body markdown")
	nativeTask := fs.String("native-task", "", "create: closed native-task-input/v1 JSON file with charter and typed task requirements")
	priority := fs.String("priority", "", "create: advisory priority")
	reason := fs.String("reason", "", "close: close reason (completed|rejected)")
	author := fs.String("author", "carina", "close/reopen: who is transitioning the item")
	color := fs.String("color", "", "signal def: presentation color")
	description := fs.String("description", "", "signal def: presentation description")
	state := fs.String("state", "", "list: filter by state (open|closed)")
	signal := fs.String("signal", "", "list: filter by signal name")
	role := fs.String("role", "", "list: filter by signal role")
	parent := fs.String("parent", "", "list: only the direct children of this item id (all states by default)")
	positional, ok := parseInterspersed(fs, args[1:])
	if !ok {
		return 2
	}
	// Which flags the caller explicitly set — the only way to tell an edit that
	// clears a field (--title "") from one that leaves it untouched.
	set := map[string]bool{}
	fs.Visit(func(f *flag.Flag) { set[f.Name] = true })
	if set["native-task"] && (*nativeTask == "" || verb != "create" || set["title"] || set["body"] || set["type"]) {
		if _, err := fmt.Fprintln(stderr, "--native-task is exclusive to create and cannot be combined with --title, --body, or --type"); err != nil {
			return 1
		}
		return 2
	}

	store, cfgDefects := openStore(*root)
	if len(cfgDefects) > 0 {
		return emitDefects(stderr, *asJSON, cfgDefects)
	}
	ctx := context.Background()

	switch verb {
	case "create":
		createSpec := model.CreateSpec{
			Type:     model.Type(*typ),
			Title:    *title,
			Body:     *body,
			Priority: model.Priority(*priority),
		}
		if *nativeTask != "" {
			var err error
			createSpec, err = nativeCreateSpec(*nativeTask)
			if err != nil {
				if _, err := fmt.Fprintln(stderr, err); err != nil {
					return 1
				}
				return 1
			}
			createSpec.Priority = model.Priority(*priority)
		}
		w, defects := store.Create(ctx, createSpec)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitItem(stdout, *asJSON, w, fmt.Sprintf("Created #%d", w.ID))
	case "show":
		id, derr := positiveID(verb, first(positional))
		if derr != nil {
			return emitDefects(stderr, *asJSON, []schema.Defect{*derr})
		}
		w, defects := store.Get(ctx, id)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		if *asJSON {
			return emitItem(stdout, true, w, "")
		}
		if code := emitItem(stdout, false, w, ""); code != 0 {
			return code
		}
		if code := emitItemLog(stdout, w); code != 0 {
			return code
		}
		return emitRollup(ctx, stdout, stderr, store, id)
	case "edit":
		id, derr := positiveID(verb, first(positional))
		if derr != nil {
			return emitDefects(stderr, *asJSON, []schema.Defect{*derr})
		}
		patch := model.EditPatch{}
		if set["title"] {
			patch.Title = title
		}
		if set["body"] {
			patch.Body = body
		}
		w, defects := store.Edit(ctx, id, patch)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitItem(stdout, *asJSON, w, "")
	case "close":
		id, derr := positiveID(verb, first(positional))
		if derr != nil {
			return emitDefects(stderr, *asJSON, []schema.Defect{*derr})
		}
		cr := model.CloseCompleted
		if *reason != "" {
			cr = model.CloseReason(*reason)
		}
		w, defects := store.Transition(ctx, id, model.StateClosed, cr, *author)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitItem(stdout, *asJSON, w, "")
	case "reopen":
		id, derr := positiveID(verb, first(positional))
		if derr != nil {
			return emitDefects(stderr, *asJSON, []schema.Defect{*derr})
		}
		w, defects := store.Transition(ctx, id, model.StateOpen, model.CloseReasonNone, *author)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitItem(stdout, *asJSON, w, "")
	case "comment":
		id, derr := positiveID(verb, first(positional))
		if derr != nil {
			return emitDefects(stderr, *asJSON, []schema.Defect{*derr})
		}
		w, defects := store.Comment(ctx, id, *author, *body)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitItem(stdout, *asJSON, w, fmt.Sprintf("Commented on #%d", id))
	case "record":
		id, derr := positiveID(verb, first(positional))
		if derr != nil {
			return emitDefects(stderr, *asJSON, []schema.Defect{*derr})
		}
		ns := first(positional[min(1, len(positional)):])
		if ns == "" {
			return emitDefects(stderr, *asJSON, []schema.Defect{{Verb: verb, Op: "usage", Reason: "no namespace given", Remedy: "carina record <id> <namespace> --body <payload>"}})
		}
		w, defects := store.Record(ctx, id, model.DecisionRecord{Namespace: model.Namespace(ns), Author: *author, Payload: *body})
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitItem(stdout, *asJSON, w, fmt.Sprintf("Recorded %s on #%d", ns, id))
	case "signal":
		return runSignal(ctx, store, stdout, stderr, *asJSON, positional, *color, *description)
	case "parent":
		return runParent(ctx, stdout, stderr, store, *asJSON, positional)
	case "dep":
		return runDep(ctx, stdout, stderr, store, *asJSON, positional)
	case "list":
		q := model.Query{State: model.State(*state), Signal: *signal, Role: model.Role(*role)}
		if *parent != "" {
			pid, derr := positiveID(verb, *parent)
			if derr != nil {
				return emitDefects(stderr, *asJSON, []schema.Defect{*derr})
			}
			q.Parent = &pid
		}
		items, defects := store.List(ctx, q)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitList(stdout, *asJSON, q, items)
	case "search":
		text := strings.Join(positional, " ")
		items, defects := store.Search(ctx, text)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitSearch(stdout, *asJSON, text, items)
	case "reindex":
		report, defects := store.Reindex(ctx)
		if len(defects) > 0 {
			return emitDefects(stderr, *asJSON, defects)
		}
		return emitReindex(stdout, *asJSON, report)
	case "ready", "triage":
		return runPredicate(ctx, stdout, stderr, store, *asJSON, verb)
	default:
		return usage(stderr)
	}
}

// runPredicate emits a dispatch set — ready or triage — over the same core the
// engine reads through the port (FR-12, FR-13, FR-16). Human output lists each
// member `#id  title`; --json emits the schema-versioned set, byte-identical to
// the port's own (DINV-6). The read is strong: no index sits on this decision
// path, so the set reflects every prior write (DINV-4).
func runPredicate(ctx context.Context, stdout, stderr io.Writer, store worktracker.WorkTracker, asJSON bool, which string) int {
	var (
		items   []model.WorkItem
		defects []schema.Defect
	)
	if which == "ready" {
		items, defects = store.ReadySet(ctx)
	} else {
		items, defects = store.TriageSet(ctx)
	}
	if len(defects) > 0 {
		return emitDefects(stderr, asJSON, defects)
	}
	return emitSet(stdout, asJSON, which, items)
}

// emitSet prints a predicate set as human lines or, in JSON mode, its schema-
// versioned machine projection (model.Set) — the exact value the port returns,
// so the CLI and port set output are byte-identical (DINV-6, FR-16).
func emitSet(w io.Writer, asJSON bool, which string, items []model.WorkItem) int {
	if asJSON {
		b, err := json.Marshal(model.Set(which, items))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return 1
		}
		return 0
	}
	if _, err := fmt.Fprintf(w, "%s set: %d\n", which, len(items)); err != nil {
		return 1
	}
	for _, it := range items {
		if _, err := fmt.Fprintf(w, "#%d  %s\n", it.ID, it.Title); err != nil {
			return 1
		}
	}
	return 0
}

// runDep dispatches the dep add/rm subcommands over the store. The reference is
// an item id or the opaque cross-repo owner/repo#id form; the store classifies
// it, refuses a malformed or dangling reference and a cycle-closing edge, and
// maintains both directions of the accepted edge.
func runDep(ctx context.Context, stdout, stderr io.Writer, store worktracker.WorkTracker, asJSON bool, positional []string) int {
	sub := first(positional)
	switch sub {
	case "add", "rm":
		if len(positional) != 3 {
			return usage(stderr)
		}
		id, derr := positiveID("dep", positional[1])
		if derr != nil {
			return emitDefects(stderr, asJSON, []schema.Defect{*derr})
		}
		ref := positional[2]
		var (
			w       model.WorkItem
			defects []schema.Defect
		)
		if sub == "add" {
			w, defects = store.AddBlockedBy(ctx, id, ref)
		} else {
			w, defects = store.RemoveBlockedBy(ctx, id, ref)
		}
		if len(defects) > 0 {
			return emitDefects(stderr, asJSON, defects)
		}
		verbed := "edge added"
		if sub == "rm" {
			verbed = "edge removed"
		}
		return emitItem(stdout, asJSON, w, fmt.Sprintf("#%d blocked_by %s: %s", id, ref, verbed))
	default:
		return usage(stderr)
	}
}

// runParent dispatches the parent set/clear subcommands over the store.
func runParent(ctx context.Context, stdout, stderr io.Writer, store worktracker.WorkTracker, asJSON bool, positional []string) int {
	switch first(positional) {
	case "set":
		if len(positional) != 3 {
			return usage(stderr)
		}
		child, derr := positiveID("parent", positional[1])
		if derr != nil {
			return emitDefects(stderr, asJSON, []schema.Defect{*derr})
		}
		parent, derr := positiveID("parent", positional[2])
		if derr != nil {
			return emitDefects(stderr, asJSON, []schema.Defect{*derr})
		}
		w, defects := store.SetParent(ctx, child, parent)
		if len(defects) > 0 {
			return emitDefects(stderr, asJSON, defects)
		}
		return emitItem(stdout, asJSON, w, fmt.Sprintf("#%d parent set to #%d", child, parent))
	case "clear":
		if len(positional) != 2 {
			return usage(stderr)
		}
		child, derr := positiveID("parent", positional[1])
		if derr != nil {
			return emitDefects(stderr, asJSON, []schema.Defect{*derr})
		}
		w, defects := store.ClearParent(ctx, child)
		if len(defects) > 0 {
			return emitDefects(stderr, asJSON, defects)
		}
		return emitItem(stdout, asJSON, w, fmt.Sprintf("#%d parent cleared", child))
	default:
		return usage(stderr)
	}
}

// runSignal dispatches the signal subcommands over the same core the engine
// consumes through the port. add/rm emit the updated item; def confirms the
// extension; ls lists the vocabulary — each schema-versioned under --json.
func runSignal(ctx context.Context, store worktracker.WorkTracker, stdout, stderr io.Writer, asJSON bool, positional []string, color, description string) int {
	sub := first(positional)
	rest := positional[1:]
	switch sub {
	case "add", "rm":
		id, derr := positiveID("signal", first(rest))
		if derr != nil {
			derr.Verb = "signal"
			return emitDefects(stderr, asJSON, []schema.Defect{*derr})
		}
		name := first(rest[min(1, len(rest)):])
		if name == "" {
			return emitDefects(stderr, asJSON, []schema.Defect{{Verb: "signal", Op: "usage", Reason: "no signal name given", Remedy: "carina signal " + sub + " <id> <name>"}})
		}
		var (
			w       model.WorkItem
			defects []schema.Defect
		)
		if sub == "add" {
			w, defects = store.AddSignal(ctx, id, name)
		} else {
			w, defects = store.RemoveSignal(ctx, id, name)
		}
		if len(defects) > 0 {
			return emitDefects(stderr, asJSON, defects)
		}
		return emitItem(stdout, asJSON, w, fmt.Sprintf("Signal %q %s #%d", name, past(sub), w.ID))
	case "def":
		name := first(rest)
		role := first(rest[min(1, len(rest)):])
		defects := store.DefineSignal(ctx, model.Signal{Name: name, Role: model.Role(role), Color: color, Description: description})
		if len(defects) > 0 {
			return emitDefects(stderr, asJSON, defects)
		}
		if asJSON {
			b, _ := json.Marshal(model.Signal{Name: name, Role: model.Role(role), Color: color, Description: description}.View())
			if _, err := fmt.Fprintln(stdout, string(b)); err != nil {
				return 1
			}
			return 0
		}
		if _, err := fmt.Fprintf(stdout, "Defined signal %q (%s)\n", name, role); err != nil {
			return 1
		}
		return 0
	case "ls":
		sigs, defects := store.Signals(ctx)
		if len(defects) > 0 {
			return emitDefects(stderr, asJSON, defects)
		}
		return emitVocabulary(stdout, asJSON, sigs)
	default:
		return usage(stderr)
	}
}

// emitRollup prints an item's children rollup — grouped by open and closed, the
// count and identity of each (FR-7, worktracker-hierarchy-index §3.1) — after the
// human show output, so a completed epic still surfaces its full task set. Items
// with no children print nothing; a read failure is reported without masking the
// item that already showed. JSON show never calls this: its machine view is the
// item alone so create/show stays byte-identical (a machine consumer reads the
// grouped set through the port's Rollup or `list --parent`).
func emitRollup(ctx context.Context, stdout, stderr io.Writer, store worktracker.WorkTracker, id int) int {
	view, defects := store.Rollup(ctx, id, worktracker.Display)
	if len(defects) > 0 {
		for _, d := range defects {
			if _, err := fmt.Fprintln(stderr, d.String()); err != nil {
				return 1
			}
		}
		return 1
	}
	if len(view.Open) == 0 && len(view.Closed) == 0 {
		return 0
	}
	if _, err := fmt.Fprintf(stdout, "open children: %d  [%s]\n", len(view.Open), joinItemIDs(view.Open)); err != nil {
		return 1
	}
	if _, err := fmt.Fprintf(stdout, "closed children: %d  [%s]\n", len(view.Closed), joinItemIDs(view.Closed)); err != nil {
		return 1
	}
	return 0
}

// joinItemIDs renders a child set as "#id #id …" for the human rollup lines.
func joinItemIDs(ws []model.WorkItem) string {
	ids := make([]string, len(ws))
	for i, w := range ws {
		ids[i] = "#" + strconv.Itoa(w.ID)
	}
	return strings.Join(ids, " ")
}

// emitReindex prints a reindex report — the markers rebuilt, items scanned, the
// trigger, and the duration — as a human line or, in --json, its schema-versioned
// machine projection (worktracker.ReindexReport). The reverse-parent index is
// derived state, so a rebuild is idempotent and safe to run at any time (DINV-6).
func emitReindex(w io.Writer, asJSON bool, report worktracker.ReindexReport) int {
	if asJSON {
		b, err := json.Marshal(report)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return 1
		}
		return 0
	}
	if _, err := fmt.Fprintf(w, "reindexed: %d markers over %d items (%s) in %s\n", report.Markers, report.Items, report.Trigger, report.Dur); err != nil {
		return 1
	}
	return 0
}

// emitItemLog prints a work item's append-log — its dialogue and frozen
// judgments — beneath the human show output. An item with an empty log prints
// nothing. Each entry names its kind, author, time, and (for a decision-record)
// its namespace, then the payload; a multi-line payload is indented under it.
func emitItemLog(w io.Writer, item model.WorkItem) int {
	if len(item.Log) == 0 {
		return 0
	}
	if _, err := fmt.Fprintf(w, "\n## Log\n"); err != nil {
		return 1
	}
	for _, e := range item.Log {
		head := fmt.Sprintf("%s · %s · %s", e.Kind, e.Author, e.At.UTC().Format(time.RFC3339))
		if e.Namespace != "" {
			head += " · " + e.Namespace
		}
		lines := strings.Split(e.Payload, "\n")
		if _, err := fmt.Fprintf(w, "- %s: %s\n", head, lines[0]); err != nil {
			return 1
		}
		for _, l := range lines[1:] {
			if _, err := fmt.Fprintf(w, "    %s\n", l); err != nil {
				return 1
			}
		}
	}
	return 0
}

// past renders the mutation verb for the human lead line.
func past(sub string) string {
	if sub == "add" {
		return "added to"
	}
	return "removed from"
}

// openStore resolves the store root from the operand config (paths.work_items)
// and returns a live-clock localfs adapter.
func openStore(root string) (worktracker.WorkTracker, []schema.Defect) {
	cfg, defects := config.Load(root)
	if len(defects) > 0 {
		return nil, defects
	}
	return localfs.New(filepath.Join(root, cfg.WorkItems), time.Now), nil
}

// parseInterspersed parses flags that may be interleaved with positional
// arguments — the stdlib flag package stops at the first non-flag, so a natural
// `carina show 1 --json` would drop the trailing flag. It returns the collected
// positionals, or ok=false on a flag error (the flagset already printed why).
func parseInterspersed(fs *flag.FlagSet, args []string) ([]string, bool) {
	var positional []string
	for len(args) > 0 {
		if err := fs.Parse(args); err != nil {
			return nil, false
		}
		args = fs.Args()
		if len(args) == 0 {
			break
		}
		positional = append(positional, args[0])
		args = args[1:]
	}
	return positional, true
}

func first(xs []string) string {
	if len(xs) == 0 {
		return ""
	}
	return xs[0]
}

func positiveID(verb, arg string) (int, *schema.Defect) {
	n, err := strconv.Atoi(arg)
	if err != nil || n <= 0 {
		return 0, &schema.Defect{Verb: verb, File: "", Op: "usage", Reason: fmt.Sprintf("id %q is not a positive integer", arg), Remedy: fmt.Sprintf("pass an item id, e.g. carina %s 1", verb)}
	}
	return n, nil
}

// emitItem prints a work item as human text (with an optional lead line) or, in
// JSON mode, its schema-versioned machine view.
func emitItem(w io.Writer, asJSON bool, item model.WorkItem, lead string) int {
	if asJSON {
		b, err := json.Marshal(item.View())
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return 1
		}
		return 0
	}
	if lead != "" {
		if _, err := fmt.Fprintln(w, lead); err != nil {
			return 1
		}
	}
	if _, err := fmt.Fprintf(w, "#%d  %s\n", item.ID, item.Title); err != nil {
		return 1
	}
	if _, err := fmt.Fprintf(w, "type: %s   state: %s\n", item.Type, item.State); err != nil {
		return 1
	}
	if item.CloseReason != "" {
		if _, err := fmt.Fprintf(w, "close_reason: %s\n", item.CloseReason); err != nil {
			return 1
		}
	}
	if item.Parent != "" {
		if _, err := fmt.Fprintf(w, "parent: #%s\n", item.Parent); err != nil {
			return 1
		}
	}
	if len(item.BlockedBy) > 0 {
		if _, err := fmt.Fprintf(w, "blocked_by: %s\n", strings.Join(item.BlockedBy, ", ")); err != nil {
			return 1
		}
	}
	if item.Priority != "" {
		if _, err := fmt.Fprintf(w, "priority: %s\n", item.Priority); err != nil {
			return 1
		}
	}
	if _, err := fmt.Fprintf(w, "created: %s   updated: %s\n", item.Created.UTC().Format(time.RFC3339), item.Updated.UTC().Format(time.RFC3339)); err != nil {
		return 1
	}
	if item.Body != "" {
		if _, err := fmt.Fprintf(w, "\n%s\n", item.Body); err != nil {
			return 1
		}
	}
	return 0
}

// emitVocabulary prints the signal vocabulary as a human table or, in JSON
// mode, its schema-versioned machine projection.
func emitVocabulary(w io.Writer, asJSON bool, sigs []model.Signal) int {
	if asJSON {
		b, err := json.Marshal(model.Vocabulary(sigs))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return 1
		}
		return 0
	}
	if _, err := fmt.Fprintf(w, "%-18s %-15s %s\n", "NAME", "ROLE", "DESCRIPTION"); err != nil {
		return 1
	}
	for _, s := range sigs {
		if _, err := fmt.Fprintf(w, "%-18s %-15s %s\n", s.Name, s.Role, s.Description); err != nil {
			return 1
		}
	}
	return 0
}

// emitList prints a list result as a human table or, in JSON mode, its
// schema-versioned machine projection (the query it ran under, the count, and
// the matched items). An empty result prints a "no items" line, never nothing.
func emitList(w io.Writer, asJSON bool, q model.Query, items []model.WorkItem) int {
	if asJSON {
		b, err := json.Marshal(model.ProjectList(q, items))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return 1
		}
		return 0
	}
	return emitItemTable(w, items)
}

// emitSearch prints a search result as a human table or, in JSON mode, its
// schema-versioned machine projection (the echoed query, the count, and the
// matched items).
func emitSearch(w io.Writer, asJSON bool, text string, items []model.WorkItem) int {
	if asJSON {
		b, err := json.Marshal(model.ProjectSearch(text, items))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		if _, err := fmt.Fprintln(w, string(b)); err != nil {
			return 1
		}
		return 0
	}
	return emitItemTable(w, items)
}

// emitItemTable prints a creation-ordered item set as a compact human table,
// the shared human form of list and search. An empty set prints a "no items"
// line so a query never falls silent.
func emitItemTable(w io.Writer, items []model.WorkItem) int {
	if len(items) == 0 {
		if _, err := fmt.Fprintln(w, "no items"); err != nil {
			return 1
		}
		return 0
	}
	for _, it := range items {
		if _, err := fmt.Fprintf(w, "#%-5d %-7s %-9s %s\n", it.ID, it.State, it.Type, it.Title); err != nil {
			return 1
		}
	}
	return 0
}

// emitDefects prints defects as human lines or, in JSON mode, one JSON object
// per line, and returns exit code 1.
func emitDefects(w io.Writer, asJSON bool, defects []schema.Defect) int {
	for _, d := range defects {
		if asJSON {
			b, _ := json.Marshal(d)
			if _, err := fmt.Fprintln(w, string(b)); err != nil {
				return 1
			}
		} else {
			if _, err := fmt.Fprintln(w, d.String()); err != nil {
				return 1
			}
		}
	}
	return 1
}
