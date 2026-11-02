package routingevidence

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/entity"
	"reflect"
	"sort"
	"sync"
)

const (
	SubjectViewsSchema = "routing-evidence-subject-views/v1"
	ProjectionSchema   = "routing-evidence-projection/v1"
	viewDigestDomain   = "routing-evidence-view-digest/v1"
	proofDigestDomain  = "routing-evidence-proof-digest/v1"
	projectionDomain   = "routing-evidence-projection-digest/v1"
)

// Reader always performs a current exact read before consulting its disposable
// cache. A cache can avoid reduction and rendering work, never the authority read.
type Reader interface {
	ReadViews(context.Context, SubjectID) (SubjectViews, error)
}

type ViewCache interface {
	Load(CacheKey) (SubjectViews, bool)
	Store(CacheKey, SubjectViews)
}

type CacheKey struct {
	EvidenceRef  string
	EvidenceHead string
	Subject      SubjectID
}

type MemoryViewCache struct {
	mu    sync.RWMutex
	views map[CacheKey]SubjectViews
}

func NewMemoryViewCache() *MemoryViewCache {
	return &MemoryViewCache{views: map[CacheKey]SubjectViews{}}
}

func (c *MemoryViewCache) Load(key CacheKey) (SubjectViews, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	view, ok := c.views[key]
	return cloneSubjectViews(view), ok
}

func (c *MemoryViewCache) Store(key CacheKey, view SubjectViews) {
	view.currentExactRead = false
	c.mu.Lock()
	defer c.mu.Unlock()
	c.views[key] = cloneSubjectViews(view)
}

type subjectReader struct {
	store Store
	cache ViewCache
}

func NewReader(store Store, cache ViewCache) (Reader, error) {
	if store == nil {
		return nil, fmt.Errorf("routing evidence reader: store is required")
	}
	return &subjectReader{store: store, cache: cache}, nil
}

// EventView is the bounded, secret-free identity face used by operators. It
// deliberately omits finding text, disposition reasons, signatures, and payloads.
type EventView struct {
	EventID            EventID        `json:"event_id"`
	Kind               EventKind      `json:"kind"`
	ContentDigest      string         `json:"content_digest"`
	ProducerClass      string         `json:"producer_class"`
	ProducerGeneration string         `json:"producer_generation"`
	ProofKeyID         string         `json:"proof_key_id"`
	TaskID             string         `json:"task_id,omitempty"`
	ProposalID         string         `json:"proposal_id,omitempty"`
	DemandID           string         `json:"demand_id,omitempty"`
	AttemptID          string         `json:"attempt_id,omitempty"`
	ProcessID          string         `json:"process_id,omitempty"`
	Harness            entity.Harness `json:"harness,omitempty"`
	Role               string         `json:"role,omitempty"`
	PolicyVersion      string         `json:"policy_version,omitempty"`
	SemanticComponents []string       `json:"semantic_components"`
}

type MeasurementView struct {
	Harness         entity.Harness `json:"harness"`
	Role            string         `json:"role"`
	PolicyVersion   string         `json:"policy_version"`
	NativeProfileID string         `json:"native_profile_id,omitempty"`
	Attempts        int            `json:"attempts"`
	Exact           int            `json:"exact"`
	Promoted        int            `json:"promoted"`
	Unsupported     int            `json:"unsupported"`
	Verified        int            `json:"verified"`
	Failed          int            `json:"failed"`
	Blocked         int            `json:"blocked"`
	Usage           Usage          `json:"usage"`
}

type SubjectViews struct {
	Schema       string            `json:"schema"`
	EvidenceHead ObservedHead      `json:"evidence_head"`
	Proof        EvidenceProof     `json:"proof"`
	Subject      SubjectView       `json:"subject"`
	State        StructuralState   `json:"state"`
	Events       []EventView       `json:"events"`
	Measurements []MeasurementView `json:"measurements"`
	Acceptance   Acceptance        `json:"acceptance"`
	Digest       string            `json:"digest"`

	currentExactRead bool
}

