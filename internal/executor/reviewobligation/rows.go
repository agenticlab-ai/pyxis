package reviewobligation

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
)

// MaxRecordBytes bounds one serialized JSON record, excluding its line ending.
// Readers and writers share this limit so an admitted row remains replayable.
const MaxRecordBytes = 1 << 20

var ErrRowTooLarge = errors.New("review obligation: serialized row exceeds size limit")

func marshalRecord(row any) ([]byte, error) {
	b, err := json.Marshal(row)
	if err != nil {
		return nil, err
	}
	if len(b) > MaxRecordBytes {
		return nil, fmt.Errorf("%w (%d > %d bytes)", ErrRowTooLarge, len(b), MaxRecordBytes)
	}
	return b, nil
}

func recordScanner(data []byte) *bufio.Scanner {
	scan := bufio.NewScanner(bytes.NewReader(data))
	// ScanLines needs room for the delimiter, including legacy CRLF records.
	scan.Buffer(make([]byte, 0, 64*1024), MaxRecordBytes+2)
	scan.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		advance, token, err := bufio.ScanLines(data, atEOF)
		if len(token) > MaxRecordBytes {
			return 0, nil, ErrRowTooLarge
		}
		return advance, token, err
	})
	return scan
}
