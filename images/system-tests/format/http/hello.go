package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
)

type Person struct {
	Name string
}

func main() {
	req, err := http.ReadRequest(bufio.NewReader(os.Stdin))
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
	}
	in, _ := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	p := &Person{}
	if len(in) != 0 {
		json.NewDecoder(bytes.NewReader(in)).Decode(p)
	}
	if p.Name == "" {
		p.Name = "World"
	}
	res := fmt.Sprintf("Hello %s!", p.Name)

	buf := bytes.NewBufferString(res)

	r := http.Response{
		Body:          ioutil.NopCloser(buf),
		StatusCode:    http.StatusOK,
		ContentLength: int64(buf.Len()),
		ProtoMajor:    1,
		ProtoMinor:    1,
	}
	r.Write(os.Stdout)
}
