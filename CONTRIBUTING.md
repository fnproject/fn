## Contributing

### Build

```
make all
```

### Run Functions service

First let's start our IronFunctions API

##### Run in Docker

```
make run-docker
```

will start Functions using an embedded `Bolt` database running on `:8080`. 

##### Running on Metal (recommended only on Linux)

```
./functions
```

will start Functions with a default of 1 async runner

### Contributing

##### Code
* Fork the repo
* Fix an issue
* Create a Pull Request
* Sign the CLA
* Good Job! Thanks for being awesome!
