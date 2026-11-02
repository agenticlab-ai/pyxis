package issueentry

import (
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/prdescription"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
)

const CheckpointSchema = "issue-checkpoint/v1"

const checkpointFile = "issue-checkpoint.json"

// Checkpoint binds the implementer's complete description to the committed
// revision it covers. It is machine-local recovery state for a task branch
// that was never published: the supervisor writes it beside the isolated
// worktree, outside every directory a harness may write, and the worktree's
// cleanup removes it. Shared evidence (association, local proof) is written
// only when a proposal exists.
type Checkpoint struct {
	Schema      string `json:"schema_version"`
	ContractID  string `json:"contract_id"`
	TaskID      string `json:"task_id"`
	Branch      string `json:"branch"`
	Revision    string `json:"revision"`
	Description string `json:"description"`
}

// ValidRevision reports a full lowercase SHA-1 or SHA-256 commit identity.
func ValidRevision(revision string) bool {
	_, err := hex.DecodeString(revision)
	return err == nil && (len(revision) == 40 || len(revision) == 64) && strings.ToLower(revision) == revision
}

// Validate refuses a record recovery cannot publish from, including an
// implementer description that misses the template.
func (c Checkpoint) Validate() error {
	if c.Schema != CheckpointSchema || !strings.HasPrefix(c.ContractID, "sha256:") || c.TaskID == "" || c.Branch == "" || !ValidRevision(c.Revision) {
		return errors.New("invalid issue checkpoint record")
	}
	if _, problems := prdescription.FromAgent(c.Description); len(problems) != 0 {
		return fmt.Errorf("issue checkpoint has an incomplete implementer description: %v", problems)
	}
	return nil
}

// RetainCheckpoint replaces dir's checkpoint record atomically.
func RetainCheckpoint(dir string, c Checkpoint) error {
	if err := c.Validate(); err != nil {
		return err
	}
	b, err := json.Marshal(c)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("prepare issue checkpoint directory: %w", err)
	}
	tmp, err := os.CreateTemp(dir, checkpointFile+".*")
	if err != nil {
		return fmt.Errorf("write issue checkpoint: %w", err)
	}
	_, writeErr := tmp.Write(b)
	closeErr := tmp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		return errors.Join(fmt.Errorf("write issue checkpoint: %w", err), os.Remove(tmp.Name()))
	}
	if err := os.Rename(tmp.Name(), filepath.Join(dir, checkpointFile)); err != nil {
		return errors.Join(fmt.Errorf("retain issue checkpoint: %w", err), os.Remove(tmp.Name()))
	}
	return nil
}

// LoadCheckpoint reads dir's checkpoint record; ok is false when none exists.
func LoadCheckpoint(dir string) (Checkpoint, bool, error) {
	b, err := os.ReadFile(filepath.Join(dir, checkpointFile))
	if errors.Is(err, os.ErrNotExist) {
		return Checkpoint{}, false, nil
	}
	if err != nil {
		return Checkpoint{}, false, fmt.Errorf("read issue checkpoint: %w", err)
	}
	var c Checkpoint
	if err := taskrouting.Decode(b, &c); err != nil {
		return Checkpoint{}, false, fmt.Errorf("decode issue checkpoint: %w", err)
	}
	if err := c.Validate(); err != nil {
		return Checkpoint{}, false, err
	}
	return c, true, nil
}
