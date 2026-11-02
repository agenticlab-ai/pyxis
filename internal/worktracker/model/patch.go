package model

// EditPatch is the input to an edit — the free fields a caller may change on an
// existing work item (docs/tech-design/local-work-tracker.md §3.9). A nil
// pointer means "leave this field untouched"; a non-nil pointer sets the field
// to its value, including an explicit empty string. This distinction is what
// lets an edit touch *only* the named fields (T2 acceptance: an edit changes
// exactly the named fields plus the updated stamp).
//
// The first lifecycle task exposes title and body — the free, hand-editable
// fields. Structured fields (signals, parent, dependencies) are mutated by
// their own dedicated verbs in later tasks, never smuggled through the patch.
type EditPatch struct {
	Title *string
	Body  *string
}

// Names reports whether the patch names at least one field to change; an empty
// patch is a no-op the store need not write.
func (p EditPatch) Names() bool { return p.Title != nil || p.Body != nil }
