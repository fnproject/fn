// Copyright 2016 Iron.io
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type Input struct {
	Name string `json:"name"`
}

func main() {
	input := &Input{}
	if err := json.NewDecoder(os.Stdin).Decode(input); err != nil {
		// log.Println("Bad payload or no payload. Ignoring.", err)
	}
	if input.Name == "" {
		input.Name = "World"
	}
	fmt.Printf("Hello %v!\n", input.Name)
}
