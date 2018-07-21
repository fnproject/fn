# Direct Invoke API 

This api allows third party systems to integrate with Fn by sending events to specific functions 



Fn supports sending cloud event format messages directly to functions, these are extended and passed onto an FDK running inside a function container. 


Invoke Function: 
```
POST  /v2/invoke/<fnId>
Content-type: application/cloudevents+json


{
    "cloudEventsVersion" : "0.1",
    "eventType" : "com.example.someevent",
    "source" : "/mycontext",
    "eventID" : "A234-1234-1234",
    "eventTime" : "2018-04-05T17:31:00Z",
    "extensions" : {
      "comExampleExtension" : "value"
    },
    "contentType" : "text/xml",
    "data" : "<much wow=\"xml\"/>"
}



```


