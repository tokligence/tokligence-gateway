package httpserver

import (
	"net/http"

	"github.com/tokligence/tokligence-gateway/internal/httpserver/protocol"
)

// timeRulesEndpoint provides endpoints for viewing time-based rule status
type timeRulesEndpoint struct {
	server *Server
}

func newTimeRulesEndpoint(server *Server) protocol.Endpoint {
	return &timeRulesEndpoint{server: server}
}

func (e *timeRulesEndpoint) Name() string { return "time_rules" }

func (e *timeRulesEndpoint) Routes() []protocol.EndpointRoute {
	return []protocol.EndpointRoute{
		{Method: http.MethodGet, Path: "/admin/time-rules/status", Handler: http.HandlerFunc(e.server.HandleGetTimeRulesStatus)},
		{Method: http.MethodPost, Path: "/admin/time-rules/apply", Handler: http.HandlerFunc(e.server.HandleApplyTimeRules)},
		{Method: http.MethodPost, Path: "/admin/time-rules/reload", Handler: http.HandlerFunc(e.server.HandleReloadTimeRules)},
	}
}

// HandleGetTimeRulesStatus returns the status of all time-based rules
func (s *Server) HandleGetTimeRulesStatus(w http.ResponseWriter, r *http.Request) {
	if s.ruleEngine == nil || !s.ruleEngine.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "Time-based rules are not enabled",
		})
		return
	}

	// Get all rules (active and inactive)
	allRules := s.ruleEngine.GetAllRules()

	response := map[string]interface{}{
		"enabled": s.ruleEngine.IsEnabled(),
		"count":   len(allRules),
		"rules":   allRules,
	}

	s.respondJSON(w, http.StatusOK, response)
}

// HandleApplyTimeRules manually triggers rule evaluation
func (s *Server) HandleApplyTimeRules(w http.ResponseWriter, r *http.Request) {
	if s.ruleEngine == nil || !s.ruleEngine.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "Time-based rules are not enabled",
		})
		return
	}

	// Trigger immediate rule application
	if err := s.ruleEngine.ApplyRulesNow(); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   http.StatusText(http.StatusInternalServerError),
			Message: "Failed to apply rules: " + err.Error(),
		})
		return
	}

	// Get active rules after application
	activeRules := s.ruleEngine.GetActiveRules()

	response := map[string]interface{}{
		"message":      "Rules applied successfully",
		"active_count": len(activeRules),
		"active_rules": activeRules,
	}

	s.respondJSON(w, http.StatusOK, response)
}

// HandleReloadTimeRules manually reloads rules from config file
func (s *Server) HandleReloadTimeRules(w http.ResponseWriter, r *http.Request) {
	if s.ruleEngine == nil || !s.ruleEngine.IsEnabled() {
		s.respondJSON(w, http.StatusNotImplemented, ErrorResponse{
			Error:   http.StatusText(http.StatusNotImplemented),
			Message: "Time-based rules are not enabled",
		})
		return
	}

	// Reload config from file
	if err := s.ruleEngine.ReloadFromFile(); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, ErrorResponse{
			Error:   http.StatusText(http.StatusInternalServerError),
			Message: "Failed to reload config: " + err.Error(),
		})
		return
	}

	// Get all rules after reload
	allRules := s.ruleEngine.GetAllRules()
	activeRules := s.ruleEngine.GetActiveRules()

	response := map[string]interface{}{
		"message":      "Config reloaded successfully",
		"total_count":  len(allRules),
		"active_count": len(activeRules),
		"rules":        allRules,
	}

	s.respondJSON(w, http.StatusOK, response)
}
