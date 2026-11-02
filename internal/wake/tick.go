package wake

import (
	"context"
	"fmt"
	"path"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/docgate/argusledger"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

// Doc is one delivery ledger the sweep reads: its repo-relative path (whose
// directory names the problem slug) and its bytes.
type Doc struct {
	Path  string
	Bytes []byte
}

// Ledgers is the sweep's source of delivery ledgers — the local-substitutable
// seam (§3.9). The host adapter reads docs/problems/*/delivery.md from the repo;
// the tests provide fixtures. Listing is the tick's only bulk read; it performs
// no walk of unawaited surfaces (AP-19 — exact reads only).
type Ledgers interface {
	List(ctx context.Context) ([]Doc, error)
}

// FrontDoor invokes the agentic CLI once for one due touch, re-entering the
// drive at its existing front door with the normalized signal (§3.2). It is the
// only side-effecting port the tick holds; a poisoned or forged signal reaches
// it reduced to slug + reason (DINV-1), so it can carry no directive. The
// per-CLI invocation is the host adapter's; the tests substitute a recorder.
type FrontDoor interface {
	Touch(ctx context.Context, s Signal) error
}

// Backstop is the bounded tick: read the registered awaits, probe each awaited
// goal object with an exact read, invoke the CLI once per due touch, exit —
// worst case one interval of latency, ~0 idle cost, zero for parks (§3.2). It
// holds only ports and the staleness bound; it keeps no state between ticks
// (nothing resident — AP-19).
type Backstop struct {
	Ledgers Ledgers
	Prober  Prober
	Door    FrontDoor
	// Problems is the configured problems-root the delivery ledgers live under
	// (config.Problems; config-driven-portability-v1 §3.2, FR-1). It keys the
	// ledger-class recognition so a relocated corpus is still read; an empty
	// value falls back to the built-in default (docs/problems), keeping a
	// default-config sweep byte-identical.
	Problems string
	// Bound is the config staleness bound (config.Channel.StalenessBound); when
	// set the tick surfaces wake-staleness defects at this touchpoint (DINV-5,
	// via argusledger.WakeStaleness). Zero disables the surfacing.
	Bound time.Duration
	// Now injects the clock for the staleness surfacing (the repo Go rules — no
	// ambient clock). Nil disables the surfacing.
	Now func() time.Time
}

// problemsRoot returns the Backstop's configured problems root, defaulting to
// the built-in when unset — so a zero-value Backstop (and every existing test
// that constructs one without the field) reads the default corpus unchanged.
func (b *Backstop) problemsRoot() string {
	if b.Problems == "" {
		return config.Defaults().Problems
	}
	return b.Problems
}

// A converged no-op's reason — why a delivery landed as a no-op instead of an
// effective touch.
type NoOpReason string

const (
	// NoOpConverged — a duplicate delivery for a slug already touched this pass;
	// the extra converges to the one effective touch (DINV-6).
	NoOpConverged NoOpReason = "converged"
	// NoOpClosed — the delivery named a closed ledger; it is not reopened
	// (DINV-3).
	NoOpClosed NoOpReason = "closed"
)

// A skipped ledger and why — the tick considered it and did not probe it.
type SkipReason string

const (
	// SkipParked — a parked problem: the backstop draws zero activity on it; it
	// wakes only on its answer append or a human touch (DINV-2).
	SkipParked SkipReason = "parked"
	// SkipClosed — a closed ledger: no outstanding awaits, never reopened
	// (DINV-3).
	SkipClosed SkipReason = "closed"
)

// EffectiveTouch is one front-door invocation the tick made — the resumed drive
// re-verifies at the goal object, so this is a resume, not a guaranteed act.
type EffectiveTouch struct {
	Signal     Signal
	Source     Source
	GoalObject string // the completed await's goal object that made the touch due
}

// NoOpTouch is a delivery the tick declined to invoke, with why — the converged
// extra of a race, or a closed-ledger delivery. It costs nothing external.
type NoOpTouch struct {
	Slug   string
	Reason NoOpReason
}

// Skip is a ledger the tick did not probe, with why (parked or closed).
type Skip struct {
	Slug   string
	Reason SkipReason
}

// TickReport is the sweep's cold-readable outcome (§3.5's one observability
// surface). Every field is data: the effective touches invoked, the converged
// no-ops, the skipped ledgers, the exact-read count that is the tick's stated
// cost (DINV-4), whether this was an idle tick, the staleness defects surfaced
// at this touchpoint (DINV-5), and any parse/grammar/probe defects (fail loud).
type TickReport struct {
	Touches []EffectiveTouch
	NoOps   []NoOpTouch
	Skips   []Skip
	Probes  int
	Idle    bool
	Stale   []schema.Defect
	Defects []schema.Defect
}

// dueTouch is one problem whose awaited completion is verified — the unit the
// convergence primitive dedupes. Source distinguishes the delivery path.
type dueTouch struct {
	Signal     Signal
	Source     Source
	GoalObject string
}

// Tick runs one backstop pass: list the delivery ledgers, skip the parked and
// closed, probe each open ledger's registered awaits by exact reads, and invoke
// the front door once per problem with a due completion — then exit. It returns
// a hard error only when the ledger source itself cannot be read; a per-ledger
// parse fault, an unprobeable await, a probe read error, or a front-door failure
// is a NAMED defect in the report and the sweep continues, so one problem's
// hiccup never blinds the others (a backstop's job is resilient delivery; the
// bound is met at the next tick for anything this one could not deliver).
//
// Cost (DINV-4): the tick reads the ledgers once and performs one exact probe
// per outstanding await on an open ledger — nothing more. An idle tick (no
// awaits anywhere) performs zero probes and zero touches; a parked or closed
// problem is skipped before any probe, so it draws zero probes and zero touches.
func (b *Backstop) Tick(ctx context.Context) (TickReport, error) {
	docs, err := b.Ledgers.List(ctx)
	if err != nil {
		return TickReport{}, fmt.Errorf("wake: listing delivery ledgers for the backstop sweep: %w", err)
	}

	var rep TickReport
	var due []dueTouch
	anyAwait := false

	problems := b.problemsRoot()
	for _, doc := range docs {
		l, defects := argusledger.Parse(problems, verb, doc.Path, doc.Bytes)
		if len(defects) > 0 {
			rep.Defects = append(rep.Defects, defects...)
			continue
		}
		if l == nil {
			continue // not an in-scope delivery ledger
		}
		slug := slugFromPath(doc.Path)

		// Skip parked and closed BEFORE any probe — the parked/closed exclusion is
		// by construction, so it costs zero probes (DINV-2, DINV-3).
		switch Dispose(l) {
		case Parked:
			rep.Skips = append(rep.Skips, Skip{Slug: slug, Reason: SkipParked})
			continue
		case Closed:
			rep.Skips = append(rep.Skips, Skip{Slug: slug, Reason: SkipClosed})
			continue
		}

		awaits, adefects := argusledger.DeriveAwaits(verb, l)
		rep.Defects = append(rep.Defects, adefects...)
		if len(awaits) > 0 {
			anyAwait = true
		}

		// Surface staleness at this touchpoint (DINV-5), reusing T7's computation.
		if b.Bound > 0 && b.Now != nil {
			rep.Stale = append(rep.Stale, argusledger.WakeStaleness(verb, doc.Path, awaits, b.Bound, b.Now())...)
		}

		// Probe each await; the FIRST verified completion makes the whole problem
		// due (one front-door touch resumes the drive, which re-verifies ALL its
		// awaits — so the touch granularity is per problem, not per await).
		var found *dueTouch
		for _, a := range awaits {
			done, pdef, perr := probeAwait(ctx, b.Prober, a)
			rep.Probes++
			if pdef != nil {
				rep.Defects = append(rep.Defects, *pdef)
				continue
			}
			if perr != nil {
				rep.Defects = append(rep.Defects, probeReadDefect(doc.Path, a, perr))
				continue
			}
			if !done || found != nil {
				continue
			}
			reason, ok := reasonForAwait(a)
			if !ok {
				continue
			}
			sig, ok := NewSignal(slug, reason)
			if !ok {
				rep.Defects = append(rep.Defects, badSlugDefect(doc.Path, slug))
				break
			}
			found = &dueTouch{Signal: sig, Source: SourceBackstop, GoalObject: a.GoalObject}
		}
		if found != nil {
			due = append(due, *found)
		}
	}

	rep.Idle = !anyAwait
	eff, noops, derr := deliver(ctx, b.Door, due)
	rep.Touches = append(rep.Touches, eff...)
	rep.NoOps = append(rep.NoOps, noops...)
	rep.Defects = append(rep.Defects, derr...)
	return rep, nil
}

// deliver invokes the front door once per distinct problem slug and records the
// extras as converged no-ops (DINV-6 — one effective resumed touch, extras
// journaled no-ops). It is the convergence point the backstop and the push
// signal path share: a signal racing the tick, or overlapping ticks, passed as
// one batch converge here to a single invocation per slug. A front-door failure
// for one touch is a named defect (returned in the defects slice) and the
// remaining touches still deliver. Cross-process races (an independent signal
// process and a tick process) converge instead through the resume protocol's own
// replay + goal-object verification, which makes a redundant invocation a
// journaled no-op by construction (§3.4 DINV-6).
func deliver(ctx context.Context, door FrontDoor, touches []dueTouch) ([]EffectiveTouch, []NoOpTouch, []schema.Defect) {
	var eff []EffectiveTouch
	var noops []NoOpTouch
	var defects []schema.Defect
	seen := map[string]bool{}
	for _, t := range touches {
		if seen[t.Signal.Slug] {
			noops = append(noops, NoOpTouch{Slug: t.Signal.Slug, Reason: NoOpConverged})
			continue
		}
		seen[t.Signal.Slug] = true
		if err := door.Touch(ctx, t.Signal); err != nil {
			defects = append(defects, doorDefect(t.Signal, err))
			continue
		}
		eff = append(eff, EffectiveTouch{Signal: t.Signal, Source: t.Source, GoalObject: t.GoalObject})
	}
	return eff, noops, defects
}

// slugFromPath extracts the problem slug from a delivery-ledger path
// docs/problems/<slug>/delivery.md — the parent directory's name.
func slugFromPath(p string) string {
	return path.Base(path.Dir(strings.ReplaceAll(p, "\\", "/")))
}

func probeReadDefect(file string, a argusledger.Await, err error) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: "probe",
		Reason: "probing await " + quote(a.GoalObject) + " failed: " + err.Error(),
		Remedy: "the backstop retries the probe on its next tick; fix the goal-object read if it persists"}
}

func doorDefect(s Signal, err error) schema.Defect {
	return schema.Defect{Verb: verb, File: s.Slug, Op: "touch",
		Reason: "the front-door touch for " + quote(s.Wire()) + " did not complete: " + err.Error(),
		Remedy: "the backstop retries delivery on its next tick; the completion and its await are never lost (ledger-derived)"}
}

func badSlugDefect(file, slug string) schema.Defect {
	return schema.Defect{Verb: verb, File: file, Op: "touch",
		Reason: "the ledger path yields an ill-formed problem slug " + quote(slug),
		Remedy: "a delivery ledger lives at docs/problems/<kebab-slug>/delivery.md so the wake signal names a valid slug"}
}
