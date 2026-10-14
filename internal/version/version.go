package version

import "regexp"

// PackageName is the name of the package, not just FlintlockD,
// all commands fall under this name.
const PackageName = "flintlock"

var (
	Version    = "undefined" // Specifies the app version
	BuildDate  = "undefined" // Specifies the build date
	CommitHash = "undefined" // Specifies the git commit hash
)

// releaseVersion matches MAJOR.MINOR.PATCH with an optional v prefix: the
// form of a release tag (v0.16.0) and the form GoReleaser writes into
// Version for a tagged build (0.16.0). A snapshot or a dev build carries a
// suffix or no version at all, and does not match.
var releaseVersion = regexp.MustCompile(`^v?(0|[1-9]\d*)\.(0|[1-9]\d*)\.(0|[1-9]\d*)$`)

// IsRelease reports whether v is the version of a release build.
func IsRelease(v string) bool {
	return releaseVersion.MatchString(v)
}
