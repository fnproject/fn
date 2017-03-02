# Using dotnet with functions

Make sure you downloaded and installed [dotnet](https://www.microsoft.com/net/core). Now create an empty dotnet project in the directory of your function:

```bash
dotnet new
```

By default dotnet creates a ```Program.cs``` file with a main method. To make it work with IronFunction's `fn` tool please rename it to ```func.cs```.
Now change the code as you desire to do whatever magic you need it to do. Once done you can now create an iron function out of it.

## Creating an IronFunction
Simply run

```bash
fn init <username>/<funcname>
```

This will create the ```func.yaml``` file required by functions, which can be built by running:


## Build the function docker image
```bash
fn build
```

## Push to docker
```bash
fn push
```

This will create a docker image and push the image to docker.

## Publishing to IronFunctions

```bash
fn routes create <app_name> </path>
```

This creates a full path in the form of `http://<host>:<port>/r/<app_name>/<function>`


## Testing

```bash
fn run
```

## Calling

```bash
fn call <app_name> <funcname>
```

or

```bash
curl http://<host>:<port>/r/<app_name>/<function>
```