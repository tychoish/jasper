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
	github.com/tychoish/jasper/x/remote v0.1.1
)

replace github.com/tychoish/jasper/x/remote => ../remote/

replace github.com/tychoish/jasper => ../../

require (
	github.com/VividCortex/ewma v1.1.1 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/fuyufjh/splunk-hec-go v0.4.0 // indirect
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/golang/protobuf v1.5.4 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/google/go-cmp v0.7.0 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/gorilla/mux v1.8.1 // indirect
	github.com/kr/pretty v0.3.1 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/phyber/negroni-gzip v1.0.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/tychoish/birch v0.4.0 // indirect
	github.com/tychoish/birch/x/ftdc v0.1.0 // indirect
	github.com/tychoish/gimlet v0.0.0-20260131191610-eac0aaade579 // indirect
	github.com/tychoish/grip/x/metrics v0.1.0 // indirect
	github.com/tychoish/grip/x/splunk v0.1.0 // indirect
	github.com/tychoish/jasper/x/splunk v0.1.0 // indirect
	github.com/ulikunitz/xz v0.5.15 // indirect
	github.com/urfave/negroni v1.0.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/net v0.49.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.33.0 // indirect
	gonum.org/v1/gonum v0.16.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260128011058-8636f8732409 // indirect
	google.golang.org/grpc v1.78.0 // indirect
	google.golang.org/protobuf v1.36.11 // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
