package main

import (
	"context"
	"fmt"
	"html"
	"net/http"

	"github.com/iron-io/functions/api/models"
	"github.com/iron-io/functions/api/server"
)

func main() {
	ctx := context.Background()

	funcServer := server.NewFromEnv(ctx)
	// Setup your custom extensions, listeners, etc here
	funcServer.AddEndpoint("GET", "/custom1", &Custom1Handler{})
	funcServer.AddEndpointFunc("GET", "/custom2", func(w http.ResponseWriter, r *http.Request) {
		// fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		fmt.Println("Custom2Handler called")
		fmt.Fprintf(w, "Hello func, %q", html.EscapeString(r.URL.Path))
	})

	// the following will be at /v1/apps/:app_name/custom2
	funcServer.AddAppEndpoint("GET", "/custom3", &Custom3Handler{})
	funcServer.AddAppEndpointFunc("GET", "/custom4", func(w http.ResponseWriter, r *http.Request, app *models.App) {
		// fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		fmt.Println("Custom4Handler called")
		fmt.Fprintf(w, "Hello app %v func, %q", app.Name, html.EscapeString(r.URL.Path))
	})
	funcServer.Start(ctx)
}

type Custom1Handler struct {
}

func (h *Custom1Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	fmt.Println("Custom1Handler called")
	fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
}

type Custom3Handler struct {
}

func (h *Custom3Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App) {
	fmt.Println("Custom3Handler called")
	fmt.Fprintf(w, "Hello app %v, %q", app.Name, html.EscapeString(r.URL.Path))
}
