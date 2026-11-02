package record

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

// crockford is the ULID alphabet: base32 without I, L, O and U.
const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID renders a universally unique lexicographically sortable identifier:
// 48 bits of millisecond time followed by 80 bits of entropy, as 26 Crockford
// base32 characters. Ids minted later sort after ids minted earlier, which is
// what makes record ids browsable in time order.
func ULID(at time.Time, entropy [10]byte) string {
	var b [16]byte
	ms := uint64(at.UnixMilli())
	b[0] = byte(ms >> 40)
	b[1] = byte(ms >> 32)
	b[2] = byte(ms >> 24)
	b[3] = byte(ms >> 16)
	b[4] = byte(ms >> 8)
	b[5] = byte(ms)
	copy(b[6:], entropy[:])
	var out [26]byte
	// 128 bits → 26 × 5 bits, most significant first (two leading zero bits).
	var acc uint64
	bits := 0
	pos := 25
	for i := 15; i >= 0; i-- {
		acc |= uint64(b[i]) << bits
		bits += 8
		for bits >= 5 && pos >= 0 {
			out[pos] = crockford[acc&31]
			acc >>= 5
			bits -= 5
			pos--
		}
	}
	for pos >= 0 {
		out[pos] = crockford[acc&31]
		acc >>= 5
		pos--
	}
	return string(out[:])
}

// BatchTime reserves count consecutive millisecond slots at or after now's
// millisecond, strictly after every predecessor ID. Advancing a whole
// millisecond makes ordering independent of kind prefixes and entropy. The range must
// fit both ULID time and the RFC 3339 created_at field before any records issue.
func BatchTime(now time.Time, count uint64, predecessors ...string) (time.Time, error) {
	now = now.UTC()
	if now.Year() < 1970 || now.Year() > 9999 {
		return time.Time{}, fmt.Errorf("%w: unrepresentable batch timestamp", ErrInvalid)
	}
	if count == 0 {
		return now, nil
	}
	started := uint64(now.UnixMilli())
	for _, id := range predecessors {
		kind := KindOfID(id)
		if !IsKind(kind) {
			return time.Time{}, fmt.Errorf("%w: invalid chronology ID %q", ErrInvalid, id)
		}
		ulid := id[len(kind)+1:]
		if ulid[0] > '7' { // ULID's 128 bits leave two leading zero bits.
			return time.Time{}, fmt.Errorf("%w: invalid chronology ID %q", ErrInvalid, id)
		}
		var ms uint64
		for _, c := range []byte(ulid[:10]) {
			ms = ms<<5 | uint64(strings.IndexByte(crockford, c))
		}
		if ms >= started {
			started = ms + 1
		}
	}
	// RFC 3339 ends earlier than ULID's 48-bit timestamp range. Check by
	// subtraction before converting or adding a batch offset; never wrap time.
	last := uint64(time.Date(9999, 12, 31, 23, 59, 59, 999000000, time.UTC).UnixMilli())
	if started > last || count-1 > last-started {
		return time.Time{}, fmt.Errorf("%w: unrepresentable batch chronology", ErrInvalid)
	}
	return time.UnixMilli(int64(started)).UTC(), nil
}

// Entropy draws ten random bytes for a new id.
func Entropy() ([10]byte, error) {
	var e [10]byte
	if _, err := rand.Read(e[:]); err != nil {
		return e, fmt.Errorf("entropy: %w", err)
	}
	return e, nil
}
