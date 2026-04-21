package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/gin-gonic/gin"
)

type alertPayload struct {
	CommonLabels      map[string]string `json:"commonLabels"`
	CommonAnnotations map[string]string `json:"commonAnnotations"`
	Alerts            []struct {
		Labels      map[string]string `json:"labels"`
		Annotations map[string]string `json:"annotations"`
		Status      string            `json:"status"`
	} `json:"alerts"`
}

func (s *Server) handleWebhookAlert(c *gin.Context) {
	var payload alertPayload
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeError(c, http.StatusBadRequest, "invalid_json", "invalid JSON", err.Error())
		return
	}

	labels := payload.CommonLabels
	if len(payload.Alerts) > 0 {
		labels = mergeStringMaps(labels, payload.Alerts[0].Labels)
	}
	if !shouldAutoAnalyse(labels) {
		c.JSON(http.StatusAccepted, gin.H{
			"status":  "ignored",
			"message": "alert auto analysis disabled by labels",
		})
		return
	}

	dag, ok := s.matchDAGForAlert(labels)
	if !ok {
		writeError(c, http.StatusNotFound, "dag_not_matched", "no DAG matched alert labels", labels)
		return
	}

	annotations := payload.CommonAnnotations
	alertStatus := ""
	if len(payload.Alerts) > 0 {
		annotations = mergeStringMaps(annotations, payload.Alerts[0].Annotations)
		alertStatus = strings.TrimSpace(payload.Alerts[0].Status)
	}

	state := models.NewGlobalState()
	state.Merge(map[string]interface{}{
		"alert_labels":      labels,
		"alert_annotations": annotations,
		"alert_payload":     payload,
		"alert_status":      alertStatus,
		"alert_type":        firstNonEmptyString(labels["alertname"], annotations["summary"]),
		"alert_name":        strings.TrimSpace(labels["alertname"]),
		"service_name":      strings.TrimSpace(labels["service"]),
		"alert_summary":     firstNonEmptyString(annotations["summary"], annotations["description"]),
	})

	exec := s.startExecution(dag, state, "webhook")
	c.JSON(http.StatusAccepted, gin.H{
		"execution_id": exec.ID,
		"status":       exec.Status,
		"dag_id":       dag.ID,
	})
}

func (s *Server) matchDAGForAlert(labels map[string]string) (*models.DAGConfig, bool) {
	if dagID := strings.TrimSpace(labels["dag_id"]); dagID != "" {
		dag, err := s.dags.Get(context.Background(), dagID)
		return dag, err == nil
	}

	dags, err := s.dags.List(context.Background())
	if err != nil {
		return nil, false
	}
	service := strings.TrimSpace(labels["service"])
	alertName := strings.TrimSpace(labels["alertname"])
	for _, dag := range dags {
		if dag == nil {
			continue
		}
		if dag.Metadata["service"] == service && dag.Metadata["alertname"] == alertName {
			return dag, true
		}
	}
	return nil, false
}

func shouldAutoAnalyse(labels map[string]string) bool {
	value := strings.ToLower(strings.TrimSpace(labels["auto_analyse"]))
	if value == "" {
		return true
	}
	switch value {
	case "1", "true", "yes", "y", "on":
		return true
	default:
		return false
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func mergeStringMaps(base map[string]string, extra map[string]string) map[string]string {
	out := make(map[string]string, len(base)+len(extra))
	for key, value := range base {
		out[key] = value
	}
	for key, value := range extra {
		out[key] = value
	}
	return out
}
