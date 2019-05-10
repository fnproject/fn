# 1.0

goal: sync invoke + http triggers, in prod-ready k8s cluster, stable APIs/releases

chores:

* all SDKs auto-release from swagger in circleci, updated, and full, working coverage for all supported langs (java, go, node, python, ruby) - this includes adding invoke, and may require using different swagger generator library to get working SDKs. possibly use OpenAPI 3.0 spec.
* FDK images release from circleCI, on a weekly(?) cron job, from their own respective repos (need permissions here, for circle&dhub), from the same base image (possibly OL), w/ security checks ideally
* FDK send version to fn on response, track this in fn (this is mostly finagling build for FDK to be able to see this at runtime, and permissions to release FDK from circle)
* Finish init images work, have for each FDK, release from circleci
* remove fields from API that are not on service (idle_timeout, cpu), some may need mechanisms to replace (shapes) to be added to OSS
* full support for private images with registry/repo:tag syntax, in CLI, API, and SDK (need UX story around context/fn name/etc). currently, validation is inconsistent across all 3, and CLI experience may be an issue when using this syntax for function names (maybe CLI should have separate image field in func.yaml instead of flattening into name? can keep pwd name logic out of box, but allow specifying image override?)
? function versioning + immutable images (use image digest)
* k8s with lb/cp/dp needs to work, under sustained load, and be secure and have useful performance. ideally, there’s a scaling story, but a static cluster may be OK for 1.0. docs need to be really solid for this, for operators specifically. need grafana dashboards, metrics docs, debugging docs, configuration docs, user logs docs, etc. this task is loaded and could be split up into a few things (k8s, perf, docs, security, etc)
* Extensions examples should work (easy-ish)
* Non-kafkaesque CLA bot (is it too much to ask? internal process blocking)
* release channels, release cadence, changelog
* shrink core from 40 to ~5?
* master is protected branch on all [supported] repos

notably missing things:

* no gateway
* no async
* no PM
* no user function metrics(?)
