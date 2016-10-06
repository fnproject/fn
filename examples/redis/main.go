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
	Redis     string        `json:"redis"`
	RedisAuth string        `json:"redisAuth"`
	Command   string        `json:"command"`
	Args      []interface{} `json:"args"`
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
	c, err := redis.Dial("tcp", pl.Redis)
	if err != nil {
		log.Println("Failed to dial redis server")
		log.Fatal(err)
		return
	}

	// Authenticate to redis server if exists the password
	if pl.RedisAuth != "" {
		if _, err := c.Do("AUTH", pl.RedisAuth); err != nil {
			log.Println("Failed to authenticate to redis server")
			log.Fatal(err)
			return
		}
	}

	// Check if payload command is valid
	if pl.Command != "GET" && pl.Command != "SET" {
		log.Println("Invalid command")
		return
	}

	// Execute command on redis server
	r, err := c.Do(pl.Command, pl.Args...)
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
