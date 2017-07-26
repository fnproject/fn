# \AppsApi

All URIs are relative to *https://127.0.0.1:8080/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AppsAppDelete**](AppsApi.md#AppsAppDelete) | **Delete** /apps/{app} | Delete an app.
[**AppsAppGet**](AppsApi.md#AppsAppGet) | **Get** /apps/{app} | Get information for a app.
[**AppsAppPatch**](AppsApi.md#AppsAppPatch) | **Patch** /apps/{app} | Updates an app.
[**AppsGet**](AppsApi.md#AppsGet) | **Get** /apps | Get all app names.
[**AppsPost**](AppsApi.md#AppsPost) | **Post** /apps | Post new app


# **AppsAppDelete**
> AppsAppDelete($app)

Delete an app.

Delete an app.


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| Name of the app. | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsAppGet**
> AppWrapper AppsAppGet($app)

Get information for a app.

This gives more details about a app, such as statistics.


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| name of the app. | 

### Return type

[**AppWrapper**](AppWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsAppPatch**
> AppWrapper AppsAppPatch($app, $body)

Updates an app.

You can set app level settings here. 


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| name of the app. | 
 **body** | [**AppWrapper**](AppWrapper.md)| App to post. | 

### Return type

[**AppWrapper**](AppWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsGet**
> AppsWrapper AppsGet()

Get all app names.

Get a list of all the apps in the system.


### Parameters
This endpoint does not need any parameter.

### Return type

[**AppsWrapper**](AppsWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsPost**
> AppWrapper AppsPost($body)

Post new app

Insert a new app


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **body** | [**AppWrapper**](AppWrapper.md)| App to post. | 

### Return type

[**AppWrapper**](AppWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

