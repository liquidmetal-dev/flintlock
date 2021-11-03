package version

// PackageName is the name of the package, not just FlintlockD,
// all commands fall under this name.
const PackageName = "flintlock"

var (
	Version    = "undefined" // Specifies the app version
	BuildDate  = "undefined" // Specifies the build date
	CommitHash = "undefined" // Specifies the git commit hash
)
