package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Trin9/SynapseFlow/backend/internal/auth"
	"github.com/Trin9/SynapseFlow/backend/internal/engine"
	"github.com/Trin9/SynapseFlow/backend/pkg/models"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func generateID() string {
	return uuid.New().String()
}

func writeError(c *gin.Context, status int, code string, message string, details interface{}) {
	c.JSON(status, apiError{
		Error:   message,
		Code:    code,
		Details: details,
	})
}

func (s *Server) validateDAGMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		dag := new(models.DAGConfig)
		if err := c.ShouldBindJSON(dag); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_json", "invalid JSON", err.Error())
			c.Abort()
			return
		}

		if _, err := engine.ParseDAG(dag); err != nil {
			writeError(c, http.StatusBadRequest, "invalid_dag", "invalid DAG", err.Error())
			c.Abort()
			return
		}

		c.Set("validated_dag", dag)
		c.Next()
	}
}

func getValidatedDAG(c *gin.Context) (*models.DAGConfig, bool) {
	v, ok := c.Get("validated_dag")
	if !ok {
		writeError(c, http.StatusInternalServerError, "internal", "internal error", "validated DAG missing")
		return nil, false
	}
	dag, ok := v.(*models.DAGConfig)
	if !ok || dag == nil {
		writeError(c, http.StatusInternalServerError, "internal", "internal error", "validated DAG wrong type")
		return nil, false
	}
	return dag, true
}

// parseAPIKeys parses the SYNAPSE_API_KEYS environment variable.
// Format: comma-separated "key:role:subject" triples.
func parseAPIKeys(raw string) map[string]*auth.Identity {
	out := make(map[string]*auth.Identity)
	if raw == "" {
		return out
	}
	for _, entry := range strings.Split(raw, ",") {
		entry = strings.TrimSpace(entry)
		parts := strings.SplitN(entry, ":", 3)
		if len(parts) != 3 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		role := auth.Role(strings.ToLower(strings.TrimSpace(parts[1])))
		subject := strings.TrimSpace(parts[2])
		if key == "" || subject == "" || !auth.IsValidRole(role) {
			continue
		}
		out[key] = &auth.Identity{Subject: subject, Role: role, Mode: "apikey", APIKey: key}
	}
	return out
}

// resolveSlackURL picks the Slack webhook URL for this DAG run.
// DAG metadata key "slack_webhook_url" takes precedence.
func (s *Server) resolveSlackURL(dag *models.DAGConfig) string {
	if dag != nil {
		if url, ok := dag.Metadata["slack_webhook_url"]; ok && url != "" {
			return url
		}
	}
	return s.slackWebhookURL
}

func buildExecutionNotification(exec *models.Execution, dag *models.DAGConfig, duration time.Duration) string {
	status := "unknown"
	if exec != nil {
		status = string(exec.Status)
	}
	message := fmt.Sprintf("*Synapse Execution %s*\nDAG: %s\nStatus: %s\nDuration: %s",
		executionID(exec), dagName(dag, exec), status, duration.Round(time.Millisecond))

	if summary := strings.TrimSpace(alertSummary(exec)); summary != "" {
		message += "\nAlert: " + summary
	}
	if conclusion := strings.TrimSpace(executionConclusion(exec)); conclusion != "" {
		message += "\nConclusion: " + conclusion
	}
	if detailsURL := strings.TrimSpace(executionDetailsURL(dag, exec)); detailsURL != "" {
		message += "\nDetails: " + detailsURL
	}
	if exec != nil && exec.Error != "" {
		message += "\nError: " + exec.Error
	}
	return message
}

func executionID(exec *models.Execution) string {
	if exec == nil || exec.ID == "" {
		return "unknown"
	}
	return exec.ID
}

func dagName(dag *models.DAGConfig, exec *models.Execution) string {
	if dag != nil && dag.Name != "" {
		return dag.Name
	}
	if exec != nil && exec.DAGName != "" {
		return exec.DAGName
	}
	return "unknown"
}

func alertSummary(exec *models.Execution) string {
	if exec == nil || exec.State == nil {
		return ""
	}
	if summary := exec.State.GetString("alert_summary"); summary != "" {
		return summary
	}
	service := exec.State.GetString("service_name")
	alertName := firstNonEmptyString(exec.State.GetString("alert_name"), exec.State.GetString("alert_type"))
	return strings.TrimSpace(strings.TrimSpace(service + " " + alertName))
}

func executionConclusion(exec *models.Execution) string {
	if exec == nil {
		return ""
	}
	for i := len(exec.Results) - 1; i >= 0; i-- {
		result := exec.Results[i]
		if result.Output == "" {
			continue
		}
		conclusion := extractConclusion(result.Output)
		if conclusion != "" {
			return conclusion
		}
	}
	return ""
}

func extractConclusion(output string) string {
	output = strings.TrimSpace(output)
	if output == "" {
		return ""
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(output), &payload); err == nil {
		for _, key := range []string{"root_cause", "summary", "conclusion", "message"} {
			if value, ok := payload[key]; ok {
				if text := strings.TrimSpace(fmt.Sprintf("%v", value)); text != "" {
					return text
				}
			}
		}
	}
	return output
}

func executionDetailsURL(dag *models.DAGConfig, exec *models.Execution) string {
	if exec == nil {
		return ""
	}
	baseURL := ""
	if dag != nil {
		baseURL = firstNonEmptyString(dag.Metadata["execution_details_base_url"], dag.Metadata["frontend_base_url"])
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	if baseURL == "" {
		return ""
	}
	return baseURL + "/executions/" + exec.ID
}
