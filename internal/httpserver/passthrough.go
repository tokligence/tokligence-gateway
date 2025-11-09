package httpserver

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	anthpkg "github.com/tokligence/tokligence-gateway/internal/httpserver/anthropic"
	"github.com/tokligence/tokligence-gateway/internal/userstore"
)

func (s *Server) guardAnthropicTokens(maxTokens int) error {
	if !s.anthropicTokenCheckEnabled {
		return nil
	}
	if maxTokens <= 0 {
		return fmt.Errorf("anthropic: max_tokens required when token guard enabled")
	}
	if s.anthropicMaxTokens > 0 && maxTokens > s.anthropicMaxTokens {
		return fmt.Errorf("anthropic: max_tokens %d exceeds limit %d", maxTokens, s.anthropicMaxTokens)
	}
	return nil
}

func (s *Server) anthropicPassthrough(w http.ResponseWriter, r *http.Request, raw []byte, stream bool, sessionUser *userstore.User, apiKey *userstore.APIKey) {
	raw = anthpkg.ClampMaxTokens(raw, 16384)
	url := s.anthBaseURL + "/v1/messages"
	if q := r.URL.RawQuery; q != "" {
		url += "?" + q
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(raw))
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	if strings.TrimSpace(s.anthAPIKey) != "" {
		req.Header.Set("x-api-key", s.anthAPIKey)
	}
	req.Header.Set("anthropic-version", s.anthVersion)
	s.debugf("anthropic.passthrough: POST %s stream=%v", url, stream)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		s.respondError(w, http.StatusBadGateway, err)
		return
	}
	defer resp.Body.Close()

	for k, vals := range resp.Header {
		switch strings.ToLower(k) {
		case "connection", "transfer-encoding", "content-length":
			continue
		}
		w.Header()[k] = vals
	}
	if stream {
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
	}
	w.WriteHeader(resp.StatusCode)
	if stream {
		flusher, ok := w.(http.Flusher)
		if !ok {
			s.respondError(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
			return
		}
		buf := make([]byte, 8192)
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				if _, werr := w.Write(buf[:n]); werr != nil {
					break
				}
				flusher.Flush()
			}
			if err != nil {
				break
			}
		}
		return
	}

	body, _ := io.ReadAll(resp.Body)
	if s.isDebug() {
		s.debugf("anthropic.passthrough: body=%s", string(previewBytes(body, 512)))
	}
	_, _ = w.Write(body)
	if resp.StatusCode != http.StatusOK {
		return
	}

	var usage struct {
		Usage struct {
			InputTokens  int `json:"input_tokens"`
			OutputTokens int `json:"output_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(body, &usage) == nil {
		s.recordUsageLedger(
			r.Context(),
			sessionUser,
			apiKey,
			int64(usage.Usage.InputTokens),
			int64(usage.Usage.OutputTokens),
			"anthropic.messages(passthrough)",
		)
	}
}

func previewBytes(b []byte, n int) []byte {
	if len(b) <= n {
		return b
	}
	return b[:n]
}
