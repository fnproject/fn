# Handling different trigger types for functions

# Use cases

* I want to be able to associate a function with a webhook so that I can handled events sent to me over HTTP
* I want to be able to associate a function with a stream (e.g. kinesis,kafka) so that I can react to events pushed onto a stream



# Narrative

## Http Trigger:

UX:
func.yaml

```
name: javafn
version: 0.0.9
runtime: java
cmd: com.example.fn.HelloFunction::handleRequest
trigger:
  type: http
  path: /foo
  headers:
     myheader: ["foo"]
  cors:
    allow...
```

CLI
```
fn deploy --app foo
....
Created app foo
Created function javafn
Created http trigger /foo
```

API:

Create app : POST/PUT
(as now)
```
PUT /v1/apps/foo/

{
    "name": "foo",
    "annotations": {
        "oracle.com/oci/subnetIds": ["ocid."]
    }
}
...
```

Create function :

```
PUT /v1/apps/foo/functions/javafn
...

{
   "image": "myimage/foo",
   "memory": 128,
   "path": "/helloworld",
   "timeout": 120,
   "updated_at": "0001-01-01T00:00:00.000Z"
   "config": {
     ...
   },
   "trigger": {
      "http": {
         "path":"/foo",
         "headers": {
                 "My Header" : ["foo"]
             }
         }
      }
   }
}

```


## Stream Trigger Trigger:

UX:
func.yaml

```
name: javafn
version: 0.0.9
runtime: java
cmd: com.example.fn.HelloFunction::handleRequest
trigger:
  type: kafka_stream
  url: ...
  topic: foo
  concurrency: by_partition
```

CLI
```
fn deploy --app foo
....
Created app foo
Created function javafn
Created http trigger /foo
```

API:

Create app : POST/PUT
(as now)
```
PUT /v1/apps/foo/

{
    "name": "foo",
    "annotations": {
        "oracle.com/oci/subnetIds": ["ocid."]
    }
}
...
```

Create function :

```
PUT /v1/apps/foo/functions/javafn
...

{
   "image": "myimage/foo",
   "memory": 128,
   "path": "/helloworld",
   "timeout": 120,
   "updated_at": "0001-01-01T00:00:00.000Z"
   "config": {
     ...
   },
   "trigger": {
      "type": "kafka_stream", // supported stream types are defined by extension in the server
      "url": "...",
      "topic": "foo",
      "concurrency": "by_partition"
   }
}

```
