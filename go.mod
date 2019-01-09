module github.com/fnproject/fn

replace cloud.google.com/go => github.com/google/go-cloud v0.4.1-0.20181025204856-f29236cc19de

require (
	github.com/aws/aws-sdk-go v1.15.57
	github.com/boltdb/bolt v0.0.0-20170907202052-fa5367d20c99
	github.com/containerd/continuity v0.0.0-20181003075958-be9bd761db19 // indirect
	github.com/coreos/go-semver v0.2.1-0.20180108230905-e214231b295a
	github.com/dchest/siphash v1.2.0
	github.com/docker/docker v0.7.3-0.20180904180028-c3e32938430e // indirect
	github.com/fnproject/fdk-go v0.0.0-20181025170718-26ed643bea68
	github.com/fsnotify/fsnotify v1.4.7
	github.com/fsouza/go-dockerclient v1.3.2-0.20181102000906-311c4469e611
	github.com/garyburd/redigo v1.6.0
	github.com/gin-contrib/cors v0.0.0-20170318125340-cf4846e6a636
	github.com/gin-contrib/sse v0.0.0-20170109093832-22d885f9ecc7 // indirect
	github.com/gin-gonic/gin v1.3.0
	github.com/go-sql-driver/mysql v1.4.0
	github.com/golang/protobuf v1.2.1-0.20190109072247-347cf4a86c1c
	github.com/google/btree v0.0.0-20180813153112-4030bb1f1f0c
	github.com/grpc-ecosystem/go-grpc-middleware v1.0.0
	github.com/jmoiron/sqlx v1.2.0
	github.com/json-iterator/go v1.1.5 // indirect
	github.com/leanovate/gopter v0.2.2
	github.com/lib/pq v1.0.0
	github.com/mattn/go-isatty v0.0.4 // indirect
	github.com/mattn/go-sqlite3 v1.9.0
	github.com/modern-go/concurrent v0.0.0-20180306012644-bacd9c7ef1dd // indirect
	github.com/modern-go/reflect2 v0.0.0-20180701023420-4b7aa43c6742 // indirect
	github.com/openzipkin/zipkin-go v0.1.2
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/prometheus/client_golang v0.9.0
	github.com/prometheus/common v0.0.0-20181019103554-16b4535ad14a // indirect
	github.com/sirupsen/logrus v1.1.1
	github.com/ugorji/go/codec v0.0.0-20181022190402-e5e69e061d4f // indirect
	go.opencensus.io v0.17.0
	golang.org/x/net v0.0.0-20181017193950-04a2e542c03f
	golang.org/x/sys v0.0.0-20181019160139-8e24a49d80f8
	golang.org/x/time v0.0.0-20180412165947-fbb02b2291d2
	google.golang.org/api v0.0.0-20181019000435-7fb5a8353b60 // indirect
	google.golang.org/grpc v1.16.0
	gopkg.in/go-playground/validator.v8 v8.18.2 // indirect
)

replace (
	github.com/Azure/go-ansiterm => ./noop
	github.com/Microsoft/go-winio => ./noop
)
