GO_FILES := $(shell find . -type f -name '*.go' -not -path "./Godeps/*" -not -path "./vendor/*")
GO_PACKAGES := $(shell go list ./... | sed "s/github.com\/heroku\/docker-registry-client/./" | grep -v "^./vendor/")

build:
	go build -v $(GO_PACKAGES)

travis: tidy test

test: build
	go fmt $(GO_PACKAGES)
	go test -race -i $(GO_PACKAGES)
	go test -race -v $(GO_PACKAGES)

# Setup & Code Cleanliness
setup: hooks tidy

hooks:
	ln -fs ../../bin/git-pre-commit.sh .git/hooks/pre-commit

tidy: goimports
	./bin/go-version-sync-check.sh
	test -z "$$(goimports -l -d $(GO_FILES) | tee /dev/stderr)"
	go vet $(GO_PACKAGES)

precommit: tidy test

goimports:
	go get golang.org/x/tools/cmd/goimports
