package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestLoadDefaults(t *testing.T) {
	t.Setenv("DB_DSN", "root:pass@tcp(localhost:4000)/smem?parseTime=true")
	t.Setenv("OPENAI_API_KEY", "test-key")
	t.Setenv("RERANK_API_KEY", "rerank-key")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, ":8080", cfg.ServerAddr)
	require.Equal(t, "https://api.openai.com/v1", cfg.OpenAIBaseURL)
	require.Equal(t, "gpt-4.1-mini", cfg.OpenAIChatModel)
	require.Equal(t, "siliconflow", cfg.RerankProvider)
	require.Equal(t, "https://api.siliconflow.cn/v1", cfg.RerankBaseURL)
	require.Equal(t, "BAAI/bge-reranker-v2-m3", cfg.RerankModel)
	require.Equal(t, 1024, cfg.EmbeddingDim)
}

func TestLoadRequiresDSNAndAPIKey(t *testing.T) {
	_, err := Load()
	require.Error(t, err)
	require.ErrorContains(t, err, "DB_DSN")
}

func TestLoadUsesConfigFileValuesBeforeEnv(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte("server_addr: ':9090'\ndb_dsn: 'file-dsn'\nopenai_api_key: 'file-key'\nopenai_chat_model: 'file-chat'\nrerank_api_key: 'file-rerank-key'\nrerank_model: 'custom-rerank'\n"), 0o600)
	require.NoError(t, err)

	t.Setenv("SMEM_CONFIG_FILE", path)
	t.Setenv("SERVER_ADDR", ":8080")
	t.Setenv("DB_DSN", "env-dsn")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("OPENAI_CHAT_MODEL", "env-chat")
	t.Setenv("RERANK_API_KEY", "env-rerank-key")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, ":9090", cfg.ServerAddr)
	require.Equal(t, "file-dsn", cfg.DBDSN)
	require.Equal(t, "file-key", cfg.OpenAIAPIKey)
	require.Equal(t, "file-chat", cfg.OpenAIChatModel)
	require.Equal(t, "file-rerank-key", cfg.RerankAPIKey)
	require.Equal(t, "custom-rerank", cfg.RerankModel)
}

func TestLoadFallsBackToEnvWhenConfigFieldMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.yaml")
	err := os.WriteFile(path, []byte("server_addr: ':9090'\n"), 0o600)
	require.NoError(t, err)

	t.Setenv("SMEM_CONFIG_FILE", path)
	t.Setenv("DB_DSN", "env-dsn")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("RERANK_API_KEY", "env-rerank-key")

	cfg, err := Load()
	require.NoError(t, err)
	require.Equal(t, ":9090", cfg.ServerAddr)
	require.Equal(t, "env-dsn", cfg.DBDSN)
	require.Equal(t, "env-key", cfg.OpenAIAPIKey)
	require.Equal(t, "env-rerank-key", cfg.RerankAPIKey)
}

func TestLoadErrorsWhenExplicitConfigFileMissing(t *testing.T) {
	t.Setenv("SMEM_CONFIG_FILE", filepath.Join(t.TempDir(), "missing.yaml"))
	t.Setenv("DB_DSN", "env-dsn")
	t.Setenv("OPENAI_API_KEY", "env-key")
	t.Setenv("RERANK_API_KEY", "env-rerank-key")

	_, err := Load()
	require.Error(t, err)
	require.ErrorContains(t, err, "SMEM_CONFIG_FILE")
}

func TestLoadRequiresRerankAPIKey(t *testing.T) {
	t.Setenv("DB_DSN", "env-dsn")
	t.Setenv("OPENAI_API_KEY", "env-key")

	_, err := Load()
	require.Error(t, err)
	require.ErrorContains(t, err, "RERANK_API_KEY")
}
