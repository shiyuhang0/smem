package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	ServerAddr        string
	DBDSN             string
	DBTLSServerName   string
	EnableDBLogReads  bool
	OpenAIBaseURL     string
	OpenAIAPIKey      string
	OpenAIChatModel   string
	RerankProvider    string
	RerankBaseURL     string
	RerankAPIKey      string
	RerankModel       string
	EmbeddingProvider string
	EmbeddingBaseURL  string
	EmbeddingAPIKey   string
	EmbeddingModel    string
	EmbeddingDim      int
}

type fileConfig struct {
	ServerAddr        *string `yaml:"server_addr"`
	DBDSN             *string `yaml:"db_dsn"`
	DBTLSServerName   *string `yaml:"db_tls_server_name"`
	EnableDBLogReads  *bool   `yaml:"enable_db_log_reads"`
	OpenAIBaseURL     *string `yaml:"openai_base_url"`
	OpenAIAPIKey      *string `yaml:"openai_api_key"`
	OpenAIChatModel   *string `yaml:"openai_chat_model"`
	RerankProvider    *string `yaml:"rerank_provider"`
	RerankBaseURL     *string `yaml:"rerank_base_url"`
	RerankAPIKey      *string `yaml:"rerank_api_key"`
	RerankModel       *string `yaml:"rerank_model"`
	EmbeddingProvider *string `yaml:"embedding_provider"`
	EmbeddingBaseURL  *string `yaml:"embedding_base_url"`
	EmbeddingAPIKey   *string `yaml:"embedding_api_key"`
	EmbeddingModel    *string `yaml:"embedding_model"`
	EmbeddingDim      *int    `yaml:"embedding_dim"`
}

func Load() (Config, error) {
	cfg := Config{
		ServerAddr:        stringWithDefault("SERVER_ADDR", ":8080"),
		DBDSN:             strings.TrimSpace(os.Getenv("DB_DSN")),
		DBTLSServerName:   strings.TrimSpace(os.Getenv("DB_TLS_SERVER_NAME")),
		EnableDBLogReads:  boolWithDefault("ENABLE_DB_LOG_READS", false),
		OpenAIBaseURL:     stringWithDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIAPIKey:      strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIChatModel:   stringWithDefault("OPENAI_CHAT_MODEL", "gpt-4.1-mini"),
		RerankProvider:    strings.TrimSpace(os.Getenv("RERANK_PROVIDER")),
		RerankBaseURL:     strings.TrimSpace(os.Getenv("RERANK_BASE_URL")),
		RerankAPIKey:      strings.TrimSpace(os.Getenv("RERANK_API_KEY")),
		RerankModel:       strings.TrimSpace(os.Getenv("RERANK_MODEL")),
		EmbeddingProvider: strings.TrimSpace(os.Getenv("EMBEDDING_PROVIDER")),
		EmbeddingBaseURL:  strings.TrimSpace(os.Getenv("EMBEDDING_BASE_URL")),
		EmbeddingAPIKey:   strings.TrimSpace(os.Getenv("EMBEDDING_API_KEY")),
		EmbeddingModel:    strings.TrimSpace(os.Getenv("EMBEDDING_MODEL")),
		EmbeddingDim:      intWithDefault("EMBEDDING_DIM", 0),
	}

	path, explicit := resolveConfigPath()
	if path != "" {
		loaded, err := loadFileConfig(path, explicit)
		if err != nil {
			return Config{}, err
		}
		if loaded != nil {
			cfg = mergeFileConfig(cfg, *loaded)
		}
	}
	cfg = normalizeEmbeddingConfig(cfg)
	cfg = normalizeRerankConfig(cfg)

	if cfg.DBDSN == "" {
		return Config{}, fmt.Errorf("DB_DSN is required")
	}
	if cfg.OpenAIAPIKey == "" {
		return Config{}, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if cfg.RerankAPIKey == "" {
		return Config{}, fmt.Errorf("RERANK_API_KEY is required")
	}
	if cfg.EmbeddingDim <= 0 {
		return Config{}, fmt.Errorf("EMBEDDING_DIM must be positive")
	}
	if cfg.RerankProvider != "siliconflow" {
		return Config{}, fmt.Errorf("RERANK_PROVIDER must be siliconflow")
	}
	if cfg.EmbeddingProvider != "ollama" && cfg.EmbeddingProvider != "openai" && cfg.EmbeddingProvider != "glm" {
		return Config{}, fmt.Errorf("EMBEDDING_PROVIDER must be ollama, openai or glm")
	}

	return cfg, nil
}

func resolveConfigPath() (string, bool) {
	if value := strings.TrimSpace(os.Getenv("SMEM_CONFIG_FILE")); value != "" {
		return value, true
	}
	defaultPath := filepath.Join(".", "config.yaml")
	if _, err := os.Stat(defaultPath); err == nil {
		return defaultPath, false
	}
	return "", false
}

func loadFileConfig(path string, explicit bool) (*fileConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) && !explicit {
			return nil, nil
		}
		if explicit {
			return nil, fmt.Errorf("SMEM_CONFIG_FILE load failed: %w", err)
		}
		return nil, err
	}
	var loaded fileConfig
	if err := yaml.Unmarshal(data, &loaded); err != nil {
		return nil, fmt.Errorf("parse config file %s: %w", path, err)
	}
	return &loaded, nil
}

