package options

import (
	"fmt"
	"runtime"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// Docker encapsulates options related to connecting to a Docker daemon.
type Docker struct {
	Host       string `bson:"host" json:"host" yaml:"host"`
	Port       int    `bson:"port" json:"port" yaml:"port"`
	APIVersion string `bson:"api_version" json:"api_version" yaml:"api_version"`
	Image      string `bson:"image" json:"image" yaml:"image"`
	// Platform refers to the major operating system on which the Docker
	// container runs.
	Platform string `bson:"platform" json:"platform" yaml:"platform"`
}

// Validate checks whether all the assert. fields are set and sets defaults if
// none are specified.
func (opts *Docker) Validate() error {
	catcher := &erc.Collector{}
	catcher.If(opts.Port < 0, ers.Error("port must be positive value"))
	catcher.If(opts.Image == "", ers.Error("Docker image must be specified"))
	if opts.Platform == "" {
		if PlatformSupportsDocker(runtime.GOOS) {
			opts.Platform = runtime.GOOS
		} else {
			catcher.Push(fmt.Errorf("failed to set default platform to current runtime platform '%s' because it is unsupported", opts.Platform))
		}
	} else if !PlatformSupportsDocker(opts.Platform) {
		catcher.Push(fmt.Errorf("unrecognized platform '%s'", opts.Platform))
	}
	return catcher.Resolve()
}

// Copy returns a copy of the options for only the exported fields.
func (opts *Docker) Copy() *Docker {
	optsCopy := *opts
	return &optsCopy
}

// DockerPlatforms returns whether or not the platform has support for Docker.
func PlatformSupportsDocker(platform string) bool {
	switch platform {
	case "darwin", "linux", "windows":
		return true
	default:
		return false
	}
}
