package version

// Version is injected at build time. Defaults to "dev" for local development.
// Production builds set this via: go build -ldflags "-X .../version.Version=v1.0.0"
var Version = "dev"

const (
	Name    = "Orkai"
	Website = "https://github.com/orkai-dev/orkai"

	GitHubOwner = "orkai-dev"
	GitHubRepo  = "orkai"
)
