# Frequently Asked Questions

# Overview

## What is the Fn Project?

The Fn Project is a container native serverless platform that you can run anywhere -- any cloud or on-premise. It’s easy to use, supports every programming language, and is extensible and performant.

## Why build another serverless framework?

Our vision at Iron.io was a lot larger than a simple FaaS platform. We set out to build a platform, a rich ecosystem, and an experience that is both welcoming and embracing to developers of all skill levels and companies from small 1-person teams to the largest global enterprises. This proved difficult as a startup ourselves, but now at Oracle we are equipped and resourced to carry out this vision. Here are some of the key differentiators we believe will set the Fn Project apart:

* **Open Source:** We believe that open source is the way software is now delivered and adopted. Everything in the Fn Project is open source under Apache 2.0 with an open and transparent governance model.

* **Multi Cloud:** Whether you are adopting multiple clouds or not, your technology stack should not lock you into one. Everything we build in the Fn Project will always cover multiple cloud providers including running on your own hardware. Serverless should feel serverless to developers, but enterprises still have a lot of actual servers that can be utilized.

* **Developer Experience:** Despite the economics and operational efficiencies being very attractive for the business and ops teams, serverless remains an architecture for developer empowerment and agility. That is why the experience is essential and must be baked into the product every step of the way. From `fn init` to `fn deploy`, we’re thinking about how to make the Fn Project natural, elegant, and fun.

* **Container Native:** Containers fundamentally change the way we package software. Our goal for the Fn Project is to abstract out the complexities of containers, even create a “containerless experience”, but expose the power of containers to those who have adopted containers as their packaging format. That is why `fn deploy` can abstract the whole process away from you, but we also support native Docker containers as Functions. This party is optionally BYOD: Bring your own Dockerfile.

* **Programming Model:** The cost, operational benefits, and hype, of serverless has created a rush to adopt, and this has led to many fantastic use cases including ops tooling, event-driven architectures, triggers in the cloud, etc., but there are still technology gaps preventing more complex serverless app design utilizing native language features, true IDE integration, testing, workflow, etc. We want to address this, starting with the release of the Java FDK and Fn Flow as initial blueprints.

* **Orchestrator Agnostic:** Kubernetes is great, and deployments of Fn can benefit from it handling all the lower level infrastructure, but it’s not the only game in town, nor do we want end users of Fn having to learn or deal with Kubernetes. A clear separation of serverless and container orchestration is important, thus allowing the project to adapt and evolve in an ever-changing cloud landscape.

* **Vision and Depth:** Fn, Flow, and the FDK’s are the foundation, but there’s a lot more to come, and over the years we’ve established a strong vision for where serverless is and needs to go. Truly going multi-cloud serverless takes a wider stack of services, and much of this work is ahead of us. We’ll start to work with the community and partners on a roadmap very soon. Join us on this journey. For ways to get involved, see below.

* **Sustainability:** No not saving the planet (although compute efficiencies of serverless will certainly have that effect), I mean many projects are flashes in the pan. It’s easy to release something and get to the front of hacker news for a few days, but it’s much harder to sustain the momentum, community, and vision all while maintaining technical and usability excellence. Our team founded and built a successful startup so we know the difficulties of the journey and now at Oracle we’re excited to see the complete vision through. We’re ready to run the marathon it takes to build a great and lasting project.

## What are the key components of Fn?

The Fn Project today consists of 4 major components:

1. Fn Server is the Functions-as-a-Service system that allows developers to easily build, deploy, and scale their functions into a multi-cloud environment. It’s fast, reliable, scalable, and container-native, which I’ll discuss more below.

2. The Fn Load Balancer (Fn LB) allows operators to deploy clusters of Fn servers and route traffic to them intelligently. Most importantly, it will route traffic to nodes where hot functions are running to ensure optimal performance, as well as distribute load if traffic to a specific function increases. It also gathers information about the entire cluster which you can use to know when to scale out (add more Fn servers) or in (decrease Fn servers).

