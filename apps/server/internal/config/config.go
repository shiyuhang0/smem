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
	ServerAddr           string
	DBDSN                string
	DBTLSServerName      string
	OpenAIBaseURL        string
	OpenAIAPIKey         string
	OpenAIChatModel      string
	OpenAIEmbeddingModel string
	EmbeddingDim         int
	RecallDefaultTopK    int
	RecallMaxTopK        int
	RecallTemperature    float64
	EnableFullText       bool
	LogLevel             string
}

type fileConfig struct {
	ServerAddr           *string  `yaml:"server_addr"`
	DBDSN                *string  `yaml:"db_dsn"`
	DBTLSServerName      *string  `yaml:"db_tls_server_name"`
	OpenAIBaseURL        *string  `yaml:"openai_base_url"`
	OpenAIAPIKey         *string  `yaml:"openai_api_key"`
	OpenAIChatModel      *string  `yaml:"openai_chat_model"`
	OpenAIEmbeddingModel *string  `yaml:"openai_embedding_model"`
	EmbeddingDim         *int     `yaml:"embedding_dim"`
	RecallDefaultTopK    *int     `yaml:"recall_default_topk"`
	RecallMaxTopK        *int     `yaml:"recall_max_topk"`
	RecallTemperature    *float64 `yaml:"recall_temperature"`
	EnableFullText       *bool    `yaml:"enable_fulltext"`
	LogLevel             *string  `yaml:"log_level"`
}

func Load() (Config, error) {
	cfg := Config{
		ServerAddr:           stringWithDefault("SERVER_ADDR", ":8080"),
		DBDSN:                strings.TrimSpace(os.Getenv("DB_DSN")),
		DBTLSServerName:      strings.TrimSpace(os.Getenv("DB_TLS_SERVER_NAME")),
		OpenAIBaseURL:        stringWithDefault("OPENAI_BASE_URL", "https://api.openai.com/v1"),
		OpenAIAPIKey:         strings.TrimSpace(os.Getenv("OPENAI_API_KEY")),
		OpenAIChatModel:      stringWithDefault("OPENAI_CHAT_MODEL", "gpt-4.1-mini"),
		OpenAIEmbeddingModel: stringWithDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),
		EmbeddingDim:         intWithDefault("EMBEDDING_DIM", 1536),
		RecallDefaultTopK:    intWithDefault("RECALL_DEFAULT_TOPK", 5),
		RecallMaxTopK:        intWithDefault("RECALL_MAX_TOPK", 10),
		RecallTemperature:    floatWithDefault("RECALL_TEMPERATURE", 1.0),
		EnableFullText:       boolWithDefault("ENABLE_FULLTEXT", true),
		LogLevel:             stringWithDefault("LOG_LEVEL", "info"),
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

	if cfg.DBDSN == "" {
		return Config{}, fmt.Errorf("DB_DSN is required")
	}
	if cfg.OpenAIAPIKey == "" {
		return Config{}, fmt.Errorf("OPENAI_API_KEY is required")
	}
	if cfg.RecallDefaultTopK < 1 || cfg.RecallDefaultTopK > 10 {
		return Config{}, fmt.Errorf("RECALL_DEFAULT_TOPK must be between 1 and 10")
	}
	if cfg.RecallMaxTopK < 1 || cfg.RecallMaxTopK > 10 {
		return Config{}, fmt.Errorf("RECALL_MAX_TOPK must be between 1 and 10")
	}
	if cfg.RecallDefaultTopK > cfg.RecallMaxTopK {
		return Config{}, fmt.Errorf("RECALL_DEFAULT_TOPK cannot exceed RECALL_MAX_TOPK")
	}
	if cfg.EmbeddingDim <= 0 {
		return Config{}, fmt.Errorf("EMBEDDING_DIM must be positive")
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
	if file.OpenAIBaseURL != nil {
		cfg.OpenAIBaseURL = strings.TrimSpace(*file.OpenAIBaseURL)
	}
	if file.OpenAIAPIKey != nil {
		cfg.OpenAIAPIKey = strings.TrimSpace(*file.OpenAIAPIKey)
	}
	if file.OpenAIChatModel != nil {
		cfg.OpenAIChatModel = strings.TrimSpace(*file.OpenAIChatModel)
	}
	if file.OpenAIEmbeddingModel != nil {
		cfg.OpenAIEmbeddingModel = strings.TrimSpace(*file.OpenAIEmbeddingModel)
	}
	if file.EmbeddingDim != nil {
		cfg.EmbeddingDim = *file.EmbeddingDim
	}
	if file.RecallDefaultTopK != nil {
		cfg.RecallDefaultTopK = *file.RecallDefaultTopK
	}
	if file.RecallMaxTopK != nil {
		cfg.RecallMaxTopK = *file.RecallMaxTopK
	}
	if file.RecallTemperature != nil {
		cfg.RecallTemperature = *file.RecallTemperature
	}
	if file.EnableFullText != nil {
		cfg.EnableFullText = *file.EnableFullText
	}
	if file.LogLevel != nil {
		cfg.LogLevel = strings.TrimSpace(*file.LogLevel)
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

func floatWithDefault(key string, fallback float64) float64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseFloat(value, 64)
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
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