// View is the concise projection input name used by the design.
type View = SubjectViews

func (r *subjectReader) ReadViews(ctx context.Context, subject SubjectID) (SubjectViews, error) {
	exact, err := r.store.ReadSubject(ctx, subject)
	if err != nil {
		return SubjectViews{}, err
	}
	if err := validateExactHead(exact); err != nil {
		return SubjectViews{}, err
	}
	key := CacheKey{EvidenceRef: exact.ObservedHead.EvidenceRef, EvidenceHead: exact.ObservedHead.Commit, Subject: subject}
	if r.cache != nil {
		if cached, ok := r.cache.Load(key); ok && validateViews(cached, false) == nil && cached.Subject.Subject == subject && reflect.DeepEqual(cached.Proof, exact.Proof) {
			cached.currentExactRead = true
			return cached, nil
		}
	}
	out, err := viewsFromExact(exact, subject)
	if err != nil {
		return SubjectViews{}, err
	}
	if r.cache != nil {
		r.cache.Store(key, out)
	}
	return out, nil
}

func viewsFromExact(exact ExactRead, subject SubjectID) (SubjectViews, error) {
	if err := validateExactHead(exact); err != nil {
		return SubjectViews{}, err
	}
	state, err := reduceCanonical(exact.Events)
	if err != nil {
		return SubjectViews{}, err
	}
	state.EvidenceHead = exact.ObservedHead.Commit
	view, err := state.SubjectView(subject)
	if err != nil {
		return SubjectViews{}, err
	}
	out := SubjectViews{Schema: SubjectViewsSchema, EvidenceHead: exact.ObservedHead, Proof: exact.Proof,
		Subject: view, State: state, Events: eventViews(exact.Events), Measurements: measurementViews(exact.Events),
		Acceptance: CurrentAcceptance(view), currentExactRead: true}
	out.Digest, err = digestValue(viewDigestDomain, out)
	if err != nil {
		return SubjectViews{}, err
	}
	return out, nil
}

func validateExactHead(exact ExactRead) error {
	if exact.ObservedHead.EvidenceRef == "" || exact.ObservedHead.Commit == "" || exact.Proof.EvidenceRef != exact.ObservedHead.EvidenceRef || exact.Proof.Commit != exact.ObservedHead.Commit {
		return NewStoreError(StoreProofInvalid, "subject view requires a proof for the exact fetched head")
	}
	if len(exact.Events) == 0 || len(exact.Events) != len(exact.Proof.Entries) || len(exact.Bytes) != len(exact.Proof.Entries) {
		return NewStoreError(StoreProofInvalid, "exact event, byte, and proof sets differ")
	}
	byID := make(map[EventID]Event, len(exact.Events))
	for _, event := range exact.Events {
		wire, err := EncodeCanonical(event)
		if err != nil {
			return err
		}
		byID[event.EventID] = event
		path, _ := EventPath(event.EventID)
		if !bytes.Equal(wire, exact.Bytes[path]) {
			return NewStoreError(StoreProofInvalid, "event %s does not match exact-read bytes", event.EventID)
		}
	}
	seen := make(map[EventID]bool, len(exact.Proof.Entries))
	prior := ""
	for _, entry := range exact.Proof.Entries {
		event, ok := byID[entry.EventID]
		path, err := EventPath(entry.EventID)
		if !ok || seen[entry.EventID] || err != nil || path != entry.Path || event.ContentDigest != entry.ContentDigest || entry.Path <= prior {
			return NewStoreError(StoreProofInvalid, "proof entry %s is disconnected from the exact event set", entry.EventID)
		}
		seen[entry.EventID], prior = true, entry.Path
	}
	if len(seen) != len(byID) {
		return NewStoreError(StoreProofInvalid, "proof does not cover the exact event set")
	}
	return nil
}

