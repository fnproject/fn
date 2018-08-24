# simple cloud-event container model

This aims to be as simple as possible, the basic idea being that we support a
very minimally modified default cloud event 0.1 format. Honestly, it's not a
lot different than what we have already, this just writes it down somewhere so
that we can bicker over the finer points on github to act as proof of work for
the latest and greatest cryptocurrency in addition to the powers that be.

## json format:

TODO idk where to put this? here for illustrative purposes...

```
{
  "eventType": "fn/http",
  "cloudEventsVersion": "0.1",
  "eventID": "6480da1a-5028-4301-acc3-fbae628207b3",
  "source": "http://example.com/repomanager",
  "eventTypeVersion": "1.0",
  "eventTime": "2018-04-01T23:12:34Z",
  "contentType": "application/json",
  "data":"",
  "extensions":{ }
}
```

## invoke

invoke accepts a cloud event under an fn id `/invoke/:fn_id`. This is
deserialized into a cloud event object, hung off the call object as
`call.event`. If we decide later somehow this was a horrible choice, we can
easily make it something more flexible (our current `call.request` itself is
inflexible). The received cloud event must be valid, having all required
fields set.

invoke will return an event as a response, as well, by the http-transport-spec
0.1 guidelines (ie structured unless binary). this is the same event returned
from the container.

## http trigger invoke

users may send in a (valid) cloudevent themselves (which will __not__ do the
following), otherwise one will be constructed as follows:

```
ce.contentType = req.headers["Content-Type"]
ce.data = req.body
ce.extensions.protocol.headers = req.Header
ce.extensions.protocol.url = req.URL
ce.extensions.protocol.method = req.Method
ce.extensions.protocol.type = "http"
```

this will be used as the `call.event` outlined in the `invoke` section
previously.

as function executions will now return an `event` as well, the http trigger is
responsible for unpacking the following fields to return as the http response:

```
resp.Header = event.extensions.header
resp.Header["Content-Type"] = event.contentType
resp.StatusCode = event.extensions.status_code
resp.Body = event.data
```

the main change involved here is that event construction is moved up to the
http trigger handler, instead of down in protocol dispatch.

## container format

TODO we could keep most of the same values we have now in env too to ease...
TODO stderr is same regardless of pipe choice?

any values from the app and function configs are put in the container's environment, same as now

there are some options to cover for the event itself outlined below

### container pipe options:

any of the below options require updates to all fdks, some which don't support
cloudevents at all right now and would take a bit longer, and others that
would need smaller changes (just the binary thing, in some cases).

### support binary with an encoding

If the field `extensions.fn_binary = 'base64'` is set, this means fn encoded
the `data` section in base64 and the fdk is responsible for decoding base64
before handing the event off.

### support binary by using a cloud event in a request over stdin

this is slightly different than the current `cloudevent` format, which is a
cloud event json object over stdin. bear in mind this was not easy for python
and ruby to do, both of which are officially supported, and other languages
are likely to have issues with this as well.

### support binary by using a cloud event in a request over a socket

this is a change in the medium and aside from needing to make some additional
protocol definitions here, this means fdks need additional work to have
functions crack open a port and run a server on it. if we do this, we could
have knative compatible functions by definition (they accept a cloud event on
a socket). additional work must be done to handle concurrency per container.

## agent

agent will continue to use a call object, which has the configuration
necessary to execute a function on it. it does not change from the present
version very much:

* remove the `call.request` field.
* find a pleasant way to deal with returning events from submit (at present
  for cloudevent format

## fdks

take the same form:

```
handle(ctx, input, output)
```

a cloud event may be retrieved from the ctx object as follows (as similarly as
possible):

```
ce = ctx.Event()
```

`input` is the 'data' section
`output` will be the 'data' section of the returned cloud event

TODO(reed): I don't care for this fdk part really, have another go / seek help

## alternatives

we could have our own 'event' format which we can translate to a cloud event
at the fdk level, not getting locked into a spec and requisite updates. this
would still require fdks be updated to support cloud events.

or if we get stuck supporting this one, then we could use 0.1 as the format to
translate into a > 0.1 cloud event later, too, possibly (via extensions for
any additions, presumably)

give up and support all of the formats.

give up and make ruby work with http format and remove all the other formats
and figure out how to make cloudevents work in that way somehow too

buy world of warcraft and cease attempting to make the world better for money

## work

* update fdks to support the 0.1 cloudevent spec (java and node? go/py/ruby
  work as is, at least mostly, at least afaik). as currently we have
  `cloudevent` format, they simply need support for this to get going.
* make decision about handling of binary/request/stdin/etc.
* hang events off calls, modifying the current protocol handlers to grab
  fields from the `event` and `call` rather than the `request` and `call`
  (this is as easy as e.g. saying `call.event.data` instead of `call.request.body`),
  possibly update lb_agent rpc to accommodate?, also turning any of the
  individual protocols themselves into an event (big maybe here? ease? we
  could skip to the chopping block and not merge this until after next step)
* announce that everyone must update their fdks and test w/ ce format (give 2 weeks?)
* nuke the other formats, nuke the format field & only ce now (breaks functions not updated)

main thing is update java and node fdks / commitment?
