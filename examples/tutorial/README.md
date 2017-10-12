
# Tutorial Series

Welcome to the Fn Tutorial Series, the best way to get started with Fn and serverless computing. In the following tutorials, we'll gradually introduce many of the key features of Fn.

## Prequisites
* [Quickstart](../../README.md) has been completed.

When starting a new shell, remember to:

```
# Log Docker into your Docker Hub account
docker login

# Set your Docker Hub username
export FN_REGISTRY=<DOCKERHUB_USERNAME>
```

## Guided Tour

### Part 1

Learn the basics about sending data into your function. Choose your language:

* [go](hello/go)
* [java](hello/java)
* [node](hello/node)
* [php](hello/php)
* [python](hello/python)
* [ruby](hello/ruby)
* [rust](hello/rust) 

### Part 2

Learn how to get parameters from a web request. [Click here](params)

### Part 3

Write your first HotFunction (stays alive to minimize latency between requests). [Click here](hotfunctions/http)

## Other Tutorials

* [Introduction to Fn](https://github.com/fnproject/tutorials/tree/master/Introduction)
* [Introduction to Java Fn](https://github.com/fnproject/tutorials/tree/master/JavaFDKIntroduction)
