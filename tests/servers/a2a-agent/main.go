// A minimal A2A test agent for MCP Gateway integration testing.
// Serves an agent card, handles tasks/send (streaming and non-streaming),
// tasks/get, and tasks/cancel with deterministic responses.
package main

import (
	"log"
	"net/http"
	"os"
)

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8090"
	}

	agent := newAgent()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/agent.json", agent.handleCard)
	mux.HandleFunc("POST /", agent.handleJSONRPC)

	log.Printf("a2a-agent listening on :%s", port)
	log.Fatal(http.ListenAndServe(":"+port, mux))
}
