//go:build windows

package tasksession

type ledgerLock struct{}

// lockLedger is a no-op on Windows: the ledger's append-only, latest-wins
// semantics keep concurrent writers safe; only counters can lose an update.
func lockLedger(string) (*ledgerLock, error) { return &ledgerLock{}, nil }

func (*ledgerLock) Close() error { return nil }
