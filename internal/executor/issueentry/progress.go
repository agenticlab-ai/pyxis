package issueentry

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

const remoteRepairSchema = "issue-remote-repair-authority/v1"

var ErrRemoteRepairExhausted = errors.New("direct issue remote CI repair authority exhausted")

type RemoteRepairAuthority struct {
	Schema     string `json:"schema_version"`
	ContractID string `json:"contract_id"`
	Cap        int    `json:"cap"`
	Reserved   int    `json:"reserved"`
}

type RemoteRepairReservation struct{ Attempt, Cap int }

// Association preserves an exact proposal identity even if publication or
// review later fails. LocalProof is a fact about one immutable revision, not a
// success verdict. Neither record contains a machine-local worktree path.
type Association struct {
	Schema         string `json:"schema_version"`
	ContractID     string `json:"contract_id"`
	ProposalNumber int    `json:"proposal_number"`
	Branch         string `json:"branch"`
	LocalRevision  string `json:"local_revision,omitempty"`
}
type LocalProof struct {
	Schema     string `json:"schema_version"`
	ContractID string `json:"contract_id"`
	Proposal   int    `json:"proposal_number"`
	Revision   string `json:"revision"`
}

func repairPath(c Contract) (string, error) {
	path, err := contractPath(c, "remote-repairs")
	return path + ".json", err
}

// AdmitWithRemoteRepairs creates a new contract and its bounded remote-repair
// authority in one CAS commit. It never backfills authority for a legacy
// contract: absence there is ambiguous and must fail closed if CI is red.
func (s *Store) AdmitWithRemoteRepairs(ctx context.Context, c Contract, cap int) error {
	if err := validateContract(c); err != nil {
		return err
	}
	if cap < 0 {
		return errors.New("remote CI repair cap is negative")
	}
	contractBytes, err := json.Marshal(c)
	if err != nil {
		return err
	}
	contract := "issue-state/contracts/" + c.SourceID + ".json"
	repair, err := repairPath(c)
	if err != nil {
		return err
	}
	authorityBytes, err := json.Marshal(RemoteRepairAuthority{Schema: remoteRepairSchema, ContractID: c.ID, Cap: cap})
	if err != nil {
		return err
	}
	_, err = s.git.Apply(ctx, "issue-state", "pyxis: admit direct issue contract and repair authority", func(snapshot gitrefstore.Snapshot) (gitrefstore.Mutation, error) {
		currentContract, currentAuthority := snapshot.Files[contract], snapshot.Files[repair]
		if len(currentContract) == 0 && len(currentAuthority) == 0 {
			return gitrefstore.Mutation{Files: map[string][]byte{contract: contractBytes, repair: authorityBytes}, Changed: true}, nil
		}
		if string(currentContract) != string(contractBytes) {
			return gitrefstore.Mutation{}, errors.New("issue already has a different admitted contract; explicit scope resolution required")
		}
		if len(currentAuthority) == 0 {
			return gitrefstore.Mutation{}, errors.New("existing issue contract has no remote CI repair authority")
		}
		var authority RemoteRepairAuthority
		if err := taskrouting.Decode(currentAuthority, &authority); err != nil || authority.Schema != remoteRepairSchema || authority.ContractID != c.ID || authority.Cap != cap || authority.Reserved < 0 || authority.Reserved > authority.Cap {
			return gitrefstore.Mutation{}, errors.New("existing issue contract has ambiguous remote CI repair authority")
		}
		return gitrefstore.Mutation{Files: map[string][]byte{contract: currentContract, repair: currentAuthority}}, nil
	})
	if err != nil {
		return fmt.Errorf("persist issue admission: %w", err)
	}
	return nil
}

// ReserveRemoteRepair consumes one attempt through the shared Git CAS before a
// fixer can launch. Reservations are intentionally never refunded.
func (s *Store) ReserveRemoteRepair(ctx context.Context, c Contract) (RemoteRepairReservation, error) {
	path, err := repairPath(c)
	if err != nil {
		return RemoteRepairReservation{}, err
	}
	var reserved RemoteRepairReservation
	_, err = s.git.Update(ctx, path, "pyxis: reserve direct issue remote CI repair", func(current []byte) ([]byte, bool, error) {
		if len(current) == 0 {
			return nil, false, errors.New("direct issue has no retained remote CI repair authority")
		}
		var authority RemoteRepairAuthority
		if err := taskrouting.Decode(current, &authority); err != nil || authority.Schema != remoteRepairSchema || authority.ContractID != c.ID || authority.Cap < 0 || authority.Reserved < 0 || authority.Reserved > authority.Cap {
			return nil, false, errors.New("direct issue remote CI repair authority is ambiguous")
		}
		if authority.Reserved >= authority.Cap {
			return nil, false, fmt.Errorf("%w (%d/%d reserved)", ErrRemoteRepairExhausted, authority.Reserved, authority.Cap)
		}
		authority.Reserved++
		reserved = RemoteRepairReservation{Attempt: authority.Reserved, Cap: authority.Cap}
		next, err := json.Marshal(authority)
		return next, true, err
	})
	if err != nil {
		return RemoteRepairReservation{}, err
	}
	return reserved, nil
}

