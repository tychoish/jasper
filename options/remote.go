package options

import (
	"fmt"
	"io/ioutil"
	"time"

	"github.com/tychoish/emt"
	"golang.org/x/crypto/ssh"
)

// RemoteConfig represents the arguments to connect to a remote host.
type RemoteConfig struct {
	Host string `bson:"host" json:"host"`
	User string `bson:"user" json:"user"`

	// Args to the SSH binary. Only used by if UseSSHLibrary is false.
	Args []string `bson:"args,omitempty" json:"args,omitempty"`

	// Determines whether to use the SSH binary or the SSH library.
	UseSSHLibrary bool `bson:"use_ssh_library,omitempty" json:"use_ssh_library,omitempty"`

	// The following apply only if UseSSHLibrary is true.
	Port          int    `bson:"port,omitempty" json:"port,omitempty"`
	Key           string `bson:"key,omitempty" json:"key,omitempty"`
	KeyFile       string `bson:"key_file,omitempty" json:"key_file,omitempty"`
	KeyPassphrase string `bson:"key_passphrase,omitempty" json:"key_passphrase,omitempty"`
	Password      string `bson:"password,omitempty" json:"password,omitempty"`
	// Connection timeout
	Timeout time.Duration `bson:"timeout,omitempty" json:"timeout,omitempty"`
}

// Remote represents options to SSH into a remote machine.
type Remote struct {
	RemoteConfig `bson:"remote_config" json:"remote_config" yaml:"remote_config"`
	Proxy        *Proxy `bson:"proxy" json:"proxy" yaml:"proxy,omitempty"`
}

// Copy returns a copy of the options for only the exported fields.
func (opts *Remote) Copy() *Remote {
	optsCopy := *opts
	if opts.Proxy != nil {
		optsCopy.Proxy = opts.Proxy.Copy()
	}
	return &optsCopy
}

// Proxy represents the remote configuration to access a remote proxy machine.
type Proxy struct {
	RemoteConfig `bson:"remote_config,inline" json:"remote_config" yaml:"remote_config"`
}

// Copy returns a copy of the options for only the exported fields.
func (opts *Proxy) Copy() *Proxy {
	optsCopy := *opts
	return &optsCopy
}

const defaultSSHPort = 22

func (opts *RemoteConfig) validate() error {
	catcher := emt.NewBasicCatcher()
	if opts.Host == "" {
		catcher.New("host cannot be empty")
	}
	if opts.Port == 0 {
		opts.Port = defaultSSHPort
	}

	if !opts.UseSSHLibrary {
		return catcher.Resolve()
	}

	numAuthMethods := 0
	for _, authMethod := range []string{opts.Key, opts.KeyFile, opts.Password} {
		if authMethod != "" {
			numAuthMethods++
		}
	}
	if numAuthMethods != 1 {
		catcher.Errorf("must specify exactly one authentication method, found %d", numAuthMethods)
	}
	if opts.Key == "" && opts.KeyFile == "" && opts.KeyPassphrase != "" {
		catcher.New("cannot set passphrase without specifying key or key file")
	}
	return catcher.Resolve()
}

func (opts *RemoteConfig) resolve() (*ssh.ClientConfig, error) {
	var auth []ssh.AuthMethod
	if opts.Key != "" || opts.KeyFile != "" {
		pubkey, err := opts.publicKeyAuth()
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

func (opts *RemoteConfig) publicKeyAuth() (ssh.AuthMethod, error) {
	var key []byte
	if opts.KeyFile != "" {
		var err error
		key, err = ioutil.ReadFile(opts.KeyFile)
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

// Validate ensures that enough information is provided to connect to a remote
// host.
func (opts *Remote) Validate() error {
	catcher := emt.NewBasicCatcher()

	if opts.Proxy != nil {
		catcher.Add(opts.Proxy.validate())
	}

	catcher.Add(opts.validate())

	return catcher.Resolve()
}

func (opts *Remote) String() string {
	if opts.User == "" {
		return opts.Host
	}

	return fmt.Sprintf("%s@%s", opts.User, opts.Host)
}

// Resolve returns the SSH client and session from the options.
func (opts *Remote) Resolve() (*ssh.Client, *ssh.Session, error) {
	if err := opts.Validate(); err != nil {
		return nil, nil, fmt.Errorf("invalid remote options: %w", err)
	}

	var client *ssh.Client
	if opts.Proxy != nil {
		proxyConfig, err := opts.Proxy.resolve()
		if err != nil {
			return nil, nil, fmt.Errorf("could not create proxy config: %w", err)
		}
		proxyClient, err := ssh.Dial("tcp", fmt.Sprintf("%s:%d", opts.Proxy.Host, opts.Proxy.Port), proxyConfig)
		if err != nil {
			return nil, nil, fmt.Errorf("could not dial proxy: %w", err)
		}

		targetConn, err := proxyClient.Dial("tcp", fmt.Sprintf("%s:%d", opts.Host, opts.Port))
		if err != nil {
			catcher := emt.NewBasicCatcher()
			catcher.Add(proxyClient.Close())
			catcher.Errorf("could not dial target host: %w", err)
			return nil, nil, catcher.Resolve()
		}

		targetConfig, err := opts.resolve()
		if err != nil {
			catcher := emt.NewBasicCatcher()
			catcher.Add(proxyClient.Close())
			catcher.Errorf("could not create target config: %w", err)
			return nil, nil, catcher.Resolve()
		}
		gatewayConn, chans, reqs, err := ssh.NewClientConn(targetConn, fmt.Sprintf("%s:%d", opts.Host, opts.Port), targetConfig)
		if err != nil {
			catcher := emt.NewBasicCatcher()
			catcher.Add(targetConn.Close())
			catcher.Add(proxyClient.Close())
			catcher.Errorf("could not establish connection to target via proxy: %w", err)
			return nil, nil, catcher.Resolve()
		}
		client = ssh.NewClient(gatewayConn, chans, reqs)
	} else {
		var err error
		config, err := opts.resolve()
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
		catcher := emt.NewBasicCatcher()
		catcher.Add(client.Close())
		catcher.Add(err)
		return nil, nil, fmt.Errorf("could not establish session: %w", catcher.Resolve())
	}
	return client, session, nil
}
