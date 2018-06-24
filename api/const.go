package api

const (
	// Gin Request context key names
	AppName string = "app_name"
	AppID   string = "app_id"
	Path    string = "path"

	// Gin  URL template parameters
	ParamAppID     string = "appId"
	ParamAppName   string = "appName"
	ParamRouteName string = "route"
	ParamTriggerID string = "triggerId"
	ParamCallID    string = "call"
	ParamFnID      string = "fnId"
)
