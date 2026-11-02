package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	localidentity "github.com/agenticlab-ai/pyxis/internal/adapters/owneridentity/local"
)

func TestRegisterLocalCommandRetainsOwnerAndOnlyPrintsPublicIdentity(t *testing.T) {
	home := t.TempDir()
	var first, second, stderr bytes.Buffer
	args := []string{"register-local", "--config-home", home}
	if err := run(t.Context(), args, &first, &stderr); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), args, &second, &stderr); err != nil || first.String() != second.String() {
		t.Fatalf("registration changed across commands: %v", err)
	}
	var registration localidentity.Registration
	decoder := json.NewDecoder(&second)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&registration); err != nil || registration.OwnerID == "" || registration.PublicKey == "" {
		t.Fatalf("public registration: %+v, %v", registration, err)
	}
	key, err := os.ReadFile(filepath.Join(home, "feature-drive", "owner.ed25519"))
	if err != nil || bytes.Contains(first.Bytes(), key) || bytes.Contains(first.Bytes(), []byte(hex.EncodeToString(key))) || stderr.Len() != 0 {
		t.Fatalf("registration leaked credential bytes or reported errors: %v", err)
	}
	if err := decoder.Decode(new(localidentity.Registration)); !errors.Is(err, io.EOF) {
		t.Fatal("public registration output included extra records")
	}
}

type registrationWriteFailure struct{}

func (registrationWriteFailure) Write([]byte) (int, error) { return 0, io.ErrClosedPipe }

func TestRegisterLocalCommandRefusesInvalidInputsAndReportsOutputFailure(t *testing.T) {
	for _, args := range [][]string{{"register-local", "--unknown"}, {"register-local", "extra"}} {
		if err := run(t.Context(), args, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
			t.Fatalf("invalid registration arguments accepted: %v", args)
		}
	}
	home := t.TempDir()
	if err := run(t.Context(), []string{"register-local", "--config-home", home}, registrationWriteFailure{}, &bytes.Buffer{}); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("output failure hidden: %v", err)
	}
	occupied := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(occupied, []byte("occupied"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := run(t.Context(), []string{"register-local", "--config-home", occupied}, &bytes.Buffer{}, &bytes.Buffer{}); err == nil {
		t.Fatal("invalid registration directory accepted")
	}
}
