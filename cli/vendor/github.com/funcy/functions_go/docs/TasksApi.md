# \TasksApi

All URIs are relative to *https://127.0.0.1:8080/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**TasksGet**](TasksApi.md#TasksGet) | **Get** /tasks | Get next task.


# **TasksGet**
> TaskWrapper TasksGet()

Get next task.

Gets the next task in the queue, ready for processing. Consumers should start processing tasks in order. No other consumer can retrieve this task.


### Parameters
This endpoint does not need any parameter.

### Return type

[**TaskWrapper**](TaskWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