type Medium string

const (
	MediumProposal        Medium = "proposal"
	MediumReport          Medium = "report"
	MediumTaskTracker     Medium = "task_tracker"
	MediumProposalTracker Medium = "proposal_tracker"
	MediumGitHubTracker   Medium = "github_tracker"
	MediumCarinaTracker   Medium = "carina_tracker"
	MediumAudit           Medium = "audit"
	MediumMeasurement     Medium = "measurement"
)

type ProjectionDigest string

type Projection struct {
	Schema       string           `json:"schema"`
	Medium       Medium           `json:"medium"`
	Subject      SubjectID        `json:"subject"`
	EvidenceRef  string           `json:"evidence_ref"`
	EvidenceHead string           `json:"evidence_head"`
	EventIDs     []EventID        `json:"event_ids"`
	Proof        EvidenceProof    `json:"proof"`
	ProofDigest  string           `json:"proof_digest"`
	ViewDigest   string           `json:"view_digest"`
	Accepting    bool             `json:"accepting"`
	Digest       ProjectionDigest `json:"digest"`
}

func Project(view SubjectViews, medium Medium) (Projection, error) {
	if !supportedMedium(medium) {
		return Projection{}, fmt.Errorf("routing evidence projection: unsupported medium %q", medium)
	}
	if err := validateViews(view, true); err != nil {
		return Projection{}, err
	}
	ids := make([]EventID, len(view.Proof.Entries))
	for i, entry := range view.Proof.Entries {
		ids[i] = entry.EventID
	}
	proofDigest, err := digestValue(proofDigestDomain, view.Proof)
	if err != nil {
		return Projection{}, err
	}
	p := Projection{Schema: ProjectionSchema, Medium: medium, Subject: view.Subject.Subject,
		EvidenceRef: view.EvidenceHead.EvidenceRef, EvidenceHead: view.EvidenceHead.Commit, EventIDs: ids,
		Proof: view.Proof, ProofDigest: proofDigest, ViewDigest: view.Digest, Accepting: acceptingProjection(view.Acceptance)}
	digest, err := digestValue(projectionDomain, p)
	p.Digest = ProjectionDigest(digest)
	return p, err
}

type ProjectionErrorCode string

const (
	ProjectionMissing      ProjectionErrorCode = "missing"
	ProjectionExtra        ProjectionErrorCode = "extra"
	ProjectionStale        ProjectionErrorCode = "stale"
	ProjectionChanged      ProjectionErrorCode = "changed"
	ProjectionDisconnected ProjectionErrorCode = "disconnected"
	ProjectionNonAccepting ProjectionErrorCode = "non_accepting"
	ProjectionInvalidated  ProjectionErrorCode = "invalidated"
)

var ErrProjectionRejected = errors.New("routing evidence projection rejected")

type ProjectionError struct {
	Code    ProjectionErrorCode `json:"code"`
	Field   string              `json:"field"`
	Missing []EventID           `json:"missing"`
	Extra   []EventID           `json:"extra"`
}

func (e *ProjectionError) Error() string {
	return fmt.Sprintf("%s: %s (%s)", ErrProjectionRejected, e.Code, e.Field)
}
func (e *ProjectionError) Unwrap() error { return ErrProjectionRejected }

