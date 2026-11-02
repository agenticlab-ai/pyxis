package tasksession

import (
	"encoding/json"
	"testing"
	"time"
)

func TestFeedbackTimestampJSONPreservesZeroValueField(t *testing.T) {
	for _, tc := range []struct {
		name string
		at   time.Time
		want string
	}{
		{"no feedback", time.Time{}, `"0001-01-01T00:00:00Z"`},
		{"serviced feedback", time.Date(2026, 9, 12, 8, 30, 0, 123, time.UTC), `"2026-09-12T08:30:00.000000123Z"`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			encoded, err := json.Marshal(Record{LastFeedbackAt: tc.at})
			if err != nil {
				t.Fatal(err)
			}
			var fields map[string]json.RawMessage
			if err := json.Unmarshal(encoded, &fields); err != nil {
				t.Fatal(err)
			}
			if got := string(fields["last_feedback_at"]); got != tc.want {
				t.Fatalf("persisted feedback timestamp = %s, want %s", got, tc.want)
			}
		})
	}
}
