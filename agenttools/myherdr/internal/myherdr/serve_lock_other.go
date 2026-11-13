//go:build js || plan9 || wasip1

package myherdr

func acquireServeLock(_ string) (serveLock, error) {
	return nil, errServeLockUnsupported
}
