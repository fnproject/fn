
The tests in these directory will test all the API endpoints and options.

One time:

```sh
bundle install
```

Start `iron/worker-api` and `iron/worker-runner`

Then:

```sh
HOST=localhost:8080 bundle exec ruby test.rb
```

To run single test, add `-n testname`

To test private images use env variables `TEST_AUTH` and `TEST_PRIVATE_IMAGE`
`TEST_AUTH` is encoded to base64 `user:pass` string.
