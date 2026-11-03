//go:build sentinelfixedrelease

package installer

// The native tests build the installed command with this tag, so that it
// installs the floor release and the tests stay the same after a new release.
func init() { sentinelFixedRelease = true }
