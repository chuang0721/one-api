package sensetime

import (
	"github.com/songquanpeng/one-api/relay/model"
)

type ErrorMessage struct {
	Code    string   `json:"code"`
	Message string   `json:"message"`
	Details []string `json:"details"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model        string    `json:"model"`
	Messages     []Message `json:"messages"`
	Temperature  float64   `json:"temperature,omitempty"`
	TopP         float64   `json:"top_p,omitempty"`
	Stream       bool      `json:"stream,omitempty"`
	MaxNewTokens int       `json:"max_new_tokens,omitempty"`
}

type ResponseMessage struct {
	Role         string `json:"role"`
	Message      string `json:"message"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type ResponseData struct {
	Id      string            `json:"id"`
	Choices []ResponseMessage `json:"choices"`
	Usage   model.Usage       `json:"usage"`
}

type ChatResponse struct {
	Data  ResponseData `json:"data"`
	Error ErrorMessage `json:"error"`
}

type StreamChoice struct {
	Role         string `json:"role"`
	Delta        string `json:"delta"`
	Index        int    `json:"index"`
	FinishReason string `json:"finish_reason"`
}

type DataStreamResponse struct {
	Id      string         `json:"id"`
	Choices []StreamChoice `json:"choices"`
}

type Status struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ChatStreamResponse struct {
	Data   DataStreamResponse `json:"data"`
	Usage  model.Usage        `json:"usage"`
	Status Status             `json:"status"`
}

type EmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type EmbeddingData struct {
	Index         int       `json:"index"`
	Embedding     []float64 `json:"embedding"`
	StatusCode    int       `json:"status_code"`
	StatusMessage string    `json:"status_message"`
}

type EmbeddingResponse struct {
	Embeddings []EmbeddingData `json:"embeddings"`
	Usage      model.Usage     `json:"usage"`
}
