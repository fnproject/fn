# Trigger

A Trigger represents an entry point for function invocations. Each type of Trigger requires specific configuration. They should be defined within the func.yaml file, as specified in [FUNC YAML](func-file.md).

## Types

### http

Configures a http endpoint for function invocation.

```
name: triggerOne
type: http
source: /trigger-path
```

This will cause the system to route requests arriving at the fn service at `/trigger-path` to the function specified in the fn.yaml file.
