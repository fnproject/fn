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

CRUD operations will all have extensions wrapping each operation. We could
maybe make this simpler by having an interface with all of our handlers that
extensions could implement instead of N*2 additional methods for `BeforeXXX`
and `AfterXXX`, we could just let a user wrap the base set of handlers with
whatever they want but need to figure out parameter parsing there.

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

Turns http requests into trigger cloud events (see flow ex. below),
passes them off to trigger manager / LB / runner.

#### Scheduler

On schedule set by user, the trigger is fired.
Scheduler would have its own API to manage schedule -> trigger mappings.

#### Queue

Listens on a stream/topic/channel, can have a filter
For every message that passes filter, fire the trigger

## Example flow

HTTP

### HTTP Router receives request

Request comes into HTTP router under pretty url, e.g. `http://example.com/repomanager`
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

## CLI changes

### fn deploy

func.yaml barely changes:

```yaml
function:
  name: yodawg
  image: hub.docker.com/fnproject/hello
  version: 0.0.7
  memory: 42

trigger:
  - name: myApp/sayhello
    type: http
```

trigger in yaml is optional. you may specify multiple triggers for each
function. if trigger is not specified, it can be specified at runtime:

`fn deploy --trigger-http sayhello`

since namespace is not specified, this will create a function at:

```
/ns/_/funcs/yodawg
```

and a trigger at:

```
/ns/_/apps/myApp/triggers/sayhello
{
  name: sayhello
  func: yodawg:0.0.7
  type: http
}
```

since the above `func: ` does not specify a host name, the func will be
expected to exist on the fn service where this trigger exists. a full url is
also possible, see below example.

it is also possible to specify a `func.yaml` with a remote function, where the
trigger will be deployed but the function in the working directory will not be
built and pushed to a docker registry, say if we had a func:

```
trigger:
  - name: /sayhello
    type: http
    func: hub.fnproject.io/funcytown/hello
```

in all of the above examples, a user will end up with a route to call:

`http://my.fn.com/sayhello`

TODO do the above with namespace instead and don't have `_` magic?

### CRUD

we'll need to change crud around routes to have CRUD for functions and
triggers, and add namespaces.

## FDKs

There are some decisions to make around added sugar we want to sprinkle on top
of cloud events. We also may want to allow users to receive/output raw cloud-events
themselves if they would like to, or at least have a way to set each of the
fields and extensions (the catch being if users can define their own they may
add additional fields) -- of course, it's possible to not use FDK and do this,
so my thinking is that we make FDKs more to simplify things than anything.
We can add features we'd like to FDKs as we go down the road. My thinking for
base level FDK functionality is:

FDKs will simply handle the json version of cloud-events defined here: 
https://github.com/cloudevents/spec/blob/master/json-format.md

FDKs will in the same manner as now, decode one of these at a time into a
$programming_language object from STDIN, after receiving one, will call a user
specified handler function, and receive a $programming_language object as
output, which it will then encode to json on STDOUT.

Handlers will take the pseudo-form:

```
Handle(CloudEvent) CloudEvent
```

where the user is forced to construct a `CloudEvent` object. Obviously, we
should have some helpful constructors like `NewCloudEvent(contentType, body string) CloudEvent` 
that do things like handle the json magic and that fills in most of the
fields. This seems like a viable option and is more flexible than just trying
to scrape up the body and have some other kind of opaque object and set
certain fields on it (as now).

there are likely other ways, but trying to keep it simple out of the gate.
note that this is a divergence from the current cloud-event implementation,
where the entire user output is shoved into the data section at the end.
there's also a possibility that I'm completely misguided on what FDKs should
look like and if you feel that way please propose a comprehensive solution and
I'd be delighted to see it. It does seem like FDKs will basically be a for
loop, a cloud event object definition and json decoder / encoder, and a bunch
of getter and setter methods (potentially, lang dependent). Maybe this doesn't
provide as much utility as it once did, but that's for us to decide.

## Proposed dicing of the pieces

* Implement HTTP router module, that takes an http request and constructs
  a cloud event, with extensions and source set with annotations, config,
  proper route, etc. (NOTE: this is NOT an LB)
* Implement queue triggerer module
* Implement trigger manager module that eats event and spits out event+sugar
* Implement runner `/run` end-to-end cloud-event format (server, CLI, FDK)
  * kill off json/http/default & runner should not construct cloud event in or out
  * only parse out extensions needed to run
* Implement functions / triggers / namespace API CRUD, datastore
* ? i'm forgetting some stuff

