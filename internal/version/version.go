package version

import (
	"runtime/debug"
	"strings"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// UserAgent is the User-Agent every request of the CLI sends.
func UserAgent() string {
	return "CubeCLI/" + forUserAgent(Version, buildInfoVersion())
}

// forUserAgent returns the release version: the one injected with ldflags, or,
// when that is still "dev" (go install does not inject it), the module version
// the Go toolchain recorded in the binary.
func forUserAgent(injected, module string) string {
	if injected != "dev" {
		return injected
	}
	module = strings.TrimPrefix(module, "v")
	if module == "" || module == "(devel)" {
		return injected
	}
	return module
}

func buildInfoVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok {
		return info.Main.Version
	}
	return ""
}
