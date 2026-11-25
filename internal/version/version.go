// Package version holds build-time version metadata injected via -ldflags.
package version

// Version, Commit, and Date are populated at release build time.
var (
	Version = "0.1.0-dev"
	Commit  = "dev"
	Date    = "unknown"
)
