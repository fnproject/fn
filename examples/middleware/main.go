package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"gitlab-odx.oracle.com/odx/functions/api/models"
	"gitlab-odx.oracle.com/odx/functions/api/server"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)

	funcServer.AddMiddlewareFunc(func(ctx server.MiddlewareContext, w http.ResponseWriter, r *http.Request, app *models.App) error {
		start := time.Now()
		fmt.Println("CustomMiddlewareFunc called at:", start)
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
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		m2 := map[string]string{"message": "Invalid Authorization token."}
		m := map[string]map[string]string{"error": m2}
		json.NewEncoder(w).Encode(m)
		return errors.New("Invalid authorization token.")
	}
	fmt.Println("auth succeeded!")
	ctx.Set("user", "I'm in!")
	return nil
}
