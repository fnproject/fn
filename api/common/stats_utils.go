package common

import (
	"github.com/sirupsen/logrus"
	"go.opencensus.io/stats"
	"go.opencensus.io/stats/view"
	"go.opencensus.io/tag"
)

func CreateView(measure stats.Measure, agg *view.Aggregation, tagKeys []string) *view.View {
	keys := makeKeys(tagKeys)
	return CreateViewWithTags(measure, agg, keys)
}

func CreateViewWithTags(measure stats.Measure, agg *view.Aggregation, tags []tag.Key) *view.View {
	return &view.View{
		Name:        measure.Name(),
		Description: measure.Description(),
		Measure:     measure,
		TagKeys:     tags,
		Aggregation: agg,
	}
}

func MakeMeasure(name string, desc string, unit string) *stats.Int64Measure {
	return stats.Int64(name, desc, unit)
}

func MakeKey(name string) tag.Key {
	key, err := tag.NewKey(name)
	if err != nil {
		logrus.WithError(err).Fatalf("Cannot create tag %s", name)
	}
	return key
}

func makeKeys(names []string) []tag.Key {
	tagKeys := make([]tag.Key, len(names))
	for i, name := range names {
		tagKeys[i] = MakeKey(name)
	}
	return tagKeys
}
