// Copyright 2017, OpenCensus Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package readme generates the README.
package readme

import (
	"context"
	"log"
	"time"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

// README.md is generated with the examples here by using embedmd.
// For more details, see https://github.com/rakyll/embedmd.

func statsExamples() {
	ctx := context.Background()

	// START measure
	videoSize, err := stats.Int64("my.org/video_size", "processed video size", "MB")
	if err != nil {
		log.Fatal(err)
	}
	// END measure
	_ = videoSize

	// START findMeasure
	m := stats.FindMeasure("my.org/video_size")
	if m == nil {
		log.Fatalln("measure not found")
	}
	// END findMeasure

	_ = m

	// START aggs
	distAgg := view.DistributionAggregation{0, 1 << 32, 2 << 32, 3 << 32}
	countAgg := view.CountAggregation{}
	sumAgg := view.SumAggregation{}
	meanAgg := view.MeanAggregation{}
	// END aggs

	_, _, _, _ = distAgg, countAgg, sumAgg, meanAgg

	// START view
	if err = view.Subscribe(&view.View{
		Name:        "my.org/video_size_distribution",
		Description: "distribution of processed video size over time",
		Measure:     videoSize,
		Aggregation: view.DistributionAggregation([]float64{0, 1 << 32, 2 << 32, 3 << 32}),
	}); err != nil {
		log.Fatalf("Failed to subscribe to view: %v", err)
	}
	// END view

	// START reportingPeriod
	view.SetReportingPeriod(5 * time.Second)
	// END reportingPeriod

	// START record
	stats.Record(ctx, videoSize.M(102478))
	// END record

	// START registerExporter
	// Register an exporter to be able to retrieve
	// the data from the subscribed views.
	view.RegisterExporter(&exporter{})
	// END registerExporter
}

// START exporter

type exporter struct{}

func (e *exporter) ExportView(vd *view.Data) {
	log.Println(vd)
}

// END exporter
