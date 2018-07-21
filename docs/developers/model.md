# Object Model

This document describes the different objects we store and the relationships between them.

## Applications

At the root of everything are applications. In `fn`, an application is essentially a grouping of functions. 


Applications can share common configuration including environment variables that are propagated to underlying functions. 

## Functions 

Functions are the unit of work in Fn - each function  is defined by a function image (uplaoded docker image) and some configuration properties that determine how that image will be executed to handle incoming events, including container properties such as the required RAM, the container format and application properties that are passed to the function as environment variables. 


Functions can be invoked directly via the invoke API or can be attached to triggers to be invoked in association with other events. 

## Triggers 

Triggers are a binding between a function and some external event source - currently Fn supports binding functions to  HTTP web services. In future we plan to support other event sources such as timers (i.e. cron), queues and other third party systems. 


For HTTP triggers when the trigger is defined,   users can invoke that function via the HTTP gateway as a standard web services. 

## Calls

A call represents an invocation of a function. Every request for a URL as defined in the routes, a call is created.
The `call_id` for each request will show up in all logs and the status of the call, as well as the logs, can be retrieved using the `call_id`.

## Logs

Logs are stored for each `call` that is made and can be retrieved with the `call_id`.
