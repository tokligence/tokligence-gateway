package openai

// ModelsResponse represents the response from /v1/models endpoint.
type ModelsResponse struct {
	Object string  `json:"object"`
	Data   []Model `json:"data"`
}

// Model represents a single model in the OpenAI API format.
type Model struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// NewModelsResponse creates a ModelsResponse with the given models.
func NewModelsResponse(models []Model) ModelsResponse {
	return ModelsResponse{
		Object: "list",
		Data:   models,
	}
}

// NewModel creates a Model instance.
func NewModel(id, ownedBy string, created int64) Model {
	return Model{
		ID:      id,
		Object:  "model",
		Created: created,
		OwnedBy: ownedBy,
	}
}
