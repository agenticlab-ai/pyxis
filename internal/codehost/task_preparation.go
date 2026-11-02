package codehost

import (
	"crypto/rand"
	"encoding/hex"
	"io"
)

const (
	TaskPreparationSchema = "local-task-preparation/v1"
	TaskMerge             = "task-merge"
)

// TaskPreparation fixes a request nonce before computing the prepared
// generation. Its later generation-bound proof and request are separate refs.
type TaskPreparation struct {
	SchemaVersion string `json:"schema_version"`
	Operation     string `json:"operation"`
	Nonce         string `json:"nonce"`
}

func NewTaskPreparation() (TaskPreparation, error) {
	return newTaskPreparation(rand.Reader)
}

func newTaskPreparation(entropy io.Reader) (TaskPreparation, error) {
	var nonce [16]byte
	if _, err := io.ReadFull(entropy, nonce[:]); err != nil {
		return TaskPreparation{}, err
	}
	return TaskPreparation{TaskPreparationSchema, TaskMerge, hex.EncodeToString(nonce[:])}, nil
}

func (p TaskPreparation) Validate() error {
	if p.SchemaVersion != TaskPreparationSchema || p.Operation != TaskMerge || !lowerHex(p.Nonce, 32) {
		return ErrReviewInvalid
	}
	return nil
}
