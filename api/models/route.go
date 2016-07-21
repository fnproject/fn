package models

import (
	"errors"
	"net/http"
)

var (
	ErrRoutesCreate   = errors.New("Could not create route")
	ErrRoutesUpdate   = errors.New("Could not update route")
	ErrRoutesRemoving = errors.New("Could not remove route from datastore")
	ErrRoutesGet      = errors.New("Could not get route from datastore")
	ErrRoutesList     = errors.New("Could not list routes from datastore")
	ErrRoutesNotFound = errors.New("Route not found")
)

type Routes []Route

type Route struct {
	Name          string      `json:"name"`
	AppName       string      `json:"appname"`
	Path          string      `json:"path"`
	Image         string      `json:"image"`
	Type          string      `json:"type"`
	ContainerPath string      `json:"container_path"`
	Headers       http.Header `json:"headers"`
}

type RouteFilter struct {
	AppName string
}
