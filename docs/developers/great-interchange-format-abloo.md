# cloud events all the things

TODOs:

TODO(reed): if we get a binary event, we need to either have a base64 protocol
for this and BE EXPLICIT about it in some capacity so that fdks can pick this
up transparently OR support e.g. the http transport binary format in all fdks.
hash this out.
TODO(reed): illustrate stacking better with full example (trigger)
TODO(reed): debug mode placeholder?
TODO(reed): define fdk code interface

primary motivations:

* proliferation of container formats is not fun to maintain in various fdks,
  and is burdensome to users to figure out which one they want, when 2/3 have
  very similar capabilities (i.e. users shouldn't have to 'think' about it).
  they also have nuances in things like e.g. headers that are not consistent.
* various interchange formats throughout the system add bugs+encoding
  overhead, swapping between different structs / encodings for no concrete
  reason. currently, there is a front door for an 'http' event, which gets
  turned into a call, and 2 slightly different formats in the backend for a
  call b/w the db/mq & grpc layers. the aim being here to unify these.
* if this standard takes off, we're going to support it anyway, and we need
  something like it whether it exists or not.


cloud events themselves are very flexible, we just need to define our exact
format so that we can replace existing functionality, this doc aims to do that.

### fully filled in cloud event (i.e. container ready)

```
{
  "eventType": "fn/htttp",
  "cloudEventsVersion": "0.1",
  "eventID": "6480da1a-5028-4301-acc3-fbae628207b3",
  "source": "http://example.com/repomanager",
  "eventTypeVersion": "1.0",
  "eventTime": "2018-04-01T23:12:34Z",
  "contentType": "application/json",
  "extensions":{
    "app":{
      "id":"123",
      "name":"name",
      "config":{
        "key":"value"
      },
      "annotations":{
        "key":"value"
      }
    },
    "fn":{
      "id":"123",
      "name":"name",
      "config":{
        "key":"value"
      },
      "annotations":{
        "key":"value"
      },
    },
    "trigger":{
      "id":"123",
      "name":"name",
      "source":"http://fn.example.com/yo"
      "annotations":{
        "key":"value"
      }
    }
    "protocol":{
      "type":"http",
      "url":"http://fn.example.com/yo?query=sup",
      "headers":{
        "key":"value"
      }
    }
  }
}
```

* trigger may not exist in events sent directly to invoke from a
  client (as opposed to being sent from a trigger)
* protocol may not exist in events sent directly to invoke from a
  client, or in events whose trigger does not add them; i.e `protocol` is
  entirely optional

Tiny benefit that config values will no longer get 'squashed' when an app and
fn have an overlapping key, and users can see whether a key is defined on the
app or the fn. Similar benefit for annotations.

### invoke

Invoke accepts an event which may or may not have all the function and
trigger details filled in in the cloud event. Invoke itself can be both an
http(s)/grpc endpoint, though cloudevent spec v0.1 does not define a grpc
format, we assume it's a direct protobuf translation of the json format.
The motivation for accepting fn-filled in or not cloud events is to support a
hybrid system, where a cluster could be configured to have fn nodes that
invoke functions but are not connected to the db; i.e. these fn nodes could
not 'fill in' cloud events and can only accept fully qualified fn cloud
events.

Fields that invoke may fill in if found empty:

* eventType: `fn/invoke` (TODO we could add req url/host+fn_id/app_id)
* eventID: generate a new ID
* eventTime: time.Now()
* cloudEventsVersion: 0.1 (? pitfalls, possibly, if we do 'latest')
* extensions.fn: the entire function object, see above
* extensions.app: the entire app object, see above


### fdk/container format changes

Currently, `cloudevent` is a supported format. At present, there is no
verification that the cloudevent has all the required fields, the first thing
to do is fix that. Additionally, at present there are environment variables
populated for various values, such as any app/route config, app name, etc,
these values will no longer be supported in the environment and must be
retrieved from a cloud event. One motivation for doing so is that this makes
it possible to re-use a container for a function between different triggers,
in a similar vein we only need to replace hot containers if the resource
config (cpu, memory, disk, etc.) or image changes, as opposed to any of the
current values used (a much larger set).

fdks should de-serialize full app, fn, and trigger objects from the cloud
event and put them on the context. For example, to access an app's name in the
ruby fdk:

```
app_name = ctx.app.name
```

TODO(reed): should we make this `format='ce'` rather than `cloudevent` so that
FDKs can pick up the difference? In theory, `format` goes away altogether
eventually and this seamlessly falls out. This may not be necessary, analyze.

### 12 steps to recovery

1)  modify usage of fn cloudevents to use fnproject/cloudevent, possibly
    modifying the lib to cover any holes necessary (assuming none, at this
    stage)
2)  add server support for the protocol dispatch of this format, which exists
    now but needs amendment to e.g. add the function to the extensions.
3)  add support to all fdks to pick up fields from the container formatted
    cloud event, these are slightly different than previous. would be awesome
    to make an image that can verify this with a simple func across all fdks.
4)  update the dp to send a cloud event over grpc rather than a serialized call
