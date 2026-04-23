package engine

// browser_chromedp.go — ChromeDPExecutor implements WebInteractionExecutor
// using the chromedp library (Go-native CDP client).
//
// Governed by AGENTS.md §2.3:
//   Hard Nodes execute deterministic browser actions via CDP.
//   LLMs never touch the browser directly.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"

	"github.com/chromedp/cdproto/accessibility"
	"github.com/chromedp/chromedp"
)

// defaultActionTimeout is used when WebAction.TimeoutMS is not set.
const defaultActionTimeout = 10 * time.Second

// ChromeDPExecutor is a production WebInteractionExecutor backed by chromedp.
// A single allocator is created per executor; callers share browser sessions
// within the same execution context to avoid the overhead of launching a new
// Chrome process per node.
type ChromeDPExecutor struct {
	// AllocCtx is the chromedp allocator context.  If nil, a new headless
	// Chrome instance will be launched on first use.
	AllocCtx context.Context
	// AllocCancel should be called when the executor is no longer needed.
	AllocCancel context.CancelFunc
}

// NewChromeDPExecutor launches a headless Chrome allocator and returns a
// ready-to-use ChromeDPExecutor.  Call executor.AllocCancel() when done.
func NewChromeDPExecutor(opts ...chromedp.ExecAllocatorOption) *ChromeDPExecutor {
	allOpts := append(chromedp.DefaultExecAllocatorOptions[:], opts...)
	allocCtx, allocCancel := chromedp.NewExecAllocator(context.Background(), allOpts...)
	return &ChromeDPExecutor{AllocCtx: allocCtx, AllocCancel: allocCancel}
}

// ExecuteActions implements WebInteractionExecutor.
func (e *ChromeDPExecutor) ExecuteActions(ctx context.Context, actions []WebAction) ([]WebPageState, error) {
	browserCtx, cancel := chromedp.NewContext(e.AllocCtx)
	defer cancel()

	states := make([]WebPageState, 0, len(actions))

	for _, action := range actions {
		state, err := e.runAction(ctx, browserCtx, action)
		if err != nil {
			state.Error = err.Error()
			states = append(states, state)
			return states, fmt.Errorf("chromedp action %s failed: %w", action.Type, err)
		}
		states = append(states, state)
	}
	return states, nil
}

