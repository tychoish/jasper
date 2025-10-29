package options

import (
	"errors"
	"fmt"
	"time"

	"github.com/tychoish/fun/erc"
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
	catcher := &erc.Collector{}
	if opts.Host == "" {
		catcher.Push(errors.New("host cannot be empty"))
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
		catcher.Push(fmt.Errorf("must specify exactly one authentication method, found %d", numAuthMethods))
	}
	if opts.Key == "" && opts.KeyFile == "" && opts.KeyPassphrase != "" {
		catcher.Push(errors.New("cannot set passphrase without specifying key or key file"))
	}
	return catcher.Resolve()
}

// Validate ensures that enough information is provided to connect to a remote
// host.
func (opts *Remote) Validate() error {
	catcher := &erc.Collector{}

	if opts.Proxy != nil {
		catcher.Push(opts.Proxy.validate())
	}

	catcher.Push(opts.validate())

	return catcher.Resolve()
}

func (opts *Remote) String() string {
	if opts.User == "" {
		return opts.Host
	}

	return fmt.Sprintf("%s@%s", opts.User, opts.Host)
}
