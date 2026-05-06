# A2A Test Agent

A minimal A2A test agent for MCP Gateway integration tests. Serves a static agent card and handles A2A task operations with deterministic responses.

## Skills

- **echo** — returns the input text unchanged (`echo:<text>`)
- **reverse** — returns the input text reversed (`reverse:<text>`)
- No prefix — defaults to echo

## Running locally

```bash
go run . &
```

## curl examples

```bash
# Get agent card
curl http://localhost:8090/.well-known/agent.json

# Echo task (non-streaming)
curl -s -X POST http://localhost:8090 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":1,"method":"tasks/send","params":{"id":"t1","message":{"role":"user","parts":[{"type":"text","text":"echo:hello"}]}}}'

# Reverse task (non-streaming)
curl -s -X POST http://localhost:8090 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":2,"method":"tasks/send","params":{"id":"t2","message":{"role":"user","parts":[{"type":"text","text":"reverse:hello"}]}}}'

# Streaming task
curl -s -X POST http://localhost:8090 \
  -H 'Content-Type: application/json' \
  -H 'Accept: text/event-stream' \
  -d '{"jsonrpc":"2.0","id":3,"method":"tasks/send","params":{"id":"t3","message":{"role":"user","parts":[{"type":"text","text":"echo:streaming"}]}}}'

# Get task state
curl -s -X POST http://localhost:8090 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":4,"method":"tasks/get","params":{"id":"t1"}}'

# Cancel task
curl -s -X POST http://localhost:8090 \
  -H 'Content-Type: application/json' \
  -d '{"jsonrpc":"2.0","id":5,"method":"tasks/cancel","params":{"id":"t1"}}'
```

## Running tests

```bash
go test github.com/Kuadrant/mcp-gateway/tests/servers/a2a-agent -v
```
