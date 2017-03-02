# Extending IronFunctions

IronFunctions is extensible so you can add custom functionality and extend the project without needing to modify the core.

There are multiple ways to extend the functionality of IronFunctions.

1. Listeners - listen to API events such as a route getting updated and react accordingly.
1. Middleware - a chain of middleware is executed before an API handler is called.
1. Add API Endpoints - extend the default IronFunctions API.

## Listeners

Listeners are the main way to extend IronFunctions.

The following listener types are supported:

* App Listeners - [GoDoc](https://godoc.org/github.com/iron-io/functions/api/server#AppListener)
* Runner Listeners - [GoDoc](https://godoc.org/github.com/iron-io/functions/api/server#RunnerListener)

### Creating a Listener

You can easily use app and runner listeners by creating a struct with valid methods satisfying the interface for the respective listener and adding it to the IronFunctions API

Example:

```
package main

import (
    "context"

    "github.com/iron-io/functions/api/server"
    "github.com/iron-io/functions/api/models"
)

type myCustomListener struct{}

func (c *myCustomListener) BeforeAppCreate(ctx context.Context, app *models.App) error { return nil }
func (c *myCustomListener) AfterAppCreate(ctx context.Context, app *models.App) error { return nil }

func (c *myCustomListener) BeforeAppUpdate(ctx context.Context, app *models.App) error { return nil }
func (c *myCustomListener) AfterAppUpdate(ctx context.Context, app *models.App) error { return nil }

func (c *myCustomListener) BeforeAppDelete(ctx context.Context, app *models.App) error { return nil }
func (c *myCustomListener) BeforeAppDelete(ctx context.Context, app *models.App) error { return nil }

function main () {
    srv := server.New(/* Here all required parameters to initialize the server */)

    srv.AddAppListener(myCustomListener)

    srv.Run()
}
```

## Middleware

Middleware enables you to add functionality to every API request. For every request, the chain of Middleware will be called
in order, allowing you to modify or reject requests, as well as write output and cancel the chain.

NOTES:

* middleware is responsible for writing output if it's going to cancel the chain.
* cancel the chain by returning an error from your Middleware's Serve method.

See examples of this in [examples/middleware/main.go](../../examples/middleware/main.go).

## Adding API Endpoints

You can add API endpoints to the IronFunctions server by using the `AddEndpoint` and `AddEndpointFunc` methods.

See examples of this in [examples/extensions/main.go](../../examples/extensions/main.go).