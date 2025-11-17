package version

// Build information. These variables are set at build time via -ldflags
var (
	// Version is the semantic version of the gateway
	Version = "v0.3.1"

	// Commit is the git commit hash
	Commit = "unknown"

	// BuiltAt is the build timestamp
	BuiltAt = "unknown"
)

// Info returns formatted version information
func Info() string {
	return Version
}

// FullInfo returns complete build information
func FullInfo() string {
	return "version=" + Version + " commit=" + Commit + " built_at=" + BuiltAt
}
