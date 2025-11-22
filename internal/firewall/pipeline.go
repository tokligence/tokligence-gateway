package firewall

import (
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Pipeline orchestrates the execution of multiple filters.
type Pipeline struct {
	mode          FirewallMode
	inputFilters  []InputFilter
	outputFilters []OutputFilter
	tokenizer     *PIITokenizer // For redact mode
	logger        *log.Logger
	mu            sync.RWMutex
}

// NewPipeline creates a new filter pipeline.
func NewPipeline(mode FirewallMode, logger *log.Logger) *Pipeline {
	return &Pipeline{
		mode:          mode,
		inputFilters:  make([]InputFilter, 0),
		outputFilters: make([]OutputFilter, 0),
		tokenizer:     NewPIITokenizerWithMemoryStore(), // Default to in-memory
		logger:        logger,
	}
}

// SetTokenizer sets a custom tokenizer (for Redis or other backends)
func (p *Pipeline) SetTokenizer(tokenizer *PIITokenizer) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.tokenizer = tokenizer
}

// SetMode updates the firewall mode.
func (p *Pipeline) SetMode(mode FirewallMode) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.mode = mode
}

// GetMode returns the current firewall mode.
func (p *Pipeline) GetMode() FirewallMode {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.mode
}

// AddFilter registers a new filter to the pipeline.
func (p *Pipeline) AddFilter(filter Filter) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Check if filter implements InputFilter
	if f, ok := filter.(InputFilter); ok {
		if f.Direction() == DirectionInput || f.Direction() == DirectionBoth {
			p.inputFilters = append(p.inputFilters, f)
			// Sort by priority
			sort.Slice(p.inputFilters, func(i, j int) bool {
				return p.inputFilters[i].Priority() < p.inputFilters[j].Priority()
			})
		}
	}

	// Check if filter implements OutputFilter (not mutually exclusive with InputFilter)
	if f, ok := filter.(OutputFilter); ok {
		if f.Direction() == DirectionOutput || f.Direction() == DirectionBoth {
			p.outputFilters = append(p.outputFilters, f)
			// Sort by priority
			sort.Slice(p.outputFilters, func(i, j int) bool {
				return p.outputFilters[i].Priority() < p.outputFilters[j].Priority()
			})
		}
	}
}

// ProcessInput runs all input filters on the request.
func (p *Pipeline) ProcessInput(ctx context.Context, fctx *FilterContext) error {
	p.mu.RLock()
	mode := p.mode
	filters := p.inputFilters
	tokenizer := p.tokenizer
	p.mu.RUnlock()

	if mode == ModeDisabled {
		return nil
	}

	fctx.Context = ctx
	fctx.Mode = mode
	fctx.Tokenizer = tokenizer
	fctx.PIITokens = make(map[string]*PIIToken)
	start := time.Now()

	p.logDebugf("processing input: mode=%s, session=%s, filters=%d", mode, fctx.SessionID, len(filters))

	for _, filter := range filters {
		filterStart := time.Now()

		if err := filter.ApplyInput(fctx); err != nil {
			p.logf("ERROR: input filter %s failed: %v", filter.Name(), err)
			// Continue processing other filters even if one fails
			continue
		}

		filterDuration := time.Since(filterStart)
		p.logDebugf("input filter %s completed in %dms", filter.Name(), filterDuration.Milliseconds())

		// In enforce mode, stop if any filter blocks the request
		if mode == ModeEnforce && fctx.Block {
			p.logf("BLOCK: input blocked by filter %s: %s", filter.Name(), fctx.BlockReason)
			return fmt.Errorf("request blocked: %s", fctx.BlockReason)
		}
	}

	// Apply modifications if any
	if len(fctx.ModifiedRequestBody) > 0 {
		originalLen := len(fctx.RequestBody)
		modifiedLen := len(fctx.ModifiedRequestBody)
		fctx.RequestBody = fctx.ModifiedRequestBody

		if mode == ModeRedact {
			tokenCount := len(fctx.PIITokens)
			p.logDebugf("redact mode: tokenized %d PII entities (size: %d→%d bytes)",
				tokenCount, originalLen, modifiedLen)
		}
	}

	totalDuration := time.Since(start)
	p.logDebugf("input pipeline completed in %dms (filters=%d, blocked=%v, modified=%v)",
		totalDuration.Milliseconds(), len(filters), fctx.Block, len(fctx.ModifiedRequestBody) > 0)

	return nil
}

