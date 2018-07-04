package main

import (
	"encoding/json"
	"os"

	"github.com/sirupsen/logrus"
)

type person struct {
	Name string
}

func main() {
	p := &person{Name: "World"}
	json.NewDecoder(os.Stdin).Decode(p)
	logrus.Printf("Hello %v!\n", p.Name)

	logrus.Println("---> stderr goes to the server logs.")
	logrus.Println("---> LINE 2")
	logrus.Println("---> LINE 3 with a break right here\nand LINE 4")
	logrus.Println("---> LINE 5 with a double line break\n ")
	logrus.Println("---> LINE 6")
}
