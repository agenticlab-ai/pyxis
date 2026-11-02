package issueentry

import (
	"context"
	"fmt"
)

// PublishSummary remerges observed concurrent edits and attempt records, with
// bounded retries. Native PR-body APIs do not provide an atomic section update;
// this fresh-read protocol does not claim a server-side compare-and-swap.
func PublishSummary(ctx context.Context, summary Summary, read func(context.Context) (string, error), write func(context.Context, string) error) error {
	for range 3 {
		body, err := read(ctx)
		if err != nil {
			return err
		}
		merged, err := MergeSummary(body, summary)
		if err != nil {
			return err
		}
		if merged == body {
			return nil
		}
		fresh, err := read(ctx)
		if err != nil {
			return err
		}
		if fresh != body {
			continue
		}
		if err := write(ctx, merged); err != nil {
			return err
		}
		after, err := read(ctx)
		if err != nil {
			return err
		}
		checked, err := MergeSummary(after, summary)
		if err != nil {
			return err
		}
		if checked == after {
			return nil
		}
	}
	return fmt.Errorf("execution summary publication did not stabilize within three attempts")
}
