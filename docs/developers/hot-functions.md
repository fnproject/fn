# Hot functions

By default, Fn uses "cold functions" where every request starts up a new container, feeds it with the payload then sends the
answer back to the caller. You can expect an average start time of [300ms per execution]((https://medium.com/travis-on-docker/the-overhead-of-docker-run-f2f06d47c9f3#.96tj75ugb)) to start the function/container.

Hot functions improve performance by starting a function then keeping it alive to handle additional requests. This
makes it a bit trickier to use because you'll have to parse a stream of requests, but if you use an [FDK](fdks.md) it's
all taken care of for you, so you should probably use an FDK in most cases.

## Making a hot function

In your `func.yaml`, add `format: json`.

From there, we recommend using one of our [FDKs](fdks.md) to handle all the parsing and formatting. But if you'd like to learn
about the nitty gritty details, [check here](function-format.md).
