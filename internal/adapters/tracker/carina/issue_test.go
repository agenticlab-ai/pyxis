package carinatracker

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestExactIssueUsesNativeReadyRulesWithExplicitAuthorization(t *testing.T) {
	for _, tc := range []struct {
		name, item, blocker string
		executable, fail    bool
	}{
		{"plain unlabelled task", `{"id":17,"type":"task","state":"open","log":[{"kind":"comment","payload":"criterion"}]}`, "", true, false},
		{"closed dependency", `{"id":17,"type":"task","state":"open","blocked_by":["9"]}`, `{"id":9,"type":"task","state":"closed"}`, true, false},
		{"outside open dependency", `{"id":17,"type":"task","state":"open","blocked_by":["9"]}`, `{"id":9,"type":"task","state":"open"}`, false, false},
		{"unreadable dependency", `{"id":17,"type":"task","state":"open","blocked_by":["9"]}`, "", false, true},
		{"foreign dependency", `{"id":17,"type":"task","state":"open","blocked_by":["other/repo#9"]}`, "", false, true},
		{"rollup", `{"id":17,"type":"feature","state":"open"}`, "", false, false},
		{"wrong identity", `{"id":18,"type":"task","state":"open"}`, "", false, true},
		{"reject disposition", `{"id":17,"type":"task","state":"open","log":[{"kind":"decision-record","namespace":"triage-disposition","payload":"reject"}]}`, "", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tr := New("/private/authoritative-root", withRunner(func(_ context.Context, args ...string) ([]byte, error) {
				if len(args) < 2 || args[0] != "show" {
					t.Fatalf("unselected scan: %v", args)
				}
				if args[1] == "17" {
					return []byte(`{"schema":"worktracker.work-item/v1",` + tc.item[1:]), nil
				}
				if args[1] == "9" && tc.blocker != "" {
					return []byte(`{"schema":"worktracker.work-item/v1",` + tc.blocker[1:]), nil
				}
				return nil, errors.New("not found")
			}))
			s, err := tr.ReadIssue(t.Context(), 17)
			if (err != nil) != tc.fail || (!tc.fail && s.Executable != tc.executable) {
				t.Fatalf("snapshot=%+v %v", s, err)
			}
			if strings.Contains(s.Body, "authoritative-root") {
				t.Fatal("root escaped into issue data")
			}
		})
	}
}

func TestExactIssueRefusesInvalidNumberClosedCodecAndUnknownSchema(t *testing.T) {
	for _, tc := range []struct {
		number int
		body   string
	}{
		{0, ""},
		{17, "broken"},
		{17, "{\"schema\":\"future/v9\",\"id\":17,\"state\":\"open\"}"},
		{17, "{\"schema\":\"worktracker.work-item/v1\",\"id\":17,\"state\":\"open\",\"unknown\":0}"},
	} {
		tr := New("/private/authoritative-root", withRunner(func(context.Context, ...string) ([]byte, error) { return []byte(tc.body), nil }))
		if _, err := tr.ReadIssue(t.Context(), tc.number); err == nil {
			t.Fatal("malformed native item accepted")
		}
	}
}
