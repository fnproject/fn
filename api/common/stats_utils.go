package common

import (
	"math"

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

// GenerateLogScaleHistogramBucketsWithRange generates histogram buckets on the log scale between the specified min and max range,
// such that the min value is in the first bucket and the max in the last.
func GenerateLogScaleHistogramBucketsWithRange(min, max float64) []float64 {
	if min <= 0 {
		logrus.Fatal("cannot generate log scale with non positive domain values")
	}
	count := int(math.Ceil(math.Log2(max/min))) + 1
	return GenerateLogScaleHistogramBuckets(max, count)[1:]
}

// GenerateLinearHistogramBuckets generates number of buckets specified by count in the range specified by min and max
func GenerateLinearHistogramBuckets(min, max float64, count int) []float64 {
	width := (max - min) / float64(count)
	var res []float64
	for i := 0; i <= count; i++ {
		res = append(res, min+(float64(i)*width))
	}
	return res
}

// GenerateLogScaleHistogramBuckets generates number of buckets specified by count on the log scale
// such that the value specified by max is in the last bucket
func GenerateLogScaleHistogramBuckets(max float64, count int) []float64 {
	res := make([]float64, count+1)

	for i := count; i > 0; i-- {
		res[i] = max / (math.Pow(2, float64(count-i)))
	}
	return res
}
