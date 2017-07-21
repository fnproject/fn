FROM funcy/go-dind

ADD . /go/src/gitlab-odx.oracle.com/odx/functions
WORKDIR /go/src/gitlab-odx.oracle.com/odx/functions

CMD ["/bin/sh", "-c", "set -o pipefail && ./api_test.sh sqlite3"]
