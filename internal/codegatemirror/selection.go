package codegatemirror

import (
	"fmt"
	"slices"
	"strings"
)

// DefaultGroup keeps configurations without grouping sequential. New checks
// also join this group unless their shared definition assigns another group.
const DefaultGroup = "default"

// DefaultPhase is the first barrier. Checks without a phase join it.
const DefaultPhase = 1

// ExecutionGroup resolves a check's optional group name.
func (c Check) ExecutionGroup() string {
	if c.Group == "" {
		return DefaultGroup
	}
	return c.Group
}

// ExecutionPhase resolves a check's optional phase. Phase 0 (unset) is phase 1.
func (c Check) ExecutionPhase() int {
	if c.Phase < 1 {
		return DefaultPhase
	}
	return c.Phase
}

// Phases lists distinct phase numbers in ascending order.
func (c Config) Phases() []int {
	seen := map[int]bool{}
	var phases []int
	for _, check := range c.Checks {
		phase := check.ExecutionPhase()
		if !seen[phase] {
			phases = append(phases, phase)
			seen[phase] = true
		}
	}
	slices.Sort(phases)
	return phases
}

// SelectCheap keeps only phase-1 checks — the local discovery set that must
// pass before coverage starts. Empty when the config has no phase-1
// checks.
func (c Config) SelectCheap() (Config, error) {
	return c.SelectPhase(DefaultPhase)
}

// SelectPhase keeps checks that belong to one barrier. Hosted later-phase
// jobs combine this with SelectGroup so they do not rerun phase 1.
func (c Config) SelectPhase(phase int) (Config, error) {
	if phase < 1 {
		return Config{}, fmt.Errorf("code-gate: phase must be >= 1")
	}
	var selected Config
	for _, check := range c.Checks {
		if check.ExecutionPhase() == phase {
			selected.Checks = append(selected.Checks, check)
		}
	}
	if len(selected.Checks) == 0 {
		return Config{}, fmt.Errorf("code-gate: no phase-%d checks", phase)
	}
	return selected, nil
}

// SelectGroupPhase keeps one group's checks at one barrier.
func (c Config) SelectGroupPhase(group string, phase int) (Config, error) {
	selected, err := c.SelectGroup(group)
	if err != nil {
		return Config{}, err
	}
	return selected.SelectPhase(phase)
}

// Groups lists the independent execution groups in first-appearance order.
func (c Config) Groups() []string {
	var groups []string
	seen := map[string]bool{}
	for _, check := range c.Checks {
		name := check.ExecutionGroup()
		if !seen[name] {
			groups = append(groups, name)
			seen[name] = true
		}
	}
	return groups
}

// SelectGroup selects one complete shared group in configuration order. Hosted
// jobs use this instead of maintaining their own lists of checks.
func (c Config) SelectGroup(name string) (Config, error) {
	if !validGroupName(name) {
		return Config{}, fmt.Errorf("code-gate: group %q must be a literal group name", name)
	}
	var selected Config
	for _, check := range c.Checks {
		if check.ExecutionGroup() == name {
			selected.Checks = append(selected.Checks, check)
		}
	}
	if len(selected.Checks) == 0 {
		return Config{}, fmt.Errorf("code-gate: unknown group %q", name)
	}
	return selected, nil
}

func validGroupName(name string) bool {
	if name == "" || !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ", rune(name[0])) {
		return false
	}
	return !strings.ContainsFunc(name, func(r rune) bool {
		return !strings.ContainsRune("abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789_-", r)
	})
}

// Select chooses comma-separated check names without changing their commands,
// groups or order within each group. An empty selection is the complete gate.
// Selected verdicts have their own cache fingerprint. Hosted partitions use
// SelectGroup; this selection is for local diagnosis or one hosted diagnostic.
func (c Config) Select(names string) (Config, error) {
	if names == "" {
		return c, nil
	}
	return c.selectChecks(names, false)
}

// SelectExcept chooses the complement of named checks. New configured checks
// automatically belong to that complement; stale or empty exclusions refuse.
func (c Config) SelectExcept(names string) (Config, error) {
	return c.selectChecks(names, true)
}

func (c Config) selectChecks(names string, exclude bool) (Config, error) {
	known := make(map[string]bool, len(c.Checks))
	for _, check := range c.Checks {
		known[check.Name] = true
	}
	wanted := map[string]bool{}
	for name := range strings.SplitSeq(names, ",") {
		if name == "" || strings.TrimSpace(name) != name {
			return Config{}, fmt.Errorf("code-gate: check names must be non-empty and comma-separated without whitespace")
		}
		if !known[name] {
			return Config{}, fmt.Errorf("code-gate: unknown check %q", name)
		}
		if wanted[name] {
			return Config{}, fmt.Errorf("code-gate: duplicate check %q", name)
		}
		wanted[name] = true
	}
	var selected Config
	for _, check := range c.Checks {
		if wanted[check.Name] != exclude {
			selected.Checks = append(selected.Checks, check)
		}
	}
	if len(selected.Checks) == 0 {
		return Config{}, fmt.Errorf("code-gate: selection leaves no checks")
	}
	return selected, nil
}
