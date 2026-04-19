package retry

import (
	"context"
	"errors"
	"math/rand"
	"net"
	"net/http"
	"time"
)

type Policy struct {
	MaxAttempts int
	Backoff     func(attempt int)
	IsRetryable func(error) bool
}

func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts: 3,
		Backoff:     DefaultBackoff,
		IsRetryable: DefaultRetryable,
	}
}

func (p Policy) Do(ctx context.Context, operation func(context.Context) error) error {
	if p.MaxAttempts <= 0 {
		p.MaxAttempts = 3
	}
	if p.Backoff == nil {
		p.Backoff = DefaultBackoff
	}
	if p.IsRetryable == nil {
		p.IsRetryable = DefaultRetryable
	}
	var lastErr error
	for attempt := 1; attempt <= p.MaxAttempts; attempt++ {
		err := operation(ctx)
		if err == nil {
			return nil
		}
		lastErr = err
		if attempt == p.MaxAttempts || !p.IsRetryable(err) {
			return err
		}
		p.Backoff(attempt)
	}
	return lastErr
}

type HTTPStatusError struct {
	StatusCode int
}

func (e HTTPStatusError) Error() string {
	return http.StatusText(e.StatusCode)
}

func DefaultRetryable(err error) bool {
	var statusErr HTTPStatusError
	if errors.As(err, &statusErr) {
		return statusErr.StatusCode == http.StatusTooManyRequests || statusErr.StatusCode >= 500
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	return errors.Is(err, context.DeadlineExceeded)
}

func DefaultBackoff(attempt int) {
	base := time.Duration(1<<max(attempt-1, 0)) * 100 * time.Millisecond
	jitter := time.Duration(rand.Int63n(int64(50 * time.Millisecond)))
	time.Sleep(base + jitter)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
