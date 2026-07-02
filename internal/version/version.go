// Package version exposes build-time version metadata shared by all three
// AmorphDB binaries (amorphd, amorph, amorphctl).
//
// The values are overridden at link time with -ldflags "-X":
//
//	go build -ldflags "\
//	  -X github.com/solifugus/amorphdb/internal/version.Version=1.2.3 \
//	  -X github.com/solifugus/amorphdb/internal/version.Commit=$(git rev-parse --short HEAD) \
//	  -X github.com/solifugus/amorphdb/internal/version.Date=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
//
// A plain `go build` with no ldflags leaves the defaults below, which is how
// developer builds identify themselves.
package version

import (
	"fmt"
	"runtime"
)

// These are set via -ldflags at build time. The zero-config defaults describe
// an untagged local build.
var (
	// Version is the release version (e.g. "1.2.3" or a date-stamp). "dev"
	// marks an unstamped local build.
	Version = "dev"
	// Commit is the short git commit the binary was built from.
	Commit = "none"
	// Date is the UTC build timestamp (RFC3339) or "unknown".
	Date = "unknown"
)

// String returns a one-line human-readable version summary, e.g.
// "1.2.3 (commit abc1234, built 2026-07-01T12:00:00Z, go1.26)".
func String() string {
	return fmt.Sprintf("%s (commit %s, built %s, %s)", Version, Commit, Date, runtime.Version())
}
