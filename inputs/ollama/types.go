package ollama

// OllamaResponse Ollama API 响应结构（/api/chat 和 /api/generate）
type OllamaResponse struct {
	Model              string  `json:"model"`
	CreatedAt          string  `json:"created_at"`
	Message            Message `json:"message,omitempty"` // chat API
	Response           string  `json:"response,omitempty"` // generate API
	Done               bool    `json:"done"`
	TotalDuration      int64   `json:"total_duration"`       // 纳秒
	LoadDuration       int64   `json:"load_duration"`        // 纳秒
	PromptEvalCount    int64   `json:"prompt_eval_count"`
	PromptEvalDuration int64   `json:"prompt_eval_duration"` // 纳秒
	EvalCount          int64   `json:"eval_count"`
	EvalDuration       int64   `json:"eval_duration"` // 纳秒
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OllamaRequest Ollama API 请求结构
type OllamaRequest struct {
	Model    string    `json:"model"`
	Prompt   string    `json:"prompt,omitempty"`   // generate API
	Messages []Message `json:"messages,omitempty"` // chat API
	Stream   bool      `json:"stream"`
}

// StreamChunk 流式响应的单个 chunk
type StreamChunk struct {
	Model              string  `json:"model"`
	CreatedAt          string  `json:"created_at"`
	Message            Message `json:"message,omitempty"`
	Response           string  `json:"response,omitempty"`
	Done               bool    `json:"done"`
	TotalDuration      int64   `json:"total_duration,omitempty"`
	LoadDuration       int64   `json:"load_duration,omitempty"`
	PromptEvalCount    int64   `json:"prompt_eval_count,omitempty"`
	PromptEvalDuration int64   `json:"prompt_eval_duration,omitempty"`
	EvalCount          int64   `json:"eval_count,omitempty"`
	EvalDuration       int64   `json:"eval_duration,omitempty"`
}

