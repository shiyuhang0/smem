package memory

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCreateInputValidate(t *testing.T) {
	tests := []struct {
		name    string
		input   CreateInput
		wantErr string
	}{
		{
			name: "valid normal input",
			input: CreateInput{
				Content: "remember that i use vim",
				Mode:    ModeNormal,
				Type:    TypeFact,
				Kinds:   []string{"preference"},
			},
		},
		{
			name:    "empty content rejected",
			input:   CreateInput{Mode: ModeNormal},
			wantErr: "content is required",
		},
		{
			name:    "invalid type rejected",
			input:   CreateInput{Content: "x", Mode: ModeNormal, Type: "wrong"},
			wantErr: "type",
		},
		{
			name:    "invalid scope rejected",
			input:   CreateInput{Content: "x", Mode: ModeNormal, Scope: "wrong"},
			wantErr: "scope",
		},
		{
			name:    "invalid mode rejected",
			input:   CreateInput{Content: "x", Mode: "wrong"},
			wantErr: "mode",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.input.Validate()
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			require.ErrorContains(t, err, tt.wantErr)
		})
	}
}

func TestRecallInputValidate(t *testing.T) {
	require.NoError(t, RecallInput{Content: "hello", TopK: 5}.Validate())

	err := RecallInput{Content: "hello", TopK: 11}.Validate()
	require.Error(t, err)
	require.ErrorContains(t, err, "top_k")
}
