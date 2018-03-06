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
$ go get -u go.opencensus.io/...
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

## Tags

Tags represent propagated key-value pairs. They are propagated using context.Context
in the same process or can be encoded to be transmitted on the wire and decoded back
to a tag.Map at the destination.

### Getting a key by a name

A key is defined by its name. To use a key, a user needs to know its name and type.

[embedmd]:# (tags.go stringKey)
```go
// Get a key to represent user OS.
key, err := tag.NewKey("my.org/keys/user-os")
if err != nil {
	log.Fatal(err)
}
```

### Creating tags and propagating them

Package tag provides a builder to create tag maps and put it
into the current context.
To propagate a tag map to downstream methods and RPCs, New
will add the produced tag map to the current context.
If there is already a tag map in the current context, it will be replaced.

[embedmd]:# (tags.go new)
```go
osKey, err := tag.NewKey("my.org/keys/user-os")
if err != nil {
	log.Fatal(err)
}
userIDKey, err := tag.NewKey("my.org/keys/user-id")
if err != nil {
	log.Fatal(err)
}

ctx, err = tag.New(ctx,
	tag.Insert(osKey, "macOS-10.12.5"),
	tag.Upsert(userIDKey, "cde36753ed"),
)
if err != nil {
	log.Fatal(err)
}
```

### Propagating a tag map in a context

If you have access to a tag.Map, you can also
propagate it in the current context:

[embedmd]:# (tags.go newContext)
```go
m := tag.FromContext(ctx)
```

In order to update existing tags from the current context,
use New and pass the returned context.

[embedmd]:# (tags.go replaceTagMap)
```go
ctx, err = tag.New(ctx,
	tag.Insert(osKey, "macOS-10.12.5"),
	tag.Upsert(userIDKey, "fff0989878"),
)
if err != nil {
	log.Fatal(err)
}
```


## Stats

### Creating, retrieving and deleting a measure

Create and load measures with units:

[embedmd]:# (stats.go measure)
```go
videoSize, err := stats.Int64("my.org/video_size", "processed video size", "MB")
if err != nil {
	log.Fatal(err)
}
```

Retrieve measure by name:

[embedmd]:# (stats.go findMeasure)
```go
m := stats.FindMeasure("my.org/video_size")
if m == nil {
	log.Fatalln("measure not found")
}
```

### Creating an aggregation

Currently 4 types of aggregations are supported. The CountAggregation is used to count
the number of times a sample was recorded. The DistributionAggregation is used to
provide a histogram of the values of the samples. The SumAggregation is used to
sum up all sample values. The MeanAggregation is used to calculate the mean of
sample values.

[embedmd]:# (stats.go aggs)
```go
distAgg := view.DistributionAggregation([]float64{0, 1 << 32, 2 << 32, 3 << 32})
countAgg := view.CountAggregation{}
sumAgg := view.SumAggregation{}
meanAgg := view.MeanAggregation{}
```

### Creating, registering and unregistering a view

Create and register a view:

[embedmd]:# (stats.go view)
```go
v, err := view.New(
	"my.org/video_size_distribution",
	"distribution of processed video size over time",
	nil,
	videoSize,
	distAgg,
)
if err != nil {
	log.Fatalf("cannot create view: %v", err)
}
if err := view.Register(v); err != nil {
	log.Fatal(err)
}
```

Find view by name:

[embedmd]:# (stats.go findView)
```go
v = view.Find("my.org/video_size_distribution")
if v == nil {
	log.Fatalln("view not found")
}
```

Unregister view:

[embedmd]:# (stats.go unregisterView)
```go
if err = view.Unregister(v); err != nil {
	log.Fatal(err)
}
```

Configure the default interval between reports of collected data.
This is a system wide interval and impacts all views. The default
interval duration is 10 seconds. Trying to set an interval with
a duration less than a certain minimum (maybe 1s) should have no effect.

[embedmd]:# (stats.go reportingPeriod)
```go
view.SetReportingPeriod(5 * time.Second)
```

### Recording measurements

Recording usage can only be performed against already registered measure
and their registered views. Measurements are implicitly tagged with the
tags in the context:

[embedmd]:# (stats.go record)
```go
stats.Record(ctx, videoSize.M(102478))
```

### Retrieving collected data for a view

Users need to subscribe to a view in order to retrieve collected data.

[embedmd]:# (stats.go subscribe)
```go
if err := v.Subscribe(); err != nil {
	log.Fatal(err)
}
```

Subscribed views' data will be exported via the registered exporters.

[embedmd]:# (stats.go registerExporter)
```go
// Register an exporter to be able to retrieve
// the data from the subscribed views.
view.RegisterExporter(&exporter{})
```

An example logger exporter is below:

[embedmd]:# (stats.go exporter)
```go

type exporter struct{}

func (e *exporter) ExportView(vd *view.Data) {
	log.Println(vd)
}

```

## Traces

### Starting and ending a span

[embedmd]:# (trace.go startend)
```go
ctx, span := trace.StartSpan(ctx, "your choice of name")
defer span.End()
```

More tracing examples are coming soon...

## Profiles

OpenCensus tags can be applied as profiler labels
for users who are on Go 1.9 and above.

[embedmd]:# (tags.go profiler)
```go
ctx, err = tag.New(ctx,
	tag.Insert(osKey, "macOS-10.12.5"),
	tag.Insert(userIDKey, "fff0989878"),
)
if err != nil {
	log.Fatal(err)
}
tag.Do(ctx, func(ctx context.Context) {
	// Do work.
	// When profiling is on, samples will be
	// recorded with the key/values from the tag map.
})
```

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
