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

* On schedule set by user, the trigger is fired.
* Scheduler would have its own API to manage schedule -> trigger mappings.

NOTE: This specific piece is not part of the immediate scope of 2.0 but
provided for illustration of other 'event sources' we'll have soon enough.

#### Queue

* Listens on a stream/topic/channel, can have a filter
* For every message that passes filter, fire the trigger

This will be a module that can configure which topics it would like
to monitor for messages, its responsibility being to pull messages from the
topic(s) and transform each payload into a cloud event; the message payload
need not be a cloud event itself, it could be anything, it's up to the queue
source to decide here; in general, fn doesn't dictate how events are acquired
or created, nor what should be done with the response from execution of a
trigger (e.g. it could be added as another message to a different configured
topic). The cloud event is then passed to fn's routing layer (trigger manager
/ lb) to be invoked with `/run`, where the cloud event contains the
`eventType` field is set to the user configured name of this event source
(e.g. `myKafka`) and the `source` field is set to a user configured name of
the trigger (e.g. `myTopic`) - this tuple of `eventType` and `source`
uniquely ids a trigger in the trigger manager.

The module itself could be compiled into an fn server or run separately. Out
of the gate, the plan is to have a simple example (probably redis) as a
reference implementation, but this is not intended to be very fancy nor handle
any resource scheduling (as is the case now), the latter getting pushed to the
function routing layer.

## Example flow

HTTP

### HTTP Router receives request

* Request comes into HTTP router under pretty url, e.g. `http://example.com/repomanager`
* Convert request to CloudEvent, eg:

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

* Pass event to trigger manager.

### Trigger manager (TM) receives event

* TM looks up the matching trigger.
* Trigger has function URI + config data. 
* TM also get app config data.
* Merges config and event to get this event:

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

Passes event to function router (aka LB) OR runner (whichever the trigger
manager is configured for, they both have the same /run endpoint. Single
machine setup would be direct to runner). This could potentially be compiled
directly into the LB or the runner and doesn't necessarily have to be an added
hop. This probably uses a cache in higher scale deploys.

### Function Router gets event

Picks runner to run it on and passes along event

### Runner gets event

* Takes function URL and looks up metadata from function registry. 
* Executes the function
* Returns CloudEvent with response as “data” body, strips out all the extension stuff and replace with response specific info.

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

### NEW IN 2.0

The main changes here are related to [splitting functions and routes](https://github.com/fnproject/fn/issues/817).
A route is now called a trigger and the route/trigger definition is no longer part of the function definition as it was before.

### Function Definition File

`func.yaml` barely changes, just removes route/trigger specific things. 

A function does not know about triggers so it only has the information required to run the function.

```yaml
function:
  name: yodawg

  runtime: ruby
  entrypoint: ruby func.rb
  version: 0.0.7
  memory: 42
```

`fn run` can run a function with this file only.

### Trigger Definition File

A `trigger.yaml` file defines the mapping from event sources to functions. A trigger
binds one event source to one function, for instance an HTTP route to a function.

```yaml
trigger:
  name: sayhello
  type: http
  # http specific param:
  path: /sayhello
  func: https://fnreg.com/namespace/myfunc
```

On `fn deploy`:

* `trigger.yaml` must exist
* if `func.yaml` exists:
  * build and push function image
  * update function registry with function definition
  * use pushed function as `func` param for trigger
* if `func.yaml` does not exist, `trigger.yaml` must have the `func` param set
* Update trigger with Trigger definition

This will create a function at:

```
/ns/_/funcs/myfunc
```

and a trigger at:

```
/ns/_/apps/myapp/triggers/sayhello
{
  name: sayhello
  func: yodawg:0.0.7
  type: http
}
```


in all of the above examples, a user will end up with a route to call:

`http://my.fn.com/sayhello`

TODO do the above with namespace instead and don't have `_` magic?

### CRUD

we'll need to change crud around routes to have CRUD for functions and
triggers, and add namespaces.

## FDKs

FDKs will work in the same manner as they do now and from a user perspective probably don't need to change.
This will be almost exactly the same as how the current JSON format works, just different fields as defined here: 
https://github.com/cloudevents/spec/blob/master/json-format.md

Handlers will take the pseudo-form:

```
handle(context, input) (output, err)
```

This form will change depending on what's right for the language, but they should all have the same 
meaning and generally the same feel:

* `context` will include all of the CloudEvent fields other than the `data` field. 
* `input` will be the `data` field
* `output` is the response `data` field
* `err` is if an error occurred and will be returned with the `CloudEvent.error` field set (see above)


### NOTES

There are some decisions to make around added sugar we want to sprinkle on top
of cloud events. We also may want to allow users to receive/output raw cloud-events
themselves if they would like to, or at least have a way to set each of the
fields and extensions (the catch being if users can define their own they may
add additional fields) -- of course, it's possible to not use FDK and do this,
so my thinking is that we make FDKs more to simplify things than anything.
We can add features we'd like to FDKs as we go down the road. My thinking for
base level FDK functionality is:

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

