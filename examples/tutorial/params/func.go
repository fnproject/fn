package main

import (
	"fmt"
	"log"
	"net/url"
	"os"
)

func main() {
	s := os.Getenv("FN_REQUEST_URL")

	fmt.Printf("FN_REQUEST_URL --> %v\n\n", s)

	u, err := url.Parse(s)
	if err != nil {
		log.Fatal(err)
	}

	m, _ := url.ParseQuery(u.RawQuery)

	if len(m) == 0 {
		fmt.Println("Try adding some URL params like &id=123")
	} else {
		for k, v := range m {
			fmt.Printf("found param: %v, val: %v\n", k, v[0])
		}
	}

}
