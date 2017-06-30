# Task

## Properties
Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**Image** | **string** | Name of Docker image to use. This is optional and can be used to override the image defined at the group level. | [default to null]
**Payload** | **string** | Payload for the task. This is what you pass into each task to make it do something. | [optional] [default to null]
**GroupName** | **string** | Group this task belongs to. | [optional] [default to null]
**Error_** | **string** | The error message, if status is &#39;error&#39;. This is errors due to things outside the task itself. Errors from user code will be found in the log. | [optional] [default to null]
**Reason** | **string** | Machine usable reason for task being in this state. Valid values for error status are &#x60;timeout | killed | bad_exit&#x60;. Valid values for cancelled status are &#x60;client_request&#x60;. For everything else, this is undefined.  | [optional] [default to null]
**CreatedAt** | [**time.Time**](time.Time.md) | Time when task was submitted. Always in UTC. | [optional] [default to null]
**StartedAt** | [**time.Time**](time.Time.md) | Time when task started execution. Always in UTC. | [optional] [default to null]
**CompletedAt** | [**time.Time**](time.Time.md) | Time when task completed, whether it was successul or failed. Always in UTC. | [optional] [default to null]
**RetryOf** | **string** | If this field is set, then this task is a retry of the ID in this field. | [optional] [default to null]
**RetryAt** | **string** | If this field is set, then this task was retried by the task referenced in this field. | [optional] [default to null]
**EnvVars** | **map[string]string** | Env vars for the task. Comes from the ones set on the Group. | [optional] [default to null]

[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


