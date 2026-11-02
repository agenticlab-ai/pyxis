package routingevidence

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"sort"

	"github.com/agenticlab-ai/pyxis/internal/implreview/assurance"
	implverdict "github.com/agenticlab-ai/pyxis/internal/implreview/verdict"
)

// FindingEvent is one complete normalized child supplied with a pass. The
// redundant parent fields are intentional: CompletePass rejects any child
// that does not independently exact-bind the enclosing pass.
type FindingEvent struct {
	Payload FindingPersistedPayload
}

// PassComposite is the only pass append shape. A writer must make the pass and
// every declared finding child reachable in one store compare-and-swap.
type PassComposite struct {
	Pass     AssurancePassCompletedPayload
	Findings []FindingEvent
}

// DispositionEvent carries the canonical event payload plus the enclosing
// identity needed to exact-read the finding. These binding fields do not enter
// the event; the service proves them against the canonical pass before append.
type DispositionEvent struct {
	PassID            string
	ReviewProcessID   string
	ProposalID        string
	ReviewedRevision  string
	TopLevelContextID string
	Payload           DispositionRecordedPayload
}

// ProcessTerminal is impl-review's already-decided aggregate input. The
// service reconstructs its pass arrays and accepting result from canonical
// pass events, so a favorable subset cannot be persisted as satisfied.
type ProcessTerminal struct {
	Payload ReviewProcessCompletedPayload
}

// NativeAct is the independently observed native-medium fact. The verifier
// obtains it by exact reference; no actor or finding attribution is trusted
// merely because it appeared in a caller-authored payload.
type NativeAct struct {
	Actor                     string
	ActorAttribution          string
	FindingID                 string
	NativeAuthorIdentityProof string
	NativeMediumActRef        string
	Reason                    string
	SourceOwnedAt             string
	Verdict                   string
}

// NativeActVerifier is consumer-owned by the proposal medium. Implementations
// must exact-read the referenced native act and independently verify its author
// proof before returning it.
type NativeActVerifier interface {
	VerifyNativeAct(context.Context, string) (NativeAct, error)
}

// ReviewEventWriter is the trusted host-side append seam. Reviewer/pass model
// contexts never receive this capability.
type ReviewEventWriter interface {
	WriteReviewEvents(context.Context, ObservedHead, []Event) (EvidenceProof, error)
}

// SignedReviewEventWriter signs orchestrator events and native-medium events
// with their separate least-privilege capabilities before one canonical append.
type SignedReviewEventWriter struct {
	Store                  Store
	OrchestratorCapability SigningCapability
	MediumCapability       SigningCapability
}

func (w SignedReviewEventWriter) WriteReviewEvents(ctx context.Context, head ObservedHead, events []Event) (EvidenceProof, error) {
	if w.Store == nil || len(events) == 0 {
		return EvidenceProof{}, errors.New("routing review: store or event set is absent")
	}
	if err := validateReviewWriteShape(events); err != nil {
		return EvidenceProof{}, err
	}
	signed := append([]Event(nil), events...)
	for i := range signed {
		capability := w.OrchestratorCapability
		if signed[i].Kind == DispositionRecorded {
			capability = w.MediumCapability
		}
		signed[i].Producer = Producer{Class: string(capability.ProducerClass), Generation: capability.ImplementationGeneration}
		unsigned, err := NewUnsignedEnvelope(signed[i])
		if err != nil {
			return EvidenceProof{}, fmt.Errorf("routing review: construct %s: %w", signed[i].Kind, err)
		}
		signed[i].ProducerProof, err = Sign(unsigned, capability)
		if err != nil {
			return EvidenceProof{}, fmt.Errorf("routing review: sign %s: %w", signed[i].Kind, err)
		}
	}
	if len(signed) == 1 {
		return w.Store.Append(ctx, AppendRequest{ObservedHead: head, Event: &signed[0]})
	}
	return w.Store.Append(ctx, AppendRequest{ObservedHead: head, Composite: &AtomicComposite{Events: signed}})
}

