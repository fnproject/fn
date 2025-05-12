module github.com/fnproject/fn

replace cloud.google.com/go => github.com/google/go-cloud v0.4.1-0.20181025204856-f29236cc19de

require (
	contrib.go.opencensus.io/exporter/jaeger v0.1.0
	contrib.go.opencensus.io/exporter/prometheus v0.1.0
	contrib.go.opencensus.io/exporter/zipkin v0.1.1
	github.com/coreos/go-semver v0.2.1-0.20180108230905-e214231b295a
	github.com/dchest/siphash v1.2.0
	github.com/fnproject/fdk-go v0.0.0-20181025170718-26ed643bea68
	github.com/fsnotify/fsnotify v1.4.7
	github.com/fsouza/go-dockerclient v1.4.0
	github.com/gin-contrib/cors v0.0.0-20170318125340-cf4846e6a636
	github.com/gin-gonic/gin v1.3.0
	github.com/go-sql-driver/mysql v1.4.0
	github.com/golang/groupcache v0.0.0-20180924190550-6f2cf27854a4
	github.com/golang/protobuf v1.3.1
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/leanovate/gopter v0.2.2
	github.com/lib/pq v1.0.0
	github.com/mattn/go-sqlite3 v1.14.28
	github.com/openzipkin/zipkin-go v0.1.6
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.8.1
	github.com/prometheus/client_golang v0.9.3-0.20190127221311-3c4408c8b829
	github.com/sirupsen/logrus v1.4.1
	github.com/stretchr/testify v1.3.0
	go.opencensus.io v0.22.1-0.20190619184131-df42942ad08f
	golang.org/x/net v0.0.0-20190501004415-9ce7a6920f09
	golang.org/x/sys v0.0.0-20190507160741-ecd444e8653b
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	google.golang.org/grpc v1.20.1
)

require (
	github.com/Azure/go-ansiterm v0.0.0-20170929234023-d6e3b3328b78 // indirect
	github.com/Microsoft/go-winio v0.4.14 // indirect
	github.com/apache/thrift v0.12.0 // indirect
	github.com/beorn7/perks v0.0.0-20180321164747-3a771d992973 // indirect
	github.com/containerd/continuity v0.0.0-20181203112020-004b46473808 // indirect
	github.com/davecgh/go-spew v1.1.1 // indirect
	github.com/docker/docker v0.7.3-0.20190309235953-33c3200e0d16 // indirect
	github.com/docker/go-connections v0.4.0 // indirect
	github.com/docker/go-units v0.3.3 // indirect
	github.com/gin-contrib/sse v0.0.0-20170109093832-22d885f9ecc7 // indirect
	github.com/gogo/protobuf v1.2.1 // indirect
	github.com/hashicorp/golang-lru v0.5.1 // indirect
	github.com/ijc/Gotty v0.0.0-20170406111628-a8b993ba6abd // indirect
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/konsorten/go-windows-terminal-sequences v1.0.1 // indirect
	github.com/mattn/go-isatty v0.0.4 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.1 // indirect
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/opencontainers/go-digest v1.0.0-rc1 // indirect
	github.com/opencontainers/image-spec v1.0.1 // indirect
	github.com/opencontainers/runc v0.1.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	github.com/prometheus/client_model v0.0.0-20190115171406-56726106282f // indirect
	github.com/prometheus/common v0.2.0 // indirect
	github.com/prometheus/procfs v0.0.0-20190117184657-bf6a532e95b1 // indirect
	github.com/stretchr/objx v0.1.1 // indirect
	github.com/ugorji/go/codec v0.0.0-20181022190402-e5e69e061d4f // indirect
	golang.org/x/sync v0.0.0-20190227155943-e225da77a7e6 // indirect
	golang.org/x/text v0.3.2 // indirect
	google.golang.org/api v0.3.2 // indirect
	google.golang.org/appengine v1.4.0 // indirect
	google.golang.org/genproto v0.0.0-20190425155659-357c62f0e4bb // indirect
	gopkg.in/go-playground/assert.v1 v1.2.1 // indirect
	gopkg.in/go-playground/validator.v8 v8.18.2 // indirect
	gopkg.in/yaml.v2 v2.2.2 // indirect
)

go 1.24
