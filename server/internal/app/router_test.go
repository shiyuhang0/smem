package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"smem/apps/server/internal/config"
)

func TestRouterHealthz(t *testing.T) {
	r := NewRouter(config.Config{})
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	resp := httptest.NewRecorder()

	r.ServeHTTP(resp, req)

	require.Equal(t, http.StatusOK, resp.Code)
	require.JSONEq(t, `{"status":"ok"}`+"\n", resp.Body.String())
}

func TestAppShutdownCancelsWorker(t *testing.T) {
	cancelled := false
	application := &App{
		workerCancel: func() { cancelled = true },
	}

	err := application.Shutdown(context.Background())
	require.NoError(t, err)
	require.True(t, cancelled)
}
