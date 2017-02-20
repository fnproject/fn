package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/server"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)

	funcServer.AddMiddlewareFunc(func(ctx server.MiddlewareContext, w http.ResponseWriter, r *http.Request, app *models.App) error {
		start := time.Now()
		fmt.Println("CustomMiddlewareFunc called at:", start)
		// TODO: probably need a way to let the chain go forward here and return back to the middleware, for things like timing, etc.
		ctx.Next()
		fmt.Println("Duration:", (time.Now().Sub(start)))
		return nil
	})
	funcServer.AddMiddleware(&CustomMiddleware{})

	funcServer.Start(ctx)
}

type CustomMiddleware struct {
}

func (h *CustomMiddleware) Serve(ctx server.MiddlewareContext, w http.ResponseWriter, r *http.Request, app *models.App) error {
	fmt.Println("CustomMiddleware called")

	// check auth header
	tokenHeader := strings.SplitN(r.Header.Get("Authorization"), " ", 3)
	if len(tokenHeader) < 2 || tokenHeader[1] != "KlaatuBaradaNikto" {
		w.WriteHeader(http.StatusUnauthorized)
		m := map[string]string{"error": "Invalid Authorization token. Sorry!"}
		json.NewEncoder(w).Encode(m)
		return errors.New("Invalid authorization token.")
	}
	fmt.Println("auth succeeded!")
	return nil
}
