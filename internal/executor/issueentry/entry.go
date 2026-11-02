// Package issueentry admits one mutable tracker item as an immutable native task.
package issueentry

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	executorconfig "github.com/agenticlab-ai/pyxis/internal/executor/config"
	"github.com/agenticlab-ai/pyxis/internal/executor/port"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/gitrefstore"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

const InputSchema = "issue-task-input/v1"
const ContractSchema = "issue-contract/v1"
const EvidenceRef = executorconfig.DefaultIssueEvidenceRef

type Citation struct {
	ID     string `json:"id"`
	Digest string `json:"digest"`
}
type Source struct {
	Provider      string     `json:"provider"`
	BindingDigest string     `json:"binding_digest"`
	Number        int        `json:"number"`
	Fingerprint   string     `json:"fingerprint"`
	Citations     []Citation `json:"citations"`
}
type Input struct {
	taskspec.NativeInput
	Schema string `json:"schema_version"`
	Source Source `json:"source"`
}
type Inspection struct {
	Schema   string             `json:"schema_version"`
	Source   Source             `json:"source"`
	Snapshot port.IssueSnapshot `json:"snapshot"`
	Comments []Citation         `json:"comment_citations"`
	Admitted *Contract          `json:"admitted_contract,omitempty"`
}
type Contract struct {
	Schema   string `json:"schema_version"`
	ID       string `json:"contract_id"`
	SourceID string `json:"source_id"`
	Input    Input  `json:"input"`
}

func Digest(b []byte) string          { d := sha256.Sum256(b); return "sha256:" + hex.EncodeToString(d[:]) }
func BindingDigest(key string) string { return Digest([]byte("issue-binding/v1\x00" + key)) }
func Inspect(provider, key string, snapshot port.IssueSnapshot) (Inspection, error) {
	if provider == "" || key == "" || snapshot.Number <= 0 {
		return Inspection{}, errors.New("missing exact issue identity")
	}
	numbers := make([]int, 0, len(snapshot.Dependencies))
	seen := make(map[int]bool)
	for _, dep := range snapshot.Dependencies {
		if dep.Number <= 0 || seen[dep.Number] {
			return Inspection{}, errors.New("invalid or duplicate dependency identity")
		}
		seen[dep.Number] = true
		numbers = append(numbers, dep.Number)
	}
	slices.Sort(numbers)
	b, err := json.Marshal(struct {
		Title        string `json:"title"`
		Body         string `json:"body"`
		Dependencies []int  `json:"dependencies"`
	}{snapshot.Title, snapshot.Body, numbers})
	if err != nil {
		return Inspection{}, err
	}
	out := Inspection{Schema: "issue-inspection/v1", Source: Source{Provider: provider, BindingDigest: BindingDigest(key), Number: snapshot.Number, Fingerprint: Digest(b), Citations: []Citation{}}, Snapshot: snapshot}
	ids := make(map[string]bool)
	for _, c := range snapshot.Comments {
		if c.ID == "" || ids[c.ID] {
			return Inspection{}, errors.New("invalid or duplicate source comment identity")
		}
		ids[c.ID] = true
		out.Comments = append(out.Comments, Citation{ID: c.ID, Digest: Digest([]byte(c.Body))})
	}
	return out, nil
}
func Decode(b []byte) (Input, error) {
	var input Input
	if len(b) > 1<<20 {
		return input, errors.New("issue intake exceeds 1 MiB")
	}
	if err := taskrouting.Decode(b, &input); err != nil {
		return input, fmt.Errorf("decode issue intake: %w", err)
	}
	return input, nil
}
func Prepare(input Input, provider, key string, number int, snapshot port.IssueSnapshot) (Contract, port.Task, error) {
	fail := func(reason string) (Contract, port.Task, error) {
		return Contract{}, port.Task{}, fmt.Errorf("issue admission refused: %s", reason)
	}
	if input.Schema != InputSchema {
		return fail("unsupported input schema")
	}
	if number <= 0 || snapshot.Number != number || !snapshot.Open || !snapshot.Executable {
		return fail("item is missing, closed, or non-executable")
	}
	inspection, err := Inspect(provider, key, snapshot)
	if err != nil {
		return Contract{}, port.Task{}, err
	}
	source := input.Source
	if source.Provider != provider || source.BindingDigest != inspection.Source.BindingDigest || source.Number != number || source.Fingerprint != inspection.Source.Fingerprint {
		return fail("source binding or fingerprint differs from exact issue")
	}
	for _, dep := range snapshot.Dependencies {
		if !dep.Closed {
			return fail(fmt.Sprintf("native dependency #%d is open", dep.Number))
		}
	}
	cited := make(map[string]bool)
	for _, want := range source.Citations {
		if cited[want.ID] {
			return fail("duplicate source citation")
		}
		cited[want.ID] = true
		if !slices.Contains(inspection.Comments, want) {
			return fail("cited source comment changed or disappeared")
		}
	}
	contract, task, marker, err := compileInput(input, key)
	if err != nil {
		return Contract{}, port.Task{}, err
	}
	bodies := []string{snapshot.Body}
	for _, c := range snapshot.Comments {
		bodies = append(bodies, c.Body)
	}
	for _, b := range bodies {
		if strings.Contains(b, "<!-- fs-spec:") {
			current, err := taskspec.ParseTask(snapshot.Body, commentBodies(snapshot.Comments))
			if err != nil {
				return fail("invalid existing task capsule: " + err.Error())
			}
			existing, err := taskspec.RenderNative(current)
			if err != nil || existing != marker {
				return fail("existing native task capsule conflicts with admitted intake")
			}
			break
		}
	}
	return contract, task, nil
}

