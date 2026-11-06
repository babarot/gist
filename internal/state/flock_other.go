//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd)

package state

import "os"

// tryLock always succeeds where flock(2) is not available, so two gist on
// such a system are not kept from one gist.
func tryLock(*os.File) (bool, error) { return true, nil }
