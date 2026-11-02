package main

import (
	"errors"
	"io"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
	"github.com/agenticlab-ai/pyxis/internal/worktracker"
	"github.com/agenticlab-ai/pyxis/internal/worktracker/model"
)

type failingOutput struct {
	remaining int
	failed    bool
}

func (w *failingOutput) Write(p []byte) (int, error) {
	if w.remaining == 0 {
		w.failed = true
		return 0, errors.New("output disconnected")
	}
	w.remaining--
	return len(p), nil
}

func TestOutputFailureIsNotReportedAsSuccess(t *testing.T) {
	item := model.WorkItem{ID: 1, Title: "Native task", Body: "charter", CloseReason: model.CloseCompleted, Parent: "2", BlockedBy: []string{"3"}, Priority: model.Priority("high"), Log: []model.LogEntry{{Payload: "first\nsecond"}}}
	for name, emit := range map[string]func(io.Writer) int{
		"item":            func(w io.Writer) int { return emitItem(w, false, item, "created") },
		"item json":       func(w io.Writer) int { return emitItem(w, true, item, "") },
		"log":             func(w io.Writer) int { return emitItemLog(w, item) },
		"set":             func(w io.Writer) int { return emitSet(w, false, "ready", []model.WorkItem{item}) },
		"set json":        func(w io.Writer) int { return emitSet(w, true, "ready", []model.WorkItem{item}) },
		"vocabulary":      func(w io.Writer) int { return emitVocabulary(w, false, []model.Signal{{Name: "agent"}}) },
		"vocabulary json": func(w io.Writer) int { return emitVocabulary(w, true, nil) },
		"list":            func(w io.Writer) int { return emitList(w, false, model.Query{}, []model.WorkItem{item}) },
		"empty list":      func(w io.Writer) int { return emitList(w, false, model.Query{}, nil) },
		"list json":       func(w io.Writer) int { return emitList(w, true, model.Query{}, nil) },
		"search json":     func(w io.Writer) int { return emitSearch(w, true, "native", nil) },
		"reindex":         func(w io.Writer) int { return emitReindex(w, false, worktracker.ReindexReport{}) },
		"reindex json":    func(w io.Writer) int { return emitReindex(w, true, worktracker.ReindexReport{}) },
	} {
		t.Run(name, func(t *testing.T) {
			for writes := range 20 {
				w := &failingOutput{remaining: writes}
				code := emit(w)
				if !w.failed {
					break
				}
				if code != 1 {
					t.Fatalf("output failure exit = %d", code)
				}
			}
			if code := emit(&failingOutput{}); code != 1 {
				t.Fatalf("first write failed but exit = %d", code)
			}
		})
	}
	if usage(&failingOutput{}) != 1 {
		t.Fatal("usage hid write failure")
	}
	for _, asJSON := range []bool{false, true} {
		if emitDefects(&failingOutput{}, asJSON, []schema.Defect{{Reason: "invalid route"}}) != 1 {
			t.Fatal("defect output failure succeeded")
		}
	}
}
