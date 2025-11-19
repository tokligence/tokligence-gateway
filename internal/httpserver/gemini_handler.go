package httpserver

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/tokligence/tokligence-gateway/internal/adapter/gemini"
)

// HandleGeminiProxy handles all Gemini API requests as a pass-through proxy
func (s *Server) HandleGeminiProxy(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get Gemini adapter
	adapter, err := s.getGeminiAdapter()
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini adapter not available: %v", err)
		}
		http.Error(w, `{"error": {"message": "Gemini API not configured", "code": 503}}`, http.StatusServiceUnavailable)
		return
	}

	// Check if this is an OpenAI-compatible request
	if strings.HasPrefix(r.URL.Path, "/v1beta/openai/") {
		s.handleGeminiOpenAICompat(ctx, w, r, adapter)
		return
	}

	// Extract path after /v1beta/
	// Example: /v1beta/models/gemini-pro:generateContent -> models/gemini-pro:generateContent
	geminiPath := strings.TrimPrefix(r.URL.Path, "/v1beta/")

	// Special case: list models endpoint (GET /v1beta/models)
	if r.Method == http.MethodGet && geminiPath == "models" {
		respBody, err := adapter.ListModels(ctx)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("Gemini listModels failed: %v", err)
			}
			http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(respBody)
		return
	}

	// Parse path to extract model name and method
	// Format: models/{model}:{method} or models/{model}
	if !strings.HasPrefix(geminiPath, "models/") {
		http.Error(w, `{"error": {"message": "invalid Gemini API path", "code": 400}}`, http.StatusBadRequest)
		return
	}

	modelAndMethod := strings.TrimPrefix(geminiPath, "models/")
	var model, method string

	// Check if path contains a method (e.g., :generateContent)
	if idx := strings.Index(modelAndMethod, ":"); idx > 0 {
		model = modelAndMethod[:idx]
		method = modelAndMethod[idx+1:]
	} else {
		// No method, assume it's GetModel
		model = modelAndMethod
		method = ""
	}

	// Read request body for POST requests
	var reqBody []byte
	if r.Method == http.MethodPost {
		reqBody, err = io.ReadAll(r.Body)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("Failed to read Gemini request body: %v", err)
			}
			http.Error(w, `{"error": {"message": "failed to read request body", "code": 400}}`, http.StatusBadRequest)
			return
		}
	}

	// Route based on method
	var respBody []byte
	switch method {
	case "generateContent":
		respBody, err = adapter.GenerateContent(ctx, model, reqBody)
	case "streamGenerateContent":
		s.handleGeminiStream(ctx, w, adapter, model, reqBody)
		return // handleGeminiStream writes response directly
	case "countTokens":
		respBody, err = adapter.CountTokens(ctx, model, reqBody)
	case "":
		// GetModel endpoint
		respBody, err = adapter.GetModel(ctx, model)
	default:
		http.Error(w, fmt.Sprintf(`{"error": {"message": "unsupported method: %s", "code": 400}}`, method), http.StatusBadRequest)
		return
	}

	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini %s failed for model %s: %v", method, model, err)
		}
		http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Return successful response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// handleGeminiStream handles streaming responses from Gemini
