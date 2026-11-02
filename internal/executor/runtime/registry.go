package runtime

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/agenticlab-ai/pyxis/internal/entity"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/nativerouting"
)

// ErrUnknownHarness is returned (wrapped) by Resolve when no harness has
// registered under the requested name. It is part of the contract and is
// matched with errors.Is.
var ErrUnknownHarness = errors.New("runtime: unknown harness")

// Factory builds one ready-to-use AgentRuntime. Each harness package
// registers its Factory from init (see Register); construction is where an
// adapter wires its production exec and defaults. A Factory may fail (returning
// a non-nil error) when the harness cannot be built, so the resolver degrades
// loudly rather than handing back a half-constructed runtime.
type Factory func() (port.AgentRuntime, error)

// ConfiguredFactory builds one runtime exclusively from the supplied native
// profile catalog. It must not discover a repository catalog on its own.
type ConfiguredFactory func(map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error)

type registration struct {
	factory           Factory
	configuredFactory ConfiguredFactory
}

// registry is a name → registration table. It is the single shared append surface for
// harnesses, but nothing is authored into it: each harness adds itself from its
// own init, so the collection is derived from the set of imported packages, not
// from a hand-kept switch (AP-27). The mutex keeps registration and resolution
// race-clean for the detector even though init runs single-threaded.
type registry struct {
	mu            sync.Mutex
	registrations map[entity.Harness]registration
}

func newRegistry() *registry {
	return &registry{registrations: make(map[entity.Harness]registration)}
}

// register adds factory under name. A duplicate name, an empty name, or a nil
// factory is a programming error (two packages claimed one harness, or a
// harness registered nothing) and panics — this runs at init, so the panic
// surfaces at program start, never mid-run.
func (r *registry) register(name entity.Harness, factory Factory, configuredFactories ...ConfiguredFactory) {
	if name == "" {
		panic("runtime: Register with an empty harness name")
	}
	if factory == nil {
		panic(fmt.Sprintf("runtime: Register(%q) with a nil factory", name))
	}
	if len(configuredFactories) > 1 {
		panic(fmt.Sprintf("runtime: Register(%q) with multiple configured factories", name))
	}
	var configuredFactory ConfiguredFactory
	if len(configuredFactories) == 1 {
		configuredFactory = configuredFactories[0]
		if configuredFactory == nil {
			panic(fmt.Sprintf("runtime: Register(%q) with a nil configured factory", name))
		}
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, dup := r.registrations[name]; dup {
		panic(fmt.Sprintf("runtime: harness %q registered twice", name))
	}
	r.registrations[name] = registration{factory: factory, configuredFactory: configuredFactory}
}

// resolve builds a fresh harness runtime for name. Adapters are cheap, stateless
// config holders (a fresh process is spawned per task, not per adapter — DINV-11),
// so a new instance per resolve is intentional and safe.
func (r *registry) resolve(name entity.Harness) (port.AgentRuntime, error) {
	r.mu.Lock()
	entry, ok := r.registrations[name]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%w %q (available: %s)", ErrUnknownHarness, name, joinHarnesses(r.names()))
	}
	rt, err := entry.factory()
	if err != nil {
		return nil, fmt.Errorf("runtime: build harness %q: %w", name, err)
	}
	if rt == nil {
		return nil, fmt.Errorf("runtime: harness %q factory returned a nil runtime", name)
	}
	return rt, nil
}

func (r *registry) resolveConfigured(name entity.Harness, profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
	r.mu.Lock()
	entry, ok := r.registrations[name]
	r.mu.Unlock()
	if !ok {
		return nil, fmt.Errorf("%w %q (available: %s)", ErrUnknownHarness, name, joinHarnesses(r.names()))
	}
	if !profileCatalogPresent(profiles) {
		return nil, fmt.Errorf("runtime: build configured harness %q: explicit profile catalog is empty", name)
	}
	if entry.configuredFactory == nil {
		return nil, fmt.Errorf("runtime: harness %q does not support configured resolution", name)
	}
	rt, err := entry.configuredFactory(profiles)
	if err != nil {
		return nil, fmt.Errorf("runtime: build configured harness %q: %w", name, err)
	}
	if rt == nil {
		return nil, fmt.Errorf("runtime: configured harness %q factory returned a nil runtime", name)
	}
	return rt, nil
}

func profileCatalogPresent(profiles map[nativerouting.Role][]nativerouting.ProfileSpec) bool {
	for _, specs := range profiles {
		if len(specs) != 0 {
			return true
		}
	}
	return false
}

func (r *registry) names() []entity.Harness {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]entity.Harness, 0, len(r.registrations))
	for name := range r.registrations {
		out = append(out, name)
	}
	slices.Sort(out)
	return out
}

func joinHarnesses(names []entity.Harness) string {
	out := make([]string, len(names))
	for i, name := range names {
		out[i] = name.String()
	}
	return strings.Join(out, ", ")
}

// globalRegistry backs the package-level Register/Resolve/Harnesses. Harnesses
// register into it from their own init; the executor resolves the configured
// harness name out of it at dispatch.
var globalRegistry = newRegistry()

// Registry is a read-only view of the live runtime registry. It lets
// conformance derive the same harness set used by dispatch without exposing a
// second registration surface or a hand-maintained backend list.
type Registry struct{ registry *registry }

// LiveRegistry returns the process's registered runtime domain.
func LiveRegistry() Registry { return Registry{registry: globalRegistry} }

// Harnesses lists the live registered harnesses in canonical order.
func (r Registry) Harnesses() []entity.Harness {
	if r.registry == nil {
		return []entity.Harness{}
	}
	return r.registry.names()
}

// Resolve builds the registered harness owner through the same path as normal
// dispatch.
func (r Registry) Resolve(name entity.Harness) (port.AgentRuntime, error) {
	if r.registry == nil {
		return nil, fmt.Errorf("%w %q (registry is empty)", ErrUnknownHarness, name)
	}
	return r.registry.resolve(name)
}

// ResolveConfigured builds a registered harness from an explicit native
// profile catalog without repository discovery.
func (r Registry) ResolveConfigured(name entity.Harness, profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
	if r.registry == nil {
		return nil, fmt.Errorf("%w %q (registry is empty)", ErrUnknownHarness, name)
	}
	return r.registry.resolveConfigured(name, profiles)
}

// Register admits a harness under name. Harness packages call it from their own
// init, so the runtime set is discovered from the imported packages — a new
// harness is a new package with a blank import, never an edit to a shared switch
// (AP-27). The optional configured factory belongs to the same registration;
// omitting it preserves the legacy-only behavior. Invalid registrations panic.
func Register(name entity.Harness, factory Factory, configuredFactory ...ConfiguredFactory) {
	globalRegistry.register(name, factory, configuredFactory...)
}

// Resolve builds the harness registered under name, or wraps ErrUnknownHarness
// when none is. The error names the available harnesses so a config typo is
// obvious.
func Resolve(name entity.Harness) (port.AgentRuntime, error) { return globalRegistry.resolve(name) }

// ResolveConfigured builds a harness exclusively from the supplied catalog.
func ResolveConfigured(name entity.Harness, profiles map[nativerouting.Role][]nativerouting.ProfileSpec) (port.AgentRuntime, error) {
	return globalRegistry.resolveConfigured(name, profiles)
}

// Harnesses lists every registered harness name, sorted, for diagnostics and
// config validation.
func Harnesses() []entity.Harness { return globalRegistry.names() }
