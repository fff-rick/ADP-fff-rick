package api

import "net/http"

func (s *Server) handleListAuditLogs(w http.ResponseWriter, r *http.Request) {
	resourceType := r.URL.Query().Get("resource_type")
	resourceID := r.URL.Query().Get("resource_id")
	writeJSON(w, http.StatusOK, s.store.ListAuditLogs(resourceType, resourceID))
}

func (s *Server) recordAudit(actorType, actorID, action, resourceType, resourceID string, details map[string]any) {
	s.store.AddAuditLog(actorType, actorID, action, resourceType, resourceID, details)

	fields := map[string]any{
		"actor_id":      actorID,
		"actor_type":    actorType,
		"resource_id":   resourceID,
		"resource_type": resourceType,
	}
	for key, value := range details {
		fields[key] = value
	}
	logEvent("audit", action, fields)
}
