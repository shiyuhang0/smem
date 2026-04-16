package app

import (
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