func validateReviewWriteShape(events []Event) error {
	if len(events) == 1 {
		if oneOf(string(events[0].Kind), string(AssurancePassCompleted), string(DispositionRecorded), string(ReviewProcessCompleted)) {
			return nil
		}
		return fmt.Errorf("routing review: single append cannot contain %s", events[0].Kind)
	}
	passes := 0
	for _, event := range events {
		switch event.Kind {
		case AssurancePassCompleted:
			passes++
		case FindingPersisted:
		default:
			return fmt.Errorf("routing review: pass composite contains %s", event.Kind)
		}
	}
	if passes != 1 {
		return fmt.Errorf("routing review: composite requires exactly one pass parent")
	}
	return nil
}

var ErrReviewEvidenceRejected = errors.New("routing review evidence rejected")

type ReviewService struct {
	store    Store
	writer   ReviewEventWriter
	verifier NativeActVerifier
}

func NewReviewService(store Store, writer ReviewEventWriter, verifier NativeActVerifier) (*ReviewService, error) {
	if store == nil || writer == nil || verifier == nil {
		return nil, errors.New("routing review: store, writer, and native-act verifier are required")
	}
	return &ReviewService{store: store, writer: writer, verifier: verifier}, nil
}

// CompletePass validates one verified review attempt and appends the pass plus
// its bijective normalized finding set atomically. An identical retry exact-
// reads the existing composite; a differing retry never reaches the writer.
func (s *ReviewService) CompletePass(ctx context.Context, composite PassComposite) (EvidenceProof, error) {
	if err := validatePassComposite(composite); err != nil {
		return EvidenceProof{}, reviewReject("pass composite", err)
	}
	read, err := s.store.ReadSubject(ctx, SubjectID{Kind: SubjectAttempt, ID: composite.Pass.AttemptID})
	if err != nil {
		return EvidenceProof{}, reviewReject("exact-read review attempt", err)
	}
	if err := validatePassAttempt(read, composite.Pass); err != nil {
		return EvidenceProof{}, reviewReject("pass attempt binding", err)
	}
	events, err := passEvents(read.Events, composite)
	if err != nil {
		return EvidenceProof{}, reviewReject("construct pass events", err)
	}
	if existing := reviewEvents(read.Events, composite.Pass.ReviewProcessID, composite.Pass.PassID, ""); len(existing) > 0 {
		if err := compareReviewEvents(existing, events); err != nil {
			return EvidenceProof{}, reviewReject("conflicting pass retry", err)
		}
		return proofForReview(read, existing)
	}
	if hasProcessTerminal(read.Events, composite.Pass.ReviewProcessID) {
		return EvidenceProof{}, reviewReject("pass composite", errors.New("review process already has a terminal"))
	}
	proof, err := s.writer.WriteReviewEvents(ctx, read.ObservedHead, events)
	if err != nil {
		return EvidenceProof{}, reviewReject("append pass composite", err)
	}
	if err := s.verifyWritten(ctx, proof, events); err != nil {
		return EvidenceProof{}, reviewReject("exact-read pass composite", err)
	}
	return proof, nil
}

