package release

// ExportedWriteSums re-sums an out dir, so a test can build the
// contaminated-set case Pack now refuses to create.
func ExportedWriteSums(dir string) ([]string, error) { return writeSums(dir) }
