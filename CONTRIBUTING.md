# Contributing to Fn

We welcome all contributions! We accept contributions through GitHub pull
requests and issues. Before setting out on making any significant
contribution, please file an issue or assign an issue to yourself.

## Reporting security issues

The Fn maintainers take security seriously. If you discover a security
issue, please bring it to their attention right away!

Please **DO NOT** file a public issue, instead send your report privately to
[TODO@fnproject.io](mailto:TODO@fnproject.io). Please have as the
subject `[FN SECURITY BUG]: $your_title`.

Security reports are greatly appreciated and we will publicly thank you for
it. We currently do not offer a paid security bounty program, but are not
ruling it out in the future. We do, however, offer swag. 

## Reporting other issues

A great way to contribute to the project is to send a detailed report when you
encounter an issue. We always appreciate a well-written, thorough bug report,
and will thank you for it!

Check that [our issues](https://github.com/fnproject/fn/issues)
doesn't already include that problem or suggestion before submitting an issue.
If you find a match, you can use the "subscribe" button to get notified on
updates. Please do *not* leave random "+1" or "I have this too" comments, as they
only clutter the discussion, and don't help resolving it. However, if you
have ways to reproduce the issue or have additional information that may help
resolving the issue, please leave a comment. Please *do* leave a "+1" thumbs
up reaction on the parent comment, this greatly helps us prioritize issues!

When reporting issues, always include:

* The output of `fn version`.

Also include the steps required to reproduce the problem if possible and
applicable. This information will help us review and fix your issue faster.
When sending lengthy log-files, consider posting them as a gist (https://gist.github.com).
Don't forget to remove sensitive data from your logfiles before posting (you can
replace those parts with "REDACTED").

## Rules of Fn core (ie: what belongs here and what doesn't)

Fn server (core) is only for the essential parts of the entire Fn ecosystem. 
These include:

- The core API (apps, routes, calls, logs)
- Executing functions (sync and async)
- Extension points (callbacks, middleware, API additions)

This does __not__ include:

- authentication
- stats/metrics
- special/optional features such as triggers, fdk's, workflows, event sources, etc.
- could be argued that additional I/O formats beyond the basic ones we support should be built as plugins too

Rule of thumb: If it could be built as an extension, then build it as an extension. 

We WILL accept any reasonable additions to extension points in order to support building extensions. 

We WILL do whatever we can to make it easy for users to add extensions (easy builds or use Go plugins). 

Graduation: Some extensions can graduate into core if they become commonplace in the community (ie: majority of users are using it). 

## How to contribute

1. Fork the repo
2. Fix an issue or create an issue and fix it
3. Create a Pull Request that fixes the issue
4. Sign the [CLA](http://www.oracle.com/technetwork/community/oca-486395.html)
5. Once processed, our CLA bot will automatically clear the CLA check on the PR
6. Good Job! Thanks for being awesome!

## Code style

The coding style suggested by the Golang community is used in fn. See the [style doc](https://github.com/golang/go/wiki/CodeReviewComments) for details.

Please follow this style to make fn easy to review, maintain and develop.

### Commit formatting

Commit messages must start with a capitalized and short summary (max. 50 chars)
written in the imperative, followed by an optional, more detailed explanatory
text which is separated from the summary by an empty line. The better the
commit message, the less dialogue we will need to get your PR merged!

Commit messages should follow best practices, including explaining the context
of the problem and how it was solved, including in caveats or follow up changes
required. They should tell the story of the change and provide readers
understanding of what led to it.

Pull requests must be cleanly rebased on top of master without multiple branches
mixed into the PR.

**Git tip**: If your PR no longer merges cleanly, use `rebase master` in your
feature branch to update your pull request rather than `merge master`.

Before you make a pull request, squash your commits into logical units of work
using `git rebase -i` and `git push -f`. A logical unit of work is a consistent
set of patches that should be reviewed together: for example, upgrading the
version of a vendored dependency and taking advantage of its now available new
feature constitute two separate units of work. Implementing a new function and
calling it in another file constitute a single logical unit of work. The very
high majority of submissions should have a single commit, so if in doubt: squash
down to one.

Include an issue reference like `Closes #XXXX` or `Fixes #XXXX` in commits that
close an issue. Including references automatically closes the issue on a merge.

When updating your PR with new commits, keep in mind that reviewers may only
be notified when you comment (not on every commit), please don't hesitate to
ping us on the PR when you are ready for another look, we will try to take a
look as soon as we can.

## Documentation

When creating a Pull Request, make sure that you also update the documentation
accordingly.

Most of the time, when making some behavior more explicit or adding a feature,
documentation update is necessary.

You will either update a file inside [docs/](./docs/) or create one. Prefer
the former over the latter. If you are unsure, do not hesitate to open a PR
with a comment asking for suggestions on how to address the documentation part
or ask in slack.

## How to build and get up and running

### Build Dependencies ###
- [Go](https://golang.org/doc/install)
- [Dep](https://github.com/golang/dep)

### Getting the Repository ###

`$ git clone https://github.com/fnproject/fn.git $GOPATH/src/github.com/fnproject/fn`

Note that Go will require the exact path given above in order to build

### Build

Requires Go >= 1.10.0.

Change to the correct directory (if not already there):

	$ cd $GOPATH/src/github.com/fnproject/fn

The first time after you clone or after dependencies get updated, run:

```sh
make dep
```

Then after every change, run:

```sh
make run
```

This builds and runs the `fn` binary. It will start Fn using an embedded `sqlite3` database running on port `8080`.

### Test

```sh
make test
```

#### Run in Docker

Start Fn inside a Docker container:

```sh
make docker-run
```

## Tests in Docker

Run tests inside a Docker container:

```sh
make docker-test

```