// RecordDisposition accepts only an exact native act whose independently
// observed author, proof, finding, and verdict equal the requested event.
func (s *ReviewService) RecordDisposition(ctx context.Context, disposition DispositionEvent) (EvidenceProof, error) {
	if err := validateDispositionBinding(disposition); err != nil {
		return EvidenceProof{}, reviewReject("disposition", err)
	}
	read, err := s.store.ReadSubject(ctx, SubjectID{Kind: SubjectProcess, ID: disposition.ReviewProcessID})
	if err != nil {
		return EvidenceProof{}, reviewReject("exact-read disposition process", err)
	}
	if err := bindDispositionFinding(read.Events, disposition); err != nil {
		return EvidenceProof{}, reviewReject("disposition finding binding", err)
	}
	observed, err := s.verifier.VerifyNativeAct(ctx, disposition.Payload.NativeMediumActRef)
	if err != nil {
		return EvidenceProof{}, reviewReject("verify native medium act", err)
	}
	want := nativeAct(disposition.Payload)
	if observed != want {
		return EvidenceProof{}, reviewReject("verify native medium act", errors.New("observed act or author proof differs from requested disposition"))
	}
	base, err := eventBase(read.Events)
	if err != nil {
		return EvidenceProof{}, reviewReject("disposition event boundary", err)
	}
	event := base
	event.Kind, event.Payload = DispositionRecorded, Payload{DispositionRecorded: &disposition.Payload}
	for _, prior := range reviewEvents(read.Events, disposition.ReviewProcessID, disposition.PassID, disposition.Payload.FindingID) {
		if prior.Kind == DispositionRecorded && prior.Payload.DispositionRecorded.NativeMediumActRef == disposition.Payload.NativeMediumActRef {
			if err := compareReviewEvents([]Event{prior}, []Event{event}); err != nil {
				return EvidenceProof{}, reviewReject("conflicting disposition retry", err)
			}
			return proofForReview(read, []Event{prior})
		}
	}
	proof, err := s.writer.WriteReviewEvents(ctx, read.ObservedHead, []Event{event})
	if err != nil {
		return EvidenceProof{}, reviewReject("append disposition", err)
	}
	if err := s.verifyWritten(ctx, proof, []Event{event}); err != nil {
		return EvidenceProof{}, reviewReject("exact-read disposition", err)
	}
	return proof, nil
}

// CompleteProcess reconstructs the complete pass manifest and deterministic
// aggregate. Missing floors become floor_violation; complete but conflicting,
// non-accepting, or non-blind sets become conflict_nonaccepting.
func (s *ReviewService) CompleteProcess(ctx context.Context, terminal ProcessTerminal) (EvidenceProof, error) {
	if err := validatePayload(&terminal.Payload); err != nil {
		return EvidenceProof{}, reviewReject("process terminal", err)
	}
	read, err := s.store.ReadSubject(ctx, SubjectID{Kind: SubjectDemand, ID: terminal.Payload.DemandID})
	if err != nil {
		return EvidenceProof{}, reviewReject("exact-read process demand", err)
	}
	computed, err := reduceProcessTerminal(read.Events, terminal.Payload)
	if err != nil {
		return EvidenceProof{}, reviewReject("reduce process", err)
	}
	base, err := eventBase(read.Events)
	if err != nil {
		return EvidenceProof{}, reviewReject("process event boundary", err)
	}
	event := base
	event.Kind, event.Payload = ReviewProcessCompleted, Payload{ReviewProcessCompleted: &computed}
	for _, prior := range read.Events {
		if prior.Kind == ReviewProcessCompleted && prior.Payload.ReviewProcessCompleted.ReviewProcessID == computed.ReviewProcessID {
			if err := compareReviewEvents([]Event{prior}, []Event{event}); err != nil {
				return EvidenceProof{}, reviewReject("conflicting process retry", err)
			}
			return proofForReview(read, []Event{prior})
		}
	}
	proof, err := s.writer.WriteReviewEvents(ctx, read.ObservedHead, []Event{event})
	if err != nil {
		return EvidenceProof{}, reviewReject("append process terminal", err)
	}
	if err := s.verifyWritten(ctx, proof, []Event{event}); err != nil {
		return EvidenceProof{}, reviewReject("exact-read process terminal", err)
	}
	return proof, nil
}

func (s *ReviewService) verifyWritten(ctx context.Context, proof EvidenceProof, want []Event) error {
	exact, err := s.store.ReadProof(ctx, proof)
	if err != nil {
		return err
	}
	return compareReviewEvents(exact.Events, want)
}

func validatePassComposite(composite PassComposite) error {
	if err := validatePayload(&composite.Pass); err != nil {
		return err
	}
	if len(composite.Findings) != composite.Pass.Findings {
		return fmt.Errorf("finding child count %d differs from declared %d", len(composite.Findings), composite.Pass.Findings)
	}
	manifest := make(map[string]NormalizedFinding, len(composite.Pass.FindingRecords))
	for _, finding := range composite.Pass.FindingRecords {
		manifest[finding.FindingID] = finding
	}
	seen := make(map[string]bool, len(composite.Findings))
	for _, child := range composite.Findings {
		p := child.Payload
		if err := validatePayload(&p); err != nil {
			return err
		}
		if seen[p.FindingID] {
			return fmt.Errorf("finding child %q is duplicated", p.FindingID)
		}
		seen[p.FindingID] = true
		want, ok := manifest[p.FindingID]
		if !ok || p.PassID != composite.Pass.PassID || p.ReviewProcessID != composite.Pass.ReviewProcessID ||
			p.ProposalID != composite.Pass.ProposalID || p.ReviewedRevision != composite.Pass.ReviewedRevision ||
			!reflect.DeepEqual(p.NormalizedFinding, want) {
			return fmt.Errorf("finding child %q does not exact-bind its pass manifest", p.FindingID)
		}
	}
	return nil
}

