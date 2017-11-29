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
	// the following will be at /v1/apps/:app_name/routes/:route_name/custom5
	// and                      /v1/apps/:app_name/routes/:route_name/custom6
	funcServer.AddRouteEndpoint("GET", "/custom5", &Custom5Handler{})
	funcServer.AddRouteEndpointFunc("GET", "/custom6", func(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route) {
		// fmt.Fprintf(w, "Hello, %q", html.EscapeString(r.URL.Path))
		fmt.Println("Custom6Handler called")
		fmt.Fprintf(w, "Hello app %v, route %v, request %q", app.Name, route.Path, html.EscapeString(r.URL.Path))
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

type Custom5Handler struct {
}

func (h *Custom5Handler) ServeHTTP(w http.ResponseWriter, r *http.Request, app *models.App, route *models.Route) {
	fmt.Println("Custom5Handler called")
	fmt.Fprintf(w, "Hello! app %v, route %v, request %q", app.Name, route.Path, html.EscapeString(r.URL.Path))
}
