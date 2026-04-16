package app

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"

	"smem/apps/server/internal/config"
	"smem/apps/server/internal/domain/memory"
	"smem/apps/server/internal/embedding"
	"smem/apps/server/internal/llm"
	"smem/apps/server/internal/retry"
	"smem/apps/server/internal/store/tidb"
	httptransport "smem/apps/server/internal/transport/http"
	"smem/apps/server/internal/workflow/ingest"
	"smem/apps/server/internal/workflow/recall"
)

type App struct {
	Config config.Config
	Server *http.Server
	DB     *gorm.DB
}

func New(cfg config.Config) (*App, error) {
	dsn, err := tidb.PrepareDSN(cfg)
	if err != nil {
		return nil, err
	}
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	if err := tidb.AutoMigrate(context.Background(), db); err != nil {
		return nil, err
	}
	repo := tidb.NewRepository(db)
	memoryService := memory.NewService(repo, newID)
	retryPolicy := retry.DefaultPolicy()
	llmProvider := llm.NewOpenAIProvider(llm.Config{
		BaseURL: cfg.OpenAIBaseURL, APIKey: cfg.OpenAIAPIKey, Model: cfg.OpenAIChatModel, Retry: retryPolicy,
	})
	embeddingProvider := embedding.NewOpenAIProvider(embedding.Config{
		BaseURL: cfg.OpenAIBaseURL, APIKey: cfg.OpenAIAPIKey, Model: cfg.OpenAIEmbeddingModel, Retry: retryPolicy,
	})
	worker := ingest.NewEmbeddingWorker(repo, embeddingProvider, cfg.EmbeddingDim)
	ingestService := ingest.NewService(memoryService, repo, worker, llmProvider)
	recallService := recall.NewService(repo, embeddingProvider, llmProvider)
	memoryHandler := httptransport.NewMemoryHandler(memoryService, ingestService, recallService)

	return &App{
		Config: cfg,
		DB:     db,
		Server: &http.Server{
			Addr:    cfg.ServerAddr,
			Handler: NewRouter(cfg, memoryHandler),
		},
	}, nil
}

func newID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		return "mem-fallback"
	}
	return "mem-" + hex.EncodeToString(buf)
}
