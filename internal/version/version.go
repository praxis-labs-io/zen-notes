// Package version carries build metadata stamped in at link time.
package version

// Version is set with -ldflags by release builds, and reads dev from a plain go build.
var Version = "dev"
