//go:build !(darwin || dragonfly || freebsd || linux || netbsd || openbsd || windows)

package state

import "os"

// tryLock always succeeds on a system with no way to lock a file here, so two
// gist on it are not kept from one gist. No release is built for one.
func tryLock(*os.File) (bool, error) { return true, nil }