// ProcessOutput runs all output filters on the response.
func (p *Pipeline) ProcessOutput(ctx context.Context, fctx *FilterContext) error {
	p.mu.RLock()
	mode := p.mode
	filters := p.outputFilters
	tokenizer := p.tokenizer
	p.mu.RUnlock()

	if mode == ModeDisabled {
		return nil
	}

	fctx.Context = ctx
	fctx.Mode = mode
	fctx.Tokenizer = tokenizer
	if fctx.PIITokens == nil {
		fctx.PIITokens = make(map[string]*PIIToken)
	}
	start := time.Now()

	p.logDebugf("processing output: mode=%s, session=%s, filters=%d", mode, fctx.SessionID, len(filters))

	for _, filter := range filters {
		filterStart := time.Now()

		if err := filter.ApplyOutput(fctx); err != nil {
			p.logf("ERROR: output filter %s failed: %v", filter.Name(), err)
			// Continue processing other filters even if one fails
			continue
		}

		filterDuration := time.Since(filterStart)
		p.logDebugf("output filter %s completed in %dms", filter.Name(), filterDuration.Milliseconds())

		// In enforce mode, stop if any filter blocks the response
		if mode == ModeEnforce && fctx.Block {
			p.logf("BLOCK: output blocked by filter %s: %s", filter.Name(), fctx.BlockReason)
			return fmt.Errorf("response blocked: %s", fctx.BlockReason)
		}
	}

	// Apply modifications if any
	if len(fctx.ModifiedResponseBody) > 0 {
		originalLen := len(fctx.ResponseBody)
		modifiedLen := len(fctx.ModifiedResponseBody)
		fctx.ResponseBody = fctx.ModifiedResponseBody

		if mode == ModeRedact {
			if detokenized, ok := fctx.Annotations["pii_detokenized"].(bool); ok && detokenized {
				p.logDebugf("redact mode: detokenized response (size: %d→%d bytes)",
					originalLen, modifiedLen)
			}
		}
	}

	totalDuration := time.Since(start)
	p.logDebugf("output pipeline completed in %dms (filters=%d, blocked=%v, modified=%v)",
		totalDuration.Milliseconds(), len(filters), fctx.Block, len(fctx.ModifiedResponseBody) > 0)

	return nil
}

// Stats returns statistics about the pipeline.
func (p *Pipeline) Stats() map[string]any {
	p.mu.RLock()
	defer p.mu.RUnlock()

	inputFilterNames := make([]string, len(p.inputFilters))
	for i, f := range p.inputFilters {
		inputFilterNames[i] = f.Name()
	}

	outputFilterNames := make([]string, len(p.outputFilters))
	for i, f := range p.outputFilters {
		outputFilterNames[i] = f.Name()
	}

	return map[string]any{
		"mode":           string(p.mode),
		"input_filters":  inputFilterNames,
		"output_filters": outputFilterNames,
		"total_filters":  len(p.inputFilters) + len(p.outputFilters),
	}
}

func (p *Pipeline) logf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Printf("[firewall] "+format, args...)
	}
}

func (p *Pipeline) logDebugf(format string, args ...any) {
	if p.logger != nil {
		p.logger.Printf("[firewall.debug] "+format, args...)
	}
}

// InputFilters returns the list of input filters.
func (p *Pipeline) InputFilters() []InputFilter {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.inputFilters
}

// OutputFilters returns the list of output filters.
func (p *Pipeline) OutputFilters() []OutputFilter {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.outputFilters
}
