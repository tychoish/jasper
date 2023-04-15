package docker

import (
	"fmt"
	"net"

	"github.com/docker/docker/client"
	"github.com/tychoish/jasper/options"
)

func ResolveOptions(opts *options.Docker) (*client.Client, error) {
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
