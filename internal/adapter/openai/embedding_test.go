package openai

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
	"time"

	openaitypes "github.com/tokligence/tokligence-gateway/internal/openai"
	"github.com/tokligence/tokligence-gateway/internal/testutil"
)

func TestCreateEmbedding_Success(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if auth := r.Header.Get("Authorization"); !strings.HasPrefix(auth, "Bearer ") {
			t.Errorf("missing or invalid Authorization header: %q", auth)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}

		// Parse request body
		var reqBody map[string]interface{}
		if err := json.NewDecoder(r.Body).Decode(&reqBody); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			http.Error(w, "bad request", http.StatusBadRequest)
			return
		}

		// Verify required fields
		if _, ok := reqBody["model"]; !ok {
			t.Error("request missing 'model' field")
		}
		if _, ok := reqBody["input"]; !ok {
			t.Error("request missing 'input' field")
		}

		// Return mock response
		response := openaitypes.EmbeddingResponse{
			Object: "list",
			Data: []openaitypes.EmbeddingData{
				{
					Object:    "embedding",
					Embedding: []float64{0.1, 0.2, 0.3, 0.4},
					Index:     0,
				},
			},
			Model: "text-embedding-ada-002",
			Usage: openaitypes.EmbeddingUsage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Hello world",
	}

	resp, err := adapter.CreateEmbedding(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateEmbedding() error = %v", err)
	}

	// Verify response
	if resp.Object != "list" {
		t.Errorf("response.Object = %q, want 'list'", resp.Object)
	}
	if len(resp.Data) == 0 {
		t.Fatal("response has no data")
	}
	if resp.Data[0].Object != "embedding" {
		t.Errorf("data[0].Object = %q, want 'embedding'", resp.Data[0].Object)
	}
	if len(resp.Data[0].Embedding) == 0 {
		t.Error("data[0].Embedding is empty")
	}
	if resp.Usage.PromptTokens == 0 {
		t.Error("response.Usage.PromptTokens is 0")
	}
}

func TestCreateEmbedding_MultipleInputs(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Verify input is an array
		input, ok := reqBody["input"].([]interface{})
		if !ok || len(input) != 2 {
			t.Errorf("expected input array with 2 elements")
		}

		response := openaitypes.EmbeddingResponse{
			Object: "list",
			Data: []openaitypes.EmbeddingData{
				{
					Object:    "embedding",
					Embedding: []float64{0.1, 0.2, 0.3},
					Index:     0,
				},
				{
					Object:    "embedding",
					Embedding: []float64{0.4, 0.5, 0.6},
					Index:     1,
				},
			},
			Model: "text-embedding-ada-002",
			Usage: openaitypes.EmbeddingUsage{
				PromptTokens: 10,
				TotalTokens:  10,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: []string{"Hello", "World"},
	}

	resp, err := adapter.CreateEmbedding(context.Background(), req)
	if err != nil {
		t.Fatalf("CreateEmbedding() error = %v", err)
	}

	// Verify we got embeddings for both inputs
	if len(resp.Data) != 2 {
		t.Errorf("len(resp.Data) = %d, want 2", len(resp.Data))
	}
}

func TestCreateEmbedding_NoInput(t *testing.T) {
	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: "https://api.openai.com/v1",
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: nil,
	}

	_, err = adapter.CreateEmbedding(context.Background(), req)
	if err == nil {
		t.Error("CreateEmbedding() expected error for nil input, got nil")
	}
	if !strings.Contains(err.Error(), "input required") {
		t.Errorf("error = %v, want error containing 'input required'", err)
	}
}

