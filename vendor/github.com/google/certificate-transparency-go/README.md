This is the beginnings of a certificate transparency log
client written in Go, along with a log scanner tool.

You'll need go v1.8 or higher to compile.

# Installation

This go code must be imported into your go workspace before you can
use it, which can be done with:

```bash
go get github.com/google/certificate-transparency-go/client
go get github.com/google/certificate-transparency-go/scanner
# etc.
```

# Building the binaries

To compile the log scanner run:

```bash
go build github.com/google/certificate-transparency-go/scanner/main/scanner.go
```

# Contributing

When sending pull requests, please ensure that everything's been run
through ```gofmt``` beforehand so we can keep everything nice and
tidy.
