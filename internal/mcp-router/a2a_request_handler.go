package mcprouter

import (
	"context"
	"strings"

	eppb "github.com/envoyproxy/go-control-plane/envoy/service/ext_proc/v3"
)

// isA2ARequest returns true if the JSON-RPC method is an A2A task operation.
func isA2ARequest(method string) bool {
	return strings.HasPrefix(method, "tasks/")
}

// HandleA2ARequest extracts A2A metadata from the parsed request and sets
// x-a2a-* headers for downstream policy enforcement, then passes the request
// through to the upstream A2A agent via normal Envoy routing.
func (s *ExtProcServer) HandleA2ARequest(ctx context.Context, mcpReq *MCPRequest) []*eppb.ProcessingResponse {
	_, span := tracer().Start(ctx, "mcp-router.a2a-request")
	defer span.End()

	headers := NewHeaders().WithA2AMethod(mcpReq.Method)

	if id, ok := mcpReq.Params["id"].(string); ok && id != "" {
		headers.WithA2ATaskID(id)
	}

	accept := mcpReq.GetSingleHeaderValue("accept")
	if strings.Contains(accept, "text/event-stream") {
		headers.WithA2AStreaming(true)
	}

	response := NewResponse()
	return response.WithRequestBodyHeadersResponse(headers.Build()).Build()
}
