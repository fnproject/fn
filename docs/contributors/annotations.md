# App and Route Annotations

App and Route annotations allow users building on the Fn platform to encode consumer-specific data inline with Apps and Routes.

Annotations can be used to either communicate and carry information externally (only by API users) or to communicate between external applications and Fn extensions.

## Use cases/Examples:

### Externally defined/consumed annotations

Software using Fn as a service,  attaches non-identifying metadata annotatinos to Fn resources for subsequent reads (e.g. my reference for this function/app )

Writer : API user, Reader: API user

E.g.  platform "platX" creates/modifies functions, has an internal reference it wants to associate with an object for later retrieval (it can't query/search Fn by this attribute)

```
POST /v1/apps

 {
 ...
  "annotations" : {
    "platx.com/ref" : "adsfasdfads"
  }
 ...
 }
```

### Extensions: Allow passing data from user or upstream input to extensions (configuration of extensions)

Writer : API user, Reader: Fn platform extension  (use), API user (informational)

```
POST /v1/apps
...
   {
     ...
       "annotations" :  {
              "my_cloud_provider.com/network_id" : "network.id"
       }
     ...
}
```

###  Extensions: Allow indicating internally derived/set values to user (API extension sets/generates annotations, prevents user from changing it)

Writer : Internal platform extension, Reader: API user.


```
GET /v1/apps/myapp
...
   {
     ...
       "annotations" :  {
              "my_cloud_provider.com/create_user" : "foo@foo.com"
       }
     ...
}
```



```
PATCH /v1/apps/myapp
...
   {
     ...
       "annotations" :  {
              "my_cloud_provider.com/create_user" : "foo@foo.com"
       }
     ...
}

HTTP/1.1  400 Invalid operation

{
   "error": "annotation key cannot be changed",
}
```

## Content Examples
example : user attaches local annotations

```json
PUT /v1/apps/foo

{
  app: {
   ...
    "annotations": {
         "mylabel": "super-cool-fn",
         "myMetaData": {
           "k1": "foo",
           "number": 5000
           "array" : [1,2,3]
         }
     }
  ...
  }
}
```

User sets extension-specific annotations:

```
PUT /v1/apps/foo
{
   ...
    "annotations": {
        "example.extension.com/v1/myval" : "val"
    }
  ...
}
```

## Key Syntax and Namespaces

A key consists of any printable (non-extended) ascii characters excluding whitespace characters.

The maximum (byte) size of a key is 128 bytes (excluding quotes).

Keys are stored as free text with the object. Normatively extensions and systems using annotations *must* use a namespace prefix based on an identified domain and followed by at least one '/' character.

Systems *should* use independent annotation keys for any value that can be changed independently.

Extensions *should not* interact with annotations keys that are not prefixed with a domain they own.

## Value syntax

Values may contain any valid JSON value (object/array/string/number) except the empty string  `""` and `null`

The serialised JSON representation (rendered without excess whitespace as a string) of a single value must not exceed a 512 bytes.

## Modifying and deleting annotation keys

A key can be modified by a PATCH operation containing a partial `annotations` object indicating the keys to update (or delete)

A key can be deleted by a PATCH operation by setting its value to an empty string.

For each element that of data that can be changed independently, you *should* use a new top-level annotation key.

## Maximum number of keys

A user may not add keys in a PATCH or PUT operation if the total number of keys after changes exceeds 100 keys.

Fn may return a larger number of keys.

## Extension interaction with resource modification

An extension  may prevent a PUT,PATCH or POST operation on a domain object based on the value of an annotation passed in by a user, in this case this should result in an HTTP  400 error with an informational message indicating that an error was present in the annotations and containing the exact key  or keys which caused the error.