// Resume reconstructs only the previously admitted task. Publication recovery
// must not depend on mutable tracker state or authorize a changed charter.
func Resume(input Input, admitted Contract, key string) (Contract, port.Task, error) {
	if err := validateContract(admitted); err != nil {
		return Contract{}, port.Task{}, err
	}
	if input.Schema != InputSchema || input.Source.BindingDigest != BindingDigest(key) {
		return Contract{}, port.Task{}, errors.New("issue recovery input binding or schema differs")
	}
	current, task, _, err := compileInput(input, key)
	if err != nil {
		return Contract{}, port.Task{}, err
	}
	if current.ID != admitted.ID {
		return Contract{}, port.Task{}, errors.New("issue already has a different admitted contract; explicit scope resolution required")
	}
	return admitted, task, nil
}

func compileInput(input Input, key string) (Contract, port.Task, string, error) {
	input.Source.Citations = slices.Clone(input.Source.Citations)
	slices.SortFunc(input.Source.Citations, func(a, b Citation) int { return strings.Compare(a.ID, b.ID) })
	sourceID := sourceIdentity(input.Source.Provider, input.Source.BindingDigest, input.Source.Number)
	native := input.NativeInput
	native.Schema = "native-task-input/v1"
	spec, _, err := taskspec.CompileInput(native, "adhoc/issue-"+sourceID)
	if err != nil {
		return Contract{}, port.Task{}, "", fmt.Errorf("compile issue charter: %w", err)
	}
	input.Requirements = *spec.Requirements
	input.NativeInput.Schema = ""
	canonical, err := json.Marshal(input)
	if err != nil {
		return Contract{}, port.Task{}, "", err
	}
	contract := Contract{Schema: ContractSchema, ID: Digest(append([]byte(ContractSchema+"\x00"), canonical...)), SourceID: sourceID, Input: input}
	spec.PlanFingerprint = contract.ID
	brief, err := taskspec.RenderInput(input.NativeInput, spec)
	if err != nil {
		return Contract{}, port.Task{}, "", err
	}
	marker, err := taskspec.RenderNative(spec)
	if err != nil {
		return Contract{}, port.Task{}, "", err
	}
	routing, err := spec.RoutingData(taskspec.DemandIdentity{Repository: key, Task: "issue-" + sourceID})
	if err != nil {
		return Contract{}, port.Task{}, "", err
	}
	task := port.Task{ID: "issue-" + sourceID, Number: input.Source.Number, SourceKey: key, Title: input.Title, Brief: brief, Requirements: routing.Requirements, DemandID: routing.ID}
	return contract, task, marker, nil
}
func commentBodies(comments []port.IssueComment) []string {
	out := make([]string, 0, len(comments))
	for _, c := range comments {
		out = append(out, c.Body)
	}
	return out
}

