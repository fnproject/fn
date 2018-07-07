# Fn Project Common Events Specification 

Fn project containers communicate with the platform via the open cloud events specification 

Cloud events is a meta-standard for defining event types and in general we do not proscribe the formats of events for a particular container. 

Containers may receive and respond with any event type. 

That said there are some common cases where having shared specifications enables integration between particular components in particular: 

* Raw HTTP interactions encapsulated in events 
* Internal errors 

This doc describes tue specifications for those events: 


# HTTP communication 

The HTTP events encapsulate requests from and responses to an  HTTP gateway: 

## HTTP Request 

```
PUT /t/myApp/myTrigger HTTP/1.1
Host: fnservice.com
Content-Type: application/json
Authorization: Bearer asdlkjasldkjas


{"my": "data"}
```

```
{
    "cloudEventsVersion" : "0.1",
    "eventType" : "io.fnproject.httpRequest",
    "eventTypeVersion" : "1.0",
    "source" : "https://fnservice.com/t/myApp/mytrigger",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00Z",
    "extensions" : {
        "ioFnProjectMethod" : "PUT",
        "ioFnProjectHeaders" : {
              "Content-type" : ["application/json"],
              "Authorization": ["Bearer asdlkjasldkjas"], 
         },
         "ioFnProjectURL": "https://fnservice.com/t/myApp/mytrigger"
         
    },
    "contentType" : "application/json",
    "data" : {"my": "data"}
}

```


```
GET /t/myApp/myTrigger HTTP/1.1
Host: fnservice.com
Accept application/json 
```

Simple GET: (no body )
```
{
    "cloudEventsVersion" : "0.1",
    "eventType" : "io.fnproject.httpRequest",
    "eventTypeVersion" : "1.0",
    "source" : "https://fnservice.com/t/myApp/mytrigger",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00Z",
    "extensions" : {
        "ioFnProjectMethod" : "GET",
        "ioFnProjectHeaders" : {
              "Accept" : ["application/json"],
         },
         ioFnProejectURL: "https://fnservice.com/t/myApp/mytrigger",   
    }
}
```


## HTTP Response 

```
{
    "cloudEventsVersion" : "0.1",
    "eventType" : "io.fnproject.httpResponse",
    "eventTypeVersion" : "1.0",
    "source" : "https://fnservice.com/t/myApp/mytrigger",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00Z",
    "extensions" : {
        "ioFnProjectHttpStatus" : 200,
        "ioFnProjectHttpHeaders" : {
              "Content-type" : ["application/json; charset=utf-8"],
         },
    }
    "contentType": "application/json; charset=utf-8",
    "data" : {"goto":"https://github.com/fnproject/fn","hello":"world!"}
   
}
```


```
HTTP/1.1 200 OK 
Content-Type: application/json; charset=utf-8
Date: Thu, 05 Jul 2018 16:36:47 GMT
Content-Length: 59

{"goto":"https://github.com/fnproject/fn","hello":"world!"}

```


# Internal FDK errors 
In the case where an FDK detects an error and is able to propagate information such as an error message back to the server. 


Errors from FDKs are represented as a `io.fnproject.fnError` event - 

Produced by FDK: 
```
{
    "cloudEventsVersion" : "0.1",
    "eventType" : "io.fnproject.fnError",
    "eventTypeVersion" : "1.0",
    "source" : "<ignored>",
    "eventID" : "<ignored>", // generated 
    "eventTime" : "<ignored>",
    "contentType": "application/json; charset=utf-8",
    "data" : {
       "message":"An error occured in the function",
    }
}
```

Transformed by Plaform 
```
{
    "cloudEventsVersion" : "0.1",
    "eventType" : "io.fnproject.fnError",
    "eventTypeVersion" : "1.0",
    "source" : "http://f",
    "eventID" : "A234-1234-1234", // generated 
    "eventTime" : "2018-04-05T17:31:00Z",
    "contentType": "application/json; charset=utf-8",
    "data" : {
       "message":"An error occured in the function",
    }
}
```

