package main

import (
	"crypto/rand"
	"fmt"
	"github.com/agenticlab-ai/pyxis/internal/executor/taskspec"
	"github.com/agenticlab-ai/pyxis/internal/taskrouting"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
	"os"
)

type nativeTaskInput = taskspec.NativeInput

// nativeCreateSpec validates the complete portable charter before Create.
func nativeCreateSpec(path string) (model.CreateSpec, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return model.CreateSpec{}, fmt.Errorf("read native task: %w", err)
	}
	var input nativeTaskInput
	if err := taskrouting.Decode(b, &input); err != nil {
		return model.CreateSpec{}, err
	}
	_, body, err := taskspec.CompileInput(input, "adhoc/"+rand.Text())
	if err != nil {
		return model.CreateSpec{}, err
	}
	return model.CreateSpec{Type: model.TypeTask, Title: input.Title, Body: body}, nil
}
