package main

import (
	"context"
	"encoding/json"
	"io"

	"fmt"
	"github.com/fnproject/fdk-go"
)

func myHandler(ctx context.Context, in io.Reader, out io.Writer) {
	fnctx := fdk.Context(ctx)
	fdk.SetHeader(out, "Content-Type", "application/json")

	contentType := fnctx.Header.Get("Content-Type")
	if contentType != "application/json" {
		fdk.WriteStatus(out, 400)
		io.WriteString(out, `{"error":"invalid content type"}`)
		return
	}

	var person struct {
		Name string `json:"name"`
	}
	err := json.NewDecoder(in).Decode(&person)
	if err != nil {
		fdk.WriteStatus(out, 500)
		io.WriteString(out, err.Error())
		return
	}

	fdk.WriteStatus(out, 200)

	name := ""
	if person.Name != "" {
		name = person.Name
	} else {
		name = "World"
	}

	_, err = io.WriteString(out, fmt.Sprintf("Hello %v", name))
	if err != nil {
		fdk.WriteStatus(out, 500)
		io.WriteString(out, err.Error())
		return
	}
}

func main() {
	fdk.Handle(fdk.HandlerFunc(myHandler))
}
