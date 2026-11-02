package gate

import "testing"

func TestResolveMergeBaseReportsUnresolvableRepository(t *testing.T) {
	if base, defects := ResolveMergeBaseAt(t.TempDir(), "missing", ""); base != "" || len(defects) != 1 {
		t.Fatalf("ResolveMergeBaseAt = %q, %v", base, defects)
	}
}
