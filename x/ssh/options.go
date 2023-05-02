package ssh

import (
	"fmt"
	"os"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/jasper/options"
	"golang.org/x/crypto/ssh"
)

func resolveClientConfig(opts *options.RemoteConfig) (*ssh.ClientConfig, error) {
	var auth []ssh.AuthMethod
	if opts.Key != "" || opts.KeyFile != "" {
		pubkey, err := resolveAuth(opts)
		if err != nil {
			return nil, fmt.Errorf("could not get public key: %w", err)
		}
		auth = append(auth, pubkey)
	}
	if opts.Password != "" {
		auth = append(auth, ssh.Password(opts.Password))
	}
	return &ssh.ClientConfig{
		Timeout:         opts.Timeout,
		User:            opts.User,
		Auth:            auth,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}, nil
}

func resolveAuth(opts *options.RemoteConfig) (ssh.AuthMethod, error) {
	var key []byte
	if opts.KeyFile != "" {
		var err error
		key, err = os.ReadFile(opts.KeyFile)
		if err != nil {
			return nil, fmt.Errorf("could not read key file: %w", err)
		}
	} else {
		key = []byte(opts.Key)
	}

	var signer ssh.Signer
	var err error
	if opts.KeyPassphrase != "" {
		signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(opts.KeyPassphrase))
	} else {
		signer, err = ssh.ParsePrivateKey(key)
	}
	if err != nil {
		return nil, fmt.Errorf("could not get signer: %w", err)
	}
	return ssh.PublicKeys(signer), nil
}

// Resolve returns the SSH client and session from the options.
func resolveClient(opts *options.Remote) (*ssh.Client, *ssh.Session, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid remote options: %w", err)
	}

	var client *ssh.Client
	if opts.Proxy != nil {
		proxyConfig, err := resolveClientConfig(&opts.Proxy.RemoteConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("could not create proxy config: %w", err)
		}
		proxyClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", opts.Proxy.Host, opts.Proxy.Port), proxyConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("could not dial proxy: %w", err)
		}

		targetConn, err := proxyClient.Dial("tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port))
		if err != nil {
			catcher := &erc.Collector{}
			catcher.Add(proxyClient.Close())
			catcher.Add(fmt.Errorf("could not dial target host: %w", err))
			return nil, nil, catcher.Resolve()
		}

		targetConfig, err := resolveClientConfig(&opts.RemoteConfig)
		if err != nil {
			catcher := &erc.Collector{}
			catcher.Add(proxyClient.Close())
			catcher.Add(fmt.Errorf("could not create target config: %w", err))
			return nil, nil, catcher.Resolve()
		}
		gatewayConn, chans, reqs, err := ssh.NewClientConn(targetConn, fmt.Sprintf("%s:%d", opts.Host, opts.Port), targetConfig)
		if err != nil {
			catcher := &erc.Collector{}
			catcher.Add(targetConn.Close())
			catcher.Add(proxyClient.Close())
			catcher.Add(fmt.Errorf("could not establish connection to target via proxy: %w", err))
			return nil, nil, catcher.Resolve()
		}
		client = ssh.NewClient(gatewayConn, chans, reqs)
	} else {
		var err error
		config, err := resolveClientConfig(&opts.RemoteConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("could not create config: %w", err)
		}
		client, err = ssh.Dial("tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port), config)
		if err != nil {
			return nil, nil, fmt.Errorf("could not dial host: %w", err)
		}
	}

	session, err := client.NewSession()
	if err != nil {
		catcher := &erc.Collector{}
		catcher.Add(client.Close())
		catcher.Add(err)
		return nil, nil, fmt.Errorf("could not establish session: %w", catcher.Resolve())
	}
	return client, session, nil
}
