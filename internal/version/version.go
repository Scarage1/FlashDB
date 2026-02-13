// Package version provides the FlashDB version string.
// The version is set at build time via -ldflags.
package version

// Version is the current FlashDB version.
// Override at build time: go build -ldflags "-X github.com/flashdb/flashdb/internal/version.Version=2.0.0"
var Version = "2.0.0"

// BuildTime is the build timestamp.
// Override at build time: go build -ldflags "-X github.com/flashdb/flashdb/internal/version.BuildTime=$(date -u +%Y-%m-%dT%H:%M:%SZ)"
var BuildTime = "unknown"
