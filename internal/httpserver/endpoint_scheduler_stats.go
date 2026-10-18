package httpserver

import (
	"net/http"
	"strconv"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

type schedulerStatsEndpoint struct {
	server *Server
}

func newSchedulerStatsEndpoint(server *Server) protocol.Endpoint {
	return &schedulerStatsEndpoint{server: server}
}

func (e *schedulerStatsEndpoint) Name() string { return "scheduler_stats" }

func (e *schedulerStatsEndpoint) Routes() []protocol.EndpointRoute {
	return []protocol.EndpointRoute{
		{Method: http.MethodGet, Path: "/admin/scheduler/stats", Handler: http.HandlerFunc(e.server.HandleSchedulerStats)},
		{Method: http.MethodGet, Path: "/admin/scheduler/queues", Handler: http.HandlerFunc(e.server.HandleSchedulerQueues)},
	}
}

// HandleSchedulerStats returns detailed scheduler statistics
func (s *Server) HandleSchedulerStats(w http.ResponseWriter, r *http.Request) {
	if !s.schedulerEnabled || s.schedulerInst == nil {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "Scheduler is disabled",
		})
		return
	}

	// Try to get detailed stats from channel scheduler
	type detailedStatsGetter interface {
		GetDetailedStats() map[string]interface{}
	}

	if detailedScheduler, ok := s.schedulerInst.(detailedStatsGetter); ok {
		stats := detailedScheduler.GetDetailedStats()
		stats["enabled"] = true
		s.respondJSON(w, http.StatusOK, stats)
		return
	}

	// Fallback to basic stats
	type basicStatsGetter interface {
		GetStats() map[string]interface{}
	}

	if basicScheduler, ok := s.schedulerInst.(basicStatsGetter); ok {
		stats := basicScheduler.GetStats()
		stats["enabled"] = true
		s.respondJSON(w, http.StatusOK, stats)
		return
	}

	// No stats available
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"message": "Stats not available for this scheduler implementation",
	})
}

// HandleSchedulerQueues returns busiest queues info
func (s *Server) HandleSchedulerQueues(w http.ResponseWriter, r *http.Request) {
	if !s.schedulerEnabled || s.schedulerInst == nil {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"enabled": false,
			"message": "Scheduler is disabled",
		})
		return
	}

	// Try to get busiest queues from channel scheduler
	type busiestQueuesGetter interface {
		GetBusiestQueues(topN int) interface{}
	}

	if channelScheduler, ok := s.schedulerInst.(busiestQueuesGetter); ok {
		topN := 10 // Default top 10
		if topNStr := r.URL.Query().Get("top"); topNStr != "" {
			if n, err := strconv.Atoi(topNStr); err == nil && n > 0 {
				topN = n
			}
		}

		busiest := channelScheduler.GetBusiestQueues(topN)
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"enabled":        true,
			"top_n":          topN,
			"busiest_queues": busiest,
		})
		return
	}

	// Fallback
	s.respondJSON(w, http.StatusOK, map[string]interface{}{
		"enabled": true,
		"message": "Queue stats not available for this scheduler implementation",
	})
}
