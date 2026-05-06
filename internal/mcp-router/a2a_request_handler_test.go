package mcprouter

import (
	"context"
	"testing"

	corev3 "github.com/envoyproxy/go-control-plane/envoy/config/core/v3"
	eppb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// a2aHeaders extracts set-header key→value from a slice of ProcessingResponses.
// It inspects both RequestBody and RequestHeaders mutation responses.
func a2aHeaders(responses []*eppb.ProcessingResponse) map[string]string {
	out := map[string]string{}
	for _, r := range responses {
		var hdrs []*corev3.HeaderValueOption
		switch v := r.Response.(type) {
		case *eppb.ProcessingResponse_RequestBody:
			if v.RequestBody.Response != nil && v.RequestBody.Response.HeaderMutation != nil {
				hdrs = v.RequestBody.Response.HeaderMutation.SetHeaders
			}
		case *eppb.ProcessingResponse_RequestHeaders:
			if v.RequestHeaders.Response != nil && v.RequestHeaders.Response.HeaderMutation != nil {
				hdrs = v.RequestHeaders.Response.HeaderMutation.SetHeaders
			}
		}
		for _, h := range hdrs {
			if h.Header != nil {
				out[h.Header.Key] = string(h.Header.RawValue)
			}
		}
	}
	return out
}

func TestIsA2ARequest(t *testing.T) {
	assert.True(t, isA2ARequest("tasks/send"))
	assert.True(t, isA2ARequest("tasks/get"))
	assert.True(t, isA2ARequest("tasks/cancel"))
	assert.True(t, isA2ARequest("tasks/subscribe"))
	assert.False(t, isA2ARequest("tools/call"))
	assert.False(t, isA2ARequest("tools/list"))
	assert.False(t, isA2ARequest("initialize"))
	assert.False(t, isA2ARequest(""))
}

func TestHandleA2ARequest_setsMethod(t *testing.T) {
	s := newTestServer(t)
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/send",
		Params:  map[string]any{},
	}

	responses := s.HandleA2ARequest(context.Background(), req)
	require.NotEmpty(t, responses)

	headers := a2aHeaders(responses)
	assert.Equal(t, "tasks/send", headers[a2aMethodHeader])
}

func TestHandleA2ARequest_setsTaskID(t *testing.T) {
	s := newTestServer(t)
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/send",
		Params:  map[string]any{"id": "task-123"},
	}

	responses := s.HandleA2ARequest(context.Background(), req)
	headers := a2aHeaders(responses)
	assert.Equal(t, "task-123", headers[a2aTaskIDHeader])
}

func TestHandleA2ARequest_noTaskID(t *testing.T) {
	s := newTestServer(t)
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/get",
		Params:  map[string]any{},
	}

	responses := s.HandleA2ARequest(context.Background(), req)
	headers := a2aHeaders(responses)
	assert.NotContains(t, headers, a2aTaskIDHeader)
}

func TestHandleA2ARequest_streamingHeader(t *testing.T) {
	s := newTestServer(t)
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/send",
		Params:  map[string]any{},
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: "accept", RawValue: []byte("text/event-stream")},
			},
		},
	}

	responses := s.HandleA2ARequest(context.Background(), req)
	headers := a2aHeaders(responses)
	assert.Equal(t, "true", headers[a2aStreamingHeader])
}

func TestHandleA2ARequest_noStreamingWhenJSONAccept(t *testing.T) {
	s := newTestServer(t)
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/send",
		Params:  map[string]any{},
		Headers: &corev3.HeaderMap{
			Headers: []*corev3.HeaderValue{
				{Key: "accept", RawValue: []byte("application/json")},
			},
		},
	}

	responses := s.HandleA2ARequest(context.Background(), req)
	headers := a2aHeaders(responses)
	assert.NotContains(t, headers, a2aStreamingHeader)
}

func TestRouteMCPRequest_dispatchesA2A(t *testing.T) {
	s := newTestServer(t)
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tasks/send",
		Params:  map[string]any{"id": "t1"},
	}

	responses := s.RouteMCPRequest(context.Background(), req)
	require.NotEmpty(t, responses)

	headers := a2aHeaders(responses)
	assert.Equal(t, "tasks/send", headers[a2aMethodHeader])
	assert.Equal(t, "t1", headers[a2aTaskIDHeader])
	// MCP method header must not be set for A2A requests
	assert.NotContains(t, headers, methodHeader)
}

func TestRouteMCPRequest_mcpToolCallNotA2A(t *testing.T) {
	s := newTestServer(t)
	req := &MCPRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "tools/call",
		Params:  map[string]any{"name": "greet"},
	}

	responses := s.RouteMCPRequest(context.Background(), req)
	require.NotEmpty(t, responses)

	headers := a2aHeaders(responses)
	assert.NotContains(t, headers, a2aMethodHeader)
}