func validatePassAttempt(read ExactRead, pass AssurancePassCompletedPayload) error {
	if read.Subject == nil || read.Subject.Attempt == nil || !read.Subject.Attempt.ProcessEligible {
		return errors.New("attempt is not a verified review outcome")
	}
	resolution, ok := resolvedEvent(read.Events, pass.AttemptID)
	if !ok || resolution.Payload.RoutingResolved.Role != "review" {
		return errors.New("review routing resolution is absent")
	}
	var outcome Event
	for _, event := range read.Events {
		if event.Kind == RoutedAttemptCompleted && event.Payload.RoutedAttemptCompleted.AttemptID == pass.AttemptID {
			outcome = event
			break
		}
	}
	rp := resolution.Payload.RoutingResolved
	if outcome.EventID == "" || outcome.Payload.RoutedAttemptCompleted.Outcome != "verified" ||
		pass.DemandID != rp.DemandID || pass.TaskID != rp.TaskID || pass.ProposalID != rp.ProposalID ||
		pass.RoutingResolutionRef != string(resolution.EventID) || pass.RoutedOutcomeRef != string(outcome.EventID) {
		return errors.New("pass differs from its exact verified route or outcome")
	}
	return nil
}

func passEvents(existing []Event, composite PassComposite) ([]Event, error) {
	base, err := eventBase(existing)
	if err != nil {
		return nil, err
	}
	pass := composite.Pass
	normalized, err := normalizedPayload(Event{Kind: AssurancePassCompleted, Payload: Payload{AssurancePassCompleted: &pass}, Schema: Schema, Version: EventVersion, Repository: base.Repository, EvidenceRef: base.EvidenceRef})
	if err != nil {
		return nil, err
	}
	pass = *normalized.(*AssurancePassCompletedPayload)
	parent := base
	parent.Kind, parent.Payload = AssurancePassCompleted, Payload{AssurancePassCompleted: &pass}
	events := []Event{parent}
	for i := range composite.Findings {
		child := base
		payload := composite.Findings[i].Payload
		child.Kind, child.Payload = FindingPersisted, Payload{FindingPersisted: &payload}
		events = append(events, child)
	}
	return events, nil
}

func validateDispositionBinding(event DispositionEvent) error {
	if event.PassID == "" || event.ReviewProcessID == "" || event.ProposalID == "" || event.ReviewedRevision == "" || event.TopLevelContextID == "" {
		return errors.New("pass, process, proposal, revision, and context bindings are required")
	}
	if err := validatePayload(&event.Payload); err != nil {
		return err
	}
	if event.Payload.AttributionVerification != "verified" {
		return errors.New("native author attribution was not independently verified")
	}
	return nil
}

func bindDispositionFinding(events []Event, disposition DispositionEvent) error {
	for _, passEvent := range events {
		if passEvent.Kind != AssurancePassCompleted {
			continue
		}
		p := passEvent.Payload.AssurancePassCompleted
		if p.PassID != disposition.PassID || p.ReviewProcessID != disposition.ReviewProcessID || p.ProposalID != disposition.ProposalID ||
			p.ReviewedRevision != disposition.ReviewedRevision || p.TopLevelContextID != disposition.TopLevelContextID {
			continue
		}
		for _, finding := range p.FindingRecords {
			if finding.FindingID == disposition.Payload.FindingID {
				return nil
			}
		}
		return errors.New("finding is not declared by the exact-bound pass")
	}
	return errors.New("exact-bound pass is absent")
}

