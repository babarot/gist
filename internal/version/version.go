// Package version holds the release version. tagpr updates it.
package version

import (
	"runtime/debug"
	"strings"
)

// Version is the current release.
const Version = "2.0.0"

// Source is "release" in the builds the release workflow makes, set at link
// time by GoReleaser, and empty in any other build. It is not state: no
// code writes it. Only a release build looks for newer releases.
var Source string

// IsRelease reports whether this build is one the release workflow made.
func IsRelease() bool { return Source == "release" }

// String is the version with the commit it was built from, when the build
// knows it: a build in a clone does. go install does not, but it tells the
// version of the module installed, a tag or one naming the commit.
func String() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		info = &debug.BuildInfo{}
	}
	return format(Version, info)
}

func format(version string, info *debug.BuildInfo) string {
	rev := setting(info, "vcs.revision")
	if rev == "" {
		// Installed with go install, the module's version is known
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
		return version
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if setting(info, "vcs.modified") == "true" {
		rev += "-dirty"
	}
	return version + " (" + rev + ")"
}

func setting(info *debug.BuildInfo, key string) string {
	for _, s := range info.Settings {
		if s.Key == key {
			return s.Value
		}
	}
	return ""
}
