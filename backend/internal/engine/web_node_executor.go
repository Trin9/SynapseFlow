package engine

// web_node_executor.go — NodeExecutor for NodeTypeWebInteraction (Sprint 7).
//
// Wires a WebInteractionExecutor (chromedp or future browser-use backend)
// into the DAG scheduler.  Node.Config must contain:
//
//   actions   []WebAction   — ordered list of browser operations
//   episode_id  string        — if set, each action result is written as a
//                               fact evidence entry via EpisodeWriter
//   continue_on_error bool    — if true, execution continues after a failed action
//
// After all actions complete the serialised []WebPageState is written to
// GlobalState under the key "web.<nodeID>.pages".

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/Trin9/SynapseFlow/backend/pkg/models"
)

// WebNodeExecutor implements engine.NodeExecutor for web_interaction nodes.
type WebNodeExecutor struct {
	// Browser is the underlying browser backend (chromedp or mock).
	// If nil, a new ChromeDPExecutor is created on first Execute call.
	Browser WebInteractionExecutor

	// Writer is used to persist evidence when the node config provides
	// an episode_id.  May be nil if episode tracking is not needed.
	Writer *EpisodeWriter
}

// Execute implements NodeExecutor.
func (ex *WebNodeExecutor) Execute(ctx context.Context, node models.Node, state *models.GlobalState) models.NodeResult {
	start := time.Now()
	result := models.NodeResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		NodeType: node.Type,
		Status:   "error",
	}

	// --- parse config ---------------------------------------------------
	actions, err := parseWebActions(node.Config)
	if err != nil {
		result.Error = fmt.Sprintf("web_node_executor: parse actions: %v", err)
		result.Duration = time.Since(start)
		return result
	}
	if len(actions) == 0 {
		result.Error = "web_node_executor: no actions configured"
		result.Duration = time.Since(start)
		return result
	}

	episodeID := configString(node.Config, "episode_id")
	continueOnErr, _ := node.Config["continue_on_error"].(bool)

	// --- resolve browser backend ----------------------------------------
	browser := ex.Browser
	if browser == nil {
		browser = NewChromeDPExecutor()
	}

	// Render template placeholders in action values using GlobalState.
	for i := range actions {
		actions[i].URL = RenderTemplate(actions[i].URL, state)
		actions[i].Value = RenderTemplate(actions[i].Value, state)
		actions[i].Script = RenderTemplate(actions[i].Script, state)
	}

	// --- execute actions ------------------------------------------------
	var allStates []WebPageState
	var execErr error

	if continueOnErr {
		for _, action := range actions {
			pages, err := browser.ExecuteActions(ctx, []WebAction{action})
			allStates = append(allStates, pages...)
			if err != nil && execErr == nil {
				execErr = err // record first error but keep going
			}
		}
	} else {
		allStates, execErr = browser.ExecuteActions(ctx, actions)
	}

	// --- write evidence if episode tracking is enabled ------------------
	if episodeID != "" && ex.Writer != nil {
		for i, ps := range allStates {
			label := fmt.Sprintf("web_action[%d]:%s", i, actions[i].Type)
			content := pageStateToJSON(ps)
			if actions[i].ResultKey != "" {
				label = actions[i].ResultKey
			}
			_ = ex.Writer.AppendFact(ctx, episodeID, node.ID, node.Type, label, content)
		}
	}

	// --- write page states to GlobalState --------------------------------
	stateKey := fmt.Sprintf("web.%s.pages", node.ID)
	statesJSON := pageStatesToJSON(allStates)
	state.Set(stateKey, statesJSON)

	// Also write individual result keys into GlobalState.
	for i, ps := range allStates {
		if i < len(actions) && actions[i].ResultKey != "" {
			state.Set(actions[i].ResultKey, ps.TextResult)
		}
	}

	result.Duration = time.Since(start)
	result.Output = statesJSON

	if execErr != nil {
		result.Error = execErr.Error()
		return result
	}
	result.Status = "success"
	return result
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func parseWebActions(cfg map[string]interface{}) ([]WebAction, error) {
	raw, ok := cfg["actions"]
	if !ok {
		return nil, nil
	}
	b, err := json.Marshal(raw)
	if err != nil {
		return nil, err
	}
	var actions []WebAction
	if err := json.Unmarshal(b, &actions); err != nil {
		return nil, err
	}
	return actions, nil
}

func configString(cfg map[string]interface{}, key string) string {
	if cfg == nil {
		return ""
	}
	v, _ := cfg[key].(string)
	return v
}

func pageStateToJSON(ps WebPageState) string {
	b, _ := json.Marshal(ps)
	return string(b)
}

func pageStatesToJSON(pages []WebPageState) string {
	b, _ := json.Marshal(pages)
	return string(b)
}

// compile-time interface check
var _ NodeExecutor = (*WebNodeExecutor)(nil)
