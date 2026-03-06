package version

// Version is the current version of craft.
// This is overridden at build time via -ldflags:
//
//	go build -ldflags "-X github.com/erdemtuna/craft/internal/version.Version=1.0.0"
var Version = "dev"
