package broker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kuadrant/mcp-gateway/internal/broker/upstream"
	"github.com/Kuadrant/mcp-gateway/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"log/slog"
	"os"
)

func TestHandleA2AAgents_empty(t *testing.T) {
	b := NewBroker(slog.New(slog.NewTextHandler(os.Stdout, nil))).(*mcpBrokerImpl)

	w := httptest.NewRecorder()
	b.handleFederatedAgentCard(w, httptest.NewRequest(http.MethodGet, "/a2a/agents", nil))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, "application/json", w.Header().Get("Content-Type"))

	var cards []upstream.A2AAgentCard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cards))
	assert.Empty(t, cards)
}

func TestHandleA2AAgents_skipsUnfetchedCards(t *testing.T) {
	b := NewBroker(slog.New(slog.NewTextHandler(os.Stdout, nil))).(*mcpBrokerImpl)
	// add a manager with no card fetched yet
	b.a2aAgents["unfetched"] = upstream.NewA2AManager(config.A2AAgent{
		Name:    "unfetched",
		CardURL: "http://unused",
		Enabled: true,
	})

	w := httptest.NewRecorder()
	b.handleFederatedAgentCard(w, httptest.NewRequest(http.MethodGet, "/a2a/agents", nil))

	var cards []upstream.A2AAgentCard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cards))
	assert.Empty(t, cards)
}

func TestHandleA2AAgents_rewritesURL(t *testing.T) {
	b := NewBroker(
		slog.New(slog.NewTextHandler(os.Stdout, nil)),
		WithGatewayExternalHostname("https://gateway.example.com"),
	).(*mcpBrokerImpl)

	mgr := upstream.NewA2AManager(config.A2AAgent{
		Name:    "echo-agent",
		CardURL: "http://upstream-echo-agent",
		Enabled: true,
	})
	mgr.SetCardForTesting(&upstream.A2AAgentCard{
		Name:    "echo-agent",
		URL:     "http://upstream-echo-agent",
		Version: "1.0.0",
		Skills:  []upstream.A2ASkill{{ID: "echo", Name: "Echo"}},
	})
	b.a2aAgents["echo-agent"] = mgr

	w := httptest.NewRecorder()
	b.handleFederatedAgentCard(w, httptest.NewRequest(http.MethodGet, "/a2a/agents", nil))

	var cards []upstream.A2AAgentCard
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &cards))
	require.Len(t, cards, 1)
	assert.Equal(t, "echo-agent", cards[0].Name)
	assert.Equal(t, "https://gateway.example.com/a2a/echo-agent", cards[0].URL)
	assert.Len(t, cards[0].Skills, 1)
}
