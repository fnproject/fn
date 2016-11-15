# Frequently Asked Questions

## Which languages are supported?

Since we use containers are the base building block, all languages can be used. There there may not be higher level 
helper libraries like our Lambda wrapper for every language, but you can use any language if you follow the 
base [function format](function-format.md).

## Where can I run IronFunctions?

Anywhere. Any cloud, on-premise, on your laptop. As long as you can run a Docker container, you can run IronFunctions.

## Which orchestration tools does IronFunctions support?

IronFunctions can be deployed using any orchestration tool. 

## Does IronFunctions require Docker?

For now, we do require Docker primarily for the packaging and distribution via Docker Registries. 
But we've built IronFunctions in a way that abstracts the container technology so we can support others as
needed. For instance, we'll probably add rkt support shortly.