3. Fn FDK’s — Starting with Java, we are releasing a number of FDK’s, or Function Development Kits, aimed at quickly bootstrapping functions in all languages, providing a data binding model for function inputs, make testing your functions easier, as well as lay the foundation for building more complex serverless applications.

4. Fn Flow allows developers to build and orchestrate higher level workflows of functions all inside their programming language of choice. It makes it easy to use parallelism, sequencing/chaining, error handling, fan in/out, etc., without learning complicated external models built with long JSON or YAML templates. Best of all, Flow tracks all of the function call graphs, allowing for visualization in the dashboard, full stack logs of entire “Flows”, and variable/memory reconstitution throughout the entire function graph.

## Why open source Fn?

We believe that an open container native cloud platform based on Docker and Kubernetes is the future.  As such we want to ensure that anyone can write and deploy functions to any cloud provider so that customers have choice.  That said, we intend to compete hard to make sure that our cloud infrastructure is the best platform to run those functions.

# Technical

## What languages can be used to write functions on Fn?

Out of the box support includes: Java, Go, Ruby, Python, PHP, Rust, .NET Core, and Node.js (including AWS Lambda compatibility).

Since we use containers as the base building block, all languages can be used. There may not be higher level helper libraries like our Lambda wrapper for every language, but you can use any language if you follow the base function format.

## What platforms does Fn run on?

Any Linux/Unix platform with Docker support which includes Linux, MacOS, and FreeBSD.

## What software do I need to have installed as a prerequisite for running Fn?

You’ll need a recent release of Docker installed.

## Why do I need Docker installed to run Fn?

Fn packages functions as Docker containers which are published to a Docker registry for deployment.  The Fn server pulls images from a Docker registry when functions are invoked.

## Where can I run Fn?

Anywhere. Any cloud, on-premise, on your laptop, even on AWS or Azure. As long as you can run a Docker container, you can run Fn. 

## Do I need any accounts to run Fn?

You don’t need any special accounts to run Fn locally.  But to deploy functions to a remote Fn server you’ll need access to a Docker registry and an account.  Docker Hub is the default registry.

## Which orchestration tools does functions support?

Functions can be deployed using any orchestration tool.

## Can I deploy functions with a custom runtime?

Yes, as Fn packages and deploys all functions as Docker containers it’s possible to provide a custom Docker image that includes your function.  As long as your container implements the Fn function contract it can contain anything.

## What is the lifecycle of a function?

Functions are packaged as Docker images and by default individual containers are created to handle a function request and are then destroyed.  However, `Hot Functions` are not disposed of after handling a single request.

## What’s a `Hot Function`?

`Hot Functions` are functions that are not destroyed after a single use but are retained and used to handle subsequent requests.  Hot Functions must accept HTTP input.

# General

## What is the Fn Project’s license?

Apache 2.0

## What is the URL of the project website?

<http://fnproject.io>

## Where is the source code hosted?

All of the project code is on Github at <http://github.com/fnproject>.

## Is Fn an entirely new platform?

No, Fn is derived from the well-received IronFunctions project.  The core IronFunctions team are now at Oracle so Fn is simply the next evolutionary step of IronFunctions built by the original developers.

## How do you pronounce “Fn”?

Formally you can say “F" "N”, but in a sentence you might informally use “effen”.  For example, when describing how you invoke a function with the Fn CLI you might say: “effen call myapp myfunction”. ;-)

# Network

## Can Fn do IO over HTTP or in general any TCP based communication?

Like most functions platforms we support HTTP.

## Can an Fn function connect to various endpoints to read/write data?

Yes, a function is not restricted in what it can connect to.

## Any limits on the network bandwidth?

We currently don't yet offer Fn as a managed service which would manage network traffic.  Currently, there are no restrictions in the core Fn platform.

# Performance

## How long can a function run? Can we make it run forever?

