package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/garyburd/redigo/redis"
)

type payload struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func main() {
	// Getting stdin
	plStdin, _ := ioutil.ReadAll(os.Stdin)

	// Transforming JSON to a *payload
	var pl payload
	err := json.Unmarshal(plStdin, &pl)
	if err != nil {
		log.Println("Invalid payload")
		log.Fatal(err)
		return
	}

	// Dialing redis server
	c, err := redis.Dial("tcp", os.Getenv("CONFIG_SERVER"))
	if err != nil {
		log.Println("Failed to dial redis server")
		log.Fatal(err)
		return
	}

	// Authenticate to redis server if exists the password
	if os.Getenv("CONFIG_REDIS_AUTH") != "" {
		if _, err := c.Do("AUTH", os.Getenv("CONFIG_REDIS_AUTH")); err != nil {
			log.Println("Failed to authenticate to redis server")
			log.Fatal(err)
			return
		}
	}

	// Check if payload command is valid
	if os.Getenv("CONFIG_COMMAND") != "GET" && os.Getenv("CONFIG_COMMAND") != "SET" {
		log.Println("Invalid command")
		return
	}

	// Execute command on redis server
	var r interface{}
	if os.Getenv("CONFIG_COMMAND") == "GET" {
		r, err = c.Do("GET", pl.Key)
	} else if os.Getenv("CONFIG_COMMAND") == "SET" {
		r, err = c.Do("SET", pl.Key, pl.Value)
	}

	if err != nil {
		log.Println("Failed to execute command on redis server")
		log.Fatal(err)
		return
	}

	// Convert reply to string
	res, err := redis.String(r, err)
	if err != nil {
		return
	}

	// Print reply
	fmt.Println(res)
}
