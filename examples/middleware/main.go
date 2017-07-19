package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitlab-odx.oracle.com/odx/functions/api/server"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)

	funcServer.AddMiddlewareFunc(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			fmt.Println("CustomMiddlewareFunc called at:", start)
			next.ServeHTTP(w, r)
			fmt.Println("Duration:", (time.Now().Sub(start)))
		})
	})

	funcServer.AddMiddleware(&CustomMiddleware{})

	funcServer.Start(ctx)
}

type CustomMiddleware struct {
}

func (h *CustomMiddleware) Serve(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("CustomMiddleware called")

		// check auth header
		tokenHeader := strings.SplitN(r.Header.Get("Authorization"), " ", 3)
		if len(tokenHeader) < 2 || tokenHeader[1] != "KlaatuBaradaNikto" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			m2 := map[string]string{"message": "Invalid Authorization token."}
			m := map[string]map[string]string{"error": m2}
			json.NewEncoder(w).Encode(m)
			return
		}
		fmt.Println("auth succeeded!")
		r = r.WithContext(context.WithValue(r.Context(), "user", "I'm in!"))
		next.ServeHTTP(w, r)
	})
}