// Store reuses the shared fetched Git CAS substrate; records are immutable.
type Store struct{ git *gitrefstore.Store }

func NewStore(repo, remote string) (*Store, error) {
	git, err := gitrefstore.New(repo, remote, EvidenceRef)
	if err != nil {
		return nil, err
	}
	return &Store{git: git}, nil
}
func (s *Store) Admit(ctx context.Context, contract Contract) error {
	if err := validateContract(contract); err != nil {
		return err
	}
	bytes, err := json.Marshal(contract)
	if err != nil {
		return err
	}
	path := "issue-state/contracts/" + contract.SourceID + ".json"
	_, err = s.git.Update(ctx, path, "pyxis: admit direct issue contract", func(current []byte) ([]byte, bool, error) {
		if len(current) != 0 && string(current) != string(bytes) {
			return nil, false, errors.New("issue already has a different admitted contract; explicit scope resolution required")
		}
		return bytes, len(current) == 0, nil
	})
	if err != nil {
		return fmt.Errorf("persist issue admission: %w", err)
	}
	return nil
}

func validateContract(c Contract) error {
	canonical, err := json.Marshal(c.Input)
	if err != nil {
		return err
	}
	sourceID := sourceIdentity(c.Input.Source.Provider, c.Input.Source.BindingDigest, c.Input.Source.Number)
	if c.Schema != ContractSchema || c.Input.Schema != InputSchema || c.SourceID != sourceID || c.ID != Digest(append([]byte(ContractSchema+"\x00"), canonical...)) {
		return errors.New("invalid immutable issue contract")
	}
	return nil
}
func sourceIdentity(provider, binding string, number int) string {
	return strings.TrimPrefix(Digest([]byte("issue-identity/v1\x00"+provider+"\x00"+binding+"\x00"+strconv.Itoa(number))), "sha256:")
}
func (s *Store) Load(ctx context.Context, provider, key string, number int) (Contract, bool, error) {
	if provider == "" || key == "" || number <= 0 {
		return Contract{}, false, errors.New("invalid exact issue identity")
	}
	id := sourceIdentity(provider, BindingDigest(key), number)
	b, err := s.git.Read(ctx, "issue-state/contracts/"+id+".json")
	if err != nil || len(b) == 0 {
		return Contract{}, false, err
	}
	var c Contract
	if err := taskrouting.Decode(b, &c); err != nil {
		return Contract{}, false, err
	}
	if err := validateContract(c); err != nil {
		return Contract{}, false, err
	}
	if c.SourceID != id {
		return Contract{}, false, errors.New("stored issue contract identity mismatch")
	}
	return c, true, nil
}

// ScopedTracker exposes one admitted task. Readiness is freshly exact-checked
// again at dispatch, and a verified blocker never goes through frontier inference.
type ScopedTracker struct {
	Reader        port.IssueReader
	Tracker       port.WorkTracker
	Contract      Contract
	Task          port.Task
	Provider, Key string
	Associated    bool
}

func (s *ScopedTracker) Frontier(ctx context.Context) ([]port.Task, error) {
	if s.Associated {
		return nil, nil
	}
	snapshot, err := s.Reader.ReadIssue(ctx, s.Task.Number)
	if err != nil {
		return nil, err
	}
	fresh, _, err := Prepare(s.Contract.Input, s.Provider, s.Key, s.Task.Number, snapshot)
	if err != nil {
		return nil, err
	}
	if fresh.ID != s.Contract.ID {
		return nil, errors.New("admitted source changed before dispatch")
	}
	return []port.Task{s.Task}, nil
}
func (s *ScopedTracker) Comment(ctx context.Context, number int, body string) error {
	if number != s.Task.Number {
		return errors.New("refusing comment on unselected issue")
	}
	return s.Tracker.Comment(ctx, number, body)
}
