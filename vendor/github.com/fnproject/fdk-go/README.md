# Go Fn Development Kit (FDK)

[![GoDoc](https://godoc.org/github.com/fnproject/fdk-go?status.svg)](https://godoc.org/github.com/fnproject/fdk-go)

fdk-go provides convenience functions for writing Go fn code

For getting started with fn, please refer to https://github.com/fnproject/fn/blob/master/README.md

# Installing fdk-go

```sh
go get github.com/fnproject/fdk-go
```

or your favorite vendoring solution :)

# Examples

For a simple getting started, see the [examples](/examples/hello) and follow
the [README](/examples/README.md). If you already have `fn` set up it
will take 2 minutes!

# Advanced example

TODO going to move to [examples](examples/) too :)
TODO make these `_example.go` instead of in markdown ;)

```go
package main

import (
  "context"
  "fmt"
  "io"
  "encoding/json"
  
  fdk "github.com/fnproject/fdk-go"
  "net/http"
)

func main() {
  fdk.Handle(fdk.HandlerFunc(myHandler))
}

// TODO make http.Handler example

func myHandler(ctx context.Context, in io.Reader, out io.Writer) {
  fnctx, ok := fdk.GetContext(ctx).(fdk.HTTPContext)
  if !ok {
    // optionally, this may be a good idea
    fdk.WriteStatus(out, 400)
    fdk.SetHeader(out, "Content-Type", "application/json")
    io.WriteString(out, `{"error":"function not invoked via http trigger"}`)
    return
  }

  contentType := fnctx.Header().Get("Content-Type")
  if contentType != "application/json" {
    // can assert content type for your api this way
    fdk.WriteStatus(out, 400)
    fdk.SetHeader(out, "Content-Type", "application/json")
    io.WriteString(out, `{"error":"invalid content type"}`)
    return
  }

  if fnctx.RequestMethod() != "PUT" {
    // can assert certain request methods for certain endpoints
    fdk.WriteStatus(out, 404)
    fdk.SetHeader(out, "Content-Type", "application/json")
    io.WriteString(out, `{"error":"route not found, method not supported"}`)
    return
  }

  var person struct {
    Name string `json:"name"`
  }
  json.NewDecoder(in).Decode(&person)

  // this is where you might insert person into a database or do something else

  all := struct {
    Name   string            `json:"name"`
    URL    string            `json:"url"`
    Header http.Header       `json:"header"`
    Config map[string]string `json:"config"`
  }{
    Name:   person.Name,
    URL:    fnctx.RequestURL(),
    Header: fnctx.Header(),
    Config: fnctx.Config(),
  }

  // you can write your own headers & status, if you'd like to
  fdk.SetHeader(out, "Content-Type", "application/json")
  fdk.WriteStatus(out, 201)
  json.NewEncoder(out).Encode(&all)
}
```
