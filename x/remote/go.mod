module github.com/tychoish/jasper/x/remote

go 1.24.0

toolchain go1.24.3

require (
	github.com/golang/protobuf v1.5.3
	github.com/mholt/archiver v3.1.1+incompatible
	github.com/tychoish/birch v0.4.0
	github.com/tychoish/fun v0.14.5
	github.com/tychoish/gimlet v0.0.0-20251028182000-6a35909ebafc
	github.com/tychoish/grip v0.4.6
	github.com/tychoish/grip/x/metrics v0.0.0-20260114024627-63e5c6d2f062
	github.com/tychoish/grip/x/splunk v0.0.0-20260114024627-63e5c6d2f062
	github.com/tychoish/jasper v0.1.4-0.20260114025018-121022e9c9e2
	github.com/tychoish/jasper/x/splunk v0.0.0-20260114025018-121022e9c9e2
	google.golang.org/grpc v1.54.0
	google.golang.org/protobuf v1.36.11
)

replace github.com/tychoish/jasper/x/splunk => ../splunk/

replace github.com/tychoish/jasper => ../../

require (
	github.com/dsnet/compress v0.0.1 // indirect
	github.com/frankban/quicktest v1.14.5 // indirect
	github.com/fuyufjh/splunk-hec-go v0.4.0 // indirect
	github.com/go-chi/chi v4.1.2+incompatible // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/golang/snappy v0.0.4 // indirect
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/nwaples/rardecode v1.1.3 // indirect
	github.com/phyber/negroni-gzip v1.0.0 // indirect
	github.com/pierrec/lz4 v2.6.1+incompatible // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/rogpeppe/go-internal v1.10.0 // indirect
	github.com/rs/cors v1.8.3 // indirect
	github.com/shirou/gopsutil v3.21.11+incompatible // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/tychoish/birch/x/ftdc v0.0.0-20260131172350-b2ec5200a119 // indirect
	github.com/ulikunitz/xz v0.5.11 // indirect
	github.com/urfave/negroni v1.0.0 // indirect
	github.com/xi2/xz v0.0.0-20171230120015-48954b6210f8 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/net v0.9.0 // indirect
	golang.org/x/sys v0.40.0 // indirect
	golang.org/x/text v0.9.0 // indirect
	google.golang.org/genproto v0.0.0-20230110181048-76db0878b65f // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
)
