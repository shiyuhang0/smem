package handler

import (
	"context"
	"errors"
	"math"
	stdhttp "net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"smem/apps/server/internal/domain/ingestjob"
	"smem/apps/server/internal/domain/memory"
)

type recallService interface {
	Recall(context.Context, memory.RecallInput) ([]memory.RecallResult, error)
}

type ingestService interface {
	Create(context.Context, memory.CreateInput) (ingestjob.Job, error)
}

type MemoryHandler struct {
	memoryService *memory.Service
	ingest        ingestService
	recall        recallService
}

func NewMemoryHandler(memoryService *memory.Service, ingest ingestService, recall recallService) *MemoryHandler {
	return &MemoryHandler{memoryService: memoryService, ingest: ingest, recall: recall}
}

func (h *MemoryHandler) Register(group *gin.RouterGroup) {
	group.POST("/memories", h.create)
	group.GET("/memories/kinds", h.listKinds)
	group.GET("/memories/:id", h.get)
	group.PUT("/memories/:id", h.update)
	group.DELETE("/memories/:id", h.delete)
	group.GET("/memories", h.list)
	group.POST("/memories/recall", h.recallMemories)
}

func (h *MemoryHandler) create(c *gin.Context) {
	var req createMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	input := memory.CreateInput{
		Content:   req.Content,
		Mode:      memory.Mode(req.Mode),
		Type:      memory.Type(req.Type),
		Kinds:     req.Kinds,
		Scope:     memory.Scope(req.Scope),
		Metadata:  req.Metadata,
		AgentID:   req.AgentID,
		SessionID: req.SessionID,
		Source:    req.Source,
	}
	if h.ingest != nil {
		job, err := h.ingest.Create(c.Request.Context(), input)
		if err != nil {
			h.writeError(c, err)
			return
		}
		c.JSON(stdhttp.StatusAccepted, toIngestJobResponse(job))
		return
	}
	c.JSON(stdhttp.StatusNotImplemented, ErrorResponse{Error: "ingest service is not configured"})
}

func (h *MemoryHandler) get(c *gin.Context) {
	item, err := h.memoryService.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, toMemoryResponse(item))
}

func (h *MemoryHandler) update(c *gin.Context) {
	var req updateMemoryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	input := memory.UpdateInput{Kinds: req.Kinds, Metadata: req.Metadata}
	if req.Content != nil {
		input.Content = req.Content
	}
	if req.Type != nil {
		v := memory.Type(*req.Type)
		input.Type = &v
	}
	if req.Scope != nil {
		v := memory.Scope(*req.Scope)
		input.Scope = &v
	}
	if req.State != nil {
		v := memory.State(*req.State)
		input.State = &v
	}
	item, err := h.memoryService.Update(c.Request.Context(), c.Param("id"), input)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(stdhttp.StatusOK, toMemoryResponse(item))
}

func (h *MemoryHandler) delete(c *gin.Context) {
	err := h.memoryService.Delete(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(stdhttp.StatusNoContent)
}

func (h *MemoryHandler) list(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))
	items, total, err := h.memoryService.List(c.Request.Context(), memory.ListInput{
		Page:     page,
		PageSize: pageSize,
		Search:   c.Query("search"),
		Kind:     c.Query("kind"),
		State:    memory.State(c.Query("state")),
		Type:     memory.Type(c.Query("type")),
	})
	if err != nil {
		h.writeError(c, err)
		return
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	c.JSON(stdhttp.StatusOK, listMemoriesResponse{
		Items:      toMemoryResponses(items),
		Page:       page,
		PageSize:   pageSize,
		Total:      total,
		TotalPages: int(math.Ceil(float64(total) / float64(pageSize))),
		HasMore:    int64(page*pageSize) < total,
	})
}

func (h *MemoryHandler) listKinds(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))
	items, err := h.memoryService.ListTopKinds(c.Request.Context(), limit)
	if err != nil {
		h.writeError(c, err)
		return
	}

	response := make([]kindCountResponse, 0, len(items))
	for _, item := range items {
		response = append(response, kindCountResponse{Kind: item.Kind, Count: item.Count})
	}
	c.JSON(stdhttp.StatusOK, listKindsResponse{Items: response})
}

func (h *MemoryHandler) recallMemories(c *gin.Context) {
	if h.recall == nil {
		c.JSON(stdhttp.StatusNotImplemented, ErrorResponse{Error: "recall service is not configured"})
		return
	}
	var req recallRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(stdhttp.StatusBadRequest, ErrorResponse{Error: err.Error()})
		return
	}
	results, err := h.recall.Recall(c.Request.Context(), memory.RecallInput{Content: req.Content, TopK: req.TopK, Temperature: req.Temperature})
	if err != nil {
		h.writeError(c, err)
		return
	}
	response := make([]gin.H, 0, len(results))
	for _, result := range results {
		response = append(response, gin.H{
			"memory": toMemoryResponse(result.Memory),
			"score":  result.Score,
			"reason": result.Reason,
		})
	}
	c.JSON(stdhttp.StatusOK, gin.H{"items": response})
}

func (h *MemoryHandler) writeError(c *gin.Context, err error) {
	status := stdhttp.StatusInternalServerError
	switch {
	case errors.Is(err, memory.ErrNotFound), errors.Is(err, ingestjob.ErrNotFound):
		status = stdhttp.StatusNotFound
	case isClientError(err):
		status = stdhttp.StatusBadRequest
	}
	c.JSON(status, ErrorResponse{Error: err.Error()})
}

func isClientError(err error) bool {
	switch err.Error() {
	case "content is required",
		"mode must be normal or smart",
		"type is invalid",
		"scope is invalid",
		"state is invalid",
		"top_k must be between 1 and 10":
		return true
	default:
		return false
	}
}

func toMemoryResponses(items []memory.Memory) []memoryResponse {
	out := make([]memoryResponse, 0, len(items))
	for _, item := range items {
		out = append(out, toMemoryResponse(item))
	}
	return out
}

func toMemoryResponse(item memory.Memory) memoryResponse {
	var lastAccessedAt *string
	if item.LastAccessedAt != nil {
		formatted := item.LastAccessedAt.UTC().Format(time.RFC3339)
		lastAccessedAt = &formatted
	}
	return memoryResponse{
		ID:             item.ID,
		Content:        item.Content,
		Type:           item.Type,
		Kind:           item.Kind,
		Kinds:          item.Kinds,
		Scope:          item.Scope,
		State:          item.State,
		Metadata:       item.Metadata,
		AgentID:        item.AgentID,
		SessionID:      item.SessionID,
		Source:         item.Source,
		Version:        item.Version,
		StoreCount:     item.StoreCount,
		UseCount:       item.UseCount,
		LastAccessedAt: lastAccessedAt,
		CreatedAt:      item.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:      item.UpdatedAt.UTC().Format(time.RFC3339),
	}
}

func toIngestJobResponse(job ingestjob.Job) ingestJobResponse {
	var nextRunAt *string
	if job.NextRunAt != nil {
		formatted := job.NextRunAt.UTC().Format(time.RFC3339)
		nextRunAt = &formatted
	}
	return ingestJobResponse{
		ID:           job.ID,
		State:        job.State,
		Mode:         job.Mode,
		ExecuteCount: job.ExecuteCount,
		LastError:    job.LastError,
		NextRunAt:    nextRunAt,
		CreatedAt:    job.CreatedAt.UTC().Format(time.RFC3339),
		UpdatedAt:    job.UpdatedAt.UTC().Format(time.RFC3339),
	}
}
