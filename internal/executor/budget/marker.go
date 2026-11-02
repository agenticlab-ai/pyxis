package budget

import "os"

// The cutover marker, per design:state-reader-v4#DINV-13 and §3.8.
//
// It records that a host's ledgers have paid their first-initialization
// barrier AND that nothing failing to charge the umbrella has written since.
// That second clause is why a same-build restart retains it while a revert to
// a build that does not charge the umbrella must invalidate it.
//
// It is contractual data, not a sentinel file (AP-6): closed, schema-
// versioned, host-scoped. The predicate below decides whether a full window
// may be skipped, so a marker is never given the benefit of the doubt.

// markerSchema is the version this build writes and is the only version it
// accepts. An UNKNOWN version is treated as absent rather than tolerated: a
// marker from a future build is a downgrade in disguise, which is exactly the
// case the barrier exists for.
const markerSchema = 1

type cutoverMarker struct {
	Schema int    `json:"schema"`
	Host   string `json:"host"`
}

// markerValid reports whether the host's barrier may be considered paid.
//
// Absent, unparseable, truncated, host-mismatched, or an unknown schema
// version all read as INVALID, which arms the barrier — the safe direction in
// every case, because the cost of a false invalid is one window and the cost
// of a false valid is a full unmetered allowance against a platform that is
// still counting.
//
// It defends against corruption, and only that. A schema-VALID forgery
// satisfies this predicate by construction — that is what a predicate is —
// and the control against forgery is access: every record is owner-only, so
// the set of principals who can write a convincing marker is the set already
// entitled to spend the budget (§3.5).
func markerValid(path, host string) bool {
	return markerValidWith(path, host, os.Geteuid)
}

func markerValidWith(path, host string, effectiveUID func() int) bool {
	data, err := readPrivateFileWith(path, effectiveUID)
	if err != nil {
		return false
	}
	var m cutoverMarker
	if decodeClosedJSON(data, &m) != nil {
		return false
	}
	return m.Schema == markerSchema && m.Host == host
}

// writeMarker records a paid barrier durably. It is the SECOND half of the
// initialization transaction: the conservative ledgers are made durable
// first, so a crash can leave the barrier unpaid-and-repayable but never
// paid-but-unenforced (DINV-12).
func writeMarker(path, host string) error {
	return writeDurable(path, cutoverMarker{Schema: markerSchema, Host: host})
}

// InvalidateMarker re-arms a host's cutover barrier, durably.
//
// This is the one sanctioned deletion of durable throttle state, and it
// precedes a downgrade to a build that does not charge the umbrella
// (prd:self-drive-wake §5.4). Completion is defined by this call RETURNING
// NIL — unlink plus directory fsync both done. If it returns an error the
// downgrade is incomplete, not merely untidy: the caller must not permit
// legacy writes and must retry, because a marker that survives leaves the
// eventual re-upgrade skipping a barrier it needs.
//
// Waiting out the drain afterwards is the operator's and the release gate's
// (prd:self-drive-wake/UB-8); this function is only the marker half.
func InvalidateMarker(host string) error {
	path, err := MarkerPath(host)
	if err != nil {
		return err
	}
	if err := ensureRecordDir(path); err != nil {
		return err
	}
	return removeDurable(path)
}
