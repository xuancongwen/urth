// Package version holds build metadata injected at link time.
package version

// Set via -ldflags "-X urth/internal/version.Version=..." in the Makefile.
var (
	Version = "dev"
	Commit  = "none"
)
