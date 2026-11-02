package runtime

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
)

var (
	ErrFenceDenied = errors.New("runtime fence: authority denied")
	ErrFenceState  = errors.New("runtime fence: invalid state transition")
)

// FenceAuthority owns only generation fencing keys and liveness state. Model
// credentials stay inside each native adapter and are never accepted here.
// Sharing one authority across adapter instances gives concurrent hosts one
// atomic spawn predicate; production deployments may host this object behind
// their runtime service boundary without changing the consumer port.
type FenceAuthority struct {
	mu       sync.Mutex
	secret   [32]byte
	random   io.Reader
	attempts map[string]*attemptFence
}

type attemptFence struct {
	harness                        entity.Harness
	proofReference, proofDigest    string
	generation                     port.OwnerGeneration
	capability                     port.LaunchCapability
	consumed, revoked, startFailed bool
	terminal, absent               bool
	handle                         port.Handle
	lastProof                      port.RuntimeFenceProof
}

var (
	defaultFenceOnce      sync.Once
	defaultFenceAuthority *FenceAuthority
	defaultFenceErr       error
)

func NewFenceAuthority() (*FenceAuthority, error) {
	a := &FenceAuthority{random: rand.Reader, attempts: make(map[string]*attemptFence)}
	if _, err := io.ReadFull(a.random, a.secret[:]); err != nil {
		return nil, fmt.Errorf("runtime fence: create signing key: %w", err)
	}
	return a, nil
}

// DefaultFenceAuthority is the process's runtime-service authority. Every
// registry-resolved adapter shares it, so separately constructed workers do
// not accidentally create separate owner domains. Multi-host deployments pass
// the single authority hosted by their runtime service via WithFenceAuthority.
func DefaultFenceAuthority() (*FenceAuthority, error) {
	defaultFenceOnce.Do(func() { defaultFenceAuthority, defaultFenceErr = NewFenceAuthority() })
	return defaultFenceAuthority, defaultFenceErr
}

// AdapterFence binds one native adapter to a shared fencing authority.
type AdapterFence struct {
	authority *FenceAuthority
	harness   entity.Harness
}

func NewAdapterFence(authority *FenceAuthority, harness entity.Harness) (*AdapterFence, error) {
	if authority == nil || harness == "" {
		return nil, errors.New("runtime fence: authority and harness are required")
	}
	return &AdapterFence{authority: authority, harness: harness}, nil
}

func (f *AdapterFence) Activate(attempt string, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	if f == nil || f.authority == nil {
		return port.LaunchCapability{}, fmt.Errorf("%w: adapter fence is absent", ErrFenceDenied)
	}
	return f.authority.activate(attempt, f.harness, generation, proof)
}