func VerifyProjection(got Projection, view SubjectViews) error {
	if err := validateViews(view, true); err != nil {
		return &ProjectionError{Code: ProjectionDisconnected, Field: "current_exact_read"}
	}
	if got.EvidenceRef != view.EvidenceHead.EvidenceRef || got.EvidenceHead != view.EvidenceHead.Commit {
		return &ProjectionError{Code: ProjectionStale, Field: "evidence_head"}
	}
	if got.Schema != ProjectionSchema || !supportedMedium(got.Medium) || got.Subject != view.Subject.Subject {
		return &ProjectionError{Code: ProjectionDisconnected, Field: "projection_identity"}
	}
	want, err := Project(view, got.Medium)
	if err != nil {
		return err
	}
	missing, extra := eventDiff(want.EventIDs, got.EventIDs)
	if len(missing) > 0 {
		return &ProjectionError{Code: ProjectionMissing, Field: "event_ids", Missing: missing, Extra: extra}
	}
	if len(extra) > 0 {
		return &ProjectionError{Code: ProjectionExtra, Field: "event_ids", Extra: extra}
	}
	if !reflect.DeepEqual(got.Proof, want.Proof) || got.ProofDigest != want.ProofDigest {
		return &ProjectionError{Code: ProjectionDisconnected, Field: "proof"}
	}
	if got.ViewDigest != want.ViewDigest || got.Accepting != want.Accepting || got.Digest != want.Digest {
		return &ProjectionError{Code: ProjectionChanged, Field: "projection_digest"}
	}
	if len(view.Subject.Invalidations) > 0 {
		return &ProjectionError{Code: ProjectionInvalidated, Field: "invalidation"}
	}
	if !want.Accepting {
		return &ProjectionError{Code: ProjectionNonAccepting, Field: "acceptance"}
	}
	return nil
}

func EncodeProjection(projection Projection) ([]byte, error) {
	if err := validateProjectionShape(projection); err != nil {
		return nil, err
	}
	return canonicalJSON(projection)
}

func DecodeProjection(data []byte) (Projection, error) {
	var projection Projection
	if err := decodeClosed(data, &projection); err != nil {
		return Projection{}, fmt.Errorf("routing evidence projection: %w", err)
	}
	wire, err := canonicalJSON(projection)
	if err != nil || !bytes.Equal(data, wire) {
		return Projection{}, fmt.Errorf("routing evidence projection: non-canonical input")
	}
	if err := validateProjectionShape(projection); err != nil {
		return Projection{}, err
	}
	return projection, nil
}

func validateProjectionShape(p Projection) error {
	if p.Schema != ProjectionSchema || !supportedMedium(p.Medium) || p.Subject.ID == "" || p.EvidenceRef == "" || p.EvidenceHead == "" || len(p.EventIDs) == 0 || len(p.EventIDs) != len(p.Proof.Entries) {
		return fmt.Errorf("routing evidence projection: incomplete identity or event set")
	}
	for i, id := range p.EventIDs {
		entry := p.Proof.Entries[i]
		path, err := EventPath(id)
		if err != nil || entry.EventID != id || entry.Path != path || entry.ContentDigest == "" || (i > 0 && p.EventIDs[i-1] >= id) {
			return fmt.Errorf("routing evidence projection: disconnected or unordered proof")
		}
	}
	if p.Proof.EvidenceRef != p.EvidenceRef || p.Proof.Commit != p.EvidenceHead || p.ProofDigest == "" || p.ViewDigest == "" || p.Digest == "" {
		return fmt.Errorf("routing evidence projection: proof or digest is incomplete")
	}
	want, err := digestValue(projectionDomain, Projection{Schema: p.Schema, Medium: p.Medium, Subject: p.Subject, EvidenceRef: p.EvidenceRef,
		EvidenceHead: p.EvidenceHead, EventIDs: p.EventIDs, Proof: p.Proof, ProofDigest: p.ProofDigest, ViewDigest: p.ViewDigest, Accepting: p.Accepting})
	if err != nil || string(p.Digest) != want {
		return fmt.Errorf("routing evidence projection: self digest differs")
	}
	return nil
}