func contractPath(c Contract, kind string) (string, error) {
	if err := validateContract(c); err != nil {
		return "", err
	}
	return "issue-state/" + kind + "/" + strings.TrimPrefix(c.ID, "sha256:"), nil
}
func (s *Store) putImmutable(ctx context.Context, path string, value any) error {
	b, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = s.git.Update(ctx, path, "pyxis: retain direct issue evidence", func(current []byte) ([]byte, bool, error) {
		if len(current) > 0 && string(current) != string(b) {
			return nil, false, errors.New("conflicting direct issue evidence")
		}
		return b, len(current) == 0, nil
	})
	return err
}
func (s *Store) Associate(ctx context.Context, c Contract, p port.Proposal) error {
	return s.associate(ctx, c, p, "")
}

// AssociateVerified retains the proposal and its local proof in one Git CAS
// write. There is no published-association state awaiting a second proof write.
func (s *Store) AssociateVerified(ctx context.Context, c Contract, p port.Proposal, revision string) error {
	if _, err := proofPath(c, revision); err != nil {
		return err
	}
	return s.associate(ctx, c, p, revision)
}

func (s *Store) associate(ctx context.Context, c Contract, p port.Proposal, revision string) error {
	path, err := contractPath(c, "proposals")
	if err != nil {
		return err
	}
	if p.Number <= 0 || p.Branch == "" {
		return errors.New("invalid issue proposal association")
	}
	return s.putImmutable(ctx, path+".json", Association{Schema: "issue-proposal/v1", ContractID: c.ID, ProposalNumber: p.Number, Branch: p.Branch, LocalRevision: revision})
}
func (s *Store) Proposal(ctx context.Context, c Contract) (port.Proposal, bool, error) {
	a, ok, err := s.association(ctx, c)
	if err != nil || !ok {
		return port.Proposal{}, false, err
	}
	return port.Proposal{Number: a.ProposalNumber, Branch: a.Branch}, true, nil
}

func (s *Store) association(ctx context.Context, c Contract) (Association, bool, error) {
	path, err := contractPath(c, "proposals")
	if err != nil {
		return Association{}, false, err
	}
	b, err := s.git.Read(ctx, path+".json")
	if err != nil || len(b) == 0 {
		return Association{}, false, err
	}
	var a Association
	if err := taskrouting.Decode(b, &a); err != nil {
		return Association{}, false, err
	}
	if a.Schema != "issue-proposal/v1" || a.ContractID != c.ID || a.ProposalNumber <= 0 || a.Branch == "" {
		return Association{}, false, errors.New("invalid stored issue proposal")
	}
	if a.LocalRevision != "" {
		if _, err := proofPath(c, a.LocalRevision); err != nil {
			return Association{}, false, err
		}
	}
	return a, true, nil
}
func proofPath(c Contract, revision string) (string, error) {
	path, err := contractPath(c, "local")
	if err != nil {
		return "", err
	}
	if !ValidRevision(revision) {
		return "", fmt.Errorf("invalid local proof revision")
	}
	return path + "/" + revision + ".json", nil
}
func (s *Store) RecordLocal(ctx context.Context, c Contract, p port.Proposal, revision string) error {
	path, err := proofPath(c, revision)
	if err != nil {
		return err
	}
	known, ok, err := s.Proposal(ctx, c)
	if err != nil {
		return err
	}
	if !ok || known != p {
		return errors.New("local proof requires the exact proposal association")
	}
	return s.putImmutable(ctx, path, LocalProof{Schema: "issue-local-proof/v1", ContractID: c.ID, Proposal: p.Number, Revision: revision})
}
func (s *Store) LocalVerified(ctx context.Context, c Contract, p port.Proposal, revision string) (bool, error) {
	path, err := proofPath(c, revision)
	if err != nil {
		return false, err
	}
	a, ok, err := s.association(ctx, c)
	if err != nil || !ok || a.ProposalNumber != p.Number || a.Branch != p.Branch {
		return false, err
	}
	if a.LocalRevision == revision {
		return true, nil
	}
	b, err := s.git.Read(ctx, path)
	if err != nil || len(b) == 0 {
		return false, err
	}
	var proof LocalProof
	if err := taskrouting.Decode(b, &proof); err != nil {
		return false, err
	}
	if proof.Schema != "issue-local-proof/v1" || proof.ContractID != c.ID || proof.Proposal != p.Number || proof.Revision != revision {
		return false, errors.New("invalid stored local proof")
	}
	return true, nil
}
