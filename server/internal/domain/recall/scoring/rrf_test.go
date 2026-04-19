package scoring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRRFMerge(t *testing.T) {
	merged := RRF([][]string{{"a", "b"}, {"b", "c"}}, 60)
	require.Equal(t, []string{"b", "a", "c"}, merged)
}
