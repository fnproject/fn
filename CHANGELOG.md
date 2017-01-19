## v0.2.0 (unreleased)

### Release Notes

### Features

- [#428](https://github.com/iron-io/functions/issues/428): Change update route from PUT to PATCH.
- [#368](https://github.com/iron-io/functions/issues/368): fn: support route headers tweaks.
- [#316](https://github.com/iron-io/functions/issues/316): fnctl: Add rustlang support.
- [#313](https://github.com/iron-io/functions/issues/313): fnctl: Add .NET core support?.
- [#310](https://github.com/iron-io/functions/issues/310): fnctl: Add python support.
- [#69](https://github.com/iron-io/functions/issues/69): Long(er) running containers for better performance aka Hot Containers.
- [#472](https://github.com/iron-io/functions/pull/472): Add global lru for routes with keys being the appname + path.
- [#484](https://github.com/iron-io/functions/pull/484): Add triggers example for OpenStack project Picasso.
- [#487](https://github.com/iron-io/functions/pull/487): Add initial load balancer.

### Bugfixes

- [#483](https://github.com/iron-io/functions/pull/483): Listen for PORT before running async/sync workers in order to prevent errors.
- [#479](https://github.com/iron-io/functions/pull/478): Add routes config set/unset back
- [#429](https://github.com/iron-io/functions/issues/429): Broken docs after merge.
- [#422](https://github.com/iron-io/functions/issues/422): The headers field in func.yaml expects an array of values for each header key.
- [#421](https://github.com/iron-io/functions/issues/421): Can't update a route and show better error message.
- [#420](https://github.com/iron-io/functions/issues/420): `fn` tool install script not being updated to new releases.
- [#419](https://github.com/iron-io/functions/issues/419): --runtime flag on init doesn't work area/fn .
- [#414](https://github.com/iron-io/functions/issues/414): make run-docker is buggy on linux .
- [#413](https://github.com/iron-io/functions/issues/413): fnctl: Creating routes ignores the route path and assigns function name.
- [#403](https://github.com/iron-io/functions/issues/403): Route update (HTTP PUT) modifies datastore entity by making it inconsistent.
- [#393](https://github.com/iron-io/functions/issues/393): Add documentation on how to use hot containers.
- [#384](https://github.com/iron-io/functions/issues/384): Multiple routines use non-threadsafe cache.
- [#381](https://github.com/iron-io/functions/issues/381): Unable to update route path through HTTP PUT area/api bug.
- [#380](https://github.com/iron-io/functions/issues/380): Unable to update app name.
- [#373](https://github.com/iron-io/functions/issues/373): fn build should fail if no version in func.yaml.
- [#369](https://github.com/iron-io/functions/issues/369): Add documentation related to SpecialHandlers.
- [#366](https://github.com/iron-io/functions/issues/366): Documentation lagging behind after Hot Containers.
- [#365](https://github.com/iron-io/functions/issues/365): Documentation lagging behind on AppListeners.
- [#364](https://github.com/iron-io/functions/issues/364): Remove app_name from per function endpoints.
- [#363](https://github.com/iron-io/functions/issues/363): Update CONTRIBUTING with some rules of PRs.
- [#360](https://github.com/iron-io/functions/issues/360): HTTP route /version is not described in swagger doc.
- [#352](https://github.com/iron-io/functions/issues/352): Improve `fn publish` command .
- [#345](https://github.com/iron-io/functions/issues/345): Check and fix for potential goroutine leak in api/runner.
- [#339](https://github.com/iron-io/functions/issues/339): Unable to run sync route execution longer than 60 seconds.
- [#320](https://github.com/iron-io/functions/issues/320): Change cli tool name to `fn`?.
- [#319](https://github.com/iron-io/functions/issues/319): Update docs to link to client libraries.
- [#304](https://github.com/iron-io/functions/issues/304): Create an fnctl dns entry and enable ssl for install of the cli tool.
- [#302](https://github.com/iron-io/functions/issues/302): Placement of app name in fnctl seems inconsistent .
- [#301](https://github.com/iron-io/functions/issues/301): can add a route with /hello but can’t delete it with /hello .. have to delete it with just hello.
- [#299](https://github.com/iron-io/functions/issues/299): More obvious USAGE line for where to include app name.
- [#298](https://github.com/iron-io/functions/issues/298): deleting a route that doesn't exist says it's deleted.
- [#296](https://github.com/iron-io/functions/issues/296): Better error messages for error on creating app.
- [#293](https://github.com/iron-io/functions/issues/293): fn: auto release for fn.
- [#288](https://github.com/iron-io/functions/issues/288): api: add upsert entrypoint for route updates.
- [#284](https://github.com/iron-io/functions/issues/284): Update iron/node image.
- [#275](https://github.com/iron-io/functions/issues/275): Functions API /tasks returns only one task ignoring query parameter `n`.
- [#274](https://github.com/iron-io/functions/issues/274): Support app deletion API .
- [#254](https://github.com/iron-io/functions/issues/254): HTTP POST to /apps/{app}/routes is not returning HTTP 409 in case of existing similar route.
- [#253](https://github.com/iron-io/functions/issues/253): HTTP POST to /app for app creation should return HTTP 409 if app already exists.
- [#252](https://github.com/iron-io/functions/issues/252): HTTP PUT to /apps/{app} creates new app instead of modifying initial.
- [#251](https://github.com/iron-io/functions/issues/251): Maybe drop the CONFIG_ prefix on user defined config vars?.
- [#235](https://github.com/iron-io/functions/issues/235): Docs: Missing Redis docs.
- [#229](https://github.com/iron-io/functions/issues/229): fnctl change suggestions.
- [#218](https://github.com/iron-io/functions/issues/218): Copy s3 event example from iron-io/lambda.
- [#216](https://github.com/iron-io/functions/issues/216): fnclt lambda commands need to automatically detect region from the AWS config.
- [#197](https://github.com/iron-io/functions/issues/197): Create an fnctl dns entry and enable ssl for install of the cli tool.
- [#182](https://github.com/iron-io/functions/issues/182): Remove error in logs when image not found.
- [#161](https://github.com/iron-io/functions/issues/161): Example slackbot - Copy guppy example over.
- [#134](https://github.com/iron-io/functions/issues/134): Dynamic runners scaling.
- [#126](https://github.com/iron-io/functions/issues/126): Detect OS and disable Memory profiling if needed.
- [#72](https://github.com/iron-io/functions/issues/72): Should the input stream include a headers section, just like HTTP?.
- [#69](https://github.com/iron-io/functions/issues/69): How to run on Openstack.
- [#20](https://github.com/iron-io/functions/issues/20): Make function testing framework.
- [#3](https://github.com/iron-io/functions/issues/3): Make "function tool" in ironcli.
- [#2](https://github.com/iron-io/functions/issues/2): Allow setting content-type on a route, then use that when responding.

## v0.1.0 [2016-11-18]

Alpha 1 Release