# Frequently Asked Questions

## Which languages are supported?

Since we use containers as the base building block, all languages can be used. There may not be higher level
helper libraries like our Lambda wrapper for every language, but you can use any language if you follow the
base [function format](function-format.md).

## Where can I run FN?

Anywhere. Any cloud, on-premise, on your laptop. As long as you can run a Docker container, you can run FN.

## Which orchestration tools does functions support?

Functions can be deployed using any orchestration tool.

## Does FN require Docker?

For now we require Docker primarily for the packaging and distribution via Docker Registries,
but we've built Functions in a way that abstracts the container technology so we can support others as
needed. For instance, we may add rkt support.
