# Fn 2.0

## NOTE: This is a proposal for updating Fn with the following goals:

* Split HTTP routes from functions
* Support CloudEvent messages as the native format throughout
* Simplify everything
* Modularize components

## Removals

* Default, HTTP and JSON formats will be removed
* Cold functions will be removed

## Terminology

Fn follows the terminology defined here: https://github.com/cloudevents/spec/blob/master/spec.md#notations-and-terminology

As well as the following:

### Event Sources

Event sources are things that emit events and pass the event messages to Fn. Three common event sources are:

* HTTP
* Scheduler
* Queue

### Functions

Functions are little programs that accept a defined input and output a defined output. They do not need to be aware of any other parts
of the system and can run standalone; from the command line, etc. These can be written in any language and are packaged up as a container image. 

* Functions read from STDIN, write to STDOUT and log to STDERR
* Functions are packaged in container images
* Input is a stream of CloudEvent messages in JSON format
* Output is a stream of CloudEvent messages in JSON format
  * With some additions to deal with things like error handling and protocol specific responses (eg: function wants to return a specific http status code if it was fired from an HTTP event source.)

#### Function Error Handling

Errors will be returned as a CloudEvent with an “error” attribute, no “data” attribute, describing the error. 

{
  "cloud-events-version": "0.1",
  "event-id": "6480da1a-5028-4301-acc3-fbae628207b3",
  "source": "http://example.com/repomanager",
  "event-type": "http-request",
  "event-type-version": "v1.5",
  "event-time": "2018-04-01T23:12:34Z",
  "content-type": "application/json",
  “error”: {
    "message": "an error occurred, blah, blah",
    "type": "Exception",
    "trace": "stack trace"
  },
  “extensions”: {
    “protocol”: {
       “headers”: …
       Etc, same as we do now for JSON format. 
    }
  }
}

Error map:
* message - required
* type - type of error, eg IOError - optional
* trace - stack trace - optional

### Triggers

Triggers bind events to functions along with configuration.

Eg: HTTP event

```json
{
    "source": "http://myapp.com/x"
}
```

Trigger will look up the source in the triggers db then update the event with the Function information plus the app and trigger configuration.

```json
{
    "extensions": {
        "trigger": {
            "function_url": "func url",
            "config": {
                MAP OF CONFIGS FROM APP AND TRIGGER
            }
        }
    }
}
```

It will then pass this message along to the Fn Router/LB for execution.

### Apps/Applications

Applications are a grouping of triggers to make up a complete application. For example, an application might have the following triggers:

* http: /x -> trigger maps /x -> function f1 with config c1
* http: /y …
* schedule: run function f2 every day at 8am
* queue: listen to topic t1 filtering on ff1, for every event run function f3

All of these things make up the application. 

## Components

The following components make up the Fn platform. 

* All components can run separately
* They all have their own main
* One main main to tie them all into one bin for a nice developer experience.

### Runner

The Runner does nothing more than take an event message, execute the function and return results.

* Main endpoint `/run` with CloudEvent as input.
* When called, runs a function and returns the results, as a CloudEvent.
* If busy, returns 503.
* Event input contains everything it needs to run, including config/secrets.
  * TODO: Need to be sure we don’t log any secrets
* Logs can be retrieved separately via `/log` endpoint so main response can be returned faster.

### Function Registry

A service that gives functions an addressable URL, eg:

```
https://functions.somewhere.com/NAMESPACE/FUNCNAME:VERSION-TAG
```

* Has metadata required to run a function provided by the creator of the function
* All functions registered here must accept and return CloudEvent messages
* This leads to reusable/shareable functions with no dependencies on other components
* Could potentially run as a separate service like Docker Hub

Q: Seems like a lot to just be a small layer on top of Docker Hub?
A: It may seem this way now for just a few fields, but down the road, there could many additional things
  like forcing functions to pass certain tests (eg: I/O formats), defining expected input parameters (can reject 
  before starting if input is wrong), etc.

### API

CRUD for all the things.

* Apps
* Triggers
* Calls
* Logs

### Function Router/LB

Deals with sending event messages to the best runner.

Same main endpoint as a runner, `/run` with CloudEvent input.

### Trigger Manager

The trigger manager is the main entrypoint for an event to enter the Fn platform.

