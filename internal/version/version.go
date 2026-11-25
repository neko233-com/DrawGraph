// Package version holds build-time version metadata injected via -ldflags.
package version

import "strings"

const (
	Repo           = "neko233-com/DrawGraph"
	DefaultChannel = "stable"
)

// Version, Commit, Date, and Channel are populated at release build time.
var (
	Version = "0.1.0-dev"
	Commit  = "dev"
	Date    = "unknown"
	Channel = DefaultChannel
)

// String returns a human-readable version identifier.
func String() string {
	v := strings.TrimSpace(Version)
	if v == "" {
		v = "0.1.0-dev"
	}
	if !strings.HasPrefix(v, "v") {
		v = "v" + v
	}
	return v
}

// IsDev reports whether this binary is an unversioned development build.
func IsDev() bool {
	v := strings.ToLower(strings.TrimSpace(Version))
	return v == "" || strings.Contains(v, "dev")
}