func (e *ChromeDPExecutor) runAction(ctx context.Context, browserCtx context.Context, action WebAction) (WebPageState, error) {
	timeout := defaultActionTimeout
	if action.TimeoutMS > 0 {
		timeout = time.Duration(action.TimeoutMS) * time.Millisecond
	}
	actCtx, cancel := context.WithTimeout(browserCtx, timeout)
	defer cancel()

	var state WebPageState

	switch action.Type {
	case WebActionNavigate:
		var currentURL, title string
		if err := chromedp.Run(actCtx,
			chromedp.Navigate(action.URL),
			chromedp.Location(&currentURL),
			chromedp.Title(&title),
		); err != nil {
			return state, err
		}
		state.URL = currentURL
		state.Title = title

	case WebActionClick:
		var currentURL, title string
		if err := chromedp.Run(actCtx,
			chromedp.Click(action.Sel, chromedp.ByQuery),
			chromedp.Location(&currentURL),
			chromedp.Title(&title),
		); err != nil {
			return state, err
		}
		state.URL = currentURL
		state.Title = title

	case WebActionFill:
		var currentURL string
		if err := chromedp.Run(actCtx,
			chromedp.Clear(action.Sel, chromedp.ByQuery),
			chromedp.SendKeys(action.Sel, action.Value, chromedp.ByQuery),
			chromedp.Location(&currentURL),
		); err != nil {
			return state, err
		}
		state.URL = currentURL

	case WebActionSelect:
		var currentURL string
		if err := chromedp.Run(actCtx,
			chromedp.SetValue(action.Sel, action.Value, chromedp.ByQuery),
			chromedp.Location(&currentURL),
		); err != nil {
			return state, err
		}
		state.URL = currentURL

	case WebActionWaitVisible:
		if err := chromedp.Run(actCtx,
			chromedp.WaitVisible(action.Sel, chromedp.ByQuery),
		); err != nil {
			return state, err
		}

	case WebActionWaitHidden:
		if err := chromedp.Run(actCtx,
			chromedp.WaitNotVisible(action.Sel, chromedp.ByQuery),
		); err != nil {
			return state, err
		}

	case WebActionScreenshot:
		var buf []byte
		if err := chromedp.Run(actCtx,
			chromedp.FullScreenshot(&buf, 90),
		); err != nil {
			return state, err
		}
		state.Screenshot = base64.StdEncoding.EncodeToString(buf)

	case WebActionEvalJS:
		var result interface{}
		if err := chromedp.Run(actCtx,
			chromedp.Evaluate(action.Script, &result),
		); err != nil {
			return state, err
		}
		b, _ := json.Marshal(result)
		state.TextResult = string(b)

	case WebActionExtractText:
		var text string
		if err := chromedp.Run(actCtx,
			chromedp.Text(action.Sel, &text, chromedp.ByQuery),
		); err != nil {
			return state, err
		}
		state.TextResult = text

	case WebActionExtractAOM:
		var nodes []*accessibility.Node
		if err := chromedp.Run(actCtx,
			chromedp.ActionFunc(func(ctx context.Context) error {
				tree, err := accessibility.GetFullAXTree().Do(ctx)
				if err != nil {
					return err
				}
				nodes = tree
				return nil
			}),
		); err != nil {
			return state, err
		}
		if len(nodes) > 0 {
			state.AOM = convertAOMTree(nodes, nodes[0])
		}

	default:
		return state, fmt.Errorf("unknown web action type: %s", action.Type)
	}

	// Capture current URL for all actions that don't do it inline.
	if state.URL == "" {
		_ = chromedp.Run(actCtx, chromedp.Location(&state.URL))
	}
	return state, nil
}

// aomValueString extracts the string representation of an accessibility.Value.
// The Value.Value field is jsontext.Value (raw JSON bytes), so we strip outer
// quotes if present.
func aomValueString(v *accessibility.Value) string {
	if v == nil {
		return ""
	}
	raw := string(v.Value)
	if len(raw) >= 2 && raw[0] == '"' && raw[len(raw)-1] == '"' {
		return raw[1 : len(raw)-1]
	}
	return raw
}

// convertAOMTree converts a flat chromedp accessibility node list into the
// AOMNode tree used by downstream Soft Nodes.  root is the starting node.
func convertAOMTree(all []*accessibility.Node, root *accessibility.Node) *AOMNode {
	if root == nil {
		return nil
	}
	node := &AOMNode{
		Role:  aomValueString(root.Role),
		Name:  aomValueString(root.Name),
		Value: aomValueString(root.Value),
	}
	// Scan Properties for checked / disabled states.
	for _, prop := range root.Properties {
		if prop == nil {
			continue
		}
		switch prop.Name {
		case accessibility.PropertyNameChecked:
			if aomValueString(prop.Value) == "true" {
				t := true
				node.Checked = &t
			}
		case accessibility.PropertyNameDisabled:
			if aomValueString(prop.Value) == "true" {
				node.Disabled = true
			}
		}
	}
	// Build an ID → node index map for child lookup.
	idxByID := make(map[accessibility.NodeID]int, len(all))
	for i, n := range all {
		idxByID[n.NodeID] = i
	}
	for _, childID := range root.ChildIDs {
		if idx, ok := idxByID[childID]; ok {
			if child := convertAOMTree(all, all[idx]); child != nil {
				node.Children = append(node.Children, *child)
			}
		}
	}
	return node
}

// compile-time interface check
var _ WebInteractionExecutor = (*ChromeDPExecutor)(nil)
