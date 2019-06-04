package main

import (
	"context"
	"fmt"
	"html"
	"net/http"

	"github.com/fnproject/fn/api/models"
	"github.com/fnproject/fn/api/server"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)
	// Setup your custom extensions, listeners, etc here
	funcServer.AddEndpoint("GET", "/custom1", &custom1Handler{})
	funcServer.AddEndpointFunc("GET", "/custom2", func(w http.ResponseWriter, r *http.Request) {
		// fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		fmt.Println("custom2Handler called")
		fmt.Fprintf(w, "Hello func, %q", html.EscapeString(r.URL.Path))
	})

	funcServer.Start(ctx)
}

type custom1Handler struct{}

func (h *custom1Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("custom1Handler called")
	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
}

type custom3Handler struct{}

func (h *custom3Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App) {
	fmt.Println("custom3Handler called")
	fmt.Fprintf(w, "Hello app %v, %q", app.Name, html.EscapeString(r.URL.Path))
}
