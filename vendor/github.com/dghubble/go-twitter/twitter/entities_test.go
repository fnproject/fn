package twitter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIndices(t *testing.T) {
	cases := []struct {
		pair          Indices
		expectedStart int
		expectedEnd   int
	}{
		{Indices{}, 0, 0},
		{Indices{25, 47}, 25, 47},
	}
	for _, c := range cases {
		assert.Equal(t, c.expectedStart, c.pair.Start())
		assert.Equal(t, c.expectedEnd, c.pair.End())
	}
}
