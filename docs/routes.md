# Routes

Each application has several functions represented through routes.

## Route level configuration

When creating or updating an app, you can pass in a map of config variables.

`config` is a map of values passed to the route runtime in the form of
environment variables.

Note: Route level configuration overrides app level configuration.

```sh
fn routes create --config k1=v1 --config k2=v2 myapp /path image
```

## Notes

Route paths are immutable. If you need to change them, the appropriate approach
is to add a new route with the modified path.