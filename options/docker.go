package options

import (
	"fmt"
	"net"
	"runtime"

	"github.com/docker/docker/client"

	"github.com/tychoish/fun/erc"
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

// Validate checks whether all the required fields are set and sets defaults if
// none are specified.
func (opts *Docker) Validate() error {
	catcher := &erc.Collector{}
	erc.When(catcher, opts.Port < 0, "port must be positive value")
	erc.When(catcher, opts.Image == "", "Docker image must be specified")
	if opts.Platform == "" {
		if PlatformSupportsDocker(runtime.GOOS) {
			opts.Platform = runtime.GOOS
		} else {
			catcher.Add(fmt.Errorf("failed to set default platform to current runtime platform '%s' because it is unsupported", opts.Platform))
		}
	} else if !PlatformSupportsDocker(opts.Platform) {
		catcher.Add(fmt.Errorf("unrecognized platform '%s'", opts.Platform))
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

// Resolve converts the Docker options into options to initialize a Docker
// client.
func (opts *Docker) Resolve() (*client.Client, error) {
	var clientOpts []client.Opt

	if opts.Host != "" && opts.Port > 0 {
		addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port))
		if err != nil {
			return nil, fmt.Errorf("could not resolve Docker daemon address %s:%d: %w", opts.Host, opts.Port, err)
		}
		clientOpts = append(clientOpts, client.WithHost(addr.String()))
	}

	if opts.APIVersion != "" {
		clientOpts = append(clientOpts, client.WithAPIVersionNegotiation())
	} else {
		clientOpts = append(clientOpts, client.WithVersion(opts.APIVersion))
	}

	client, err := client.NewClientWithOpts(clientOpts...)
	if err != nil {
		return nil, fmt.Errorf("could not create Docker client: %w", err)
	}
	return client, nil
}