func (s *Server) handleGeminiStream(ctx context.Context, w http.ResponseWriter, adapter *gemini.GeminiAdapter, model string, reqBody []byte) {
	eventChan, err := adapter.StreamGenerateContent(ctx, model, reqBody)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini streamGenerateContent failed for model %s: %v", model, err)
		}
		http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		if s.logger != nil {
			s.logger.Printf("Streaming not supported")
		}
		return
	}

	// Stream events to client
	for event := range eventChan {
		if event.IsError() {
			if s.logger != nil {
				s.logger.Printf("Gemini stream error: %v", event.Error)
			}
			// Write error as SSE event
			fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"error": {"message": "%s"}}`, event.Error.Error()))
			flusher.Flush()
			return
		}

		if event.IsDone() {
			// Stream completed successfully
			return
		}

		// Forward raw SSE event from Gemini
		fmt.Fprintf(w, "data: %s\n\n", string(event.Data))
		flusher.Flush()
	}
}

// HandleGeminiGenerateContent handles POST /gemini/v1beta/models/{model}:generateContent
func (s *Server) HandleGeminiGenerateContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract model from path parameter
	model := s.extractModelFromPath(r.URL.Path, "generateContent")
	if model == "" {
		http.Error(w, `{"error": {"message": "model name required in path", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Read request body
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Failed to read Gemini request body: %v", err)
		}
		http.Error(w, `{"error": {"message": "failed to read request body", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Get Gemini adapter
	adapter, err := s.getGeminiAdapter()
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini adapter not available: %v", err)
		}
		http.Error(w, `{"error": {"message": "Gemini API not configured", "code": 503}}`, http.StatusServiceUnavailable)
		return
	}

	// Call Gemini API
	respBody, err := adapter.GenerateContent(ctx, model, reqBody)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini generateContent failed for model %s: %v", model, err)
		}
		http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Return raw Gemini response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleGeminiStreamGenerateContent handles POST /gemini/v1beta/models/{model}:streamGenerateContent
func (s *Server) HandleGeminiStreamGenerateContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract model from path parameter
	model := s.extractModelFromPath(r.URL.Path, "streamGenerateContent")
	if model == "" {
		http.Error(w, `{"error": {"message": "model name required in path", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Read request body
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Failed to read Gemini stream request body: %v", err)
		}
		http.Error(w, `{"error": {"message": "failed to read request body", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Get Gemini adapter
	adapter, err := s.getGeminiAdapter()
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini adapter not available: %v", err)
		}
		http.Error(w, `{"error": {"message": "Gemini API not configured", "code": 503}}`, http.StatusServiceUnavailable)
		return
	}

	// Call Gemini streaming API
	eventChan, err := adapter.StreamGenerateContent(ctx, model, reqBody)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini streamGenerateContent failed for model %s: %v", model, err)
		}
		http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Set SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(http.StatusOK)

	flusher, ok := w.(http.Flusher)
	if !ok {
		if s.logger != nil {
			s.logger.Printf("Streaming not supported")
		}
		return
	}

	// Stream events to client
	for event := range eventChan {
		if event.IsError() {
			if s.logger != nil {
				s.logger.Printf("Gemini stream error: %v", event.Error)
			}
			// Write error as SSE event
			fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"error": {"message": "%s"}}`, event.Error.Error()))
			flusher.Flush()
			return
		}

		if event.IsDone() {
			// Stream completed successfully
			return
		}

		// Forward raw SSE event from Gemini
		fmt.Fprintf(w, "data: %s\n\n", string(event.Data))
		flusher.Flush()
	}
}

// HandleGeminiCountTokens handles POST /gemini/v1beta/models/{model}:countTokens
func (s *Server) HandleGeminiCountTokens(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract model from path parameter
	model := s.extractModelFromPath(r.URL.Path, "countTokens")
	if model == "" {
		http.Error(w, `{"error": {"message": "model name required in path", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Read request body
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Failed to read Gemini countTokens request body: %v", err)
		}
		http.Error(w, `{"error": {"message": "failed to read request body", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Get Gemini adapter
	adapter, err := s.getGeminiAdapter()
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini adapter not available: %v", err)
		}
		http.Error(w, `{"error": {"message": "Gemini API not configured", "code": 503}}`, http.StatusServiceUnavailable)
		return
	}

	// Call Gemini API
	respBody, err := adapter.CountTokens(ctx, model, reqBody)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini countTokens failed for model %s: %v", model, err)
		}
		http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Return raw Gemini response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleGeminiListModels handles GET /gemini/v1beta/models
func (s *Server) HandleGeminiListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get Gemini adapter
	adapter, err := s.getGeminiAdapter()
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini adapter not available: %v", err)
		}
		http.Error(w, `{"error": {"message": "Gemini API not configured", "code": 503}}`, http.StatusServiceUnavailable)
		return
	}

	// Call Gemini API
	respBody, err := adapter.ListModels(ctx)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini listModels failed: %v", err)
		}
		http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Return raw Gemini response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// HandleGeminiGetModel handles GET /gemini/v1beta/models/{model}
