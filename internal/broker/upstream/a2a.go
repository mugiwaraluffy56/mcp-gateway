package upstream

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	"github.com/Kuadrant/mcp-gateway/internal/config"
)

// A2AAgentCard holds the subset of an A2A agent card used by the gateway.
type A2AAgentCard struct {
	Name         string            `json:"name"`
	Description  string            `json:"description"`
	URL          string            `json:"url"`
	Version      string            `json:"version"`
	Capabilities A2ACapabilities   `json:"capabilities"`
	Skills       []A2ASkill        `json:"skills"`
}

// A2ACapabilities represents the capabilities declared in an agent card.
type A2ACapabilities struct {
	Streaming              bool `json:"streaming"`
	PushNotifications      bool `json:"pushNotifications"`
	StateTransitionHistory bool `json:"stateTransitionHistory"`
}

// A2ASkill represents a single skill declared in an agent card.
type A2ASkill struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description"`
	InputModes  []string `json:"inputModes"`
	OutputModes []string `json:"outputModes"`
}

// A2AManager fetches and caches the agent card for one upstream A2A agent.
type A2AManager struct {
	cfg    config.A2AAgent
	card   *A2AAgentCard
	mu     sync.RWMutex
	client *http.Client
}

// NewA2AManager creates a new A2AManager for the given agent config.
func NewA2AManager(cfg config.A2AAgent) *A2AManager {
	return &A2AManager{
		cfg:    cfg,
		client: &http.Client{},
	}
}

// Refresh fetches the agent card from the configured CardURL and caches it.
func (m *A2AManager) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.cfg.CardURL, nil)
	if err != nil {
		return fmt.Errorf("build card request for %s: %w", m.cfg.Name, err)
	}
	req.Header.Set("Accept", "application/json")

	resp, err := m.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch card for %s: %w", m.cfg.Name, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("card fetch for %s returned %d", m.cfg.Name, resp.StatusCode)
	}

	var card A2AAgentCard
	if err := json.NewDecoder(resp.Body).Decode(&card); err != nil {
		return fmt.Errorf("decode card for %s: %w", m.cfg.Name, err)
	}

	m.mu.Lock()
	m.card = &card
	m.mu.Unlock()
	return nil
}

// GetCard returns the cached agent card, or nil if Refresh has not been called successfully.
func (m *A2AManager) GetCard() *A2AAgentCard {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.card
}

// Name returns the configured agent name.
func (m *A2AManager) Name() string {
	return m.cfg.Name
}

// SetCardForTesting injects a card directly, for use in tests only.
func (m *A2AManager) SetCardForTesting(card *A2AAgentCard) {
	m.mu.Lock()
	m.card = card
	m.mu.Unlock()
}
