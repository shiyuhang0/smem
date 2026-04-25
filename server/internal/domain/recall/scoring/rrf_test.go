package scoring

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAdaptiveRRFKUsesTwoNWithinBounds(t *testing.T) {
	short := []string{"a", "b", "c", "d"}
	medium := make([]string, 15)
	large := make([]string, 40)

	require.Equal(t, 10.0, adaptiveRRFK([][]string{short}))
	require.Equal(t, 30.0, adaptiveRRFK([][]string{short, medium}))
	require.Equal(t, 60.0, adaptiveRRFK([][]string{large}))
}
