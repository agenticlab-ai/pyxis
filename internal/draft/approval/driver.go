package approval

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"regexp"
	"slices"
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/docgate/authoritycontract"
	"github.com/agenticlab-ai/pyxis/internal/docgate/config"
	"github.com/agenticlab-ai/pyxis/internal/docgate/schema"
)

const DriverSchema = "argus-approval/v1"

const driverPrefix = "<!-- pyxis-argus-approval/v1\n"
const maxDriverEncodedBytes = 32768

var fullHead = regexp.MustCompile(`^[0-9a-f]{40}$`)
var contractDigest = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
var driveName = regexp.MustCompile(`^[a-z][a-z0-9-]*$`)

// DriverDecision is authenticated by its containing GitHub review, not by the
// principal label. A COMMENTED review is intentional for same-account authors:
// GitHub prohibits their APPROVED reviews. It is never called a human approval.
type DriverDecision struct {
	Schema       string `json:"schema"`
	Principal    string `json:"principal"`
	Repository   string `json:"repository"`
	PullRequest  int    `json:"pull_request"`
	Head         string `json:"head"`
	PolicyCommit string `json:"policy_commit"`
	Drive        string `json:"drive"`
	Digest       string `json:"digest,omitempty"`
	Evidence     string `json:"evidence"`
	CaptureLine  string `json:"capture_line,omitempty"`
}

func (d DriverDecision) Validate() error {
	if d.Schema != DriverSchema || d.Principal != "argus" || d.Repository == "" || d.PullRequest < 1 ||
		!fullHead.MatchString(d.Head) || !fullHead.MatchString(d.PolicyCommit) || !driveName.MatchString(d.Drive) ||
		d.Digest != "" && !contractDigest.MatchString(d.Digest) || strings.TrimSpace(d.Evidence) == "" ||
		len(d.Evidence) > 16384 || len(d.CaptureLine) > 8192 || strings.ContainsRune(d.Evidence+d.CaptureLine, '\x00') {
		return fmt.Errorf("invalid Argus approval binding or review evidence")
	}
	return nil
}

func (d DriverDecision) Body() (string, error) {
	if err := d.Validate(); err != nil {
		return "", err
	}
	body, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return "", fmt.Errorf("encode Argus approval: %w", err)
	}
	if len(body) > maxDriverEncodedBytes {
		return "", fmt.Errorf("encoded Argus approval exceeds %d bytes; shorten evidence or capture text", maxDriverEncodedBytes)
	}
	return "Argus approves the exact reviewed revision under the recorded driver grant.\n\n" + driverPrefix + string(body) + "\n-->", nil
}

// ParseDriver rejects open schemas, trailing JSON, duplicate markers, and
// malformed tagged decisions. Ordinary review comments are not decisions.
func ParseDriver(body string) (DriverDecision, error) {
	var d DriverDecision
	if strings.Count(body, driverPrefix) != 1 {
		return d, fmt.Errorf("argus approval requires one typed decision")
	}
	_, rest, _ := strings.Cut(body, driverPrefix)
	encoded, trailing, ok := strings.Cut(rest, "\n-->")
	if !ok || strings.TrimSpace(trailing) != "" || len(encoded) > maxDriverEncodedBytes {
		return d, fmt.Errorf("invalid Argus approval envelope")
	}
	decoder := json.NewDecoder(bytes.NewBufferString(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&d); err != nil {
		return d, fmt.Errorf("decode Argus approval: %w", err)
	}
	if err := decoder.Decode(new(any)); !errors.Is(err, io.EOF) {
		return d, fmt.Errorf("argus approval contains trailing data")
	}
	canonical, err := json.MarshalIndent(d, "", "  ")
	if err != nil || !bytes.Equal([]byte(encoded), canonical) {
		return d, fmt.Errorf("argus approval must use canonical decision bytes")
	}
	return d, d.Validate()
}

// DriverGranted reuses the authority-contract parser and its structural
// evaluator. The caller must supply bytes from trusted mainline, not worktree
// overlays or a candidate branch. Revocation wins over inherited bundles.
func DriverGranted(body []byte) bool {
	c, defects := authoritycontract.Parse("approve", schema.AuthorityContractPath, body)
	if len(defects) != 0 || len(authoritycontract.Evaluate("approve", schema.AuthorityContractPath, c)) != 0 || c.CurrentTier == "revoked" {
		return false
	}
	rows := make(map[string][]string, len(c.Ladder))
	for _, row := range c.Ladder {
		rows[row.Tier] = row.Tokens
	}
	for _, tier := range c.Class.TierOrder {
		if slices.Contains(rows[tier], "approve-merge") {
			return true
		}
		if tier == c.CurrentTier {
			break
		}
	}
	return false
}

// SelectDriver checks account identity from the host separately from the
// declared principal. A later changes-requested or dismissed review revokes an
// earlier decision. Ordinary comments do not silently replace it.
func SelectDriver(policy config.ApprovalSettings, repository string, pull int, head, base, digest string, granted bool, reviews []Review) (Approval, error) {
	if !granted || policy.OperatorLogin == "" || policy.ArgusLogin == "" || strings.EqualFold(policy.ArgusLogin, policy.OperatorLogin) {
		return Approval{}, ErrNoApproval
	}
	var latest *Review
	for i := range reviews {
		r := &reviews[i]
		if !strings.EqualFold(r.Author, policy.ArgusLogin) || r.ID < 1 || strings.EqualFold(r.State, "PENDING") {
			continue
		}
		if strings.EqualFold(r.State, "COMMENTED") && !strings.Contains(r.Body, driverPrefix) {
			continue
		}
		if latest == nil || r.ID > latest.ID {
			latest = r
		}
	}
	if latest == nil || latest.CommitID != head || !strings.EqualFold(latest.State, "APPROVED") && !strings.EqualFold(latest.State, "COMMENTED") {
		return Approval{}, ErrNoApproval
	}
	d, err := ParseDriver(latest.Body)
	if err != nil || d.Repository != repository || d.PullRequest != pull || d.Head != head || d.PolicyCommit != base || d.Digest != digest {
		return Approval{}, ErrNoApproval
	}
	return Approval{Operator: latest.Author, Head: head, CaptureLine: d.CaptureLine, ReviewID: latest.ID, Kind: "argus", PolicyCommit: base, Evidence: d.Evidence}, nil
}
