# Function Memory Requirements

A good practice to get the best performance on your IronFunctions API is define the required memory for each function.

## Understanding IronFunctions memory management

When IronFunctions starts it registers the total available memory in your system in order to know during its runtime if the system has the required amount of free memory to run each function.
Every function starts the runner reduces the amount of memory used by that function from the available memory register.
When the function finishes the runner returns the used memory to the available memory register.

By default the required memory of a function is *128 mb*.

## Defining function's required memory

You can define the function's required memory in the route creation or updating it.

### Creating function memory

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"<route name>",
        "image":"<route image>",
        "memory": <memory mb number>
    }
}' http://localhost:8080/v1/apps/<app name>/routes
```

Eg. Creating `/myapp/hello` with required memory as `100mb`

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "path":"/hello",
        "image":"iron/hello",
        "memory": 100
    }
}' http://localhost:8080/v1/apps/myapp/routes
```

### Updating function memory

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "memory": <memory mb number>
    }
}' http://localhost:8080/v1/apps/<app name>/routes/<route name>
```

Eg. Updating `/myapp/hello` required memory as `100mb`

```
curl -H "Content-Type: application/json" -X POST -d '{
    "route": {
        "memory": 100
    }
}' http://localhost:8080/v1/apps/myapp/routes/hello
```