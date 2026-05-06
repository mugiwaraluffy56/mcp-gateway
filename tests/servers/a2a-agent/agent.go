package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"
)

// agent card types

type agentCard struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Version      string            `json:"version"`
	Capabilities agentCapabilities `json:"capabilities"`
	Skills       []agentSkill      `json:"skills"`
}

type agentCapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

type agentSkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputModes  []string `json:"inputModes"`
	OutputModes []string `json:"outputModes"`
}

// JSON-RPC types

type jsonRPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      any             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type jsonRPCResponse struct {
	JSONRPC string  `json:"jsonrpc"`
	ID      any     `json:"id"`
	Result  any     `json:"result,omitempty"`
	Error   *rpcErr `json:"error,omitempty"`
}

type rpcErr struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// A2A task types

type taskSendParams struct {
	ID      string      `json:"id"`
	Message taskMessage `json:"message"`
}

type taskGetParams struct {
	ID string `json:"id"`
}

type taskCancelParams struct {
	ID string `json:"id"`
}

type taskMessage struct {
	Role  string      `json:"role"`
	Parts []taskPart  `json:"parts"`
}

type taskPart struct {
	Type string `json:"type"`
	Text string `json:"text,omitempty"`
}

type taskStatus struct {
	State     string `json:"state"`
	Timestamp string `json:"timestamp"`
}

type taskArtifact struct {
	Parts     []taskPart `json:"parts"`
	Index     int        `json:"index"`
	LastChunk bool       `json:"lastChunk"`
}

type taskResult struct {
	ID       string       `json:"id"`
	Status   taskStatus   `json:"status"`
	Artifacts []taskArtifact `json:"artifacts,omitempty"`
}

// SSE event types

type taskStatusEvent struct {
	ID     string     `json:"id"`
	Status taskStatus `json:"status"`
	Final  bool       `json:"final"`
}

type taskArtifactEvent struct {
	ID       string       `json:"id"`
	Artifact taskArtifact `json:"artifact"`
	Final    bool         `json:"final"`
}

// agent

type agent struct {
	mu    sync.RWMutex
	tasks map[string]*taskResult
}

func newAgent() *agent {
	return &agent{tasks: make(map[string]*taskResult)}
}

var card = agentCard{
	Name:        "test-a2a-agent",
	Description: "Deterministic A2A test agent for MCP Gateway integration tests",
	URL:         "http://a2a-agent.mcp-test.svc.cluster.local",
	Version:     "1.0.0",
	Capabilities: agentCapabilities{
		Streaming:              true,
		PushNotifications:      false,
		StateTransitionHistory: false,
	},
	Skills: []agentSkill{
		{
			ID:          "echo",
			Name:        "Echo",
			Description: "Returns the input message unchanged",
			InputModes:  []string{"text"},
			OutputModes: []string{"text"},
		},
		{
			ID:          "reverse",
			Name:        "Reverse",
			Description: "Returns the input message reversed",
			InputModes:  []string{"text"},
			OutputModes: []string{"text"},
		},
	},
}

func (a *agent) handleCard(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(card)
}

func (a *agent) handleJSONRPC(w http.ResponseWriter, r *http.Request) {
	var req jsonRPCRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeRPCError(w, nil, -32700, "parse error")
		return
	}

	switch req.Method {
	case "tasks/send":
		a.handleTaskSend(w, r, req)
	case "tasks/get":
		a.handleTaskGet(w, req)
	case "tasks/cancel":
		a.handleTaskCancel(w, req)
	default:
		writeRPCError(w, req.ID, -32601, fmt.Sprintf("method not found: %s", req.Method))
	}
}

func (a *agent) handleTaskSend(w http.ResponseWriter, r *http.Request, req jsonRPCRequest) {
	var params taskSendParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	input := extractText(params.Message.Parts)
	result := a.processTask(params.ID, input)

	if strings.Contains(r.Header.Get("Accept"), "text/event-stream") {
		a.streamTask(w, result)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func (a *agent) handleTaskGet(w http.ResponseWriter, req jsonRPCRequest) {
	var params taskGetParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	a.mu.RLock()
	result, ok := a.tasks[params.ID]
	a.mu.RUnlock()

	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func (a *agent) handleTaskCancel(w http.ResponseWriter, req jsonRPCRequest) {
	var params taskCancelParams
	if err := json.Unmarshal(req.Params, &params); err != nil {
		writeRPCError(w, req.ID, -32602, "invalid params")
		return
	}

	a.mu.Lock()
	result, ok := a.tasks[params.ID]
	if ok {
		result.Status = taskStatus{State: "canceled", Timestamp: now()}
		result.Artifacts = nil
	}
	a.mu.Unlock()

	if !ok {
		writeRPCError(w, req.ID, -32001, "task not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      req.ID,
		Result:  result,
	})
}

func (a *agent) processTask(id, input string) *taskResult {
	output := applySkill(input)
	result := &taskResult{
		ID:     id,
		Status: taskStatus{State: "completed", Timestamp: now()},
		Artifacts: []taskArtifact{
			{
				Parts:     []taskPart{{Type: "text", Text: output}},
				Index:     0,
				LastChunk: true,
			},
		},
	}
	a.mu.Lock()
	a.tasks[id] = result
	a.mu.Unlock()
	return result
}

func (a *agent) streamTask(w http.ResponseWriter, result *taskResult) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	writeSSE(w, flusher, "TaskStatusUpdateEvent", taskStatusEvent{
		ID:     result.ID,
		Status: taskStatus{State: "working", Timestamp: now()},
		Final:  false,
	})

	if len(result.Artifacts) > 0 {
		writeSSE(w, flusher, "TaskArtifactUpdateEvent", taskArtifactEvent{
			ID:       result.ID,
			Artifact: result.Artifacts[0],
			Final:    false,
		})
	}

	writeSSE(w, flusher, "TaskStatusUpdateEvent", taskStatusEvent{
		ID:     result.ID,
		Status: taskStatus{State: "completed", Timestamp: now()},
		Final:  true,
	})
}

// applySkill maps input text to output. Prefix "reverse:" reverses the text,
// everything else (including "echo:" prefix or no prefix) echoes it back.
func applySkill(input string) string {
	if after, ok := strings.CutPrefix(input, "reverse:"); ok {
		runes := []rune(after)
		for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
			runes[i], runes[j] = runes[j], runes[i]
		}
		return string(runes)
	}
	if after, ok := strings.CutPrefix(input, "echo:"); ok {
		return after
	}
	return input
}

func extractText(parts []taskPart) string {
	for _, p := range parts {
		if p.Type == "text" {
			return p.Text
		}
	}
	return ""
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, event string, data any) {
	b, _ := json.Marshal(data)
	fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, b)
	flusher.Flush()
}

func writeRPCError(w http.ResponseWriter, id any, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(jsonRPCResponse{
		JSONRPC: "2.0",
		ID:      id,
		Error:   &rpcErr{Code: code, Message: msg},
	})
}

func now() string {
	return time.Now().UTC().Format(time.RFC3339)
}