func validateViews(view SubjectViews, requireCurrent bool) error {
	if view.Schema != SubjectViewsSchema || view.Subject.Subject.ID == "" || view.EvidenceHead.EvidenceRef == "" || view.EvidenceHead.Commit == "" || view.Proof.EvidenceRef != view.EvidenceHead.EvidenceRef || view.Proof.Commit != view.EvidenceHead.Commit {
		return fmt.Errorf("routing evidence views: incomplete exact-head identity")
	}
	if requireCurrent && !view.currentExactRead {
		return fmt.Errorf("routing evidence views: a current exact read is required")
	}
	want, err := digestValue(viewDigestDomain, SubjectViews{Schema: view.Schema, EvidenceHead: view.EvidenceHead, Proof: view.Proof, Subject: view.Subject,
		State: view.State, Events: view.Events, Measurements: view.Measurements, Acceptance: view.Acceptance})
	if err != nil || view.Digest != want {
		return fmt.Errorf("routing evidence views: digest differs")
	}
	return nil
}

func digestValue(domain string, value any) (string, error) {
	wire, err := canonicalJSON(value)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(append([]byte(domain+"\x00"), wire...))
	return hex.EncodeToString(sum[:]), nil
}

func acceptingProjection(a Acceptance) bool {
	return a.ReviewSatisfied && a.ProjectionEligible || a.ActivationEligible
}

func supportedMedium(m Medium) bool {
	switch m {
	case MediumProposal, MediumReport, MediumTaskTracker, MediumProposalTracker, MediumGitHubTracker, MediumCarinaTracker, MediumAudit, MediumMeasurement:
		return true
	default:
		return false
	}
}

func eventDiff(want, got []EventID) (missing, extra []EventID) {
	w, g := map[EventID]bool{}, map[EventID]bool{}
	for _, id := range want {
		w[id] = true
	}
	for _, id := range got {
		g[id] = true
	}
	for id := range w {
		if !g[id] {
			missing = append(missing, id)
		}
	}
	for id := range g {
		if !w[id] {
			extra = append(extra, id)
		}
	}
	sortEventIDs(missing)
	sortEventIDs(extra)
	return missing, extra
}

func cloneSubjectViews(view SubjectViews) SubjectViews {
	wire, err := json.Marshal(view)
	if err != nil {
		return SubjectViews{}
	}
	var out SubjectViews
	if json.Unmarshal(wire, &out) != nil {
		return SubjectViews{}
	}
	return out
}

func eventViews(events []Event) []EventView {
	out := make([]EventView, 0, len(events))
	for _, event := range events {
		v := EventView{EventID: event.EventID, Kind: event.Kind, ContentDigest: event.ContentDigest, ProducerClass: event.Producer.Class,
			ProducerGeneration: event.Producer.Generation, ProofKeyID: event.ProducerProof.KeyID, SemanticComponents: []string{}}
		fillEventView(&v, event)
		sort.Strings(v.SemanticComponents)
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].EventID < out[j].EventID })
	return out
}

