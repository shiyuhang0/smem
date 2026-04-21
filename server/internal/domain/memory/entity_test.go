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

func TestPrimaryKind(t *testing.T) {
	require.Equal(t, "", PrimaryKind(nil))
	require.Equal(t, "preference", PrimaryKind([]string{"preference", "note"}))
}
