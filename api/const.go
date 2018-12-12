package api

const (
	// AppName is the app name context key
	AppName string = "app_name"
	// AppID is the app id context key
	AppID string = "app_id"
	// FnID is the fn id context key
	FnID string = "fn_id"

	// ParamAppID is the url path parameter for app id
	ParamAppID string = "appID"
	// ParamAppName is the url path parameter for app name
	ParamAppName string = "appName"
	// ParamTriggerID is the url path parameter for trigger id
	ParamTriggerID string = "triggerID"
	// ParamCallID is the url path parameter for call id
	ParamCallID string = "callID"
	// ParamFnID is the url path parameter for fn id
	ParamFnID string = "fnID"
	// ParamTriggerSource is the triggers source parameter
	ParamTriggerSource string = "triggerSource"

	//ParamTriggerType is the trigger type parameter - only used in hybrid API
	ParamTriggerType string = "triggerType"
)