Main endpoint is `/run` with CloudEvent input. 

Flow:

* Input is a CloudEvent.
* Looks up matching trigger from API based on event.source (maybe other params too).
* Calls out to LB (or Runner if no LB) to run function.

See **Trigger** above for more information.

### Event Sources

#### HTTP Router

Reads HTTP triggers to make routing table, mapping HTTP endpoints to triggers (or directly to functions?). 
When matching request comes in, either through trigger manager or straight to LB/runner.

#### Scheduler

On schedule set by user, the trigger is fired.

#### Queue

Listens on a stream/topic/channel, can have a filter
For every message that passes filter, fire the trigger

## Example flow

HTTP

### HTTP Router receives request

Request comes into HTTP router. 
Router looks at routing table for match (set on triggers). 
If no match, return 404.
Convert request to CloudEvent, eg:

```json
{
  "cloud-events-version": "0.1",
  "event-id": "6480da1a-5028-4301-acc3-fbae628207b3",
  "source": "http://example.com/repomanager",
  "event-type": "http-request",
  "event-type-version": "v1.5",
  "event-time": "2018-04-01T23:12:34Z",
  "content-type": "application/json",
  "data": {
     REQUEST_BODY
  },
  “extensions”: {
    “protocol”: {
       “headers”: …
       Etc, same as we do now for JSON format. 
    }
  }
}
```

Pass event to trigger manager.

### Trigger manager (TM) receives event

TM looks up the matching trigger.
Trigger has function URI + config data. 
TM also get app config data.
Merges config and event to get this event:

```json
{
  "cloud-events-version": "0.1",
  "event-id": "6480da1a-5028-4301-acc3-fbae628207b3",
  "source": "http://example.com/repomanager",
  "event-type": "http-request",
  "event-type-version": "v1.5",
  "event-time": "2018-04-01T23:12:34Z",
  "content-type": "application/json",
  "data": {
     REQUEST_BODY
  },
  “extensions”: {
    “protocol”: {
       “headers”: …
       Etc, same as we do now for JSON format. 
    },
    “config”: {
        ALL APP AND TRIGGER CONFIGS
     }
     “function”: “https://funcs.fnproject.io/NAMESPACE/FUNC:VERSION”
  }
}
```

Passes event to function router (aka LB) OR runner (whichever the trigger manager is configured for, they both have the same /run endpoint. Single machine setup would be direct to runner). 

### Function Router gets event

Picks runner to run it on and passes along event

### Runner gets event

Takes function URL and looks up metadata from function registry. 
Executes the function
Returns CloudEvent with response as “data” body, strips out all the extension stuff and replace with response specific info.

```json
{
  "cloud-events-version": "0.1",
  "event-id": "6480da1a-5028-4301-acc3-fbae628207b3",
  "source": "http://example.com/repomanager",
  "event-type": "http-request",
  "event-type-version": "v1.5",
  "event-time": "2018-04-01T23:12:34Z",
  "content-type": "application/json",
  "data": {
     RESPONSE_BODY
  },
  “extensions”: {
    “protocol”: {
       “headers”: …
       Etc, same as we do now for JSON format responses 
    },
  }
}
```

## Models

```go
type Function struct {
            ID string `json:”id” db:”id”`
	// repo-host/Namespace/Name:Tag make up the fully qualified name
	Namespace string `json:”namespace”`
	Name      string `json:"name" db:"name"`
Image    string `json:”image”`
	Resources ResourceConfig `json:”resources” db:”resources”`
Config Config `json:”config”` // ???
// TODO tell me there's not annotations here too?
	CreatedAt   strfmt.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   strfmt.DateTime `json:"updated_at,omitempty" db:"updated_at"`
}
```

