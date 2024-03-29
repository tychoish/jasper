package options

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/tychoish/fun/assert"
	"github.com/tychoish/fun/assert/check"
)

func TestCredentials(t *testing.T) {
	pemRootCert := []byte(`
-----BEGIN CERTIFICATE-----
MIIEBDCCAuygAwIBAgIDAjppMA0GCSqGSIb3DQEBBQUAMEIxCzAJBgNVBAYTAlVT
MRYwFAYDVQQKEw1HZW9UcnVzdCBJbmMuMRswGQYDVQQDExJHZW9UcnVzdCBHbG9i
YWwgQ0EwHhcNMTMwNDA1MTUxNTU1WhcNMTUwNDA0MTUxNTU1WjBJMQswCQYDVQQG
EwJVUzETMBEGA1UEChMKR29vZ2xlIEluYzElMCMGA1UEAxMcR29vZ2xlIEludGVy
bmV0IEF1dGhvcml0eSBHMjCCASIwDQYJKoZIhvcNAQEBBQADggEPADCCAQoCggEB
AJwqBHdc2FCROgajguDYUEi8iT/xGXAaiEZ+4I/F8YnOIe5a/mENtzJEiaB0C1NP
VaTOgmKV7utZX8bhBYASxF6UP7xbSDj0U/ck5vuR6RXEz/RTDfRK/J9U3n2+oGtv
h8DQUB8oMANA2ghzUWx//zo8pzcGjr1LEQTrfSTe5vn8MXH7lNVg8y5Kr0LSy+rE
ahqyzFPdFUuLH8gZYR/Nnag+YyuENWllhMgZxUYi+FOVvuOAShDGKuy6lyARxzmZ
EASg8GF6lSWMTlJ14rbtCMoU/M4iarNOz0YDl5cDfsCx3nuvRTPPuj5xt970JSXC
DTWJnZ37DhF5iR43xa+OcmkCAwEAAaOB+zCB+DAfBgNVHSMEGDAWgBTAephojYn7
qwVkDBF9qn1luMrMTjAdBgNVHQ4EFgQUSt0GFhu89mi1dvWBtrtiGrpagS8wEgYD
VR0TAQH/BAgwBgEB/wIBADAOBgNVHQ8BAf8EBAMCAQYwOgYDVR0fBDMwMTAvoC2g
K4YpaHR0cDovL2NybC5nZW90cnVzdC5jb20vY3Jscy9ndGdsb2JhbC5jcmwwPQYI
KwYBBQUHAQEEMTAvMC0GCCsGAQUFBzABhiFodHRwOi8vZ3RnbG9iYWwtb2NzcC5n
ZW90cnVzdC5jb20wFwYDVR0gBBAwDjAMBgorBgEEAdZ5AgUBMA0GCSqGSIb3DQEB
BQUAA4IBAQA21waAESetKhSbOHezI6B1WLuxfoNCunLaHtiONgaX4PCVOzf9G0JY
/iLIa704XtE7JW4S615ndkZAkNoUyHgN7ZVm2o6Gb4ChulYylYbc3GrKBIxbf/a/
zG+FA1jDaFETzf3I93k9mTXwVqO94FntT0QJo544evZG0R0SnU++0ED8Vf4GXjza
HFa9llF7b1cq26KqltyMdMKVvvBulRP/F/A8rLIQjcxz++iPAsbw+zOzlTvjwsto
WHPbqCRiOwY1nQ2pM714A5AuTHhdUDqB1O6gyHA43LL5Z/qHQF1hwFGPa4NrzQU6
yuGnBXj8ytqU0CwIPX4WecigUCAkVDNx
-----END CERTIFICATE-----`)
	jsonRootCert, err := json.Marshal(pemRootCert)
	assert.NotError(t, err)

	pemCert := []byte(`-----BEGIN CERTIFICATE-----
MIIBhTCCASugAwIBAgIQIRi6zePL6mKjOipn+dNuaTAKBggqhkjOPQQDAjASMRAw
DgYDVQQKEwdBY21lIENvMB4XDTE3MTAyMDE5NDMwNloXDTE4MTAyMDE5NDMwNlow
EjEQMA4GA1UEChMHQWNtZSBDbzBZMBMGByqGSM49AgEGCCqGSM49AwEHA0IABD0d
7VNhbWvZLWPuj/RtHFjvtJBEwOkhbN/BnnE8rnZR8+sbwnc/KhCk3FhnpHZnQz7B
5aETbbIgmuvewdjvSBSjYzBhMA4GA1UdDwEB/wQEAwICpDATBgNVHSUEDDAKBggr
BgEFBQcDATAPBgNVHRMBAf8EBTADAQH/MCkGA1UdEQQiMCCCDmxvY2FsaG9zdDo1
NDUzgg4xMjcuMC4wLjE6NTQ1MzAKBggqhkjOPQQDAgNIADBFAiEA2zpJEPQyz6/l
Wf86aX6PepsntZv2GYlA5UpabfT2EZICICpJ5h/iI+i341gBmLiAFQOyTDT+/wQc
6MF9+Yw1Yy0t
-----END CERTIFICATE-----`)
	jsonCert, err := json.Marshal(pemCert)
	assert.NotError(t, err)

	pemKey := []byte(`-----BEGIN EC PRIVATE KEY-----
MHcCAQEEIIrYSSNQFaA2Hwf1duRSxKtLYX5CB04fSeQ6tF1aY/PuoAoGCCqGSM49
AwEHoUQDQgAEPR3tU2Fta9ktY+6P9G0cWO+0kETA6SFs38GecTyudlHz6xvCdz8q
EKTcWGekdmdDPsHloRNtsiCa697B2O9IFA==
-----END EC PRIVATE KEY-----
`)
	jsonKey, err := json.Marshal(pemKey)
	assert.NotError(t, err)

	makeFile := func(t *testing.T) *os.File {
		file, err := os.CreateTemp("", "creds")
		assert.NotError(t, err)
		return file
	}

	for testName, testCase := range map[string]func(t *testing.T){
		"NewCredeentialsNonexistentFile": func(t *testing.T) {
			creds, err := NewCredentialsFromFile("nonexistent_file")
			check.Error(t, err)
			check.Zero(t, creds)
		},
		"NewCredentialsEmptyFile": func(t *testing.T) {
			file := makeFile(t)
			defer func() {
				check.NotError(t, file.Close())
				check.NotError(t, os.RemoveAll(file.Name()))
			}()
			creds, err := NewCredentialsFromFile(file.Name())
			check.Error(t, err)
			check.Zero(t, creds)
		},
		"NewCredentialsMissingFields": func(t *testing.T) {
			file := makeFile(t)
			defer func() {
				check.NotError(t, file.Close())
				check.NotError(t, os.RemoveAll(file.Name()))
			}()
			_, err := file.Write([]byte(fmt.Sprintf(`{
				"ca_cert": %s,
				"cert": %s
			}`, jsonRootCert, jsonCert)))
			assert.NotError(t, err)
			_, err = NewCredentialsFromFile(file.Name())
			check.Error(t, err)
		},
		"NewCredentialsSucceeds": func(t *testing.T) {
			file := makeFile(t)
			defer func() {
				check.NotError(t, file.Close())
				check.NotError(t, os.RemoveAll(file.Name()))
			}()
			_, err := file.Write([]byte(fmt.Sprintf(`{
				"ca_cert": %s,
				"cert": %s,
				"key": %s
			}`, jsonRootCert, jsonCert, jsonKey)))
			assert.NotError(t, err)
			creds, err := NewCredentialsFromFile(file.Name())
			assert.NotError(t, err)
			assert.True(t, creds != nil)
			check.Equal(t, string(pemRootCert), string(creds.CACert))
			check.Equal(t, string(pemCert), string(creds.Cert))
			check.Equal(t, string(pemKey), string(creds.Key))
		},
		"Export": func(t *testing.T) {
			creds := &CertificateCredentials{
				CACert: pemRootCert,
				Cert:   pemCert,
				Key:    pemKey,
			}
			credBytes, err := creds.Export()
			assert.NotError(t, err)
			check.True(t, bytes.Contains(credBytes, jsonRootCert))
			check.True(t, bytes.Contains(credBytes, jsonCert))
			check.True(t, bytes.Contains(credBytes, jsonKey))
		},
		"ResolveInvalidCert": func(t *testing.T) {
			creds := &CertificateCredentials{
				CACert: []byte("foo"),
				Cert:   pemCert,
				Key:    pemKey,
			}
			config, err := creds.Resolve()
			check.Error(t, err)
			check.Zero(t, config)
		},
		"ResolveMissingFields": func(t *testing.T) {
			creds := &CertificateCredentials{
				CACert: pemRootCert,
				Cert:   pemCert,
			}
			config, err := creds.Resolve()
			check.Error(t, err)
			check.Zero(t, config)
		},
		"ResolveSucceeds": func(t *testing.T) {
			creds := &CertificateCredentials{
				CACert: pemRootCert,
				Cert:   pemCert,
				Key:    pemKey,
			}
			config, err := creds.Resolve()
			assert.NotError(t, err)
			check.True(t, config != nil)
		},
	} {
		t.Run(testName, func(t *testing.T) {
			testCase(t)
		})
	}
}
