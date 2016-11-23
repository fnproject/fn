# Using rust with functions

The easiest way to create a iron function in rust is via ***cargo*** and ***fn***.

## Prerequisites
First create an epty rust project as follows:
```bash
cargo init --name func --bin
```

Make sure the project name is ***func*** and is of type ***bin***. Now just edit your code, once done you can create an iron function.

## Creating an IronFunction
Simply run

```bash
fn init --runtime=rust <username>/<funcname>
```

This will create the ```func.yaml``` file required by functions, which can be built by running:

```bash
fn build
```

## Testing

```bash
fn run
```
