package engine

// web_executor.go — WebInteractionExecutor abstraction (Sprint 7).
//
// Design (from AGENTS.md §2.3):
//   All UI-automation actions are 100% deterministic Hard Node operations.
//   LLMs must NEVER search for elements, click buttons, or fill forms.
//   This file defines the interface + domain types so that:
//     • ChromeDPExecutor (browser_chromedp.go) implements the interface for
//       production use via the CDP protocol.
//     • Any future browser-use MCP server sidecar can implement the same
//       interface without touching WebNodeExecutor.

import "context"

// ---------------------------------------------------------------------------
// Action types
// ---------------------------------------------------------------------------

// WebActionType is the kind of browser operation to perform.
type WebActionType string

const (
	WebActionNavigate    WebActionType = "navigate"     // load a URL
	WebActionClick       WebActionType = "click"        // click an element
	WebActionFill        WebActionType = "fill"         // type text into an input
	WebActionSelect      WebActionType = "select"       // choose a <select> option
	WebActionWaitVisible WebActionType = "wait_visible" // poll until element visible
	WebActionWaitHidden  WebActionType = "wait_hidden"  // poll until element gone
	WebActionScreenshot  WebActionType = "screenshot"   // capture PNG (base64)
	WebActionEvalJS      WebActionType = "eval_js"      // run a JS expression
	WebActionExtractText WebActionType = "extract_text" // read innerText of element
	WebActionExtractAOM  WebActionType = "extract_aom"  // capture AOM snapshot
)

// WebAction is a single deterministic browser operation.
// Selectors should be CSS selectors or XPath (prefixed "xpath=").
type WebAction struct {
	Type   WebActionType `json:"type"`
	URL    string        `json:"url,omitempty"`    // WebActionNavigate
	Sel    string        `json:"sel,omitempty"`    // CSS / XPath selector
	Value  string        `json:"value,omitempty"`  // WebActionFill / Select
	Script string        `json:"script,omitempty"` // WebActionEvalJS
	// ResultKey is the GlobalState key under which the action result is stored.
	// If empty the result is discarded.
	ResultKey string `json:"result_key,omitempty"`
	// TimeoutMS overrides the per-action timeout (default: 10 000 ms).
	TimeoutMS int `json:"timeout_ms,omitempty"`
}

// ---------------------------------------------------------------------------
// Page state (AOM)
// ---------------------------------------------------------------------------

// AOMNode is a lightweight node in the Accessibility Object Model tree.
// Only semantic fields are captured; raw HTML is stripped to reduce token
// cost when the page state is passed to a downstream Soft Node.
type AOMNode struct {
	Role     string    `json:"role"`
	Name     string    `json:"name,omitempty"`
	Value    string    `json:"value,omitempty"`
	Checked  *bool     `json:"checked,omitempty"`
	Disabled bool      `json:"disabled,omitempty"`
	Children []AOMNode `json:"children,omitempty"`
}

// WebPageState is returned after every WebAction execution.
// It is written to GlobalState so that downstream Soft Nodes can reason
// about "was the action successful?" without touching the browser.
type WebPageState struct {
	URL        string   `json:"url"`
	Title      string   `json:"title"`
	AOM        *AOMNode `json:"aom,omitempty"`         // from WebActionExtractAOM
	Screenshot string   `json:"screenshot,omitempty"`  // base64 PNG
	TextResult string   `json:"text_result,omitempty"` // from extract_text / eval_js
	Error      string   `json:"error,omitempty"`
}

// ---------------------------------------------------------------------------
// Interface
// ---------------------------------------------------------------------------

// WebInteractionExecutor is the port that browser backends implement.
// It executes a sequence of WebActions against a real browser and returns
// the accumulated page states.  It is intentionally simple so that both
// the chromedp backend and a future browser-use MCP backend can satisfy it.
type WebInteractionExecutor interface {
	// ExecuteActions runs actions in order, returning one WebPageState per
	// action.  Execution stops on the first error unless ContinueOnError is set
	// in the node's Config.
	ExecuteActions(ctx context.Context, actions []WebAction) ([]WebPageState, error)
}
