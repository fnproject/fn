# \RoutesApi

All URIs are relative to *https://127.0.0.1:8080/v1*

Method | HTTP request | Description
------------- | ------------- | -------------
[**AppsAppRoutesGet**](RoutesApi.md#AppsAppRoutesGet) | **Get** /apps/{app}/routes | Get route list by app name.
[**AppsAppRoutesPost**](RoutesApi.md#AppsAppRoutesPost) | **Post** /apps/{app}/routes | Create new Route
[**AppsAppRoutesRouteDelete**](RoutesApi.md#AppsAppRoutesRouteDelete) | **Delete** /apps/{app}/routes/{route} | Deletes the route
[**AppsAppRoutesRouteGet**](RoutesApi.md#AppsAppRoutesRouteGet) | **Get** /apps/{app}/routes/{route} | Gets route by name
[**AppsAppRoutesRoutePatch**](RoutesApi.md#AppsAppRoutesRoutePatch) | **Patch** /apps/{app}/routes/{route} | Update a Route


# **AppsAppRoutesGet**
> RoutesWrapper AppsAppRoutesGet($app)

Get route list by app name.

This will list routes for a particular app.


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| Name of app for this set of routes. | 

### Return type

[**RoutesWrapper**](RoutesWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsAppRoutesPost**
> RouteWrapper AppsAppRoutesPost($app, $body)

Create new Route

Create a new route in an app, if app doesn't exists, it creates the app


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| name of the app. | 
 **body** | [**RouteWrapper**](RouteWrapper.md)| One route to post. | 

### Return type

[**RouteWrapper**](RouteWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsAppRoutesRouteDelete**
> AppsAppRoutesRouteDelete($app, $route)

Deletes the route

Deletes the route.


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| Name of app for this set of routes. | 
 **route** | **string**| Route name | 

### Return type

void (empty response body)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsAppRoutesRouteGet**
> RouteWrapper AppsAppRoutesRouteGet($app, $route)

Gets route by name

Gets a route by name.


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| Name of app for this set of routes. | 
 **route** | **string**| Route name | 

### Return type

[**RouteWrapper**](RouteWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

# **AppsAppRoutesRoutePatch**
> RouteWrapper AppsAppRoutesRoutePatch($app, $route, $body)

Update a Route

Update a route


### Parameters

Name | Type | Description  | Notes
------------- | ------------- | ------------- | -------------
 **app** | **string**| name of the app. | 
 **route** | **string**| route path. | 
 **body** | [**RouteWrapper**](RouteWrapper.md)| One route to post. | 

### Return type

[**RouteWrapper**](RouteWrapper.md)

### Authorization

No authorization required

### HTTP request headers

 - **Content-Type**: application/json
 - **Accept**: application/json

[[Back to top]](#) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to Model list]](../README.md#documentation-for-models) [[Back to README]](../README.md)

