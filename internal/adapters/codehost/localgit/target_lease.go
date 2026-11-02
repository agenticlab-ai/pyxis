package localgit

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

type unchangedRef struct{ Ref, OID string }

// withUnchangedRefs holds a Git verification transaction across the existing
// atomic push. A no-op target refspec is omitted by Git and cannot lease an
// unchanged target. The prepared verification locks that ref without changing
// it; EOF aborts the read-only transaction and releases its lock on every exit.
func withUnchangedRefs(ctx context.Context, origin string, refs []unchangedRef, publish func() error) (err error) {
	cmd := proposalCommand(ctx, origin, "update-ref", "--no-deref", "--stdin")
	input, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	output, err := cmd.StdoutPipe()
	if err != nil {
		return errors.Join(err, input.Close())
	}
	var diagnostic bytes.Buffer
	cmd.Stderr = &diagnostic
	// Closing stdin lets Git abort and remove the prepared lock on cancellation.
	cmd.Cancel = input.Close
	cmd.WaitDelay = 5 * time.Second
	if err := cmd.Start(); err != nil {
		return errors.Join(err, input.Close(), output.Close())
	}
	defer func() {
		// Cancellation may already have closed this pipe. Wait owns process reap.
		_ = input.Close()
		if waitErr := cmd.Wait(); waitErr != nil {
			err = errors.Join(err, fmt.Errorf("%w: target lease: %v: %s", codehost.ErrProposalConflict, waitErr, strings.TrimSpace(diagnostic.String())))
		}
		if ctx.Err() != nil {
			err = errors.Join(err, ctx.Err())
		}
	}()
	commands := []string{"start"}
	for _, ref := range refs {
		commands = append(commands, fmt.Sprintf("verify %s %s", ref.Ref, ref.OID))
	}
	commands = append(commands, "prepare", "")
	if _, err := fmt.Fprint(input, strings.Join(commands, "\n")); err != nil {
		return err
	}
	reader := bufio.NewReader(output)
	for _, want := range []string{"start: ok", "prepare: ok"} {
		ack, readErr := reader.ReadString('\n')
		if readErr != nil || strings.TrimSpace(ack) != want {
			return fmt.Errorf("%w: acquire target lease: %v", codehost.ErrProposalConflict, readErr)
		}
	}
	return publish()
}
