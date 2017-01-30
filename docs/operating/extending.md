# Extending IronFunctions

IronFunctions is extensible so you can add custom functionality and extend the project without needing to modify the core.

There are 4 different ways to extend the functionality of IronFunctions. 

1. Listeners - listen to API events such as a route getting updated and react accordingly.
1. Middleware - a chain of middleware is executed before an API handler is called.
1. Add API Endpoints - extend the default IronFunctions API. 
1. Special Handlers - TODO: DO WE NEED THIS ANYMORE??

## Listeners

Listeners are the main way to extend IronFunctions. 

You can easily use listeners basically creating a struct with [valid methods](#Listener Requirements) and adding it to the `IronFunctions API`.

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

### Creating a Listener

These are all available listeners:

#### App Listeners

See the godoc for AppListener [in this file](/iron-io/functions/blob/master/api/server/apps_listeners.go)

#### Runner Listeners

See the godoc for RunnerListner [in this file](/iron-io/functions/blob/master/api/server/runner_listeners.go).

## Adding API Endpoints

You can add API endpoints by using the `AddEndpoint` and `AddEndpointFunc` methods to the IronFunctions server. 

See examples of this in [/examples/extensions/main.go](/examples/extensions/main.go). 

## Middleware

Middleware enables you to add functionality to every API request. For every request, the chain of Middleware will be called 
in order allowing you to modify or reject requests, as well as write output and cancel the chain. 

NOTES:

* middleware is responsible for writing output if it's going to cancel the chain.
* cancel the chain by returning an error from your Middleware's Serve method.

## Special Handlers

To understand how **Special Handlers** works you need to understand what are **Special Routes**.

**Special Routes** are routes that doesn't match any other API route. 

With **Special Handlers** you can change the behavior of special routes in order to define which function is going to be executed.

For example, let's use special handlers to define `mydomain` as the `appname` for any request for `mydomain.com`.

```
type SpecialHandler struct{}

func (h *SpecialHandler) Handle(c server.HandlerContext) error {
    host := c.Request().Host
    if host == "mydomain.com" {
        c.Set("app", "mydomain")
    }
}

func main () {
    sh := &SpecialHandler{}

    srv := server.New(/* Here all required parameters to initialize the server */)
    srv.AddSpecialHandler(sh)
    srv.Run()
}
``` 

With the code above, a request to `http://mydomain.com/hello` will trigger the function `/mydomain/hello`