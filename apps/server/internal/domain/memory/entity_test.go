package memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMemorySearchable(t *testing.T) {
	require.True(t, Memory{State: StateActive}.Searchable())
	require.False(t, Memory{State: StateCreating}.Searchable())
	require.False(t, Memory{State: StateArchived}.Searchable())
}
