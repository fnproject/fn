IronFunctions is extensible so you can add custom functionality and extend the project without needing to modify the core. 

## Listeners

This is the main way to do it. To add listeners, copy main.go and use one of the following functions on the Server. 

### AppListener

Implement `ifaces/AppListener` interface, then add it using:

```go
server.AddAppListener(myAppListener)
```
