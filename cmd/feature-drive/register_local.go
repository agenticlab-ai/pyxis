package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"

	localidentity "github.com/agenticlab-ai/pyxis/internal/adapters/owneridentity/local"
)

func registerLocal(ctx context.Context, args []string, stdout, stderr io.Writer) error {
	flags := flag.NewFlagSet("feature-drive register-local", flag.ContinueOnError)
	flags.SetOutput(stderr)
	configHome := flags.String("config-home", "", "Pyxis configuration home (defaults to the user's configuration directory/pyxis)")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("usage: feature-drive register-local [--config-home <directory>]")
	}
	identity, err := localidentity.Open(ctx, *configHome)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(identity.Registration()); err != nil {
		return fmt.Errorf("write local owner public registration: %w", err)
	}
	return nil
}