// Launch holds the authority's attempt lock across the native start call. The
// linearization point is therefore the real cmd.Start seam: either revocation
// wins and spawn is never called, or capability consumption plus spawn wins and
// a concurrent fence can observe only running/unknown, never proven absence.
func (f *AdapterFence) Launch(ctx context.Context, capability port.LaunchCapability, spawn func() (port.Handle, error)) (port.Handle, error) {
	if f == nil || f.authority == nil || spawn == nil {
		return nil, fmt.Errorf("%w: launch fence is unavailable", ErrFenceDenied)
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	a := f.authority
	a.mu.Lock()
	defer a.mu.Unlock()
	state, err := a.authorizeCapability(capability, f.harness)
	if err != nil {
		return nil, err
	}
	if state.consumed || state.revoked || state.terminal {
		return nil, fmt.Errorf("%w: capability is consumed, revoked, or terminal", ErrFenceDenied)
	}
	state.consumed, state.lastProof = true, port.RuntimeFenceProof{}
	handle, err := spawn()
	if err != nil {
		state.startFailed = true
		return nil, err
	}
	if handle == nil {
		state.startFailed = true
		return nil, fmt.Errorf("%w: native spawn returned a nil handle", ErrFenceState)
	}
	state.handle = handle
	return handle, nil
}

func (f *AdapterFence) FenceAndObserve(ctx context.Context, attempt string, generation port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	if f == nil || f.authority == nil {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{}, fmt.Errorf("%w: adapter fence is absent", ErrFenceDenied)
	}
	return f.authority.fenceAndObserve(ctx, attempt, f.harness, generation)
}

func (f *AdapterFence) MarkTerminal(handle port.Handle) { f.authority.markTerminal(handle) }

// AuthorizeTerminal fences canonical outcome append with the same exact
// attempt/generation/capability binding as launch. It is intentionally
// repeatable for byte-identical store retries; canonical evidence cardinality
// rejects any conflicting second terminal.
func (f *AdapterFence) AuthorizeTerminal(capability port.LaunchCapability) error {
	f.authority.mu.Lock()
	defer f.authority.mu.Unlock()
	state, err := f.authority.authorizeCapability(capability, f.harness)
	if err != nil {
		return err
	}
	if state.generation != capability.OwnerGeneration {
		return fmt.Errorf("%w: stale terminal generation", ErrFenceDenied)
	}
	return nil
}

func (a *FenceAuthority) activate(attempt string, harness entity.Harness, generation port.OwnerGeneration, proof port.OwnershipProof) (port.LaunchCapability, error) {
	if attempt == "" || harness == "" || generation < 1 || proof.Reference == "" || proof.Digest == "" ||
		proof.AttemptID != attempt || proof.Harness != harness || proof.OwnerGeneration != generation {
		return port.LaunchCapability{}, fmt.Errorf("%w: activation proof binding differs", ErrFenceDenied)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	prior := a.attempts[attempt]
	if prior == nil {
		if generation != 1 || proof.Kind != port.OwnershipProofResolution {
			return port.LaunchCapability{}, fmt.Errorf("%w: first generation requires routing_resolved", ErrFenceState)
		}
	} else {
		if generation != prior.generation+1 || proof.Kind != port.OwnershipProofTransfer || !prior.revoked {
			return port.LaunchCapability{}, fmt.Errorf("%w: generation does not follow a revoked owner", ErrFenceState)
		}
		if err := a.verifyFenceProof(proof.Fence); err != nil || proof.Fence.AttemptID != attempt ||
			proof.Fence.Harness != harness || proof.Fence.OwnerGeneration != prior.generation ||
			proof.Fence.Nonce != prior.lastProof.Nonce || proof.Fence.ProofReference != prior.lastProof.ProofReference ||
			!transferable(proof.Fence.Observation) {
			return port.LaunchCapability{}, fmt.Errorf("%w: transfer lacks the exact transferable fence proof", ErrFenceDenied)
		}
	}
	nonce, err := a.nonce()
	if err != nil {
		return port.LaunchCapability{}, err
	}
	capability := port.LaunchCapability{AttemptID: attempt, Harness: harness, OwnerGeneration: generation,
		ProofReference: proof.Reference, ProofDigest: proof.Digest, Nonce: nonce}
	capability.Signature = a.sign(capabilityMessage(capability))
	digest := sha256.Sum256(append(capabilityMessage(capability), capability.Signature...))
	capability.CapabilityRef = hex.EncodeToString(digest[:])
	a.attempts[attempt] = &attemptFence{harness: harness, generation: generation, proofReference: proof.Reference,
		proofDigest: proof.Digest, capability: capability, terminal: prior != nil && prior.terminal}
	return capability, nil
}

func (a *FenceAuthority) authorizeCapability(capability port.LaunchCapability, harness entity.Harness) (*attemptFence, error) {
	if capability.AttemptID == "" || capability.Harness != harness || capability.OwnerGeneration < 1 ||
		capability.ProofReference == "" || capability.ProofDigest == "" || capability.Nonce == "" || capability.CapabilityRef == "" || capability.Signature == "" ||
		!a.verify(capabilityMessage(capability), capability.Signature) {
		return nil, fmt.Errorf("%w: capability signature or binding is invalid", ErrFenceDenied)
	}
	digest := sha256.Sum256(append(capabilityMessage(capability), capability.Signature...))
	if capability.CapabilityRef != hex.EncodeToString(digest[:]) {
		return nil, fmt.Errorf("%w: capability reference is invalid", ErrFenceDenied)
	}
	state := a.attempts[capability.AttemptID]
	if state == nil || state.harness != harness || state.generation != capability.OwnerGeneration ||
		state.proofReference != capability.ProofReference || state.proofDigest != capability.ProofDigest ||
		state.capability.Nonce != capability.Nonce || state.capability.Signature != capability.Signature {
		return nil, fmt.Errorf("%w: capability is stale or unknown", ErrFenceDenied)
	}
	return state, nil
}

func (a *FenceAuthority) fenceAndObserve(ctx context.Context, attempt string, harness entity.Harness, generation port.OwnerGeneration) (port.RuntimeFenceProof, port.RuntimeObservation, error) {
	if err := ctx.Err(); err != nil {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{}, err
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	state := a.attempts[attempt]
	if state == nil || state.harness != harness || state.generation != generation {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{}, fmt.Errorf("%w: attempt generation is not current", ErrFenceDenied)
	}
	state.revoked = true
	kind := port.RuntimeUnknown
	switch {
	case state.terminal:
		kind = port.RuntimeTerminal
	case state.absent:
		kind = port.RuntimeAbsentProven
	case !state.consumed || state.startFailed:
		kind = port.RuntimeNotStarted
	case state.handle != nil:
		kind = port.RuntimeRunning
	}
	observation := port.RuntimeObservation{AttemptID: attempt, OwnerGeneration: generation, Kind: kind}
	if kind == port.RuntimeRunning {
		observation.Handle = state.handle
	}
	if state.lastProof.Nonce != "" && state.lastProof.Observation == kind {
		return state.lastProof, observation, nil
	}
	nonce, err := a.nonce()
	if err != nil {
		return port.RuntimeFenceProof{}, port.RuntimeObservation{}, err
	}
	proof := port.RuntimeFenceProof{AttemptID: attempt, Harness: harness, OwnerGeneration: generation, Observation: kind, Nonce: nonce}
	proof.Signature = a.sign(fenceMessage(proof))
	digest := sha256.Sum256(append(fenceMessage(proof), proof.Signature...))
	proof.ProofReference = hex.EncodeToString(digest[:])
	state.lastProof = proof
	return proof, observation, nil
}

// ProveAbsent is the runtime-service seam used after host loss when its native
// process inventory positively proves absence. An empty attestation never
// changes unknown liveness; the attestation itself remains private to the
// runtime and is deliberately absent from canonical routing evidence.
func (a *FenceAuthority) ProveAbsent(attempt string, generation port.OwnerGeneration, attestation string) error {
	if attestation == "" {
		return fmt.Errorf("%w: absence attestation is empty", ErrFenceDenied)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	state := a.attempts[attempt]
	if state == nil || state.generation != generation || state.terminal || !state.consumed || state.handle != nil {
		return fmt.Errorf("%w: attempt generation is not observable", ErrFenceDenied)
	}
	state.handle, state.absent, state.lastProof = nil, true, port.RuntimeFenceProof{}
	return nil
}

// ObserveHostLoss removes only the local reattach handle. It deliberately
// yields unknown, never absence; recovery must obtain a positive native
// attestation before calling ProveAbsent.
func (a *FenceAuthority) ObserveHostLoss(attempt string, generation port.OwnerGeneration) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	state := a.attempts[attempt]
	if state == nil || state.generation != generation || !state.consumed || state.terminal {
		return fmt.Errorf("%w: attempt generation is not running", ErrFenceDenied)
	}
	state.handle, state.lastProof = nil, port.RuntimeFenceProof{}
	return nil
}

// ReapUnknown records an operator-authorized blocked terminal for an ambiguous
// host-loss observation. It never proves absence and therefore never makes a
// replacement launch possible; a later owner may only retry terminal append.
func (a *FenceAuthority) ReapUnknown(attempt string, generation port.OwnerGeneration, authorization string) error {
	if authorization == "" {
		return fmt.Errorf("%w: operator reap authorization is empty", ErrFenceDenied)
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	state := a.attempts[attempt]
	if state == nil || state.generation != generation || !state.consumed || state.handle != nil || state.absent || state.terminal {
		return fmt.Errorf("%w: attempt is not in unknown liveness", ErrFenceDenied)
	}
	state.terminal, state.lastProof = true, port.RuntimeFenceProof{}
	return nil
}

func (a *FenceAuthority) markTerminal(handle port.Handle) {
	if a == nil || handle == nil {
		return
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	for _, state := range a.attempts {
		if state.handle == handle {
			state.terminal, state.handle, state.lastProof = true, nil, port.RuntimeFenceProof{}
			return
		}
	}
}

func transferable(kind port.RuntimeObservationKind) bool {
	return kind == port.RuntimeNotStarted || kind == port.RuntimeTerminal || kind == port.RuntimeAbsentProven
}

func (a *FenceAuthority) nonce() (string, error) {
	var raw [24]byte
	if _, err := io.ReadFull(a.random, raw[:]); err != nil {
		return "", fmt.Errorf("runtime fence: create nonce: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw[:]), nil
}

func (a *FenceAuthority) sign(message []byte) string {
	mac := hmac.New(sha256.New, a.secret[:])
	_, _ = mac.Write(message)
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *FenceAuthority) verify(message []byte, signature string) bool {
	want, err := base64.RawURLEncoding.DecodeString(signature)
	if err != nil {
		return false
	}
	mac := hmac.New(sha256.New, a.secret[:])
	_, _ = mac.Write(message)
	return hmac.Equal(want, mac.Sum(nil))
}

func (a *FenceAuthority) verifyFenceProof(proof port.RuntimeFenceProof) error {
	if proof.AttemptID == "" || proof.Harness == "" || proof.OwnerGeneration < 1 || proof.Nonce == "" || proof.Signature == "" ||
		!a.verify(fenceMessage(proof), proof.Signature) {
		return fmt.Errorf("%w: fence proof signature is invalid", ErrFenceDenied)
	}
	digest := sha256.Sum256(append(fenceMessage(proof), proof.Signature...))
	if proof.ProofReference != hex.EncodeToString(digest[:]) {
		return fmt.Errorf("%w: fence proof reference is invalid", ErrFenceDenied)
	}
	return nil
}

func capabilityMessage(capability port.LaunchCapability) []byte {
	copy := capability
	copy.Signature, copy.CapabilityRef = "", ""
	b, _ := json.Marshal(copy)
	return b
}

func fenceMessage(proof port.RuntimeFenceProof) []byte {
	copy := proof
	copy.Signature, copy.ProofReference = "", ""
	b, _ := json.Marshal(copy)
	return b
}
