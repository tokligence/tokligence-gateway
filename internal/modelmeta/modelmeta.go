package modelmeta

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

// Entry describes basic limits for a model.
type Entry struct {
	Model            string `json:"model"`
	Provider         string `json:"provider,omitempty"`
	ContextTokens    int    `json:"context_tokens,omitempty"`
	MaxCompletionCap int    `json:"max_completion_cap,omitempty"`
	UpdatedAt        string `json:"updated_at,omitempty"`
}

// Store holds loaded metadata with simple lookups.
type Store struct {
	mu      sync.RWMutex
	entries map[string]Entry
	source  string
	client  *http.Client
	logger  Logger
}

// Logger is a minimal logging interface.
type Logger interface {
	Printf(format string, args ...any)
}

// LoaderConfig controls where to load metadata from.
type LoaderConfig struct {
	LocalPath       string
	RemoteURL       string
	RefreshInterval time.Duration
	HTTPClient      *http.Client
}

// NewStore returns an empty store.
func NewStore() *Store {
	return &Store{
		entries: make(map[string]Entry),
		client:  http.DefaultClient,
	}
}

// SetLogger sets an optional logger for warnings/errors.
func (s *Store) SetLogger(l Logger) {
	s.logger = l
}

// MaxCompletionCap returns (cap, true) if known; otherwise (0, false).
func (s *Store) MaxCompletionCap(model string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	e, ok := s.entries[strings.ToLower(strings.TrimSpace(model))]
	if !ok {
		return 0, false
	}
	if e.MaxCompletionCap <= 0 {
		return 0, false
	}
	return e.MaxCompletionCap, true
}

// Load refreshes metadata from local path; returns number of entries loaded.
func (s *Store) Load(path string) (int, error) {
	if strings.TrimSpace(path) == "" {
		return 0, errors.New("modelmeta: empty path")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	var entries []Entry
	if err := json.Unmarshal(b, &entries); err != nil {
		return 0, err
	}
	s.apply(entries, path)
	return len(entries), nil
}

// Fetch pulls metadata from a remote URL (JSON array of Entry).
func (s *Store) Fetch(url string) (int, error) {
	if strings.TrimSpace(url) == "" {
		return 0, errors.New("modelmeta: empty url")
	}
	client := s.client
	if client == nil {
		client = http.DefaultClient
	}
	resp, err := client.Get(url)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return 0, errors.New(resp.Status)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, err
	}
	var entries []Entry
	if err := json.Unmarshal(body, &entries); err != nil {
		return 0, err
	}
	s.apply(entries, url)
	return len(entries), nil
}

// apply replaces current entries.
func (s *Store) apply(entries []Entry, src string) {
	m := make(map[string]Entry)
	for _, e := range entries {
		model := strings.ToLower(strings.TrimSpace(e.Model))
		if model == "" {
			continue
		}
		m[model] = e
	}
	s.mu.Lock()
	s.entries = m
	s.source = src
	s.mu.Unlock()
}

// StartAutoRefresh starts a goroutine that periodically reloads from remote if set, else local.
func (s *Store) StartAutoRefresh(cfg LoaderConfig) {
	if cfg.RefreshInterval <= 0 {
		cfg.RefreshInterval = 24 * time.Hour
	}
	if cfg.HTTPClient != nil {
		s.client = cfg.HTTPClient
	}
	firstLoad := func() {
		if cfg.RemoteURL != "" {
			if _, err := s.Fetch(cfg.RemoteURL); err == nil {
				return
			} else if s.logger != nil {
				s.logger.Printf("modelmeta: remote fetch failed (%s): %v", cfg.RemoteURL, err)
			}
		}
		if cfg.LocalPath != "" {
			if _, err := s.Load(cfg.LocalPath); err != nil && s.logger != nil {
				s.logger.Printf("modelmeta: local load failed (%s): %v", cfg.LocalPath, err)
			}
		}
	}
	firstLoad()
	ticker := time.NewTicker(cfg.RefreshInterval)
	go func() {
		for range ticker.C {
			if cfg.RemoteURL != "" {
				if _, err := s.Fetch(cfg.RemoteURL); err == nil {
					continue
				} else if s.logger != nil {
					s.logger.Printf("modelmeta: periodic remote fetch failed (%s): %v", cfg.RemoteURL, err)
				}
			}
			if cfg.LocalPath != "" {
				if _, err := s.Load(cfg.LocalPath); err != nil && s.logger != nil {
					s.logger.Printf("modelmeta: periodic local load failed (%s): %v", cfg.LocalPath, err)
				}
			}
		}
	}()
}
