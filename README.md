# rejected.ai

Local-first, AI-native **interview intelligence** platform. It evaluates engineering
candidates through conversation and **accumulating evidence** rather than keyword
matching. The defining behavior is the evidence loop:

```
Question → Evidence → Confidence Update → New Evidence → Confidence Update → Retroactive Re-scoring
```

Scores are never frozen — a later answer that reveals deeper understanding lifts the
confidence attributed to earlier shorthand answers, and every score is explainable via
the evidence ledger.

## Quick Start

### Prerequisites
- **Go 1.26+**
- **MongoDB** (`mongod`) running on `:27017`
- **Ollama** serving the configured models (local, default backend):
  - `ollama pull gemma4:e4b` (generation) and `ollama pull nomic-embed-text` (embeddings)
- For the UI: **Node 20+**
- For the demo script below: **jq**

> On a CPU-only machine the local model is slow (minutes per LLM call). The backend,
> tests, and non-LLM endpoints (audio/video/trends) are unaffected.

### Run the backend
```bash
cp config.example.json config.json          # already present in this repo
go build -o bin/server ./cmd/server
./bin/server                                 # serves :8080, CORS open for the UI

curl -s localhost:8080/healthz
# {"llm_backend":"ollama","llm_model":"gemma4:e4b","mongo":"ok","status":"ok"}
```

Prefer Anthropic over local Ollama? Set `"LLM_BACKEND": "anthropic"` and
`"ANTHROPIC_API_KEY"` in `config.json`.

### Fastest path: the seed demo
End-to-end run — ingests a sample JD + resume, starts an interview, answers each
question, then prints the confidence-evolution timeline and any **retroactive**
evidence revisions:
```bash
./scripts/seed_demo.sh
```

### Manual flow (curl)
```bash
BASE=http://localhost:8080

# 1. Ingest a JD and a resume (paste raw text, or upload a file via multipart "file").
JD_ID=$(curl -sf -X POST $BASE/api/job-descriptions \
  -H 'Content-Type: application/json' \
  -d "{\"raw\": $(jq -Rs . < scripts/sample_jd.txt)}" | jq -r .id)
CV_ID=$(curl -sf -X POST $BASE/api/resumes \
  -H 'Content-Type: application/json' \
  -d "{\"raw\": $(jq -Rs . < scripts/sample_resume.txt)}" | jq -r .id)

# 2. Start an interview (builds capability graphs, asks the first question).
IV_ID=$(curl -sf -X POST $BASE/api/interviews \
  -H 'Content-Type: application/json' \
  -d "{\"job_description_id\":\"$JD_ID\",\"candidate_profile_id\":\"$CV_ID\",\"level\":\"Senior Engineer\",\"type\":\"Mixed\",\"duration_min\":20}" \
  | jq -r .interview.id)

# 3. Answer turns (repeat; the response carries the next question until completed).
curl -sf -X POST $BASE/api/interviews/$IV_ID/answer \
  -H 'Content-Type: application/json' -d '{"answer":"For idempotency I derived a dedup key..."}'

# 4. Generate the final hiring-intelligence report.
curl -sf -X POST $BASE/api/interviews/$IV_ID/report | jq .

# 5. Full interview record (turns, evidence ledger, confidence snapshots, transcripts, video).
curl -sf $BASE/api/interviews/$IV_ID | jq .
```

### Optional signals & learning
```bash
# Audio (Phase 9): supply a transcript for measurable speech signals (WPM, fillers, latency).
curl -sf -X POST $BASE/api/interviews/$IV_ID/transcript \
  -H 'Content-Type: application/json' \
  -d '{"turn":1,"transcript":"Um, so I used a dedup key...","duration_sec":5,"latency_ms":1200}'

# Video (Phase 10): supply per-frame metrics for measurable engagement/attention/participation.
curl -sf -X POST $BASE/api/interviews/$IV_ID/video-metadata \
  -H 'Content-Type: application/json' \
  -d '{"turn":1,"metrics":{"frames_analyzed":1800,"frames_face_present":1710,"frames_gaze_on_screen":1520,"on_camera_sec":60,"duration_sec":62}}'

# Cross-interview learning (Phase 11): a candidate's competency trends across interviews.
curl -sf -X POST $BASE/api/candidates/$CV_ID/trends | jq .
```
Both audio and video also accept raw uploads (`/audio`, `/video`) when a transcription
engine (`WHISPER_BIN`/`WHISPER_MODEL`) or video detector (`VIDEO_DETECTOR_BIN`) is
configured; otherwise they return `501` pointing at the metrics endpoints above.

### Frontend (Next.js UI)
```bash
cd web
npm install
npm run dev          # http://localhost:3000  (reads NEXT_PUBLIC_API_BASE)
```
Flow: paste JD + resume on **Home** → answer questions on **Interview** (live confidence
sidebar) → generate the **Report** dashboard (recommendation + citations, cool/normal/hot
competency breakdown, strongest signals, risk areas, evaluator panel, score-evolution
sparklines, and the retroactive re-scoring log).

### Tests
```bash
go build ./... && go vet ./... && go test ./...
```

## Design principles

- **Evidence over keywords.** Every score traces to specific answer turns; later answers
  retroactively re-weight earlier evidence.
- **Measurable, never inferred.** Audio and video signals are counts/percentages/durations
  only — the platform deliberately never infers honesty, intelligence, or personality.
- **Deterministic where it can be.** Cross-interview trends are pure arithmetic over stored
  scores — no LLM, fully explainable.
- **Local-first.** Defaults to Ollama + local MongoDB; Anthropic is opt-in.

## Documentation

Design notes and API/format specs live in [`notes/`](notes/):

| Doc | Contents |
|---|---|
| [ARCHITECTURE.md](notes/ARCHITECTURE.md) | Stack, package layout, data flow, collections |
| [RUNNING.md](notes/RUNNING.md) | Detailed local-run guide (backend, UI, tests) |
| [API_FORMAT.md](notes/API_FORMAT.md) | Every endpoint's request/response shape |
| [VIDEO_METADATA_FORMAT.md](notes/VIDEO_METADATA_FORMAT.md) | Phase 10 video signals + detector contract |
| [TRENDS_FORMAT.md](notes/TRENDS_FORMAT.md) | Phase 11 cross-interview trends |

> The `docs/` folder is reserved for the GitHub Pages site.
