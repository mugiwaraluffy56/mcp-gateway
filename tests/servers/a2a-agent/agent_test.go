package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHandleCard(t *testing.T) {
	a := newAgent()
	w := httptest.NewRecorder()
	a.handleCard(w, httptest.NewRequest(http.MethodGet, "/.well-known/agent.json", nil))

	assert.Equal(t, http.StatusOK, w.Code)

	var got agentCard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &got))
	assert.Equal(t, "test-a2a-agent", got.Name)
	assert.Len(t, got.Skills, 2)
	assert.Equal(t, "echo", got.Skills[0].ID)
	assert.Equal(t, "reverse", got.Skills[1].ID)
	assert.True(t, got.Capabilities.Streaming)
}

func TestHandleTaskSend_echo(t *testing.T) {
	a := newAgent()
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"t1","message":{"role":"user","parts":[{"type":"text","text":"echo:hello"}]}}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

	assert.Equal(t, http.StatusOK, w.Code)

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)

	result := toTaskResult(t, resp.Result)
	assert.Equal(t, "t1", result.ID)
	assert.Equal(t, "completed", result.Status.State)
	require.Len(t, result.Artifacts, 1)
	assert.Equal(t, "hello", result.Artifacts[0].Parts[0].Text)
}

func TestHandleTaskSend_reverse(t *testing.T) {
	a := newAgent()
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"t2","message":{"role":"user","parts":[{"type":"text","text":"reverse:hello"}]}}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)

	result := toTaskResult(t, resp.Result)
	assert.Equal(t, "olleh", result.Artifacts[0].Parts[0].Text)
}

func TestHandleTaskSend_noPrefix(t *testing.T) {
	a := newAgent()
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"t3","message":{"role":"user","parts":[{"type":"text","text":"just text"}]}}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	result := toTaskResult(t, resp.Result)
	assert.Equal(t, "just text", result.Artifacts[0].Parts[0].Text)
}

func TestHandleTaskSend_streaming(t *testing.T) {
	a := newAgent()
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"t4","message":{"role":"user","parts":[{"type":"text","text":"echo:stream"}]}}}`
	req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body))
	req.Header.Set("Accept", "text/event-stream")
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, req)

	assert.Equal(t, "text/event-stream", w.Header().Get("Content-Type"))

	events := parseSSEEvents(w.Body.String())
	require.Len(t, events, 3)
	assert.Equal(t, "TaskStatusUpdateEvent", events[0].name)
	assert.Equal(t, "TaskArtifactUpdateEvent", events[1].name)
	assert.Equal(t, "TaskStatusUpdateEvent", events[2].name)

	var working taskStatusEvent
	require.NoError(t, json.Unmarshal([]byte(events[0].data), &working))
	assert.Equal(t, "working", working.Status.State)
	assert.False(t, working.Final)

	var done taskStatusEvent
	require.NoError(t, json.Unmarshal([]byte(events[2].data), &done))
	assert.Equal(t, "completed", done.Status.State)
	assert.True(t, done.Final)
}

func TestHandleTaskGet(t *testing.T) {
	a := newAgent()

	sendBody := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"tg1","message":{"role":"user","parts":[{"type":"text","text":"echo:hi"}]}}}`
	a.handleJSONRPC(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(sendBody)))

	getBody := `{"jsonrpc":"2.0","id":2,"method":"tasks/get","params":{"id":"tg1"}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(getBody)))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)

	result := toTaskResult(t, resp.Result)
	assert.Equal(t, "tg1", result.ID)
	assert.Equal(t, "completed", result.Status.State)
}

func TestHandleTaskGet_notFound(t *testing.T) {
	a := newAgent()
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/get","params":{"id":"missing"}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32001, resp.Error.Code)
}

func TestHandleTaskCancel(t *testing.T) {
	a := newAgent()

	sendBody := `{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"tc1","message":{"role":"user","parts":[{"type":"text","text":"echo:bye"}]}}}`
	a.handleJSONRPC(httptest.NewRecorder(), httptest.NewRequest(http.MethodPost, "/", strings.NewReader(sendBody)))

	cancelBody := `{"jsonrpc":"2.0","id":2,"method":"tasks/cancel","params":{"id":"tc1"}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(cancelBody)))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Nil(t, resp.Error)

	result := toTaskResult(t, resp.Result)
	assert.Equal(t, "canceled", result.Status.State)
}

func TestHandleTaskCancel_notFound(t *testing.T) {
	a := newAgent()
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/cancel","params":{"id":"missing"}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32001, resp.Error.Code)
}

func TestUnknownMethod(t *testing.T) {
	a := newAgent()
	body := `{"jsonrpc":"2.0","id":1,"method":"tasks/subscribe","params":{}}`
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader(body)))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32601, resp.Error.Code)
}

func TestMalformedRequest(t *testing.T) {
	a := newAgent()
	w := httptest.NewRecorder()
	a.handleJSONRPC(w, httptest.NewRequest(http.MethodPost, "/", strings.NewReader("not json")))

	var resp jsonRPCResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotNil(t, resp.Error)
	assert.Equal(t, -32700, resp.Error.Code)
}

func TestApplySkill(t *testing.T) {
	assert.Equal(t, "hello", applySkill("echo:hello"))
	assert.Equal(t, "olleh", applySkill("reverse:hello"))
	assert.Equal(t, "plain", applySkill("plain"))
	assert.Equal(t, "", applySkill(""))
	assert.Equal(t, "olleh dlrow", applySkill("reverse:world hello"))
}

// helpers

type sseEvent struct {
	name string
	data string
}

func parseSSEEvents(body string) []sseEvent {
	var events []sseEvent
	var current sseEvent
	scanner := bufio.NewScanner(strings.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			if current.name != "" {
				events = append(events, current)
				current = sseEvent{}
			}
			continue
		}
		if name, ok := strings.CutPrefix(line, "event: "); ok {
			current.name = name
		} else if data, ok := strings.CutPrefix(line, "data: "); ok {
			current.data = data
		}
	}
	return events
}

func toTaskResult(t *testing.T, v any) taskResult {
	t.Helper()
	b, err := json.Marshal(v)
	require.NoError(t, err)
	var result taskResult
	require.NoError(t, json.Unmarshal(b, &result))
	return result
}

var _ = bytes.NewReader // suppress unused import warning if needed
