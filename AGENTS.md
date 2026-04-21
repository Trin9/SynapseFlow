# Agentic Design Principles (AGENTS.md)

This document serves as the "constitution" for all Large Language Model (LLM) behavior design, prompt architecture, and graph flow control within the **Synapse** platform.
If you are an AI developer or architect taking over the platform in the future, please strictly adhere to the following principles when designing any new node flows, Prompt templates, or MCP tools.

## 1. Core Paradigm: Flow Engineering > Prompt Engineering

We have completely abandoned the traditional Prompt Skill approach (such as the so-called Panic Analyzer), which relies on "thousands of words of natural language constraints, letting the large model call tools freely throughout the process." Large models are extremely expensive, slow, and prone to fatal hallucinations.

**In Synapse:**
* The LLM is a **decision node**, not a free-form global **scheduler**.
* The scheduler is the robust Golang DAG Engine (Directed Acyclic Graph Engine), which enforces allowed actions, routing constraints, and loop limits.

This distinction is important:

1. The LLM may decide **which bounded next action** should happen, such as whether to search code, search logs, request a snippet, or emit a report.
2. The LLM must not become an unconstrained tool-calling agent that invents its own execution surface at runtime.
3. The engine remains responsible for actual scheduling, execution, state transitions, circuit breaking, and human handoff.

## 2. Node Dualism: Hard Nodes vs. Soft Nodes

All nodes defined on the canvas are strictly divided into two categories:

### 2.1 Hard Nodes (Muscles and Limbs)
* **Essence**: Do not call any LLMs; 100% deterministic code execution.
* **Scope**: Executing Bash scripts, initiating MCP HTTP queries (querying DB/ES logs), running preset Python checking scripts, etc.
* **Rule**: Hard nodes execute deterministic data collection or actions. In some flows they may run eagerly when the alert signature already determines the first-round evidence; in other flows they may run only after a soft node emits a structured retrieval plan. Hard nodes are execution surfaces, not reasoning surfaces.

### 2.2 Soft Nodes (Brain and Senses)
* **Essence**: Call LLM APIs (like Claude/GPT) to execute complex pattern recognition and reasoning.
* **Trigger Timing**: Soft nodes may run either after baseline evidence collection or immediately after an incident packet arrives, depending on whether the flow is an analysis flow or a bounded planning loop.
* **Operational Modes**:
  1. **Planner / Router Mode**: Given the incident packet and current state, decide the next bounded action and emit structured fields such as action type, search scope, query, or completion status.
  2. **Analysis Mode**: Given sufficient collected facts, perform root-cause analysis, attribution, summarization, or report generation.
* **Prompt Design Principle**: A soft node may ask for the **next bounded retrieval step**, but it must not be asked to act as an unconstrained tool-calling agent. The correct pattern is: "Based on the current state, choose one allowed next action and emit strict JSON," not "freely explore the system and decide what to do however you want."
* **Structured Output**: Soft nodes must forcefully output standardized JSON (such as next action, search parameters, conclusion type, root cause details, whether data is missing, etc.) so that the engine can proceed with conditional routing.

### 2.3 Automation Boundaries: Strict Separation of Duties between WebMCP and LLMs
When introducing **WebMCP** for UI frontend automation testing, we must strictly draw the boundary between the machine (browser) and the AI (LLM) to prevent "using a sledgehammer to crack a nut."
* **❌ Strictly Forbidden for LLMs (Execution Layer)**: Searching for elements on a page, identifying buttons, inputting text, switching tabs. These actions are extremely fragile and expensive. They must be solidified into 100% deterministic **Hard Nodes** by intercepting underlying WebMCP semantic tool calls, and be replayed at light speed by the engine via the CDP protocol.
* **✅ Mandatory for LLMs (Decision & Extension Layer)**:
  1. **Complex Assertion Verification**: Mounted as a **Soft Node** at the end of a flow, receiving page state data returned by WebMCP (not screenshots), to analyze "whether this business process (e.g., shipping) was successful."
  2. **Traceability and Summarization**: When a flow errors out, analyzing and attributing the cause, writing test reports or troubleshooting documents.
  3. **Dynamic Traversal Routing**: Acting as a `Router Node` at certain business branches, dynamically generating different input parameters based on the current system feedback (e.g., trying different approval flows, iterating over different sites) to drive the execution of downstream WebMCP nodes.

## 3. Finite State Machine & Circuit Breaker

Traditional DAGs flow in a single direction, but real-world troubleshooting often requires loops like "insufficient clues -> reflect -> change keywords and query again."

**Synapse's Graph Loop Control Specifications:**
1. **Unified `Global State` Pool**: Maintain a unified state object throughout the entire graph execution. The outputs of soft nodes (LLMs) (e.g., finding that the initial TraceID query didn't yield full logs, requiring a query based on a newly associated OrderID) will be appended/patched to the `State`.
2. **State-based Conditional Routing (Router Edges)**: The JSON exported from the frontend must specify routes such as: "If the soft node outputs `next_action == search_code`, jump to the code-search hard node," or "If `missing_info == true`, loop back to another bounded evidence-collection node." When the next hard node is triggered again, it reads new search parameters from the updated `State`, effectively preventing an infinite loop using the same conditions.
3. **Circuit Breaker**: The underlying engine must implicitly inject a `loop_count` for every graph that has a loop circuit. If a certain loop executes more than N times (e.g., 3 times), it must forcefully break out of the loop, automatically jump to a **"Human-in-the-loop"** node, and suspend the state machine waiting for SRE intervention. This prevents the LLM from entering a "hallucination deadlock" and burning through the billing limit.

## 4. Dual-Loop Memory Bank (The Shadow Memory Agent)

Drawing inspiration from the RAG loop design in top open-source projects, Synapse absolutely forbids "hardcoding" and cramming all troubleshooting experiences into the System Prompt of the main process.

### 4.1 Active Memory (Closed-Loop Extraction)
After a troubleshooting session concludes and the conclusion is manually verified, the engine will spin up a completely independent background **Shadow Extraction Agent**.
* **Its Duty**: Read the complete "Global State" of this troubleshooting session (the paths taken, the features acquired, the manual modifications), strip away irrelevant noise, and extract pure "alert features and troubleshooting path templates".
* **Its Permissions**: Extremely restricted (Sandbox environment). It can only use write-permission MCP tools to store this structured experience (Markdown/JSON) into a designated knowledge base (Vector DB) or memory folder. It is forbidden to run any external network or environment queries.

### 4.2 Passive Recall (Execution Preloading)
When a new alert event triggers the starting point of the troubleshooting graph, the system adds a parallel "Experience Recall Node" to the first set of hard nodes.
Using similarity search (based on log error stack features or keywords), it instantly retrieves the previously written "troubleshooting topology paths and precautions" from the vector database. This is passed as additional Context (an external brain) alongside the subsequent analysis nodes. This realizes **Feature-Precise Routing**, completely solving the problem of LLM "memory forgetting" when reusing complex business experiences.

---
*By adhering to these design principles, Synapse can truly become a new generation of enterprise-level AI infrastructure with high availability, low Token costs, and "long-term evolving memory".*
