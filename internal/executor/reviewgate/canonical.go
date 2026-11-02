package reviewgate

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

const MaxSnapshotBytes = 1 << 20

func Canonical(snapshot Snapshot) ([]byte, error) {
	data, err := json.Marshal(snapshot)
	if err != nil {
		return nil, err
	}
	if len(data)+1 > MaxSnapshotBytes {
		return nil, fmt.Errorf("%w: complete aggregate exceeds retention bound", ErrIncomplete)
	}
	return append(data, '\n'), nil
}

// Parse rejects unknown nested fields, duplicate keys, trailing values, and
// noncanonical encodings. Validate still requires current origin/policy reads.
func Parse(data []byte) (Snapshot, error) {
	var snapshot Snapshot
	if len(data) > MaxSnapshotBytes {
		return snapshot, fmt.Errorf("%w: aggregate exceeds retention bound", ErrIncomplete)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("%w: closed aggregate: %w", ErrIncomplete, err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return Snapshot{}, fmt.Errorf("%w: trailing aggregate data", ErrIncomplete)
	}
	canonical, err := Canonical(snapshot)
	if err != nil {
		return Snapshot{}, err
	}
	if !bytes.Equal(data, canonical) {
		return Snapshot{}, fmt.Errorf("%w: aggregate is not canonical", ErrIncomplete)
	}
	return snapshot, nil
}
