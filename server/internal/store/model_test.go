package store

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestStringSliceValueReturnsNilForNilSlice(t *testing.T) {
	var values StringSlice

	got, err := values.Value()

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestStringSliceValuePreservesEmptySlice(t *testing.T) {
	values := StringSlice{}

	got, err := values.Value()

	require.NoError(t, err)
	require.Equal(t, "[]", got)
}

func TestJSONMapValueReturnsNilForNilMap(t *testing.T) {
	var values JSONMap

	got, err := values.Value()

	require.NoError(t, err)
	require.Nil(t, got)
}

func TestJSONMapValuePreservesEmptyMap(t *testing.T) {
	values := JSONMap{}

	got, err := values.Value()

	require.NoError(t, err)
	require.Equal(t, "{}", got)
}

func TestFloat32SliceValueReturnsNilForNilEmbedding(t *testing.T) {
	var values Float32Slice

	got, err := values.Value()

	require.NoError(t, err)
	require.Nil(t, got)
}
