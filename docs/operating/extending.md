# Extending IronFunctions

IronFunctions is extensible so you can add custom functionality and extend the project without needing to modify the core.

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

    srv.AddAppCreateListener(myCustomListener)
    srv.AddAppUpdateListener(myCustomListener)
    srv.AddAppDeleteListener(myCustomListener)

    srv.Run()
}
```

### Creating a Listener

These are all available listeners:

#### App Listeners

To be a valid listener your struct should respect interfaces combined or alone found [in this file](/iron-io/functions/blob/master/api/server/apps_listeners.go)

##### AppCreateListener

_Triggers before and after every app creation that happens in the API_ 

Triggered on requests to the following routes:

- POST /v1/apps
- POST /v1/apps/:app/routes

##### AppUpdateListener

_Triggers before and after every app updates that happens in the API_

Triggered during requests to the following routes:

- PUT /v1/apps

##### AppDeleteListener

_Triggers before and after every app deletion that happens in the API_

Triggered during requests to the following routes:

- DELETE /v1/apps/:app

#### Runner Listeners

To be a valid listener your struct should respect interfaces combined or alone found [in this file](/iron-io/functions/blob/master/api/server/runner_listeners.go).

##### RunnerListener

_Triggers before and after every function run_

Triggered during requests to the following routes:

- GET /r/:app/:route
- POST /r/:app/:route

## Adding API Endpoints

You can add API endpoints by using the `AddEndpoint` and `AddEndpointFunc` methods to the IronFunctions server. 

See examples of this in [/examples/extensions/main.go](/examples/extensions/main.go). 

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