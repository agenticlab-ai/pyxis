// Command native is a hermetic executable for registered-adapter tests.
package main

import (
	"errors"
	"io"
	"log/slog"
	"os"
	"path/filepath"
)

func main() {
	if err := run(); err != nil {
		slog.Error("native fixture failed", "error", err)
		os.Exit(1)
	}
}

func run() error {
	outputs := map[string]string{
		"claude":       `{"type":"result","subtype":"success","is_error":false,"num_turns":1,"usage":{"input_tokens":1,"output_tokens":1}}`,
		"codex":        `{"type":"turn.completed","usage":{"total_tokens":2}}`,
		"cursor-agent": `{"type":"result","subtype":"success","is_error":false,"result":"done","usage":{"input_tokens":1,"output_tokens":1}}`,
		"opencode":     `{"tokens":{"input":1,"output":1},"steps":1}`,
	}
	output, ok := outputs[filepath.Base(os.Args[0])]
	if !ok {
		return errors.New("unknown native fixture executable name")
	}
	if len(os.Args) > 1 && (os.Args[1] == "login" || os.Args[1] == "--version") {
		_, err := os.Stdout.Write([]byte(output + "\n"))
		return err
	}
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return err
	}
	captured := "args:"
	for _, argument := range os.Args[1:] {
		captured += "<" + argument + ">"
	}
	captured += "\nstdin:" + string(input)
	if err := os.WriteFile(".native-capture", []byte(captured), 0o600); err != nil {
		return err
	}
	_, err = os.Stdout.Write([]byte(output + "\n"))
	return err
}
