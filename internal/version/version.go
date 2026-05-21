package version

import (
	"runtime/debug"
	"strings"
)

// Version is set at build time via -ldflags (e.g. v1.3.6 from a release tag).
var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	for _, setting := range info.Settings {
		if setting.Key == "vcs.tag" && setting.Value != "" {
			Version = setting.Value
			return
		}
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		Version = info.Main.Version
	}
}

// BaseRelease returns the release tag portion of Version (e.g. v1.3.7 from v1.3.7-0.20260513043908-740f9b18+dirty).
func BaseRelease() string {
	v := strings.TrimSuffix(Version, "+dirty")
	if idx := strings.IndexByte(v, '-'); idx > 0 && strings.HasPrefix(v, "v") {
		return v[:idx]
	}
	return v
}

// Display returns the user-facing version (short release tag only, e.g. v1.3.7).
func Display() string {
	return BaseRelease()
}
