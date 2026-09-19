// Package api provides REST API handlers for HiveStack HA operations.
package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/maddydevel/HiveStack/internal/auth"
	"github.com/maddydevel/HiveStack/internal/ha"
)

// registerHARoutes mounts HA endpoints on the API server.
func (s *APIServer) registerHARoutes() {
	s.mux.HandleFunc("GET /api/v1/ha/status", auth.RequireAuth(s.handleHAStatus))
	s.mux.HandleFunc("POST /api/v1/ha/failover", auth.RequireAuth(s.handleHAFailover))
	s.mux.HandleFunc("PUT /api/v1/vms/{id}/ha-policy", auth.RequireAuth(s.handleSetHAPolicy))
	s.mux.HandleFunc("GET /api/v1/vms/{id}/ha-policy", auth.RequireAuth(s.handleGetHAPolicy))
	s.mux.HandleFunc("GET /api/v1/ha/nodes", auth.RequireAuth(s.handleListHANodes))
	s.mux.HandleFunc("GET /api/v1/ha/history", auth.RequireAuth(s.handleHAHistory))
}

// handleHAStatus returns the overall HA subsystem status.
func (s *APIServer) handleHAStatus(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	status := map[string]interface{}{
		"enabled":    true,
		"controller": "running",
		"nodes_online": 0,
		"nodes_suspect": 0,
		"nodes_offline": 0,
		"active_failovers": 0,
		"thresholds": map[string]interface{}{
			"heartbeat_interval": "30s",
			"suspect_threshold":  3,
			"offline_threshold":  5,
		},
	}

	if s.haController != nil {
		ctrlStatus := s.haController.GetStatus()
		if v, ok := ctrlStatus["online_nodes"]; ok {
			status["nodes_online"] = v
		}
		if v, ok := ctrlStatus["suspect_nodes"]; ok {
			status["nodes_suspect"] = v
		}
		if v, ok := ctrlStatus["offline_nodes"]; ok {
			status["nodes_offline"] = v
		}
		if v, ok := ctrlStatus["active_failovers"]; ok {
			status["active_failovers"] = v
		}
	}

	s.respondJSON(w, http.StatusOK, status)
}

// handleHAFailover triggers a manual failover for a node.
func (s *APIServer) handleHAFailover(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	var req struct {
		NodeID  string `json:"node_id"`
		Force   bool   `json:"force,omitempty"`
		DryRun  bool   `json:"dry_run,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}
	if req.NodeID == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "node_id is required"})
		return
	}

	if s.haController == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "HA controller not configured"})
		return
	}

	// Check RBAC
	if err := s.rbac.CheckPermission(getRoleFromClaims(claims), "ha", "failover"); err != nil {
		s.respondJSON(w, http.StatusForbidden, map[string]string{"error": fmt.Sprintf("permission denied: %v", err)})
		return
	}

	if req.DryRun {
		s.respondJSON(w, http.StatusOK, map[string]interface{}{
			"status":   "dry_run",
			"node_id":  req.NodeID,
			"message":  "Failover would be triggered (dry run)",
		})
		return
	}

	if err := s.haController.TriggerFailover(context.Background(), req.NodeID); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.respondJSON(w, http.StatusAccepted, map[string]interface{}{
		"status":  "initiated",
		"node_id": req.NodeID,
		"message": "Failover initiated",
	})
}

// handleSetHAPolicy sets the HA policy for a VM.
func (s *APIServer) handleSetHAPolicy(w http.ResponseWriter, r *http.Request) {
	claims, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	vmID := r.PathValue("id")
	if vmID == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "VM ID is required"})
		return
	}

	var req struct {
		Mode         string   `json:"mode"`
		Priority     int      `json:"priority"`
		AntiAffinity []string `json:"anti_affinity"`
		MaxRestarts  int      `json:"max_restarts"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
		return
	}

	// Check RBAC
	if err := s.rbac.CheckPermission(getRoleFromClaims(claims), "vm", "update"); err != nil {
		s.respondJSON(w, http.StatusForbidden, map[string]string{"error": fmt.Sprintf("permission denied: %v", err)})
		return
	}

	// Verify VM exists
	_, err := s.db.GetVM(r.Context(), vmID)
	if err != nil {
		s.respondJSON(w, http.StatusNotFound, map[string]string{"error": "VM not found"})
		return
	}

	policy := ha.HAPolicy{
		VMID:         vmID,
		Mode:         ha.HAMode(req.Mode),
		Priority:     req.Priority,
		AntiAffinity: req.AntiAffinity,
		MaxRestarts:  req.MaxRestarts,
	}

	if err := policy.Validate(); err != nil {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}

	if s.policyManager == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "HA policy manager not configured"})
		return
	}

	actor := claims.UserID
	if actor == "" {
		actor = "api"
	}
	if err := s.policyManager.SetPolicy(vmID, policy, actor); err != nil {
		s.respondJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	s.respondJSON(w, http.StatusOK, policy)
}

// handleGetHAPolicy returns the HA policy for a VM.
func (s *APIServer) handleGetHAPolicy(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	vmID := r.PathValue("id")
	if vmID == "" {
		s.respondJSON(w, http.StatusBadRequest, map[string]string{"error": "VM ID is required"})
		return
	}

	if s.policyManager == nil {
		s.respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "HA policy manager not configured"})
		return
	}

	policy, ok := s.policyManager.GetPolicy(vmID)
	if !ok {
		// Return default policy
		defaultPolicy := ha.DefaultHAPolicy(vmID)
		s.respondJSON(w, http.StatusOK, defaultPolicy)
		return
	}

	s.respondJSON(w, http.StatusOK, policy)
}

// handleListHANodes returns health status of all monitored nodes.
func (s *APIServer) handleListHANodes(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if s.haController == nil {
		s.respondJSON(w, http.StatusOK, []map[string]interface{}{})
		return
	}

	health := s.haController.GetAllHealth()
	nodes := make([]map[string]interface{}, 0, len(health))
	for id, h := range health {
		nodes = append(nodes, map[string]interface{}{
			"node_id":            id,
			"state":              h.GetState().String(),
			"missed_heartbeats":  h.GetMissedCount(),
			"vms":                h.VMs,
			"resources":          h.Resources,
		})
	}

	s.respondJSON(w, http.StatusOK, nodes)
}

// handleHAHistory returns failover history.
func (s *APIServer) handleHAHistory(w http.ResponseWriter, r *http.Request) {
	_, ok := auth.ClaimsFromContext(r.Context())
	if !ok {
		s.respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	if s.haOrchestrator == nil {
		s.respondJSON(w, http.StatusOK, []map[string]interface{}{})
		return
	}

	limit := 50
	history := s.haOrchestrator.GetFailoverHistory(limit)
	s.respondJSON(w, http.StatusOK, history)
}

// getRoleFromClaims extracts the role from JWT claims.
func getRoleFromClaims(claims *auth.Claims) string {
	if claims == nil {
		return ""
	}
	if len(claims.Scopes) > 0 {
		return claims.Scopes[0]
	}
	return "viewer"
}
