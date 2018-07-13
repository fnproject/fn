package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"log"

	fdk "github.com/fnproject/fdk-go"
)

func main() {
	fdk.Handle(fdk.HandlerFunc(myHandler))
}

func myHandler(ctx context.Context, in io.Reader, out io.Writer) {
	var input map[string]interface{}

	body, err := ioutil.ReadAll(in)
	if err != nil {
		log.Print("could not read input")
		fdk.WriteStatus(out, 530)
		return
	}

	err = json.Unmarshal(body, &input)
	if err != nil {
		log.Print("could not unmarshal json input")
		fdk.WriteStatus(out, 531)
		return
	}

	output, err := json.Marshal(&input)
	if err != nil {
		log.Print("could not marshal json output")
		fdk.WriteStatus(out, 532)
		return
	}

	written, err := io.Copy(out, bytes.NewReader(output))
	if err != nil {
		log.Print("could not write output")
		fdk.WriteStatus(out, 533)
		return
	}

	if written != int64(len(output)) {
		log.Print("partial write of output")
		fdk.WriteStatus(out, 534)
		return
	}
}
