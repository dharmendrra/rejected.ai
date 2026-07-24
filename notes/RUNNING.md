# Running rejected.ai locally

## Prerequisites
- `mongod` running on `:27017`
- `ollama` serving the configured model + embeddings:
  - `ollama pull gemma4:e4b` (default) and `ollama pull nomic-embed-text`
  - For a faster (lower-quality) local experience, set `OLLAMA_MODEL` to `gemma4:e2b`.
- Go 1.26+, Node 20+ (for the UI).

> Note: on a CPU-only machine the local model is slow (minutes per call). The
> backend, tests, and API are unaffected; only live LLM latency is.

## Backend (Go API)
```bash
cp config.example.json config.json   # already present
go build -o bin/server ./cmd/server
./bin/server                          # serves :8090, CORS open for the UI
curl -s localhost:8090/healthz
```

Switch to Anthropic: set `"LLM_BACKEND": "anthropic"` + `"ANTHROPIC_API_KEY"` in `config.json`.

## Frontend (Next.js UI)
```bash
cd web
npm install
npm run dev            # http://localhost:3000  (reads NEXT_PUBLIC_API_BASE)
```

UI flow:
1. **Home** (`/`) — paste a JD + resume (or "Load sample"), pick level/type/duration, Start.
2. **Interview** (`/interview/{id}`) — answer each question; the **Live confidence** sidebar
   updates after every answer; follow-ups appear when a competency needs validation.
3. **Report** (`/interview/{id}/report`) — generate the hiring-intelligence dashboard:
   recommendation + citations, competency breakdown (cool/normal/hot), strongest signals,
   risk areas, evaluator panel, **score evolution** sparklines, and the **retroactive
   re-scoring** log (earlier evidence reinterpreted by later answers).

## Tests
```bash
go test ./...     # includes the deterministic retroactive re-scoring + report pipeline tests
```

## Scripted demo (no UI)
```bash
./scripts/seed_demo.sh   # full live flow against the API (slow on CPU)
```
