package localgit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/agenticlab-ai/pyxis/internal/codehost"
)

func TestLocalDraftLockTagIsClosedCanonicalAndWorkflowBound(t *testing.T) {
	t.Parallel()
	o, store, request := landingFixture(t, true)
	result, err := store.LandDraft(t.Context(), request)
	if err != nil {
		t.Fatal(err)
	}
	ref := "refs/tags/lock/widget/v1"
	original := fixtureGit(t, o.Origin, "rev-parse", ref)
	raw, err := proposalGit(t.Context(), o.Origin, nil, nil, "cat-file", "-p", ref)
	if err != nil {
		t.Fatal(err)
	}
	header, body, _ := bytes.Cut(raw, []byte("\n\n"))
	for _, failure := range []string{"oversized", "missing separator", "malformed JSON", "unknown field", "second JSON value", "noncanonical JSON", "wrong feature", "different retained lock", "wrong object header"} {
		t.Run(failure, func(t *testing.T) {
			altered := append([]byte(nil), raw...)
			switch failure {
			case "oversized":
				altered = append(altered, bytes.Repeat([]byte("x"), 64<<10)...)
			case "missing separator":
				altered = append(append([]byte(nil), header...), '\n')
			case "malformed JSON":
				altered = append(append(append([]byte(nil), header...), '\n', '\n'), []byte("{\n")...)
			case "unknown field":
				altered = bytes.Replace(altered, []byte("\"schema_version\":"), []byte("\"unknown\":true,\"schema_version\":"), 1)
			case "second JSON value":
				altered = append(altered, []byte("{}\n")...)
			case "noncanonical JSON":
				var formatted bytes.Buffer
				if err := json.Indent(&formatted, body, "", "  "); err != nil {
					t.Fatal(err)
				}
				altered = append(append(append([]byte(nil), header...), '\n', '\n'), formatted.Bytes()...)
			case "wrong feature", "different retained lock":
				lock := result.Lock
				if failure == "wrong feature" {
					lock.Feature = "another-feature"
				} else {
					lock.ProposalsSinceOpened++
				}
				encoded, err := json.Marshal(lock)
				if err != nil {
					t.Fatal(err)
				}
				altered = append(append(append([]byte(nil), header...), '\n', '\n'), append(encoded, '\n')...)
			case "wrong object header":
				altered = bytes.Replace(altered, []byte("object "+result.Lock.MergeCommit), []byte("object "+request.Binding.TargetOID), 1)
			}
			oid, err := proposalGit(t.Context(), o.Origin, altered, nil, "hash-object", "--literally", "-w", "-t", "tag", "--stdin")
			if err != nil {
				t.Fatal(err)
			}
			fixtureGit(t, o.Origin, "update-ref", ref, strings.TrimSpace(string(oid)))
			if _, err := store.VerifyDraftLock(t.Context(), "widget", 1); err == nil {
				t.Fatal("malformed or unrelated tag certified a lock")
			}
			fixtureGit(t, o.Origin, "update-ref", ref, original)
		})
	}
	clone := proposalClone(t, o)
	fixtureGit(t, clone.Root, "update-ref", ref, request.Binding.TargetOID)
	before := fixtureGit(t, o.Origin, "show-ref")
	if _, err := NewDraftLandings(clone).VerifyDraftLock(t.Context(), "widget", 1); err != nil {
		t.Fatalf("cached tag blocked origin lock verification: %v", err)
	}
	if fixtureGit(t, clone.Root, "rev-parse", ref) != request.Binding.TargetOID {
		t.Fatal("verification overwrote a conflicting cached tag")
	}
	if fixtureGit(t, o.Origin, "show-ref") != before {
		t.Fatal("verification changed the retained origin")
	}
	for _, selector := range []struct {
		feature string
		version int
	}{{"", 1}, {"widget", 0}} {
		if _, err := store.VerifyDraftLock(t.Context(), selector.feature, selector.version); err == nil {
			t.Fatal("invalid lock selector accepted")
		}
	}
	bad := o
	bad.Root = t.TempDir()
	if _, err := NewDraftLandings(bad).VerifyDraftLock(t.Context(), "widget", 1); err == nil {
		t.Fatal("unconfigured reader accepted")
	}
}

func TestLocalDraftLockVersionInventoryRefusesGapsAndMalformedTags(t *testing.T) {
	t.Parallel()
	o, store, request := landingFixture(t, true)
	tag, err := prepareLockTag(t.Context(), o.Root, "refs/tags/lock/widget/v1", codehost.LocalDraftLock{MergeCommit: request.Binding.Head})
	if err != nil {
		t.Fatal(err)
	}
	fixtureGit(t, o.Origin, "fetch", "--no-write-fetch-head", o.Root, tag)
	for _, state := range []string{"lightweight", "occupied version"} {
		t.Run(state, func(t *testing.T) {
			oid := tag
			if state == "lightweight" {
				oid = request.Binding.Head
			}
			fixtureGit(t, o.Origin, "update-ref", "refs/tags/lock/widget/v1", oid)
			before := fixtureGit(t, o.Origin, "show-ref")
			if _, err := store.LandDraft(t.Context(), request); err == nil {
				t.Fatal("changed version inventory allowed publication")
			}
			if fixtureGit(t, o.Origin, "show-ref") != before {
				t.Fatal("refusal changed the origin")
			}
			fixtureGit(t, o.Origin, "update-ref", "-d", "refs/tags/lock/widget/v1")
		})
	}
	fixtureGit(t, o.Origin, "update-ref", "refs/tags/lock/widget/v1", tag)
	for _, ref := range []string{"refs/tags/lock/widget/v02", "refs/tags/lock/widget/pending", "refs/tags/lock/widget/v3"} {
		fixtureGit(t, o.Origin, "update-ref", ref, tag)
		if _, err := originLockVersion(t.Context(), o.Origin, "widget"); err == nil {
			t.Fatalf("invalid inventory accepted: %s", ref)
		}
		fixtureGit(t, o.Origin, "update-ref", "-d", ref)
	}
	fixtureGit(t, o.Origin, "update-ref", "refs/tags/lock/widget/v2", tag)
	if v, err := originLockVersion(t.Context(), o.Origin, "widget"); err != nil || v != 2 {
		t.Fatalf("contiguous annotated versions refused: %d %v", v, err)
	}
	if _, err := originLockVersion(t.Context(), o.Origin, ""); err == nil {
		t.Fatal("missing feature accepted")
	}
	if _, err := originLockVersion(t.Context(), t.TempDir(), "widget"); err == nil {
		t.Fatal("unreadable origin accepted")
	}
}
