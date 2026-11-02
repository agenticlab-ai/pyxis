package localgit

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strconv"
	"strings"
	"sync"
)

// acceptedRefs holds names Git's check-ref-format accepted. The check is a
// pure syntax rule, so a repeated name needs no further Git process.
var acceptedRefs sync.Map

// checkRefFormat reports Git's verdict on ref. An accepted name runs Git again
// only when the context or root could no longer run it, so those failures
// surface exactly as before.
func checkRefFormat(ctx context.Context, root, ref string) error {
	if _, accepted := acceptedRefs.Load(ref); accepted && ctx.Err() == nil {
		if info, err := os.Stat(root); err == nil && info.IsDir() {
			return nil
		}
	}
	if _, err := proposalGit(ctx, root, nil, nil, "check-ref-format", ref); err != nil {
		return err
	}
	acceptedRefs.Store(ref, struct{}{})
	return nil
}

// catObject is one cat-file --batch answer. A missing name has no type, and
// an object larger than the caller's bound has no body.
type catObject struct {
	Type string
	Size int
	Body []byte
}

var errCatFileResponse = errors.New("local Git cat-file: malformed response")

// catObjects answers specs in order from one cat-file process. It stops at the
// first object larger than limit without reading that body.
func catObjects(ctx context.Context, root string, limit int, specs ...string) ([]catObject, error) {
	var input bytes.Buffer
	for _, spec := range specs {
		if strings.ContainsAny(spec, "\r\n\x00") {
			return nil, errors.New("local Git cat-file: invalid object name")
		}
		input.WriteString(spec + "\n")
	}
	cmd := proposalCommand(ctx, root, "cat-file", "--batch")
	cmd.Stdin = &input
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	objects, complete, readErr := readCatObjects(bufio.NewReader(stdout), limit, len(specs))
	if !complete {
		// The process may still be writing a body nobody will read. Its exit
		// status is then irrelevant: the read result decides.
		_ = cmd.Process.Kill()
	}
	waitErr := cmd.Wait()
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if readErr != nil {
		return nil, readErr
	}
	if complete && waitErr != nil {
		return nil, waitErr
	}
	return objects, nil
}

// readCatObjects reports whether it consumed every answer. It returns early,
// with the oversized object last, when a body exceeds limit.
func readCatObjects(r *bufio.Reader, limit, count int) ([]catObject, bool, error) {
	objects := make([]catObject, 0, count)
	for len(objects) < count {
		header, err := r.ReadString('\n')
		if err != nil {
			return nil, false, errCatFileResponse
		}
		fields := strings.Fields(header)
		if len(fields) != 3 || !objectID(fields[0]) {
			// "<name> missing" and "<name> ambiguous" carry no body.
			objects = append(objects, catObject{})
			continue
		}
		size, err := strconv.Atoi(fields[2])
		if err != nil || size < 0 {
			return nil, false, errCatFileResponse
		}
		object := catObject{Type: fields[1], Size: size}
		if size > limit {
			return append(objects, object), false, nil
		}
		body := make([]byte, size+1)
		if _, err := io.ReadFull(r, body); err != nil || body[size] != '\n' {
			return nil, false, errCatFileResponse
		}
		object.Body = body[:size]
		objects = append(objects, object)
	}
	return objects, true, nil
}

// proposalRecord reads oid's proposal.json after confirming oid names a
// commit. One cat-file process answers both; every other answer repeats the
// separate reads, so their refusals are unchanged.
func proposalRecord(ctx context.Context, origin, oid string) ([]byte, error) {
	objects, err := catObjects(ctx, origin, maxTaskProposalBytes, oid, oid+":proposal.json")
	if err == nil && len(objects) == 2 && objects[0].Type == "commit" && objects[1].Type == "blob" {
		if objects[1].Size > maxTaskProposalBytes {
			return nil, invalidProposal("proposal record is too large")
		}
		return objects[1].Body, nil
	}
	if err := commitObject(ctx, origin, oid); err != nil {
		return nil, err
	}
	size, err := proposalGit(ctx, origin, nil, nil, "cat-file", "-s", oid+":proposal.json")
	if err != nil {
		return nil, invalidProposal("proposal object has no readable record")
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(size)))
	if err != nil || n > maxTaskProposalBytes {
		return nil, invalidProposal("proposal record is too large")
	}
	body, err := proposalGit(ctx, origin, nil, nil, "show", oid+":proposal.json")
	if err != nil {
		return nil, invalidProposal("proposal object has no readable record")
	}
	if len(body) != n || len(body) > maxTaskProposalBytes {
		return nil, invalidProposal("proposal record is too large")
	}
	return body, nil
}