We support 'hot functions' (see the end of [this tutorial](https://github.com/fnproject/tutorials/blob/master/JavaFDKIntroduction/README.md) for an example).  Hot functions will continue to live if they are used but, if not, will eventually be cleaned up.

Function timeout is configurable. Please see <https://github.com/fnproject/fn/blob/master/docs/function-file.md>

## How does the service trace the liveliness of the function? If my function dies/crashes will the service provision it again?

This is not a microservices platform so the notion is slightly different.  Functions are run when called.  If you call a function that doesn't exist then one will be started.

## How does scaling work? Should this be initiated by a customer?

Fn runs more instances of the function based on requests.  It is automatic.

## When one host is overloaded, will this service automatically relocate the function to another host?

When running on Kubernetes (K8s) workloads are spread across available hosts.

Functions currently do not relocate. Please see our [FnLB project inside Fn Project](https://github.com/fnproject/fn/tree/master/fnlb) for how to route requests to available hosts. K8s can also help with this.

# Java

## Any Java runtime dependencies? Can we use concurrent libraries or any third party?

We currently support Maven which allows you to specify your dependencies. Therefore, you can use concurrency and third party libraries.

## What versions of Java are supported (JDK 8, JDK 9)?

We default to Java 9 now but if you want Java 8, the runtime name is `java8` (when you init a function or in your `func.yaml`).

# Persistence

## Does your runtime provide any persistent store to save the function state information? 

State management is not part of Fn but you can use any storage service or database to store data.

## Can we build a state function which will update itself based on the previous computation? Will it persist across function restarts?

The problem you face is the lack of a guarantee of which instance of a hot function is called.  Standard practice is to externalize state.  The State Service is essentially Oracle Coherence which offers excellent performance as you may know.

Also check out Fn Flow if you’re writing Java (currently the primary supported language).

## Do you provide any UI/tool to track our Fn status?

Fn has a dashboard that can be found at <https://github.com/fnproject/ui>. Fn Flow also has an experimental dashboard at <https://github.com/fnproject/flowui>. 

# Fn Java Function Development Kit (FDK)

## What is the Fn Java FDK?

The Fn Java FDK helps you build, test and run Java functions on the Fn platform.

The FDK is comprised of:

* a Java library (known as the `api`)
* a runtime Docker image (known as the `runtime`)
* a JUnit rule
* a build-time Docker image for repeatable builds

## Is the FDK required in order to run Java on Fn?

No. You can still write Java functions on Fn without using the FDK. However using the FDK will make several things easier for you:

1. A curated base image for Java 8 and Java 9 means that you don't have to build and maintain your own image. These images contain optimizations for quick JVM startup times.
2. Accessing configuration from Fn is easy through FDK APIs.
3. Input and output type coercion reduces the amount of serialization and formatting boilerplate that you have to write.
4. A JUnit rule provides a realistic test harness for you to test your function in isolation.

# FN Flow

## What is Fn Flow?

Fn Flow is a [Java API](https://github.com/fnproject/fn-java-fdk/blob/master/docs/FnFlowsUserGuide.md) and [corresponding service](https://github.com/fnproject/completer) that helps you create complex, long-running, fault-tolerant functions using a promises-style asynchronous API. Check out the [Fn Flow docs](https://github.com/fnproject/fn-java-fdk/blob/master/docs/FnFlowsUserGuide.md) for more information.

### How do I get the FDK?

The FDK is automatically added to your project if you built your function using `fn init --runtime=java`. The `api` and `testing` JARs are published on [our bintray](https://bintray.com/fnproject/fnproject) and the `runtime` is published in our [Docker hub repository](https://hub.docker.com/r/fnproject/fn-java-fdk/).

## How do I add the FDK to an existing project?

* Find the latest release from the [releases page](https://github.com/fnproject/fn-java-fdk/releases). For example `1.0.32`.
* The FDK JAR is published on [Bintray](https://bintray.com/fnproject/fnproject). Add the repository to your `pom.xml`, `repositories` section:

```xml
        <repository>
            <id>fn-release-repo</id>
            <url>https://dl.bintray.com/fnproject/fnproject</url>
            <releases>
                <enabled>true</enabled>
            </releases>
            <snapshots>
                <enabled>false</enabled>
            </snapshots>
        </repository>
```

* Add the dependency to your `dependency` section. Make sure that the `version` tag matches the latest release that you looked up above.
   
```xml
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>api</artifactId>
            <version>1.0.32</version>
        </dependency>
```

* If you want the JUnit support also add:
   
```xml
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>testing</artifactId>
            <version>1.0.32</version>
            <scope>test</scope>
        </dependency>
```

## What is the latest version of the FDK?

Have a look on our [releases page](https://github.com/fnproject/fn-java-fdk/releases).

## How do I upgrade to the latest version of the FDK?

* Find the latest version from our [releases page](https://github.com/fnproject/fn-java-fdk/releases).
* Update the Java `api` version: change your dependency management system to reference the latest artifact. For example to upgrade a maven build to use `1.0.32`:

```xml
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>api</artifactId>
            <version>1.0.32</version>
        </dependency>
        <dependency>
            <groupId>com.fnproject.fn</groupId>
            <artifactId>testing</artifactId>
            <version>1.0.32</version>
            <scope>test</scope>
        </dependency>
```
* Update your runtime image to latest: 
               
```sh
docker pull fnproject/fn-java-fdk
```

* Or update your runtime image to a specific version:
       
```sh
docker pull fnproject/fn-java-fdk:1.0.32
docker tag fnproject/fn-java-fdk:1.0.32 fnproject/fn-java-fdk:latest
```
   
> You should keep versions of `com.fnproject.fn.api`, `com.fnproject.fn.testing` and your runtime Docker image in sync with each other.

* Update your build image:
   
```sh
docker pull fnproject/fn-java-fdk-build
```
   
## I think I found a bug - how do I report it?

Please create an [issue on our GitHub repo](https://github.com/fnproject/fn-java-fdk/issues).

## My question is not answered here - how do I get more help?

We hang out on the [Fn project slack](https://join.slack.com/t/fnproject/shared_invite/enQtMjIwNzc5MTE4ODg3LTdlYjE2YzU1MjAxODNhNGUzOGNhMmU2OTNhZmEwOTcxZDQxNGJiZmFiMzNiMTk0NjU2NTIxZGEyNjI0YmY4NTA) in #fn-flow, #fn-java-fdk and #completer.

## How do I write a function that takes advantage of the Fn Flow API?

Please see our [Fn Flow User Guide](https://github.com/fnproject/fn-java-fdk/blob/master/docs/FnFlowsUserGuide.md) for information on how to get started with Fn Flow in Java.

## When should I use Fn Flow?

Use Fn Flow when:
   
- You want to compose the work of several other Fn functions in a reliable and scalable way.
- You find yourself wanting to write code that blocks on the result of another Fn call.
- You feel tempted to reach for another workflow system or library.

## Is Fn Flow related to `java.util.concurrent.Flow`?

No. `java.util.concurrent.Flow` is a stream-processing API for running code in a single JVM. Fn Flow is a distributed promise API for writing long-running, fault-tolerant asynchronous functions.

## Is Fn Flow related to any other Java library called Flow?

No. Fn Flow was released in 2017 and is not related to any other Java library.

## What is the relationship between Fn Flow and Java's `CompletionStage` and `CompletableFuture` API?

Fn Flow was 'inspired by' the `CompletionStage` API and shares a number of similar methods. However, as the semantics of Fn Flow are subtly different, and because we require our operations to implement `Serializable` we have implemented a new API.

## Can I run my own Fn Flow server?

Fn Flow is open source just like the rest of the Fn project. You can [get the code](https://github.com/fnproject/completer) and [run a server](https://github.com/fnproject/completer#running-the-completer) easily.