func mergeFileConfig(cfg Config, file fileConfig) Config {
	if file.ServerAddr != nil {
		cfg.ServerAddr = strings.TrimSpace(*file.ServerAddr)
	}
	if file.DBDSN != nil {
		cfg.DBDSN = strings.TrimSpace(*file.DBDSN)
	}
	if file.DBTLSServerName != nil {
		cfg.DBTLSServerName = strings.TrimSpace(*file.DBTLSServerName)
	}
	if file.EnableDBLogReads != nil {
		cfg.EnableDBLogReads = *file.EnableDBLogReads
	}
	if file.OpenAIBaseURL != nil {
		cfg.OpenAIBaseURL = strings.TrimSpace(*file.OpenAIBaseURL)
	}
	if file.OpenAIAPIKey != nil {
		cfg.OpenAIAPIKey = strings.TrimSpace(*file.OpenAIAPIKey)
	}
	if file.OpenAIChatModel != nil {
		cfg.OpenAIChatModel = strings.TrimSpace(*file.OpenAIChatModel)
	}
	if file.RerankProvider != nil {
		cfg.RerankProvider = strings.TrimSpace(*file.RerankProvider)
	}
	if file.RerankBaseURL != nil {
		cfg.RerankBaseURL = strings.TrimSpace(*file.RerankBaseURL)
	}
	if file.RerankAPIKey != nil {
		cfg.RerankAPIKey = strings.TrimSpace(*file.RerankAPIKey)
	}
	if file.RerankModel != nil {
		cfg.RerankModel = strings.TrimSpace(*file.RerankModel)
	}
	if file.EmbeddingProvider != nil {
		cfg.EmbeddingProvider = strings.TrimSpace(*file.EmbeddingProvider)
	}
	if file.EmbeddingBaseURL != nil {
		cfg.EmbeddingBaseURL = strings.TrimSpace(*file.EmbeddingBaseURL)
	}
	if file.EmbeddingAPIKey != nil {
		cfg.EmbeddingAPIKey = strings.TrimSpace(*file.EmbeddingAPIKey)
	}
	if file.EmbeddingModel != nil {
		cfg.EmbeddingModel = strings.TrimSpace(*file.EmbeddingModel)
	}
	if file.EmbeddingDim != nil {
		cfg.EmbeddingDim = *file.EmbeddingDim
	}
	return cfg
}

func normalizeRerankConfig(cfg Config) Config {
	cfg.RerankProvider = strings.TrimSpace(cfg.RerankProvider)
	if cfg.RerankProvider == "" {
		cfg.RerankProvider = "siliconflow"
	}
	if cfg.RerankBaseURL == "" {
		cfg.RerankBaseURL = "https://api.siliconflow.cn/v1"
	}
	if cfg.RerankModel == "" {
		cfg.RerankModel = "BAAI/bge-reranker-v2-m3"
	}
	return cfg
}

func normalizeEmbeddingConfig(cfg Config) Config {
	cfg.EmbeddingProvider = strings.TrimSpace(cfg.EmbeddingProvider)
	if cfg.EmbeddingProvider == "" {
		cfg.EmbeddingProvider = "ollama"
	}
	if cfg.EmbeddingProvider == "openai" {
		if cfg.EmbeddingBaseURL == "" {
			cfg.EmbeddingBaseURL = cfg.OpenAIBaseURL
		}
		if cfg.EmbeddingAPIKey == "" {
			cfg.EmbeddingAPIKey = cfg.OpenAIAPIKey
		}
		if cfg.EmbeddingModel == "" {
			cfg.EmbeddingModel = "text-embedding-3-small"
		}
		if cfg.EmbeddingDim == 0 {
			cfg.EmbeddingDim = 1536
		}
		return cfg
	}
	if cfg.EmbeddingProvider == "glm" {
		if cfg.EmbeddingBaseURL == "" {
			cfg.EmbeddingBaseURL = "https://open.bigmodel.cn/api/paas/v4"
		}
		if cfg.EmbeddingModel == "" {
			cfg.EmbeddingModel = "embedding-3"
		}
		if cfg.EmbeddingDim == 0 {
			cfg.EmbeddingDim = 1536
		}
		return cfg
	}
	if cfg.EmbeddingBaseURL == "" {
		cfg.EmbeddingBaseURL = "http://localhost:11434"
	}
	if cfg.EmbeddingModel == "" {
		cfg.EmbeddingModel = "bge-m3"
	}
	if cfg.EmbeddingDim == 0 {
		cfg.EmbeddingDim = 1024
	}
	return cfg
}

func stringWithDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func intWithDefault(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func boolWithDefault(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	b, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return b
}