```go
type ResourceConfig struct {
	Memory     uint64         `json:"memory" db:"memory"`
	CPUs        MilliCPUs    `json:"cpus" db:"cpus"` 
	Disk 	      uint64 	 `json:”disk” db:”disk”
	Timeout     int32           `json:"timeout" db:"timeout"`
	IdleTimeout int32           `json:"idle_timeout" db:"idle_timeout"`
}
```

```go
type Trigger struct {
	// The grouping of routes into an app is specific to this type of trigger
	AppID string `json:"app_id" db:"app_id"`
	// FunctionName a fully qualified function name referencing a function in a repository. eg: `funchub.fnproject.io/jimbo/somefunc:1.2.3`
	FunctionName string `json:"function" db:"function"`
	// Config is required by the function, but set in the trigger, ie: the caller.
	Config Config `json:"config,omitempty" db:"config"`
            // Override the function’s resource config
Resources ResourceConfig `json:"config,omitempty" db:"config"`
	// Annotations config for environment function will be running in
	Annotations Annotations     `json:"annotations,omitempty" db:"annotations"`
	CreatedAt   strfmt.DateTime `json:"created_at,omitempty" db:"created_at"`
	UpdatedAt   strfmt.DateTime `json:"updated_at,omitempty" db:"updated_at"`
}
```

```go
type CloudEvent struct {
	EventType string `json:”event-type”`
	EventTypeVersion string `json:”event-type-version”`
	CloudEventsVersion string `json:”cloud-events-version”`
	Source string `json:”source”`
EventID string `json:”event-id”`	
	EventTime strfmt.DateTime `json:”event-time”`
	SchemaURL string `json:”schema-url”`
	ContentType string `json:”content-type”`
	Extensions map[string]interface{} `json:”extensions”` // map[string]string ?
Data string `json:”data”`
}
```

```go
type App struct {
	ID string `json:”id”`
	Name string `json:”name”`
	Config Config `json:”config”`
	Annotations Annotations `json:”annotations”`
}
```

### API Definitions (all components)

```
// :ns is a URL safe user-generated string, e.g. `war-party`
GET /ns
GET /ns/:ns
PUT /ns/:ns
DELETE /ns/:ns

// :app is a URL safe user-generated string, e.g. `operation-kino`
GET /ns/:ns/apps
GET /ns/:ns/apps/:app
PUT /ns/:ns/apps/:app
DELETE /ns/:ns/apps/:app

// :trigger is a URL safe user-generated string, e.g. `himmler`
GET /ns/:ns/apps/:app/triggers
GET /ns/:ns/apps/:app/triggers/:trigger
PUT /ns/:ns/apps/:app/triggers/:trigger
DELETE /ns/:ns/apps/:app/triggers/:trigger

// :function is a URL safe user-generated string, e.g. `bat`
GET /ns/:ns/functions
GET /ns/:ns/functions/:function
PUT /ns/:ns/functions/:function
DELETE /ns/:ns/functions/:function

// TODO is this our only 'invoke' point? do we also have /cloudevent or something?
// what is the format here, do we consume/produce raw cloud event or raw body?
// it would be really nice to just input/output event here and let the router do other stuff. 
ANY /r/:ns/:app/:trigger

POST /fire (or /run to make it consistent with the all of the other components).
INPUT: CloudEvent ce
Trigger manager looks up ce.source in trigger db (or some field to look up on).
If matching trigger, then fire it. 
This way, an event source doesn’t need to know anything about the triggers or functions.
This does not mean that Fn will eat queues or anything, it’s still a push model.
```


# AN OBIT FOR THE DEATH OF GENERIC TRIGGERS

After much discussion around making a generic triggering mechanism, we've relapsed to effectively splitting functions out from routes and naming routes 'triggers' for the sake of renaming things to be inline with the industry-recognized terminology. 

The issue with creating a generic triggering mechanism was that any non-http type of event source (queue, scheduler, etc.) will effectively end up calling an http invoke endpoint (or something semantically equivalent, like grpc). Then there was the burden of the triggerer (a queue trigger, e.g.), having to look up a list of triggers that it needs to service in order to know which resources to look for events at (e.g. queue topics), this data set was challenging to normalize and the API for an event source to keep up to date was not obvious. Ultimately, having event source specific triggers was unsavory for these reasons as we would need an ingress point akin to the http invoke endpoint anyway for each trigger (as proposed here) in addition to mechanisms for the event sources to service their triggers. We decided it was better to put the onus of storing the the event sources themselves (like a route table, or which topics to service, or a list of schedules) onto the event sources themselves for these reasons. 

For additional flexibility in this department, we may add a code interface ala `fnrunner.Invoke(Trigger, Event) (Event, error)` where a user could have extensions to do event processing without having to go through http (for, say, chewing on a queue), but this is not part of the immediate plan.

