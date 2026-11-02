// Package association defines the explicit join between memory records and
// repository code. A record names code either with a code:<path> tag or with
// an about relation to a record whose subject is a code/* anchor.
package association

import (
	"strings"

	"github.com/agenticlab-ai/pyxis/internal/memory/record"
)

// Lookup finds a record by its immutable id.
type Lookup interface {
	Get(string) *record.Record
}

// Scopes returns the code paths explicitly associated with r. Tags are
// considered before about relations and duplicate paths are returned once.
//
// Scopes does not interpret record status: callers that evaluate the current
// memory must call it only for active records. More than one lookup may be
// supplied when a record set is layered over a base set.
func Scopes(r *record.Record, lookups ...Lookup) []string {
	if r == nil {
		return nil
	}
	seen := map[string]bool{}
	var scopes []string
	add := func(scope string) {
		if scope == "" || seen[scope] {
			return
		}
		seen[scope] = true
		scopes = append(scopes, scope)
	}
	addTags(r, add)
	for _, id := range r.Rel(record.About) {
		anchor := find(lookups, id)
		if anchor != nil && record.Namespace(anchor.Subject) == "code" {
			if !addTags(anchor, add) {
				add(strings.TrimPrefix(anchor.Subject, "code/"))
			}
		}
	}
	return scopes
}

// addTags adds only the record's own tags. It deliberately does not follow
// about relations: an anchor's association is one hop, never a recursive
// graph walk. The result reports whether a code tag was declared, including
// an empty one that add may reject as not being a path.
func addTags(r *record.Record, add func(string)) bool {
	found := false
	for _, tag := range r.Tags {
		if strings.HasPrefix(tag, "code:") {
			found = true
			add(strings.TrimPrefix(tag, "code:"))
		}
	}
	return found
}

func find(lookups []Lookup, id string) *record.Record {
	for _, records := range lookups {
		if records == nil {
			continue
		}
		if r := records.Get(id); r != nil {
			return r
		}
	}
	return nil
}

// Covers reports whether scope is codePath itself or one of its directory
// ancestors. Prefixes meet only at a slash boundary, so internal/foo does not
// cover internal/foobar.
func Covers(scope, codePath string) bool {
	if scope == "" || codePath == "" {
		return false
	}
	return scope == codePath || strings.HasPrefix(codePath, scope+"/")
}

// Matches reports whether two repository-relative code paths identify the
// same path or one is below the other. Search uses the symmetric relationship;
// evaluators use Covers when only an association's descendants count.
func Matches(a, b string) bool {
	return Covers(a, b) || Covers(b, a)
}
