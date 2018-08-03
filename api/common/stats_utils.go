package common

import (
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

func CreateView(measure stats.Measure, agg *view.Aggregation, tagKeys []string) *view.View {
	return &view.View{
		Name:        measure.Name(),
		Description: measure.Description(),
		Measure:     measure,
		TagKeys:     makeKeys(tagKeys),
		Aggregation: agg,
	}
}

func MakeMeasure(name string, desc string, unit string) *stats.Int64Measure {
	return stats.Int64(name, desc, unit)
}

func makeKeys(names []string) []tag.Key {
	tagKeys := make([]tag.Key, len(names))
	for i, name := range names {
		key, err := tag.NewKey(name)
		if err != nil {
			logrus.Fatal(err)
		}
		tagKeys[i] = key
	}
	return tagKeys
}
