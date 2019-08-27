package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGenerateLogScaleHistogramBucketsWithRange(t *testing.T) {
	assert.Equal(t, []float64{6.25, 12.5, 25, 50, 100}, GenerateLogScaleHistogramBucketsWithRange(7, 100))
}

func TestGenerateLinearHistogramBuckets(t *testing.T) {
	assert.Equal(t, []float64{5, 7, 9, 11, 13, 15}, GenerateLinearHistogramBuckets(5, 15, 5))
}

func TestGenerateLogScaleHistogramBuckets(t *testing.T) {
	assert.Equal(t, []float64{0, 0.1953125, 0.390625, 0.78125, 1.5625, 3.125, 6.25, 12.5, 25, 50, 100}, GenerateLogScaleHistogramBuckets(100, 10))
}
