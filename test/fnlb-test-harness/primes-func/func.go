package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// return list of primes less than N
// source: http://stackoverflow.com/a/21923233
func sieveOfEratosthenes(N int) (primes []int) {
	b := make([]bool, N)
	for i := 2; i < N; i++ {
		if b[i] == true {
			continue
		}
		primes = append(primes, i)
		for k := i * i; k < N; k += i {
			b[k] = true
		}
	}
	return
}

func main() {
	start := time.Now()
	maxPrime := 1000000
	numLoops := 1

	// Parse the query string
	s := strings.Split(os.Getenv("REQUEST_URL"), "?")
	if len(s) > 1 {
		for _, pair := range strings.Split(s[1], "&") {
			kv := strings.Split(pair, "=")
			if len(kv) > 1 {
				key, value := kv[0], kv[1]
				if key == "max" {
					maxPrime, _ = strconv.Atoi(value)
				}
				if key == "loops" {
					numLoops, _ = strconv.Atoi(value)
				}
			}
		}
	}

	// Repeat the calculation of primes simply to give the CPU more work to do without consuming additional memory
	for i := 0; i < numLoops; i++ {
		primes := sieveOfEratosthenes(maxPrime)
		_ = primes
		if i == numLoops-1 {
			//fmt.Printf("Highest three primes: %d %d %d\n", primes[len(primes) - 1], primes[len(primes) - 2], primes[len(primes) - 3])
		}
	}
	fmt.Printf("{\"durationSeconds\": %f, \"hostname\": \"%s\", \"max\": %d, \"loops\": %d}", time.Since(start).Seconds(), os.Getenv("HOSTNAME"), maxPrime, numLoops)
}