func nativeAct(p DispositionRecordedPayload) NativeAct {
	return NativeAct{Actor: p.Actor, ActorAttribution: p.ActorAttribution, FindingID: p.FindingID,
		NativeAuthorIdentityProof: p.NativeAuthorIdentityProof, NativeMediumActRef: p.NativeMediumActRef,
		Reason: p.Reason, SourceOwnedAt: p.SourceOwnedAt, Verdict: p.Verdict}
}

func reduceProcessTerminal(events []Event, requested ReviewProcessCompletedPayload) (ReviewProcessCompletedPayload, error) {
	passes := make([]Event, 0, requested.RequiredPasses)
	for _, event := range events {
		if event.Kind == AssurancePassCompleted && event.Payload.AssurancePassCompleted.ReviewProcessID == requested.ReviewProcessID {
			passes = append(passes, event)
		}
	}
	sort.Slice(passes, func(i, j int) bool {
		return passes[i].Payload.AssurancePassCompleted.PassIndex < passes[j].Payload.AssurancePassCompleted.PassIndex
	})
	out := requested
	out.CompletedPasses = len(passes)
	out.PassAttemptIDs, out.PassVerdicts, out.TopLevelContextIDs = []string{}, []string{}, []string{}
	conflict := false
	contexts, attempts := map[string]bool{}, map[string]bool{}
	for i, event := range passes {
		p := event.Payload.AssurancePassCompleted
		if i == 0 {
			if requested.ProcessGeneration != p.ProcessGeneration || requested.DemandID != p.DemandID || requested.TaskID != p.TaskID ||
				requested.ProposalID != p.ProposalID || requested.ReviewedRevision != p.ReviewedRevision ||
				requested.EffectiveDepth != p.EffectiveDepth || requested.RequiredPasses != p.RequiredPasses {
				conflict = true
			}
			out.ProcessGeneration, out.DemandID, out.TaskID = p.ProcessGeneration, p.DemandID, p.TaskID
			out.ProposalID, out.ReviewedRevision = p.ProposalID, p.ReviewedRevision
			out.EffectiveDepth, out.RequiredPasses = p.EffectiveDepth, p.RequiredPasses
		}
		if p.PassIndex != i+1 || p.ProcessGeneration != out.ProcessGeneration || p.DemandID != out.DemandID || p.TaskID != out.TaskID ||
			p.ProposalID != out.ProposalID || p.ReviewedRevision != out.ReviewedRevision || p.EffectiveDepth != out.EffectiveDepth || p.RequiredPasses != out.RequiredPasses ||
			contexts[p.TopLevelContextID] || attempts[p.AttemptID] {
			conflict = true
		}
		contexts[p.TopLevelContextID], attempts[p.AttemptID] = true, true
		out.PassAttemptIDs = append(out.PassAttemptIDs, p.AttemptID)
		out.PassVerdicts = append(out.PassVerdicts, p.Verdict)
		out.TopLevelContextIDs = append(out.TopLevelContextIDs, p.TopLevelContextID)
	}
	if len(passes) > 0 {
		aggregate, aggregateErr := implverdict.MostRestrictive(out.PassVerdicts)
		if aggregateErr != nil {
			return ReviewProcessCompletedPayload{}, aggregateErr
		}
		out.AggregateVerdict = aggregate
	}
	complete := len(passes) == out.RequiredPasses
	wantDepth, wantPasses, policyErr := assurance.Required(out.PlannedAssurance, out.ProposalBand)
	if policyErr != nil {
		return ReviewProcessCompletedPayload{}, policyErr
	}
	floorViolation := out.EffectiveDepth != wantDepth || out.RequiredPasses != wantPasses || !complete
	if !reflect.DeepEqual(requested.PassAttemptIDs, out.PassAttemptIDs) || !reflect.DeepEqual(requested.PassVerdicts, out.PassVerdicts) ||
		!reflect.DeepEqual(requested.TopLevelContextIDs, out.TopLevelContextIDs) || requested.CompletedPasses != out.CompletedPasses ||
		(len(passes) > 0 && requested.AggregateVerdict != out.AggregateVerdict) {
		conflict = true
	}
	if len(out.PassVerdicts) > 1 {
		for _, verdict := range out.PassVerdicts[1:] {
			conflict = conflict || verdict != out.PassVerdicts[0]
		}
	}
	for _, verdict := range out.PassVerdicts {
		conflict = conflict || (verdict != "approve" && verdict != "approve-with-advisories")
	}
	switch {
	case floorViolation:
		out.Result = "floor_violation"
	case conflict:
		out.Result = "conflict_nonaccepting"
	default:
		out.Result = "satisfied"
	}
	if err := validatePayload(&out); err != nil {
		return ReviewProcessCompletedPayload{}, err
	}
	return out, nil
}

