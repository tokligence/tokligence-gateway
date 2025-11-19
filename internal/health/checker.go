package health

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// Status represents the health status of a component.
type Status string

const (
	StatusHealthy   Status = "healthy"
	StatusDegraded  Status = "degraded"
	StatusUnhealthy Status = "unhealthy"
)

// CheckResult holds the result of a health check.
type CheckResult struct {
	Status    Status        `json:"status"`
	Message   string        `json:"message,omitempty"`
	Latency   time.Duration `json:"latency_ms"`
	Timestamp time.Time     `json:"timestamp"`
	Error     string        `json:"error,omitempty"`
}

// Component represents a system component that can be health-checked.
type Component struct {
	Name string
	Type string // database, http, cache, etc.
	CheckResult
}

// Checker performs health checks on system components.
type Checker struct {
	components []Component
	mu         sync.RWMutex

	// Dependencies
	identityDB *sql.DB
	ledgerDB   *sql.DB

	// Upstream endpoints
	openaiBaseURL    string
	anthropicBaseURL string

	// Timeouts
	dbTimeout       time.Duration
	httpTimeout     time.Duration
	maxDatabasesLatency time.Duration
}

// Config holds health checker configuration.
type Config struct {
	// Databases
	IdentityDB *sql.DB
	LedgerDB   *sql.DB

	// Upstream endpoints
	OpenAIBaseURL    string
	AnthropicBaseURL string

	// Timeouts
	DBTimeout       time.Duration
	HTTPTimeout     time.Duration
	MaxDatabaseLatency time.Duration
}

// New creates a new health checker.
func New(cfg Config) *Checker {
	if cfg.DBTimeout == 0 {
		cfg.DBTimeout = 2 * time.Second
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 5 * time.Second
	}
	if cfg.MaxDatabaseLatency == 0 {
		cfg.MaxDatabaseLatency = 100 * time.Millisecond
	}

	return &Checker{
		identityDB:          cfg.IdentityDB,
		ledgerDB:            cfg.LedgerDB,
		openaiBaseURL:       cfg.OpenAIBaseURL,
		anthropicBaseURL:    cfg.AnthropicBaseURL,
		dbTimeout:           cfg.DBTimeout,
		httpTimeout:         cfg.HTTPTimeout,
		maxDatabasesLatency: cfg.MaxDatabaseLatency,
	}
}

// Check performs all health checks and returns overall status.
func (c *Checker) Check(ctx context.Context) HealthStatus {
	var wg sync.WaitGroup
	results := make(chan Component, 10)

	// Check identity database
	if c.identityDB != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- c.checkDatabase(ctx, "identity_db", c.identityDB)
		}()
	}

	// Check ledger database
	if c.ledgerDB != nil {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- c.checkDatabase(ctx, "ledger_db", c.ledgerDB)
		}()
	}

	// Check OpenAI endpoint
	if c.openaiBaseURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- c.checkHTTPEndpoint(ctx, "openai_api", c.openaiBaseURL)
		}()
	}

	// Check Anthropic endpoint
	if c.anthropicBaseURL != "" {
		wg.Add(1)
		go func() {
			defer wg.Done()
			results <- c.checkHTTPEndpoint(ctx, "anthropic_api", c.anthropicBaseURL)
		}()
	}

	// Close results channel when all checks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	components := make([]Component, 0)
	for comp := range results {
		components = append(components, comp)
	}

	c.mu.Lock()
	c.components = components
	c.mu.Unlock()

	// Determine overall status
	return c.calculateOverallStatus(components)
}

// checkDatabase checks database connectivity and performance.
func (c *Checker) checkDatabase(ctx context.Context, name string, db *sql.DB) Component {
	comp := Component{
		Name: name,
		Type: "database",
		CheckResult: CheckResult{
			Timestamp: time.Now(),
		},
	}

	start := time.Now()

	// Create context with timeout
	dbCtx, cancel := context.WithTimeout(ctx, c.dbTimeout)
	defer cancel()

	// Simple ping to check connectivity
	err := db.PingContext(dbCtx)
	comp.Latency = time.Since(start)

	if err != nil {
		comp.Status = StatusUnhealthy
		comp.Error = err.Error()
		comp.Message = "Database unreachable"
		return comp
	}

	// Check latency
	if comp.Latency > c.maxDatabasesLatency {
		comp.Status = StatusDegraded
		comp.Message = fmt.Sprintf("High latency: %v", comp.Latency)
	} else {
		comp.Status = StatusHealthy
		comp.Message = "Connected"
	}

	return comp
}

// checkHTTPEndpoint checks if an HTTP endpoint is reachable.
func (c *Checker) checkHTTPEndpoint(ctx context.Context, name, baseURL string) Component {
	comp := Component{
		Name: name,
		Type: "http",
		CheckResult: CheckResult{
			Timestamp: time.Now(),
		},
	}

	// Skip if no URL provided
	if baseURL == "" {
		comp.Status = StatusHealthy
		comp.Message = "Not configured"
		return comp
	}

	start := time.Now()

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: c.httpTimeout,
	}

	// Try to reach the endpoint (just check if it's up)
	req, err := http.NewRequestWithContext(ctx, "GET", baseURL, nil)
	if err != nil {
		comp.Status = StatusUnhealthy
		comp.Error = err.Error()
		comp.Latency = time.Since(start)
		return comp
	}

	resp, err := client.Do(req)
	comp.Latency = time.Since(start)

	if err != nil {
		comp.Status = StatusDegraded
		comp.Error = err.Error()
		comp.Message = "Endpoint unreachable"
		return comp
	}
	defer resp.Body.Close()

	// Accept any response (even 4xx/5xx) as "reachable"
	// We just want to know if the service is up
	comp.Status = StatusHealthy
	comp.Message = fmt.Sprintf("Reachable (HTTP %d)", resp.StatusCode)

	return comp
}

// calculateOverallStatus determines overall health based on component statuses.
func (c *Checker) calculateOverallStatus(components []Component) HealthStatus {
	overallStatus := StatusHealthy
	criticalUnhealthy := false

	for _, comp := range components {
		switch comp.Status {
		case StatusUnhealthy:
			// Database failures are critical
			if comp.Type == "database" {
				criticalUnhealthy = true
			}
			if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		case StatusDegraded:
			if overallStatus == StatusHealthy {
				overallStatus = StatusDegraded
			}
		}
	}

	// If any critical component is unhealthy, overall is unhealthy
	if criticalUnhealthy {
		overallStatus = StatusUnhealthy
	}

	return HealthStatus{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Components: components,
	}
}

// HealthStatus represents the overall health of the system.
type HealthStatus struct {
	Status     Status      `json:"status"`
	Timestamp  time.Time   `json:"timestamp"`
	Components []Component `json:"components"`
}

// GetLastStatus returns the last health check result.
func (c *Checker) GetLastStatus() HealthStatus {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if len(c.components) == 0 {
		return HealthStatus{
			Status:    StatusHealthy,
			Timestamp: time.Now(),
		}
	}

	return c.calculateOverallStatus(c.components)
}
