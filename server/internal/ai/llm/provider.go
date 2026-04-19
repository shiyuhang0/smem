package llm

import "context"

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type Provider interface {
	GenerateText(context.Context, []Message) (string, error)
}
