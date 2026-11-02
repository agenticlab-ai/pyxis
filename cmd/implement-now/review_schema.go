package main

import (
	"fmt"
	"io"

	"github.com/agenticlab-ai/pyxis/internal/implreview/report"
)

var compileReviewSchema = report.JSONSchema

func reviewSchemaTool(args []string, stdout, stderr io.Writer) int {
	if len(args) != 0 {
		if _, err := fmt.Fprintln(stderr, "review schema: --internal-review-schema accepts no arguments"); err != nil {
			return 1
		}
		return 2
	}
	schema, err := compileReviewSchema()
	if err != nil {
		if _, writeErr := fmt.Fprintf(stderr, "review schema: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}
	if _, err := stdout.Write(schema); err != nil {
		if _, writeErr := fmt.Fprintf(stderr, "review schema: write output: %v\n", err); writeErr != nil {
			return 1
		}
		return 1
	}
	return 0
}
