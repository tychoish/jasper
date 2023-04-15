module github.com/tychoish/x/splunk

go 1.20

require (
	github.com/stretchr/testify v1.8.2
	github.com/tychoish/fun v0.8.5
	github.com/tychoish/grip v0.2.4
	github.com/tychoish/grip/x/splunk v0.0.0-20230414135146-97625602a7ee
	github.com/tychoish/jasper v0.0.0-20230415083349-ed64615c7d7e
)

require (
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/fuyufjh/splunk-hec-go v0.4.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.3.0 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	golang.org/x/crypto v0.5.0 // indirect
	golang.org/x/sys v0.7.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)

replace github.com/tychoish/jasper => ../../
