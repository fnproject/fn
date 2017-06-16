package envconfig

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSliceTokenizer(t *testing.T) {
	str := "foobar,barbaz"
	tnz := newSliceTokenizer(str)

	b := tnz.scan()
	require.Nil(t, tnz.Err())
	require.Equal(t, true, b)

	require.Equal(t, "foobar", tnz.text())

	b = tnz.scan()
	require.Nil(t, tnz.Err())
	require.Equal(t, true, b)
	require.Equal(t, "barbaz", tnz.text())

	b = tnz.scan()
	require.Nil(t, tnz.Err())
	require.Equal(t, false, b)
}

func TestSliceOfStructsTokenizer(t *testing.T) {
	str := "{foobar,100},{barbaz,200}"
	tnz := newSliceTokenizer(str)

	b := tnz.scan()
	require.Nil(t, tnz.Err())
	require.Equal(t, true, b)

	require.Equal(t, "{foobar,100}", tnz.text())

	b = tnz.scan()
	require.Nil(t, tnz.Err())
	require.Equal(t, true, b)
	require.Equal(t, "{barbaz,200}", tnz.text())

	b = tnz.scan()
	require.Nil(t, tnz.Err())
	require.Equal(t, false, b)
}