func TestCreateEmbedding_APIError(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		response   interface{}
		wantErrMsg string
	}{
		{
			name:       "401 unauthorized",
			statusCode: http.StatusUnauthorized,
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid API key",
					"type":    "invalid_request_error",
					"code":    "invalid_api_key",
				},
			},
			wantErrMsg: "Invalid API key",
		},
		{
			name:       "400 bad request",
			statusCode: http.StatusBadRequest,
			response: map[string]interface{}{
				"error": map[string]interface{}{
					"message": "Invalid model specified",
					"type":    "invalid_request_error",
				},
			},
			wantErrMsg: "Invalid model specified",
		},
		{
			name:       "500 server error",
			statusCode: http.StatusInternalServerError,
			response:   map[string]interface{}{"error": "internal server error"},
			wantErrMsg: "http 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tt.statusCode)
				json.NewEncoder(w).Encode(tt.response)
			}))
			defer server.Close()

			adapter, err := New(Config{
				APIKey:  "sk-test123",
				BaseURL: server.URL,
			})
			if err != nil {
				t.Fatalf("New() error = %v", err)
			}

			req := openaitypes.EmbeddingRequest{
				Model: "text-embedding-ada-002",
				Input: "Hello",
			}

			_, err = adapter.CreateEmbedding(context.Background(), req)
			if err == nil {
				t.Error("CreateEmbedding() expected error, got nil")
				return
			}
			if !strings.Contains(err.Error(), tt.wantErrMsg) {
				t.Errorf("error = %v, want error containing %q", err, tt.wantErrMsg)
			}
		})
	}
}

func TestCreateEmbedding_WithOptionalParams(t *testing.T) {
	dimensions := 512

	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var reqBody map[string]interface{}
		json.NewDecoder(r.Body).Decode(&reqBody)

		// Verify optional parameters
		if dims, ok := reqBody["dimensions"].(float64); !ok || int(dims) != dimensions {
			t.Errorf("dimensions = %v, want %d", reqBody["dimensions"], dimensions)
		}
		if format, ok := reqBody["encoding_format"].(string); !ok || format != "float" {
			t.Errorf("encoding_format = %v, want 'float'", reqBody["encoding_format"])
		}
		if user, ok := reqBody["user"].(string); !ok || user != "test-user" {
			t.Errorf("user = %v, want 'test-user'", reqBody["user"])
		}

		response := openaitypes.EmbeddingResponse{
			Object: "list",
			Data: []openaitypes.EmbeddingData{
				{
					Object:    "embedding",
					Embedding: make([]float64, dimensions),
					Index:     0,
				},
			},
			Model: "text-embedding-3-large",
			Usage: openaitypes.EmbeddingUsage{
				PromptTokens: 5,
				TotalTokens:  5,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.EmbeddingRequest{
		Model:          "text-embedding-3-large",
		Input:          "Hello",
		EncodingFormat: "float",
		Dimensions:     &dimensions,
		User:           "test-user",
	}

	resp, err := adapter.CreateEmbedding(context.Background(), req)
	if err != nil {
		t.Errorf("CreateEmbedding() error = %v", err)
	}

	// Verify response has correct dimensions
	if len(resp.Data) > 0 && len(resp.Data[0].Embedding) != dimensions {
		t.Errorf("embedding dimensions = %d, want %d", len(resp.Data[0].Embedding), dimensions)
	}
}

func TestCreateEmbedding_Timeout(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:         "sk-test123",
		BaseURL:        server.URL,
		RequestTimeout: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Hello",
	}

	_, err = adapter.CreateEmbedding(context.Background(), req)
	if err == nil {
		t.Error("CreateEmbedding() expected timeout error, got nil")
	}
}

func TestCreateEmbedding_ContextCancellation(t *testing.T) {
	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:  "sk-test123",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	req := openaitypes.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Hello",
	}

	_, err = adapter.CreateEmbedding(ctx, req)
	if err == nil {
		t.Error("CreateEmbedding() expected context cancellation error, got nil")
	}
}

func TestCreateEmbedding_OrganizationHeader(t *testing.T) {
	orgID := "org-test123"

	server := testutil.NewIPv4Server(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if org := r.Header.Get("OpenAI-Organization"); org != orgID {
			t.Errorf("OpenAI-Organization = %q, want %q", org, orgID)
		}

		response := openaitypes.EmbeddingResponse{
			Object: "list",
			Data: []openaitypes.EmbeddingData{
				{Object: "embedding", Embedding: []float64{0.1, 0.2}, Index: 0},
			},
			Model: "text-embedding-ada-002",
			Usage: openaitypes.EmbeddingUsage{PromptTokens: 5, TotalTokens: 5},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	adapter, err := New(Config{
		APIKey:       "sk-test123",
		BaseURL:      server.URL,
		Organization: orgID,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	req := openaitypes.EmbeddingRequest{
		Model: "text-embedding-ada-002",
		Input: "Hello",
	}

	_, err = adapter.CreateEmbedding(context.Background(), req)
	if err != nil {
		t.Errorf("CreateEmbedding() error = %v", err)
	}
}
