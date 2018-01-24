# Testing Functions

`fn` has testing built in that allows you to create inputs and expected outputs and verify the expected output with actual output. 

## Write a Test File

Create a file called `test.json` in your functions directory (beside your `func.yaml` file). Here's a simple example:

```json
{
    "tests": [
        {
            "input": {
                "body": {
                    "name": "Johnny"
                }
            },
            "output": {
                "body": {
                    "message": "Hello Johnny"
                }
            }
        },
        {
            "input": {
                "body": ""
            },
            "output": {
                "body": {
                    "message": "Hello World"
                }
            }
        }
    ]
}
```

The example above has two tests, one with the following input:

```json
{
    "name": "Johnny"
}
```

and a second one with no input. 

The first one is expected to return a json response with the following:

```json
{
    "message": "Hello Johnny"
}
```

And the second should return:

```json
{
    "message": "Hello World"
}
```

## Run Tests

In your function directory, run:

```sh
fn test
```

You can also test against a remote `fn` server by using the `--remote` flag. eg:

```sh
fn test --remote myapp
```

To test your entire Fn application:

```sh
fn test --all
```
