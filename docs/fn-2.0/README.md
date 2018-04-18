# Fn 2.0

## NOTE: This is a proposal for updating Fn with the following goals:

* Split HTTP routes from functions
* Support CloudEvent messages as the native format throughout
* Simplify everything
* Modularize components
* Namespacing

## Removals

* Default, HTTP and JSON formats will be removed
* Cold functions will be removed
* Async, queues are event sources now

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

```
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
```

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

All of these things make up the application. Applications also define 
additional annotations and configuration to be added to each trigger.

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

* Namespaces
* Apps
* Triggers
* Functions
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

TODO(reed): this could be bypassed since http is more 'push' and we can forward
trigger to trigger manager?

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

## Creating a function workflow

func.yaml barely changes:

```yaml
function:
  name: yodawg
  image: hub.docker.com/fnproject/hello
  version: 0.0.7
  memory: 42

trigger:
  name: /sayhello
  type: http
```

trigger in yaml is optional. if trigger is not specified, it can be specified:

`fn deploy --trigger-http /sayhello`

since namespace is not specified, this will create a function at:

```
/ns/_/funcs/yodawg
```

and a trigger at:

```
/ns/_/triggers/sayhello
{
  func: _/yodawg:0.0.7
}
```

since the above `func: ` does not specify a host name, the func will be
expected to exist on the fn service where this trigger exists. a full url is
also possible, see below example.

TODO what does the above look like with namespace specified? just extract from
`name:` field?

it is also possible to specify a `func.yaml` with a remote function, where the
trigger will be deployed but the function in the working directory will not be
built and pushed to a docker registry, say if we had a func:

```
trigger:
  name: /sayhello
  type: http
  func: hub.fnproject.io/funcytown/hello
```
