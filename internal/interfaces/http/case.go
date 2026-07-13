package api

import (
	"net/http"
	"strconv"
	"strings"

	"adp/internal/domain/model"
)

func (s *Server) handleListIncidentCases(w http.ResponseWriter, r *http.Request) {
	filter := model.IncidentCaseFilter{
		Query:       strings.TrimSpace(r.URL.Query().Get("q")),
		TriggerType: strings.TrimSpace(r.URL.Query().Get("trigger_type")),
		FaultType:   strings.TrimSpace(r.URL.Query().Get("fault_type")),
		Limit:       parsePositiveInt(r.URL.Query().Get("limit")),
	}

	writeJSON(w, http.StatusOK, s.store.ListIncidentCases(filter))
}

func (s *Server) handleSuggestIncidentCases(w http.ResponseWriter, r *http.Request) {
	description := strings.TrimSpace(r.URL.Query().Get("description"))
	triggerType := strings.TrimSpace(r.URL.Query().Get("trigger_type"))
	faultType := strings.TrimSpace(r.URL.Query().Get("fault_type"))
	limit := parsePositiveInt(r.URL.Query().Get("limit"))
	if limit == 0 {
		limit = 3
	}

	cases := s.store.FindSimilarIncidentCases(description, triggerType, faultType, limit)
	writeJSON(w, http.StatusOK, map[string]any{
		"reference_cases":  cases,
		"historical_hints": buildHistoricalHints(cases),
	})
}

func buildHistoricalHints(cases []model.IncidentCase) []string {
	hints := make([]string, 0, len(cases))
	for _, incidentCase := range cases {
		if len(incidentCase.Suggestions) == 0 {
			continue
		}
		hints = append(hints, incidentCase.Title+": "+incidentCase.Suggestions[0])
	}
	return hints
}

func parsePositiveInt(value string) int {
	if value == "" {
		return 0
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0
	}
	return parsed
}
