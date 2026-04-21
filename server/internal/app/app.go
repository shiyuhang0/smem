package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"net/http"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"

	"smem/apps/server/internal/ai/embedding"
	"smem/apps/server/internal/ai/llm"
	"smem/apps/server/internal/ai/retry"
	"smem/apps/server/internal/config"
	"smem/apps/server/internal/domain/ingest"
	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/domain/recall"
	"smem/apps/server/internal/handler"
	"smem/apps/server/internal/store"
)

type App struct {
	Config       config.Config
	Server       *http.Server
	DB           *gorm.DB
	workerCancel context.CancelFunc
}

func New(cfg config.Config) (*App, error) {
	dsn, err := store.PrepareDSN(cfg)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{
		Logger: store.NewFilteringLogger(gormlogger.Default.LogMode(gormlogger.Info), cfg.EnableDBLogReads),
	})
	if err != nil {
		return nil, err
	}
	if err := store.ApplyMigrations(context.Background(), db); err != nil {
		return nil, err
	}
	memoryRepo := store.NewRepository(db)
	jobRepo := store.NewIngestJobRepository(db)
	txManager := store.NewTransactionManager(db)
	memoryService := memory.NewService(memoryRepo)
	retryPolicy := retry.DefaultPolicy()
	llmProvider := llm.NewOpenAIProvider(llm.Config{
		BaseURL: cfg.OpenAIBaseURL, APIKey: cfg.OpenAIAPIKey, Model: cfg.OpenAIChatModel, Retry: retryPolicy,
	})
	embeddingProvider, err := newEmbeddingProvider(cfg, retryPolicy)
	if err != nil {
		return nil, err
	}
	recallService := recall.NewService(memoryRepo, embeddingProvider, llmProvider)
	ingestService := ingest.NewService(jobRepo, newIngestJobID)
	workerCtx, workerCancel := context.WithCancel(context.Background())
	jobWorker := ingest.NewJobWorker(jobRepo, txManager, recallService, llmProvider, embeddingProvider, newMemoryID, newWorkerID())
	jobWorker.Start(workerCtx)
	memoryHandler := handler.NewMemoryHandler(memoryService, ingestService, recallService)

	return &App{
		Config:       cfg,
		DB:           db,
		workerCancel: workerCancel,
		Server: &http.Server{
			Addr:    cfg.ServerAddr,
			Handler: NewRouter(cfg, memoryHandler),
		},
	}, nil
}

func (a *App) Shutdown(ctx context.Context) error {
	if a.workerCancel != nil {
		a.workerCancel()
	}

	var result error
	if a.Server != nil {
		result = errors.Join(result, a.Server.Shutdown(ctx))
	}
	if a.DB != nil {
		sqlDB, err := a.DB.DB()
		if err != nil {
			result = errors.Join(result, err)
		} else {
			result = errors.Join(result, sqlDB.Close())
		}
	}
	return result
}

func newEmbeddingProvider(cfg config.Config, retryPolicy retry.Policy) (embedding.Provider, error) {
	providerConfig := embedding.Config{
		BaseURL: cfg.EmbeddingBaseURL, APIKey: cfg.EmbeddingAPIKey, Model: cfg.EmbeddingModel, Retry: retryPolicy,
	}
	switch cfg.EmbeddingProvider {
	case "ollama":
		return embedding.NewOllamaProvider(providerConfig), nil
	case "openai":
		return embedding.NewOpenAIProvider(providerConfig), nil
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", cfg.EmbeddingProvider)
	}
}

func newMemoryID() string {
	return newPrefixedID("mem-")
}

func newIngestJobID() string {
	return newPrefixedID("ing-")
}

func newWorkerID() string {
	return newPrefixedID("worker-")
}

func newPrefixedID(prefix string) string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return prefix + "fallback"
	}
	return prefix + hex.EncodeToString(buf)
}
