package approval

import (
	"errors"
	"strings"
	"testing"
)

func TestSelectRequiresOperatorsLatestApprovalOfExactHead(t *testing.T) {
	head := strings.Repeat("a", 40)
	other := strings.Repeat("b", 40)
	for _, tc := range []struct {
		name    string
		reviews []Review
		capture string
		want    bool
	}{
		{"capture line", []Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: head, Body: "12 min; FND-1 inside §0"}}, "12 min; FND-1 inside §0", true},
		{"lineless", []Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: head}}, "", true},
		{"older head", []Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: other}}, "", false},
		{"agent", []Review{{ID: 1, Author: "draft-agent", State: "APPROVED", CommitID: head}}, "", false},
		{"Argus", []Review{{ID: 1, Author: "argus", State: "APPROVED", CommitID: head}}, "", false},
		{"dismissed", []Review{{ID: 1, Author: "mdarabi", State: "DISMISSED", CommitID: head}}, "", false},
		{"latest changes requested", []Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: head}, {ID: 2, Author: "mdarabi", State: "CHANGES_REQUESTED", CommitID: head}}, "", false},
		{"latest approval", []Review{{ID: 2, Author: "mdarabi", State: "APPROVED", CommitID: head}, {ID: 1, Author: "mdarabi", State: "CHANGES_REQUESTED", CommitID: head}}, "", true},
		{"later comment", []Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: head}, {ID: 2, Author: "mdarabi", State: "COMMENTED", CommitID: head}}, "", true},
		{"pending review", []Review{{ID: 1, Author: "mdarabi", State: "APPROVED", CommitID: head}, {ID: 2, Author: "mdarabi", State: "PENDING", CommitID: head}}, "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := Select("mdarabi", head, tc.reviews)
			if !tc.want {
				if !errors.Is(err, ErrNoApproval) {
					t.Fatalf("approval=%+v error=%v", got, err)
				}
				return
			}
			if err != nil || got.Operator != "mdarabi" || got.Head != head || got.CaptureLine != tc.capture {
				t.Fatalf("approval=%+v error=%v", got, err)
			}
		})
	}
}

func TestSelectRejectsMissingScope(t *testing.T) {
	if _, err := Select("", "head", nil); err == nil {
		t.Fatal("missing operator accepted")
	}
	if _, err := Select("mdarabi", "", nil); err == nil {
		t.Fatal("missing head accepted")
	}
}
