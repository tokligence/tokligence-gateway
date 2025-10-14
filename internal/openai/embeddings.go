package openai

// EmbeddingRequest represents a request to create embeddings.
type EmbeddingRequest struct {
	Model          string   `json:"model"`
	Input          interface{} `json:"input"` // string or []string
	EncodingFormat string   `json:"encoding_format,omitempty"` // "float" or "base64"
	Dimensions     *int     `json:"dimensions,omitempty"` // for text-embedding-3 models
	User           string   `json:"user,omitempty"`
}

// EmbeddingResponse represents the response from an embeddings request.
type EmbeddingResponse struct {
	Object string           `json:"object"`
	Data   []EmbeddingData  `json:"data"`
	Model  string           `json:"model"`
	Usage  EmbeddingUsage   `json:"usage"`
}

// EmbeddingData represents a single embedding result.
type EmbeddingData struct {
	Object    string    `json:"object"`
	Embedding []float64 `json:"embedding"`
	Index     int       `json:"index"`
}

// EmbeddingUsage represents token usage for embeddings.
type EmbeddingUsage struct {
	PromptTokens int `json:"prompt_tokens"`
	TotalTokens  int `json:"total_tokens"`
}

// NewEmbeddingResponse creates an EmbeddingResponse.
func NewEmbeddingResponse(model string, embeddings [][]float64, promptTokens int) EmbeddingResponse {
	data := make([]EmbeddingData, len(embeddings))
	for i, emb := range embeddings {
		data[i] = EmbeddingData{
			Object:    "embedding",
			Embedding: emb,
			Index:     i,
		}
	}

	return EmbeddingResponse{
		Object: "list",
		Data:   data,
		Model:  model,
		Usage: EmbeddingUsage{
			PromptTokens: promptTokens,
			TotalTokens:  promptTokens,
		},
	}
}
