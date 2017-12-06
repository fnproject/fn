# Writing Extensions

Fn is extensible so you can add custom functionality and extend the project without needing to modify the core.

There are multiple ways to extend the functionality of Fn.

1. Listeners - listen to API events such as a route getting updated and react accordingly.
1. Middleware - a chain of middleware is executed before an API handler is called.
1. Add API Endpoints - extend the default Fn API.

To create an extension, there are just a couple of rules to follow.

* All config should be via ENV vars.

## Code

Your extension code needs to register itself with an `init()` method which states it's name and a function
to be called during setup:

```go
func init() {
    server.RegisterExtension(&fnext.Extension{
        Name:  "github.com/treeder/fn-ext-example/logspam", // Should be the import name
        Setup: setup, // Fn will call this during startup
    })
}

func setup(s *fnext.ExtServer) error {
    // Add all the hooks your extension needs here
    s.AddCallListener(&LogSpam{})
}
```

See https://github.com/treeder/fn-ext-example for full example.

## Listeners

Listeners are the main way to extend Fn.

The following listener types are supported:

* App Listeners - [GoDoc](https://godoc.org/github.com/fnproject/fn/api/server#AppListener)
* Call Listeners - [GoDoc](https://godoc.org/github.com/fnproject/fn/api/server#CallListener)

### Creating a Listener

You can easily use add listeners by creating a struct with valid methods satisfying the interface
for the respective listener, adding it to `main.go` then compiling.

Example:

```go
package main

import (
    "context"

    "github.com/fnproject/fn/api/server"
    "github.com/fnproject/fn/api/models"
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

See examples of this in [examples/middleware/main.go](../../examples/middleware/main.go).

## Adding API Endpoints

You can add API endpoints to the Fn server by using the `AddEndpoint` and `AddEndpointFunc` methods.

See examples of this in [examples/extensions/main.go](../../examples/extensions/main.go).