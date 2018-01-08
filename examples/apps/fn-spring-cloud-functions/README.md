# Example Spring Cloud Function

This is an example [spring cloud function](https://github.com/spring-cloud/spring-cloud-function) 
project running on Fn using the 
[`SpringCloudFunctionInvoker`](/runtime/src/main/java/com/fnproject/fn/runtime/spring/SpringCloudFunctionInvoker.java).

Firstly, if you have used `fn` before you'll want to make sure you have the latest runtime image which includes the Spring support:

```bash
$ docker pull fnproject/fdk-java:latest
```

Then you can build and deploy the app

```bash
fn build
fn deploy --local --app spring-cloud-fn

# Set up a couple of routes for different functions
fn routes create spring-cloud-fn /upper
fn routes config set spring-cloud-fn /upper FN_SPRING_FUNCTION upperCase

fn routes create spring-cloud-fn /lower
fn routes config set spring-cloud-fn /lower FN_SPRING_FUNCTION lowerCase
```

Now you can call those functions using `fn call` or curl:

```bash
$ echo "Hi there" | fn call spring-cloud-fn /upper
HI THERE

$ curl -d "Hi There" http://localhost:8080/r/spring-cloud-fn/lower
hi there
```


## Code walkthrough

```java
@Configuration
```
Defines that the class is a 
[Spring configuration class](https://docs.spring.io/spring-framework/docs/current/javadoc-api/org/springframework/context/annotation/Configuration.html) 
with `@Bean` definitions inside of it.

```java
@Import(ContextFunctionCatalogAutoConfiguration.class)
```
Specifies that this configuration uses a [`InMemoryFunctionCatalog`](https://github.com/spring-cloud/spring-cloud-function/blob/a973b678f1d4d6f703a530e2d9e071b6d650567f/spring-cloud-function-context/src/main/java/org/springframework/cloud/function/context/InMemoryFunctionCatalog.java)
that provides the beans necessary
for the `SpringCloudFunctionInvoker`.

```java
    ...
    @FnConfiguration
    public static void configure(RuntimeContext ctx) {
        ctx.setInvoker(new SpringCloudFunctionInvoker(SCFExample.class));
    }
```

Sets up the Fn Java FDK to use the SpringCloudFunctionInvoker which performs function discovery and invocation.

```java
    // Unused - see https://github.com/fnproject/fdk-java/issues/113
    public void handleRequest() { }
```

Currently the runtime expects a method to invoke, however this isn't used in the SpringCloudFunctionInvoker so we declare an empty method simply to keep the runtime happy. This will not be necessary for long - see the linked issue on GitHub.


```java
    @Bean
    public Function<String, String> upperCase(){
        return String::toUpperCase;
    }

    @Bean
    public Function<String, String> lowerCase(){
        return String::toLowerCase;
    }
```

Finally the heart of the configuration; the bean definitions of the functions to invoke.

Note that these methods are not the functions themselves. They are factory methods which *return* the functions. As the Beans are constructed by Spring it is possible to use `@Autowired` dependency injection.
