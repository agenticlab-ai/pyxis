package main

import (
	"encoding/json"
	"io"

	"github.com/agenticlab-ai/pyxis/internal/taskrequirements"
)

func printDimensions(w io.Writer) error {
	return json.NewEncoder(w).Encode(taskrequirements.Values())
}
