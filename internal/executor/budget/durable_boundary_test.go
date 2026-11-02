package budget

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

type faultDurableFile struct {
	name     string
	chmodErr error
	writeErr error
	syncErr  error
	closeErr error
}

func (f *faultDurableFile) Name() string            { return f.name }
func (f *faultDurableFile) Chmod(os.FileMode) error { return f.chmodErr }
func (f *faultDurableFile) Sync() error             { return f.syncErr }
func (f *faultDurableFile) Close() error            { return f.closeErr }
func (f *faultDurableFile) Write(p []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	return len(p), nil
}

func TestWriteDurableSurfacesEveryPreRenameFailure(t *testing.T) {
	path := filepath.Join(t.TempDir(), "record.json")
	wantErr := errors.New("injected durable failure")
	if err := writeDurableWith(durableOps{}, path, make(chan int)); err == nil {
		t.Fatal("unsupported JSON value was encoded")
	}

	baseOps := func(file *faultDurableFile) durableOps {
		return durableOps{
			createTemp: func(string, string) (durableFile, error) { return file, nil },
			rename:     func(string, string) error { return nil },
			remove:     func(string) error { return nil },
			openDir: func(string) (durableFile, error) {
				return &faultDurableFile{name: filepath.Dir(path)}, nil
			},
		}
	}

	cases := []struct {
		name string
		ops  func() durableOps
	}{
		{name: "create", ops: func() durableOps {
			return durableOps{createTemp: func(string, string) (durableFile, error) { return nil, wantErr }}
		}},
		{name: "chmod", ops: func() durableOps {
			return baseOps(&faultDurableFile{name: path + ".tmp", chmodErr: wantErr})
		}},
		{name: "write", ops: func() durableOps {
			return baseOps(&faultDurableFile{name: path + ".tmp", writeErr: wantErr})
		}},
		{name: "sync", ops: func() durableOps {
			return baseOps(&faultDurableFile{name: path + ".tmp", syncErr: wantErr})
		}},
		{name: "close", ops: func() durableOps {
			return baseOps(&faultDurableFile{name: path + ".tmp", closeErr: wantErr})
		}},
		{name: "rename", ops: func() durableOps {
			ops := baseOps(&faultDurableFile{name: path + ".tmp"})
			ops.rename = func(string, string) error { return wantErr }
			return ops
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if err := writeDurableWith(tc.ops(), path, ledger{}); !errors.Is(err, wantErr) {
				t.Fatalf("writeDurableWith error = %v", err)
			}
		})
	}
}

func TestDurableRemovalAndDirectorySyncFailures(t *testing.T) {
	wantErr := errors.New("injected durable failure")
	path := filepath.Join(t.TempDir(), "record.json")
	if err := removeDurableWith(durableOps{
		remove: func(string) error { return wantErr },
	}, path); !errors.Is(err, wantErr) {
		t.Fatalf("removeDurableWith error = %v", err)
	}
	if err := fsyncDirWith(durableOps{
		openDir: func(string) (durableFile, error) { return nil, wantErr },
	}, filepath.Dir(path)); !errors.Is(err, wantErr) {
		t.Fatalf("fsyncDirWith open error = %v", err)
	}
	if err := fsyncDirWith(durableOps{
		openDir: func(string) (durableFile, error) {
			return &faultDurableFile{name: filepath.Dir(path), syncErr: wantErr}, nil
		},
	}, filepath.Dir(path)); !errors.Is(err, wantErr) {
		t.Fatalf("fsyncDirWith sync error = %v", err)
	}
}
