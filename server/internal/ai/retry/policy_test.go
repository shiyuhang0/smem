package retry

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDoRetriesRetryableErrorsUpToThreeTimes(t *testing.T) {
	attempts := 0
	policy := Policy{MaxAttempts: 3, Backoff: func(int) {}, IsRetryable: func(err error) bool { return true }}

	err := policy.Do(context.Background(), func(context.Context) error {
		attempts++
		return errors.New("retry me")
	})

	require.Error(t, err)
	require.Equal(t, 3, attempts)
}

func TestDoStopsOnNonRetryableError(t *testing.T) {
	attempts := 0
	policy := Policy{MaxAttempts: 3, Backoff: func(int) {}, IsRetryable: func(err error) bool { return false }}

	err := policy.Do(context.Background(), func(context.Context) error {
		attempts++
		return errors.New("bad request")
	})

	require.Error(t, err)
	require.Equal(t, 1, attempts)
}