func (s *Server) HandleGeminiGetModel(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Extract model from path parameter
	model := s.extractModelFromPath(r.URL.Path, "")
	if model == "" {
		http.Error(w, `{"error": {"message": "model name required in path", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Get Gemini adapter
	adapter, err := s.getGeminiAdapter()
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini adapter not available: %v", err)
		}
		http.Error(w, `{"error": {"message": "Gemini API not configured", "code": 503}}`, http.StatusServiceUnavailable)
		return
	}

	// Call Gemini API
	respBody, err := adapter.GetModel(ctx, model)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Gemini getModel failed for model %s: %v", model, err)
		}
		http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
		return
	}

	// Return raw Gemini response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(respBody)
}

// extractModelFromPath extracts the model name from the Gemini API path
// Path format: /gemini/v1beta/models/{model}:action or /gemini/v1beta/models/{model}
func (s *Server) extractModelFromPath(path string, action string) string {
	// Remove prefix
	path = strings.TrimPrefix(path, "/gemini/v1beta/models/")

	// Remove action suffix if present
	if action != "" {
		path = strings.TrimSuffix(path, ":"+action)
	}

	return strings.TrimSpace(path)
}

// getGeminiAdapter returns the Gemini adapter instance
func (s *Server) getGeminiAdapter() (*gemini.GeminiAdapter, error) {
	if s.geminiAdapter == nil {
		return nil, fmt.Errorf("gemini adapter not initialized")
	}
	return s.geminiAdapter, nil
}

// InitGeminiAdapter initializes the Gemini adapter with configuration
func (s *Server) InitGeminiAdapter(apiKey, baseURL string, timeoutSeconds int) error {
	if strings.TrimSpace(apiKey) == "" {
		if s.logger != nil {
			s.logger.Printf("Gemini API key not provided, Gemini endpoints will be unavailable")
		}
		return nil
	}

	cfg := gemini.Config{
		APIKey:  apiKey,
		BaseURL: baseURL,
	}

	if timeoutSeconds > 0 {
		cfg.RequestTimeout = time.Duration(timeoutSeconds) * time.Second
	}

	adapter, err := gemini.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to initialize Gemini adapter: %w", err)
	}

	s.geminiAdapter = adapter
	if s.logger != nil {
		s.logger.Printf("Gemini adapter initialized successfully")
	}
	return nil
}

// GeminiHealthCheck checks if Gemini adapter is available and healthy
func (s *Server) GeminiHealthCheck() map[string]interface{} {
	result := map[string]interface{}{
		"provider": "gemini",
		"status":   "unavailable",
	}

	if s.geminiAdapter == nil {
		result["error"] = "adapter not initialized"
		return result
	}

	result["status"] = "available"
	return result
}

// handleGeminiOpenAICompat handles requests to OpenAI-compatible endpoints
func (s *Server) handleGeminiOpenAICompat(ctx context.Context, w http.ResponseWriter, r *http.Request, adapter *gemini.GeminiAdapter) {
	// OpenAI-compatible paths: /v1beta/openai/chat/completions
	if !strings.HasPrefix(r.URL.Path, "/v1beta/openai/chat/completions") {
		http.Error(w, `{"error": {"message": "only /v1beta/openai/chat/completions is supported", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Read request body
	reqBody, err := io.ReadAll(r.Body)
	if err != nil {
		if s.logger != nil {
			s.logger.Printf("Failed to read Gemini OpenAI-compat request body: %v", err)
		}
		http.Error(w, `{"error": {"message": "failed to read request body", "code": 400}}`, http.StatusBadRequest)
		return
	}

	// Check if streaming is requested
	var reqData map[string]interface{}
	if err := json.Unmarshal(reqBody, &reqData); err != nil {
		http.Error(w, `{"error": {"message": "invalid JSON", "code": 400}}`, http.StatusBadRequest)
		return
	}

	stream, _ := reqData["stream"].(bool)

	if stream {
		// Handle streaming response
		eventChan, err := adapter.OpenAIChatCompletionStream(ctx, reqBody)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("Gemini OpenAI-compat stream failed: %v", err)
			}
			http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
			return
		}

		// Set SSE headers
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			if s.logger != nil {
				s.logger.Printf("Streaming not supported")
			}
			return
		}

		// Stream events to client
		for event := range eventChan {
			if event.IsError() {
				if s.logger != nil {
					s.logger.Printf("Gemini OpenAI-compat stream error: %v", event.Error)
				}
				fmt.Fprintf(w, "data: %s\n\n", fmt.Sprintf(`{"error": {"message": "%s"}}`, event.Error.Error()))
				flusher.Flush()
				return
			}

			if event.IsDone() {
				fmt.Fprintf(w, "data: [DONE]\n\n")
				flusher.Flush()
				return
			}

			// Forward SSE event from Gemini
			fmt.Fprintf(w, "data: %s\n\n", string(event.Data))
			flusher.Flush()
		}
	} else {
		// Handle non-streaming response
		respBody, err := adapter.OpenAIChatCompletion(ctx, reqBody)
		if err != nil {
			if s.logger != nil {
				s.logger.Printf("Gemini OpenAI-compat failed: %v", err)
			}
			http.Error(w, fmt.Sprintf(`{"error": {"message": "%s", "code": 500}}`, err.Error()), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write(respBody)
	}
}
