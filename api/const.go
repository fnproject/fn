package api

const (
	// AppName is the app name context key & url path parameter
	AppName string = "app_name"
	// AppID is the app id context key & url path parameter
	AppID string = "app_id"
	// FnID is the fn id context key & url path parameter

	// TriggerID is the url path parameter for trigger id
	TriggerID string = "trigger_id"
	// CallID is the url path parameter for call id
	CallID string = "call_id"
	// FnID is the url path parameter for fn id
	FnID string = "fn_id"
	// TriggerSource is the triggers source parameter
	TriggerSource string = "trigger_source"

	//TriggerType is the trigger type parameter - only used in hybrid API
	TriggerType string = "trigger_type"
)
