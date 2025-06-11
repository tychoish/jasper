package options

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"os"

	"github.com/tychoish/fun/erc"
	"github.com/tychoish/fun/ers"
)

// CertificateCredentials represent a bundle of assets for doing TLS
// authentication.
type CertificateCredentials struct {
	// CACert is the PEM-encoded client CA certificate. If the credentials are
	// used by a client, this should be the certificate of the root CA to verify
	// the server certificate. If the credentials are used by a server, this
	// should be the certificate of the root CA to verify the client
	// certificate.
	CACert []byte `bson:"ca_cert" json:"ca_cert" yaml:"ca_cert"`
	// Cert is the PEM-encoded certificate.
	Cert []byte `bson:"cert" json:"cert" yaml:"cert"`
	// Key is the PEM-encoded private key.
	Key []byte `bson:"key" json:"key" yaml:"key"`

	// ServerName is the name of the service being contacted.
	ServerName string `bson:"server_name" json:"server_name" yaml:"server_name"`
}

// NewCredentials initializes a new Credential struct.
func NewCredentials(caCert, cert, key []byte) (*CertificateCredentials, error) {
	creds := &CertificateCredentials{
		CACert: caCert,
		Cert:   cert,
		Key:    key,
	}

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	return creds, nil
}

// NewCredentialsFromFile parses the PEM-encoded credentials in JSON format in
// the file at path into a Credentials struct.
func NewCredentialsFromFile(path string) (*CertificateCredentials, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("error reading credentials file: %w", err)
	}

	creds := CertificateCredentials{}
	if err := json.Unmarshal(contents, &creds); err != nil {
		return nil, fmt.Errorf("error unmarshalling contents of credentials file: %w", err)
	}

	if err := creds.Validate(); err != nil {
		return nil, fmt.Errorf("read invalid credentials from file: %w", err)
	}

	return &creds, nil
}

// Validate checks that the Credentials are all set to non-empty values.
func (c *CertificateCredentials) Validate() error {
	catcher := &erc.Collector{}

	catcher.When(len(c.CACert) == 0, ers.Error("CA certificate should not be empty"))
	catcher.When(len(c.Cert) == 0, ers.Error("certificate should not be empty"))
	catcher.When(len(c.Key) == 0, ers.Error("key should not be empty"))

	return catcher.Resolve()
}

// Resolve converts the Credentials struct into a tls.Config.
func (c *CertificateCredentials) Resolve() (*tls.Config, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	caCerts := x509.NewCertPool()
	if !caCerts.AppendCertsFromPEM(c.CACert) {
		return nil, errors.New("failed to append client CA certificate")
	}

	cert, err := tls.X509KeyPair(c.Cert, c.Key)
	if err != nil {
		return nil, fmt.Errorf("problem loading key pair: %w", err)
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},

		// Server-specific options
		ClientAuth: tls.RequireAndVerifyClientCert,
		ClientCAs:  caCerts,

		// Client-specific options
		RootCAs:    caCerts,
		ServerName: c.ServerName,
	}, nil
}

// Export exports the Credentials struct into JSON-encoded bytes.
func (c *CertificateCredentials) Export() ([]byte, error) {
	if err := c.Validate(); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", err)
	}

	b, err := json.Marshal(c)
	if err != nil {
		return nil, fmt.Errorf("error exporting credentials: %w", err)
	}

	return b, nil
}
