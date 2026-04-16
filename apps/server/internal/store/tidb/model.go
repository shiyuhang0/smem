package tidb

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"

	"smem/apps/server/internal/domain/memory"
)

type StringSlice []string

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	buf, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}

func (s *StringSlice) Scan(value any) error {
	if value == nil {
		*s = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported string slice value %T", value)
	}
	var out []string
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*s = StringSlice(out)
	return nil
}

type JSONMap map[string]any

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	buf, err := json.Marshal(map[string]any(m))
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}

func (m *JSONMap) Scan(value any) error {
	if value == nil {
		*m = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported json map value %T", value)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*m = JSONMap(out)
	return nil
}

type Float32Slice []float32

func (s Float32Slice) Value() (driver.Value, error) {
	if s == nil {
		return nil, nil
	}
	buf, err := json.Marshal([]float32(s))
	if err != nil {
		return nil, err
	}
	return string(buf), nil
}

func (s *Float32Slice) Scan(value any) error {
	if value == nil {
		*s = nil
		return nil
	}
	var data []byte
	switch v := value.(type) {
	case []byte:
		data = v
	case string:
		data = []byte(v)
	default:
		return fmt.Errorf("unsupported float32 slice value %T", value)
	}
	var out []float32
	if err := json.Unmarshal(data, &out); err != nil {
		return err
	}
	*s = Float32Slice(out)
	return nil
}

type MemoryModel struct {
	ID             string       `gorm:"primaryKey;size:64"`
	Content        string       `gorm:"type:text;not null"`
	Embedding      Float32Slice `gorm:"type:json"`
	ContentHash    string       `gorm:"size:64;uniqueIndex;not null"`
	Type           string       `gorm:"size:32"`
	Kinds          StringSlice  `gorm:"type:json"`
	Scope          string       `gorm:"size:32;index;not null"`
	State          string       `gorm:"size:32;index;not null"`
	Metadata       JSONMap      `gorm:"type:json"`
	AgentID        string       `gorm:"size:128;index"`
	SessionID      string       `gorm:"size:128;index"`
	Source         string       `gorm:"size:128"`
	Version        int          `gorm:"not null"`
	StoreCount     int          `gorm:"not null"`
	UseCount       int          `gorm:"not null"`
	LastAccessedAt *time.Time
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

func (MemoryModel) TableName() string {
	return "memories"
}

func fromDomain(m memory.Memory) MemoryModel {
	return MemoryModel{
		ID:             m.ID,
		Content:        m.Content,
		Embedding:      Float32Slice(m.Embedding),
		ContentHash:    m.ContentHash,
		Type:           string(m.Type),
		Kinds:          StringSlice(m.Kinds),
		Scope:          string(m.Scope),
		State:          string(m.State),
		Metadata:       JSONMap(m.Metadata),
		AgentID:        m.AgentID,
		SessionID:      m.SessionID,
		Source:         m.Source,
		Version:        m.Version,
		StoreCount:     m.StoreCount,
		UseCount:       m.UseCount,
		LastAccessedAt: m.LastAccessedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}

func (m MemoryModel) toDomain() memory.Memory {
	return memory.Memory{
		ID:             m.ID,
		Content:        m.Content,
		Embedding:      []float32(m.Embedding),
		ContentHash:    m.ContentHash,
		Type:           memory.Type(m.Type),
		Kinds:          []string(m.Kinds),
		Scope:          memory.Scope(m.Scope),
		State:          memory.State(m.State),
		Metadata:       map[string]any(m.Metadata),
		AgentID:        m.AgentID,
		SessionID:      m.SessionID,
		Source:         m.Source,
		Version:        m.Version,
		StoreCount:     m.StoreCount,
		UseCount:       m.UseCount,
		LastAccessedAt: m.LastAccessedAt,
		CreatedAt:      m.CreatedAt,
		UpdatedAt:      m.UpdatedAt,
	}
}
