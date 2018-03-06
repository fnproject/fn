// Copyright 2018, OpenCensus Authors
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

package view_test

import (
	"log"

	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
)

func Example_view() {
	m, err := stats.Int64("my.org/measure/openconns", "open connections", "")
	if err != nil {
		log.Fatal(err)
	}

	v, err := view.New(
		"my.org/views/openconns",
		"open connections distribution over one second time window",
		nil,
		m,
		view.DistributionAggregation([]float64{0, 1000, 2000}),
	)
	if err != nil {
		log.Fatal(err)
	}
	if err := v.Subscribe(); err != nil {
		log.Fatal(err)
	}

	// Use stats.RegisterExporter to export collected data.
}
