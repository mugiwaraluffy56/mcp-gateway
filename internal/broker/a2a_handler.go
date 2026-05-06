package broker

import (
	"encoding/json"
	"net/http"

	"github.com/Kuadrant/mcp-gateway/internal/broker/upstream"
)

// handleFederatedAgentCard aggregates cached agent cards from all registered A2A agents
// and rewrites their URL field to route through the gateway external hostname.
func (m *mcpBrokerImpl) handleFederatedAgentCard(w http.ResponseWriter, _ *http.Request) {
	m.a2aLock.RLock()
	managers := make([]*upstream.A2AManager, 0, len(m.a2aAgents))
	for _, mgr := range m.a2aAgents {
		managers = append(managers, mgr)
	}
	m.a2aLock.RUnlock()

	cards := make([]upstream.A2AAgentCard, 0, len(managers))
	for _, mgr := range managers {
		card := mgr.GetCard()
		if card == nil {
			continue
		}
		rewritten := *card
		rewritten.URL = m.gatewayExternalHostname + "/a2a/" + mgr.Name()
		cards = append(cards, rewritten)
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(cards)
}
