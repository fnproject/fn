package main

import (
	"fmt"
	"github.com/gorilla/mux"
	"log"
	"net/http"
)

func main() {

	r := mux.NewRouter()
	r.HandleFunc("/", Hello)

	http.Handle("/", r)
	log.Fatal(http.ListenAndServe(":8081", nil))
}

func Hello(w http.ResponseWriter, req *http.Request) {
	fmt.Fprintln(w, "Hello world!")
}
