# Object Model

This document describes the different objects we store and the relationships between them.

## Applications

At the root of everything are applications. In `fn`, an application is essentially a grouping of functions
with path mappings (routes) to each function. For instance, consider the following URLs for the app called `myapp`:

```
http://myapp.com/hello
http://myapp.com/users
```

This is an app with 2 routes:

1. A mapping of the path `/hello` to a function called `hello`
1. A mapping of the path `/users` to a function called `users`

## Routes

An app consists of 1 or more routes. A route stores the mapping between URL paths and functions (ie: container iamges).

## Calls

A call represents an invocation of a function. Every request for a URL as defined in the routes, a call is created.
The `call_id` for each request will show up in all logs and the status of the call, as well as the logs, can be retrieved using the `call_id`.

## Logs

Logs are stored for each `call` that is made and can be retrieved with the `call_id`.
