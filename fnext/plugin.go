package fnext

import (
	"fmt"
	"plugin"
)

const (
	listenerSymbolName   = "Listener"
	middlewareSymbolName = "Handle"
	overriderSymbolName  = "Overrider"
)

func symbolFromPlugin(path, symbolName string) (plugin.Symbol, error) {
	plugin, err := plugin.Open(path)
	if err != nil {
		return nil, err
	}
	return plugin.Lookup(symbolName)
}

// NewPluginMiddleware creates an Fn Middleware from a Golang plugin
func NewPluginMiddleware(path string) (Middleware, error) {
	handleSymbol, err := symbolFromPlugin(path, middlewareSymbolName)
	if err != nil {
		return nil, err
	}
	handler, ok := handleSymbol.(*MiddlewareFunc)
	if !ok {
		return nil, fmt.Errorf("%s is not a valid middleware function in plugin: %s", middlewareSymbolName, path)
	}
	return handler, nil
}

// NewPluginAppListener creates an Fn AppListener from a Golang plugin
func NewPluginAppListener(path string) (AppListener, error) {
	listenerSymbol, err := symbolFromPlugin(path, listenerSymbolName)
	if err != nil {
		return nil, err
	}
	callListener, ok := listenerSymbol.(AppListener)
	if !ok {
		return nil, fmt.Errorf("%s is not a AppListener", listenerSymbolName)
	}
	return callListener, nil
}

// NewPluginFnListener creates an Fn FnListener from a Golang plugin
func NewPluginFnListener(path string) (FnListener, error) {
	listenerSymbol, err := symbolFromPlugin(path, listenerSymbolName)
	if err != nil {
		return nil, err
	}
	callListener, ok := listenerSymbol.(FnListener)
	if !ok {
		return nil, fmt.Errorf("%s is not a FnListener", listenerSymbolName)
	}
	return callListener, nil
}

// NewPluginTriggerListener creates an Trigger TriggerListener from a Golang plugin
func NewPluginTriggerListener(path string) (TriggerListener, error) {
	listenerSymbol, err := symbolFromPlugin(path, listenerSymbolName)
	if err != nil {
		return nil, err
	}
	callListener, ok := listenerSymbol.(TriggerListener)
	if !ok {
		return nil, fmt.Errorf("%s is not a TriggerListener", listenerSymbolName)
	}
	return callListener, nil
}

// NewPluginCallListener creates an Fn CallListener from a Golang plugin
func NewPluginCallListener(path string) (CallListener, error) {
	listenerSymbol, err := symbolFromPlugin(path, listenerSymbolName)
	if err != nil {
		return nil, err
	}
	callListener, ok := listenerSymbol.(CallListener)
	if !ok {
		return nil, fmt.Errorf("%s is not a CallListener", listenerSymbolName)
	}
	return callListener, nil
}

// NewPluginCallOverrider creates an Fn CallOverrider from a Golang plugin
func NewPluginCallOverrider(path string) (CallOverrider, error) {
	overriderSymbol, err := symbolFromPlugin(path, overriderSymbolName)
	if err != nil {
		return nil, err
	}
	callOverrider, ok := overriderSymbol.(*CallOverrider)
	if !ok {
		return nil, fmt.Errorf("%s is not a valid call overrider in plugin: %s", overriderSymbolName, path)
	}
	return *callOverrider, nil
}
