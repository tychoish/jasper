module github.com/tychoish/jasper/x/jamboy

go 1.24.0

require (
	github.com/google/uuid v1.6.0
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/stretchr/testify v1.11.1
	github.com/tychoish/amboy v0.1.0
	github.com/tychoish/fun v0.14.5
	github.com/tychoish/grip v0.4.6
	github.com/tychoish/jasper v0.1.4
	github.com/tychoish/jasper/x/remote v0.0.0-20230502230321-07d6256076b2
)

replace github.com/tychoish/jasper/x/remote => ../remote/

replace github.com/tychoish/jasper => ../../

require (
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
