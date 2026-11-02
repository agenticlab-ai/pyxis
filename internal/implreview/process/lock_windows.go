//go:build windows

package process

import "errors"

type fileLock struct{}

func tryLockExclusive(string) (*fileLock, error) {
	return nil, errors.New("impl-review process: cross-process coordinator fencing is unavailable on windows")
}

func (*fileLock) Close() error { return nil }
