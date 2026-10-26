package buildinfo

// Version is kept aligned with VERSION so direct developer builds are honest.
// Canonical release/acceptance builds override all three values through
// scripts/build.sh using Go linker flags.
var (
	Version = "0.0.21-dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)
