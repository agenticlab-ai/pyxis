// Package phase is the pipeline-measurement phase-boundary detector: a
// stack that brackets a harness's own session record into phases — a
// skill-invocation marker (a Skill tool-call or a slash command) pushes a
// phase, an artifact-write marker (the frozen doc / PR) pops it — and
// attributes each turn to the innermost currently-open phase
// (docs/tech-design/pipeline-measurement-v2.md §3.8, DINV-5).
//
// This package owns only the stack mechanics and the boundary predicates —
// the detector "rides" the same normalized turn sequence a spend-source
// adapter reads (design §3.8), it does not define a new seam. Turning a
// harness's real session record into a Turn sequence — deciding, from that
// harness's own transcript shape, which entry is a skill invocation or an
// artifact write — is a harness-specific adapter's concern (a later task),
// not this package's.
package phase

// Turn is one entry in a harness's own session record, normalized to what
// the phase-stack detector needs: which harness produced it, and whether it
// is a boundary marker. Design §3.9 calls this unit "per-turn usage" — the
// same unit a spend-source adapter will later sum within a phase bracket;
// this package attributes turns to phases but never reads or carries spend
// (out of scope here; a later task's concern).
type Turn struct {
	// Harness names the harness that produced this turn (e.g. "claude-code",
	// "codex", "cursor", "opencode"). A phase takes its Harness from the
	// turn that opened it (design §3.8, UB-4).
	Harness string

	// SkillInvocation is the invoked skill/command name when this turn is a
	// skill-invocation marker — a Skill tool-call or a slash command
	// (design §3.8, D-3) — and empty for any other turn. See
	// IsSkillInvocation.
	SkillInvocation string

	// ArtifactWrite is true when this turn is an artifact-write marker — the
	// frozen doc or PR that closes a phase (design §3.8, D-3). See
	// IsArtifactWrite.
	ArtifactWrite bool

	// Ordinal is this turn's position in the harness's own session record,
	// as whatever normalizes the record into a Turn sequence assigns it.
	// Opaque to this package — the detector neither reads nor requires it —
	// but a spend-source adapter that reads the same record (design §3.7's
	// S-2; §3.8: "rides S-2, not a new seam") can stamp it here and, after
	// Detect/Phases brackets the record, read each Phase.Turns[i].Ordinal
	// back to that turn's own per-model usage: the two reads join on one
	// harness record without sharing a second bracketing implementation.
	Ordinal int
}

// IsSkillInvocation reports whether turn is a skill-invocation marker that
// opens a new phase, and the id of the phase to open — the invoked skill or
// command name (design §3.8: "a Skill tool-call or slash-command"). This is
// the stack's push predicate.
func IsSkillInvocation(turn Turn) (phaseID string, ok bool) {
	return turn.SkillInvocation, turn.SkillInvocation != ""
}

// IsArtifactWrite reports whether turn is an artifact-write marker that
// closes the innermost open phase (design §3.8: "the artifact-write — the
// frozen doc / PR"). This is the stack's pop predicate.
func IsArtifactWrite(turn Turn) bool {
	return turn.ArtifactWrite
}

// Phase is one bracket the stack found: every turn between a
// skill-invocation open marker and its matching artifact-write close
// marker, or — for a phase the record never closed — every turn up to the
// end of the record (UB-3).
type Phase struct {
	// ID is the phase identity: the invoked skill/command name that opened
	// it.
	ID string

	// Harness is the harness that ran this phase — the Harness of the turn
	// that opened it (design §3.8, UB-4). A run that switches harness
	// mid-pipeline yields one phase per harness, each naming its own.
	Harness string

	// Turns is every turn attributed to this phase, in record order,
	// including the open marker and (if present) the close marker
	// themselves. A nested sub-phase's turns are never included here
	// (DINV-5): they belong solely to the nested Phase, never blended into
	// this one.
	Turns []Turn

	// Complete is true once this phase's artifact-write close marker
	// arrived. false means the record ended (or Phases was read) while this
	// phase was still the innermost open one — an aborted phase, recorded
	// with every turn it accrued, never dropped or merged into the next
	// phase (UB-3).
	Complete bool
}

// Stack is the phase-boundary detector (design §3.8, D-3): fed a harness's
// own session record one Turn at a time, in record order, it brackets the
// record into Phases and attributes each turn to the innermost open one
// (innermost-active-phase, DINV-5). The zero value is ready to use.
type Stack struct {
	all  []*Phase // every phase ever opened, in open order — Phases' return value
	open []*Phase // the currently-open phases (a stack), outermost first
}

// Push feeds the next turn from the harness's session record to the stack.
//
//   - An IsSkillInvocation turn opens a new phase — nested under whatever
//     phase is currently open, if any — and is itself attributed to that
//     new phase (the phase it opens, not its parent).
//   - An IsArtifactWrite turn is attributed to the innermost open phase and
//     then closes it (Complete becomes true), reopening that phase's parent
//     (if any) as the innermost. A close arriving with nothing open belongs
//     to no phase and is dropped.
//   - Any other turn is attributed to the innermost open phase, or belongs
//     to no phase (and is dropped) if the stack is currently empty.
func (s *Stack) Push(turn Turn) {
	if id, ok := IsSkillInvocation(turn); ok {
		p := &Phase{ID: id, Harness: turn.Harness}
		s.all = append(s.all, p)
		s.open = append(s.open, p)
		s.attribute(turn)
		return
	}
	if IsArtifactWrite(turn) {
		if len(s.open) == 0 {
			return
		}
		s.attribute(turn)
		s.open[len(s.open)-1].Complete = true
		s.open = s.open[:len(s.open)-1]
		return
	}
	s.attribute(turn)
}

// attribute appends turn to the innermost open phase, if any.
func (s *Stack) attribute(turn Turn) {
	if len(s.open) == 0 {
		return
	}
	top := s.open[len(s.open)-1]
	top.Turns = append(top.Turns, turn)
}

// Phases returns every phase found so far, in the order each was opened. A
// phase still open — no close marker has arrived yet — is included,
// Complete == false, carrying every turn it has accrued so far: an aborted
// phase is never dropped or merged into another phase (UB-3). Phases may be
// called repeatedly as more turns are pushed; each call is a fresh
// snapshot.
func (s *Stack) Phases() []Phase {
	out := make([]Phase, len(s.all))
	for i, p := range s.all {
		out[i] = *p
	}
	return out
}

// Detect brackets turns in one pass — equivalent to pushing each onto a new
// Stack in order and reading back Phases() — for a caller that already has
// the whole harness record parsed. Stack itself is exported for a caller
// that wants to feed turns incrementally (e.g. while streaming a transcript).
func Detect(turns []Turn) []Phase {
	var s Stack
	for _, t := range turns {
		s.Push(t)
	}
	return s.Phases()
}
