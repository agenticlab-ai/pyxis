package model

// OpenChildren filters a child set to the open ones, preserving the given order
// — the substance of an epic's rollup, "the count and identity of its open
// children" (prd:local-work-tracker/FR-7). It is one medium-neutral
// implementation the CLI and the engine share over the scan-derived Children
// read, so the rollup can never disagree across the two front doors (DINV-6);
// the returned slice's length is the count and its elements are the identities.
func OpenChildren(children []WorkItem) []WorkItem {
	open := make([]WorkItem, 0, len(children))
	for _, c := range children {
		if c.State == StateOpen {
			open = append(open, c)
		}
	}
	return open
}

// GroupChildren partitions a child set into its open and closed members,
// preserving order — the substance of the all-states epic rollup, "children
// grouped by open and closed" (worktracker-hierarchy-index §3.1, A-7). It is the
// all-states successor to OpenChildren: one medium-neutral implementation the CLI
// and the engine share over the Children read, so the grouped rollup can never
// disagree across the two front doors (DINV-7), and a completed epic still
// surfaces its full task set through the closed group.
func GroupChildren(children []WorkItem) (open, closed []WorkItem) {
	for _, c := range children {
		if c.State == StateClosed {
			closed = append(closed, c)
		} else {
			open = append(open, c)
		}
	}
	return open, closed
}
