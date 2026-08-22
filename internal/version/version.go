// Package version carries build metadata stamped in at link time.
package version

// Version is overwritten with -ldflags by whatever builds a release. The
// default is what a plain `go build` produces.
var Version = "dev"