func eventBase(events []Event) (Event, error) {
	if len(events) == 0 {
		return Event{}, errors.New("canonical event boundary is absent")
	}
	return Event{EvidenceRef: events[0].EvidenceRef, Repository: events[0].Repository, Schema: Schema, Version: EventVersion}, nil
}

func hasProcessTerminal(events []Event, processID string) bool {
	for _, event := range events {
		if event.Kind == ReviewProcessCompleted && event.Payload.ReviewProcessCompleted.ReviewProcessID == processID {
			return true
		}
	}
	return false
}

func reviewEvents(events []Event, processID, passID, findingID string) []Event {
	selected := []Event{}
	declared := map[string]bool{}
	for _, event := range events {
		if event.Kind == AssurancePassCompleted && event.Payload.AssurancePassCompleted.ReviewProcessID == processID && (passID == "" || event.Payload.AssurancePassCompleted.PassID == passID) {
			selected = append(selected, event)
			for _, finding := range event.Payload.AssurancePassCompleted.FindingRecords {
				declared[finding.FindingID] = true
			}
		}
	}
	for _, event := range events {
		switch event.Kind {
		case FindingPersisted:
			p := event.Payload.FindingPersisted
			if p.ReviewProcessID == processID && (passID == "" || p.PassID == passID) && (findingID == "" || p.FindingID == findingID) {
				selected = append(selected, event)
			}
		case DispositionRecorded:
			if findingID != "" && declared[event.Payload.DispositionRecorded.FindingID] && event.Payload.DispositionRecorded.FindingID == findingID {
				selected = append(selected, event)
			}
		}
	}
	return selected
}

func compareReviewEvents(got, want []Event) error {
	if len(got) != len(want) {
		return fmt.Errorf("review event count is %d, want %d", len(got), len(want))
	}
	key := func(event Event) string {
		switch event.Kind {
		case FindingPersisted:
			return string(event.Kind) + "\x00" + event.Payload.FindingPersisted.PassID + "\x00" + event.Payload.FindingPersisted.FindingID
		case DispositionRecorded:
			return string(event.Kind) + "\x00" + event.Payload.DispositionRecorded.FindingID + "\x00" + event.Payload.DispositionRecorded.NativeMediumActRef
		default:
			return string(event.Kind)
		}
	}
	actual := make(map[string]Event, len(got))
	for _, event := range got {
		actual[key(event)] = event
	}
	for _, expected := range want {
		found, ok := actual[key(expected)]
		if !ok || found.Repository != expected.Repository || found.EvidenceRef != expected.EvidenceRef || !reflect.DeepEqual(found.Payload, expected.Payload) {
			return fmt.Errorf("%s differs from requested review evidence", key(expected))
		}
	}
	return nil
}

func proofForReview(read ExactRead, events []Event) (EvidenceProof, error) {
	entries := make([]EvidenceEntry, 0, len(events))
	for _, event := range events {
		path, err := EventPath(event.EventID)
		if err != nil {
			return EvidenceProof{}, err
		}
		entries = append(entries, EvidenceEntry{Path: path, EventID: event.EventID, ContentDigest: event.ContentDigest})
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Path < entries[j].Path })
	return EvidenceProof{EvidenceRef: read.ObservedHead.EvidenceRef, Commit: read.ObservedHead.Commit, Entries: entries}, nil
}

func reviewReject(stage string, err error) error {
	return fmt.Errorf("%w: %s: %v", ErrReviewEvidenceRejected, stage, err)
}
