# OpenCensus Libraries for Go

[![Build Status][travis-image]][travis-url]
[![Windows Build Status][appveyor-image]][appveyor-url]
[![GoDoc][godoc-image]][godoc-url]
[![Gitter chat][gitter-image]][gitter-url]

OpenCensus Go is a Go implementation of OpenCensus, a toolkit for
collecting application performance and behavior monitoring data.
Currently it consists of three major components: tags, stats, and tracing.

This project is still at a very early stage of development. The API is changing
rapidly, vendoring is recommended.


## Installation

```
$ go get -u go.opencensus.io
```

## Prerequisites

OpenCensus Go libraries require Go 1.8 or later.

## Exporters

OpenCensus can export instrumentation data to various backends. 
Currently, OpenCensus supports:

* [Prometheus][exporter-prom] for stats
* [OpenZipkin][exporter-zipkin] for traces
* Stackdriver [Monitoring][exporter-stackdriver] and [Trace][exporter-stackdriver]
* [Jaeger][exporter-jaeger] for traces
* [AWS X-Ray][exporter-xray] for traces

## Tags

Tags represent propagated key-value pairs. They are propagated using context.Context
in the same process or can be encoded to be transmitted on the wire and decoded back
to a tag.Map at the destination.

### Getting a key by a name

A key is defined by its name. To use a key, a user needs to know its name and type.

[embedmd]:# (tags.go stringKey)

### Creating tags and propagating them

Package tag provides a builder to create tag maps and put it
into the current context.
To propagate a tag map to downstream methods and RPCs, New
will add the produced tag map to the current context.
If there is already a tag map in the current context, it will be replaced.

[embedmd]:# (tags.go new)

### Propagating a tag map in a context

If you have access to a tag.Map, you can also
propagate it in the current context:

[embedmd]:# (tags.go newContext)

In order to update existing tags from the current context,
use New and pass the returned context.

[embedmd]:# (tags.go replaceTagMap)


## Stats

### Measures

Measures are used for recording data points with associated units.
Creating a Measure:

[embedmd]:# (stats.go measure)

### Recording Measurements

Measurements are data points associated with Measures.
Recording implicitly tags the set of Measurements with the tags from the
provided context:

[embedmd]:# (stats.go record)

### Views

Views are how Measures are aggregated. You can think of them as queries over the
set of recorded data points (Measurements).

Views have two parts: the tags to group by and the aggregation type used.

Currently four types of aggregations are supported:
* CountAggregation is used to count the number of times a sample was recorded.
* DistributionAggregation is used to provide a histogram of the values of the samples.
* SumAggregation is used to sum up all sample values.
* MeanAggregation is used to calculate the mean of sample values.

[embedmd]:# (stats.go aggs)

Here we create a view with the DistributionAggregation over our Measure.
All Measurements will be aggregated together irrespective of their tags,
i.e. no grouping by tag.

[embedmd]:# (stats.go view)

Subscribe begins collecting data for the view. Subscribed views' data will be
exported via the registered exporters.

[embedmd]:# (stats.go registerExporter)

An example logger exporter is below:

[embedmd]:# (stats.go exporter)

Configure the default interval between reports of collected data.
This is a system wide interval and impacts all views. The default
interval duration is 10 seconds.

[embedmd]:# (stats.go reportingPeriod)


## Traces

### Starting and ending a span

[embedmd]:# (trace.go startend)

More tracing examples are coming soon...

## Profiles

OpenCensus tags can be applied as profiler labels
for users who are on Go 1.9 and above.

[embedmd]:# (tags.go profiler)

A screenshot of the CPU profile from the program above:

![CPU profile](https://i.imgur.com/jBKjlkw.png)


[travis-image]: https://travis-ci.org/census-instrumentation/opencensus-go.svg?branch=master
[travis-url]: https://travis-ci.org/census-instrumentation/opencensus-go
[appveyor-image]: https://ci.appveyor.com/api/projects/status/vgtt29ps1783ig38?svg=true
[appveyor-url]: https://ci.appveyor.com/project/opencensusgoteam/opencensus-go/branch/master
[godoc-image]: https://godoc.org/go.opencensus.io?status.svg
[godoc-url]: https://godoc.org/go.opencensus.io
[gitter-image]: https://badges.gitter.im/census-instrumentation/lobby.svg
[gitter-url]: https://gitter.im/census-instrumentation/lobby?utm_source=badge&utm_medium=badge&utm_campaign=pr-badge&utm_content=badge


[new-ex]: https://godoc.org/go.opencensus.io/tag#example-NewMap
[new-replace-ex]: https://godoc.org/go.opencensus.io/tag#example-NewMap--Replace

[exporter-prom]: https://godoc.org/go.opencensus.io/exporter/prometheus
[exporter-stackdriver]: https://godoc.org/go.opencensus.io/exporter/stackdriver
[exporter-zipkin]: https://godoc.org/go.opencensus.io/exporter/zipkin
[exporter-jaeger]: https://godoc.org/go.opencensus.io/exporter/jaeger
[exporter-xray]: https://godoc.org/go.opencensus.io/exporter/xray
