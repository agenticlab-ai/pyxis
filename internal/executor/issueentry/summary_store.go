package issueentry

import (
	"context"
	"encoding/json"
	"errors"
)

// RetainSummary merges observations by host-generated invocation identity. A
// replay replaces its own provisional observation and never counts usage twice.
func (s *Store) RetainSummary(ctx context.Context, c Contract, current Summary) (Summary, error) {
	path, err := contractPath(c, "summaries")
	if err != nil {
		return Summary{}, err
	}
	if err := validateSummary(current); err != nil {
		return Summary{}, err
	}
	if current.ContractID != c.ID {
		return Summary{}, errors.New("summary contract mismatch")
	}
	result, err := s.git.Update(ctx, path+".json", "pyxis: retain issue execution observations", func(b []byte) ([]byte, bool, error) {
		merged := current
		merged.Attempts = append([]Attempt{}, current.Attempts...)
		if len(b) > 0 {
			prior, err := DecodeSummary(b)
			if err != nil {
				return nil, false, err
			}
			merged, err = mergeObservations(prior, current)
			if err != nil {
				return nil, false, err
			}
		}
		merged.UsageScope = "retained_invocations"
		merged = totalObservations(merged)
		wire, err := json.Marshal(merged)
		return wire, string(wire) != string(b), err
	})
	if err != nil {
		return Summary{}, err
	}
	return DecodeSummary(result)
}

func mergeObservations(prior, current Summary) (Summary, error) {
	if prior.ContractID != current.ContractID || prior.Source != current.Source || prior.Proposal != current.Proposal {
		return Summary{}, errors.New("conflicting issue summary identity")
	}
	merged := current
	merged.Attempts = append([]Attempt{}, prior.Attempts...)
	newLaunch := false
	for _, a := range current.Attempts {
		found := false
		for i, old := range merged.Attempts {
			if old.ID == a.ID {
				if old.Role != a.Role || old.Harness != a.Harness || old.Model != a.Model || old.Effort != a.Effort {
					return Summary{}, errors.New("conflicting invocation launch identity")
				}
				// An older provisional publisher cannot erase terminal observations.
				if old.Status == "running" || a.Status != "running" {
					merged.Attempts[i] = a
				}
				found = true
				break
			}
		}
		if !found {
			newLaunch = true
			merged.Attempts = append(merged.Attempts, a)
		}
	}
	if !newLaunch && current.Terminal == "running" && prior.Terminal != "running" {
		merged.Terminal, merged.Revision = prior.Terminal, prior.Revision
		merged.ExecutionElapsedMS = prior.ExecutionElapsedMS
	}
	merged.UsageScope = "retained_invocations"
	return totalObservations(merged), nil
}

func totalObservations(merged Summary) Summary {
	total, known, partial := int64(0), 0, false
	for _, a := range merged.Attempts {
		if a.Tokens.Value != nil {
			total += *a.Tokens.Value
			known++
			partial = partial || a.Tokens.State == "partial"
		}
	}
	merged.TotalTokens = Unavailable("no retained token observations")
	if known > 0 {
		merged.TotalTokens = Counter{State: "observed", Value: &total}
		if partial || known < len(merged.Attempts) {
			merged.TotalTokens.State = "partial"
			merged.TotalTokens.Reason = "some retained invocation usage is unavailable"
		}
	}
	return merged
}