func fillEventView(v *EventView, event Event) {
	switch event.Kind {
	case ExecutionDemandFrozen:
		p := event.Payload.ExecutionDemandFrozen
		v.TaskID, v.DemandID, v.PolicyVersion = p.TaskID, p.DemandID, p.PolicyVersion

	case RoutingResolved:
		p := event.Payload.RoutingResolved
		v.TaskID, v.ProposalID, v.DemandID, v.AttemptID, v.Harness, v.Role, v.PolicyVersion = p.TaskID, p.ProposalID, p.DemandID, p.AttemptID, p.Harness, p.Role, p.PolicyVersion
		v.SemanticComponents = append(profileComponents(p.RequestedProfile), p.PromotedComponents...)
	case RoutingResolutionFailed:
		p := event.Payload.RoutingResolutionFailed
		v.TaskID, v.ProposalID, v.DemandID, v.AttemptID, v.Harness, v.Role, v.PolicyVersion = p.TaskID, p.ProposalID, p.DemandID, p.AttemptID, p.Harness, p.Role, p.PolicyVersion
		v.SemanticComponents = append(profileComponents(p.RequestedProfile), p.UnmetComponents...)
	case RoutedAttemptCompleted:
		p := event.Payload.RoutedAttemptCompleted
		v.TaskID, v.ProposalID, v.DemandID, v.AttemptID = p.TaskID, p.ProposalID, p.DemandID, p.AttemptID
	case RoutedTaskOutcome:
		p := event.Payload.RoutedTaskOutcome
		v.TaskID, v.ProposalID, v.DemandID, v.AttemptID = p.TaskID, p.ProposalRef, p.DemandID, p.AttemptID
	case AttemptOwnerTransferred:
		v.AttemptID = event.Payload.AttemptOwnerTransferred.AttemptID
	case AssurancePassCompleted:
		p := event.Payload.AssurancePassCompleted
		v.TaskID, v.ProposalID, v.DemandID, v.AttemptID, v.ProcessID = p.TaskID, p.ProposalID, p.DemandID, p.AttemptID, p.ReviewProcessID
	case FindingPersisted:
		p := event.Payload.FindingPersisted
		v.ProposalID, v.ProcessID = p.ProposalID, p.ReviewProcessID
	case ReviewProcessCompleted:
		p := event.Payload.ReviewProcessCompleted
		v.TaskID, v.ProposalID, v.DemandID, v.ProcessID = p.TaskID, p.ProposalID, p.DemandID, p.ReviewProcessID
	}
}

func profileComponents(profile Profile) []string {
	return []string{"assurance=" + profile.Assurance, "capability=" + profile.Capability, "context=" + profile.Context, "deliberation=" + profile.Deliberation}
}

func measurementViews(events []Event) []MeasurementView {
	byAttempt, outcome := map[string]*MeasurementView{}, map[string]*RoutedAttemptCompletedPayload{}
	for _, event := range events {
		if event.Kind == RoutedAttemptCompleted {
			outcome[event.Payload.RoutedAttemptCompleted.AttemptID] = event.Payload.RoutedAttemptCompleted
		}
	}
	for _, event := range events {
		var attemptID, resolution string
		var bucket MeasurementView
		switch event.Kind {
		case RoutingResolved:
			p := event.Payload.RoutingResolved
			attemptID, resolution = p.AttemptID, p.Resolution
			bucket = MeasurementView{Harness: p.Harness, Role: p.Role, PolicyVersion: p.PolicyVersion, NativeProfileID: p.NativeProfileID}
		case RoutingResolutionFailed:
			p := event.Payload.RoutingResolutionFailed
			attemptID, resolution = p.AttemptID, "unsupported"
			bucket = MeasurementView{Harness: p.Harness, Role: p.Role, PolicyVersion: p.PolicyVersion}
		default:
			continue
		}
		key := bucket.Harness.String() + "\x00" + string(bucket.Role) + "\x00" + bucket.PolicyVersion + "\x00" + bucket.NativeProfileID
		if byAttempt[key] == nil {
			copy := bucket
			byAttempt[key] = &copy
		}
		m := byAttempt[key]
		m.Attempts++
		switch resolution {
		case "exact":
			m.Exact++
		case "promoted":
			m.Promoted++
		default:
			m.Unsupported++
		}
		if terminal := outcome[attemptID]; terminal != nil {
			switch terminal.Outcome {
			case "verified":
				m.Verified++
			case "failed":
				m.Failed++
			case "blocked":
				m.Blocked++
			}
			m.Usage.CacheCreationTokens += terminal.Usage.CacheCreationTokens
			m.Usage.CacheReadTokens += terminal.Usage.CacheReadTokens
			m.Usage.InputTokens += terminal.Usage.InputTokens
			m.Usage.OutputTokens += terminal.Usage.OutputTokens
		}
	}
	keys := make([]string, 0, len(byAttempt))
	for key := range byAttempt {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]MeasurementView, 0, len(keys))
	for _, key := range keys {
		out = append(out, *byAttempt[key])
	}
	return out
}
