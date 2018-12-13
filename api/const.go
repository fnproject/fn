package api

const (
	// AppName is the app name context key
	AppName string = "app_name"
	// AppID is the app id context key
	AppID string = "app_id"
	// FnID is the fn id context key
	FnID string = "fn_id"

	// ParamAppID is the url path parameter for app id
	ParamAppID string = "app_id"
	// ParamAppName is the url path parameter for app name
	ParamAppName string = "app_name"
	// ParamTriggerID is the url path parameter for trigger id
	ParamTriggerID string = "trigger_id"
	// ParamCallID is the url path parameter for call id
	ParamCallID string = "call_id"
	// ParamFnID is the url path parameter for fn id
	ParamFnID string = "fn_id"
	// ParamTriggerSource is the triggers source parameter
	ParamTriggerSource string = "trigger_source"

	//ParamTriggerType is the trigger type parameter - only used in hybrid API
	ParamTriggerType string = "trigger_type"
)
