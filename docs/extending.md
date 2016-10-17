# Extending IronFunctions

IronFunctions is extensible so you can add custom functionality and extend the project without needing to modify the core.

## Listeners

Listeners are the main way to extend IronFunctions. 

To add listeners, copy `main.go` into your own repo and add your own listener implementations. When ready, 
compile your main package to create your extended version of IronFunctions.

### AppListener

Implement `ifaces/AppListener` interface, then add it using:

```go
server.AddAppListener(myAppListener)
```
