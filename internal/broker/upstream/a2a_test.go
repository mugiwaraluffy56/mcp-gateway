package upstream

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Kuadrant/mcp-gateway/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestA2AManager_Refresh(t *testing.T) {
	card := A2AAgentCard{
		Name:    "test-agent",
		URL:     "http://test-agent",
		Version: "1.0.0",
		Skills:  []A2ASkill{{ID: "echo", Name: "Echo"}},
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(card)
	}))
	defer srv.Close()

	mgr := NewA2AManager(config.A2AAgent{Name: "test-agent", CardURL: srv.URL, Enabled: true})

	assert.Nil(t, mgr.GetCard())
	require.NoError(t, mgr.Refresh(context.Background()))

	got := mgr.GetCard()
	require.NotNil(t, got)
	assert.Equal(t, "test-agent", got.Name)
	assert.Len(t, got.Skills, 1)
}

func TestA2AManager_Refresh_httpError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	mgr := NewA2AManager(config.A2AAgent{Name: "bad-agent", CardURL: srv.URL, Enabled: true})
	err := mgr.Refresh(context.Background())
	require.Error(t, err)
	assert.Nil(t, mgr.GetCard())
}

func TestA2AManager_Refresh_invalidJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	mgr := NewA2AManager(config.A2AAgent{Name: "bad-agent", CardURL: srv.URL, Enabled: true})
	err := mgr.Refresh(context.Background())
	require.Error(t, err)
	assert.Nil(t, mgr.GetCard())
}

func TestA2AManager_GetCard_nilBeforeRefresh(t *testing.T) {
	mgr := NewA2AManager(config.A2AAgent{Name: "agent", CardURL: "http://unused", Enabled: true})
	assert.Nil(t, mgr.GetCard())
}

func TestA2AManager_Name(t *testing.T) {
	mgr := NewA2AManager(config.A2AAgent{Name: "my-agent", CardURL: "http://unused", Enabled: true})
	assert.Equal(t, "my-agent", mgr.Name())
}
