# rejected.ai — REST API Format (Phases 0–4)

Base URL: `http://localhost:8080`. All bodies are JSON unless noted. Endpoints that
make LLM calls can take many seconds with the local Ollama model.

## `GET /healthz`

Liveness + dependency health.

```json
{ "llm_backend": "ollama", "llm_model": "gemma4:e4b", "mongo": "ok", "status": "ok" }
```
`status` is `"degraded"` (HTTP 503) if Mongo is unreachable.

## `POST /api/job-descriptions`

Ingest a job description. Two input modes:

- **Pasted text** (JSON): `{ "raw": "<jd text>" }` (optionally `"filename"` to force extraction).
- **File upload** (multipart/form-data): field `file` containing a `.pdf`, `.docx`, or `.txt`.

Returns `201` with the structured `JobDescription`:

```json
{
  "id": "665f...",
  "title": "Senior Backend Engineer — Payments Platform",
  "responsibilities": ["..."],
  "required_skills": ["..."],
  "preferred_skills": ["..."],
  "leadership_expectations": ["..."],
  "technical_expectations": ["..."],
  "domain_expectations": ["..."],
  "communication_expectations": ["..."],
  "created_at": "2026-06-03T..."
}
```

## `POST /api/resumes`

Same input modes. Returns `201` with the structured `CandidateProfile`:

```json
{
  "id": "665f...",
  "name": "Priya Nair",
  "experience": ["..."],
  "technologies": ["..."],
  "architecture_evidence": ["..."],
  "leadership_evidence": ["..."],
  "delivery_evidence": ["..."],
  "operational_evidence": ["..."],
  "domain_evidence": ["..."],
  "ai_engineering_evidence": ["..."],
  "created_at": "..."
}
```

## `POST /api/interviews`

Create a session. Builds the capability graphs, derives the competency set, and
returns the first question.

Request:
```json
{
  "job_description_id": "665f...",
  "candidate_profile_id": "665f...",
  "level": "Senior Engineer",
  "type": "Mixed",
  "duration_min": 20
}
```
Defaults: level `"Senior Engineer"`, type `"Mixed"`, duration `20`.

Response `201`:
```json
{
  "interview": { "id": "...", "competencies": ["idempotency", "leadership", "..."], "status": "active", "...": "..." },
  "graphs": {
    "candidate": [ { "name": "...", "category": "...", "evidence": ["..."], "strength": 0.7 } ],
    "target":    [ { "name": "...", "category": "...", "importance": "required", "weight": 0.9 } ],
    "strengths": ["..."], "gaps": ["..."], "unknowns": ["..."], "risk_areas": ["..."],
    "validation_targets": [ { "competency": "...", "reason": "...", "priority": 0.9 } ]
  },
  "question": { "id": "...", "turn": 1, "kind": "question", "question": "...", "target_competencies": ["..."] }
}
```

## `POST /api/interviews/{id}/answer`

Submit an answer to the current open question. Runs evidence extraction, confidence
re-scoring (with retroactive revision), then returns the next question or completes.

Request: `{ "answer": "<candidate answer>" }`

Response `200`:
```json
{
  "turn": { "turn": 1, "question": "...", "answer": "...", "answered": true, "compression_ratio": 0.21 },
  "evidence": [
    { "id": "...", "turn": 1, "competency": "idempotency", "concepts": ["exactly-once","dedup"],
      "polarity": "positive", "strength": 0.35, "supporting_quote": "duplicate handling",
      "interpretation": "...", "revisions": [] }
  ],
  "snapshots": [
    { "competency": "idempotency", "turn": 1, "confidence": 0.4, "cool": 0.5, "normal": 0.4, "hot": 0.25,
      "evidence_count": 1, "evidence_turns": [1], "rationale": "..." }
  ],
  "next": { "turn": 2, "question": "...", "target_competencies": ["..."] },
  "completed": false
}
```
When `completed` is `true`, `next` is omitted and the interview status becomes `completed`.

## `GET /api/interviews/{id}`

Full record for inspection/replay:
```json
{
  "interview": { "...": "..." },
  "graphs": { "...": "..." },
  "turns": [ { "turn": 1, "question": "...", "answer": "...", "compression_ratio": 0.21 } ],
  "evidence": [ { "competency": "idempotency", "turn": 1, "strength": 0.7,
                  "revisions": [ { "at_turn": 4, "old_strength": 0.35, "new_strength": 0.7, "note": "..." } ] } ],
  "confidence": [ { "competency": "idempotency", "turn": 1, "normal": 0.4 },
                  { "competency": "idempotency", "turn": 4, "normal": 0.85 } ]
}
```

### Retroactive re-scoring (the defining behavior)

`evidence[].revisions` is the audit trail of reinterpretation: when a later answer
clarifies an earlier shorthand answer, the earlier evidence item's `strength` is raised
and a `Revision` (with `at_turn` and `note`) is appended. The `confidence` timeline shows
the competency's `normal` score climbing across turns as a result.

### Follow-ups, assumptions, clarification/deflection (Phase 5)

`POST .../answer` may return a `next` turn with `"kind": "followup"` instead of a new
main question, when a targeted competency is below the confidence threshold or the answer
was a deflection — the engine seeks clarification before scoring down. Each answered turn
also carries per-answer analysis:

```json
"turn": {
  "assumptions": ["durable persistence is available", "request IDs exist"],
  "response_type": "answer",          // answer | clarification | deflection
  "response_reasoning": "..."
}
```

## `POST /api/interviews/{id}/report` (Phase 6–7)

Generates the final assessment: finalizes competency scores, runs the evaluator persona
panel, derives strongest signals and categorized risks, and produces the explainable
recommendation. Makes several LLM calls (slow). Idempotent — re-running replaces prior
report documents. `GET` the same path returns a previously generated report (404 if none).

Response `200`:
```json
{
  "interview": { "...": "..." },
  "competency_scores": [
    { "competency": "idempotency", "confidence": 0.85, "cool": 0.9, "normal": 0.85, "hot": 0.7,
      "evidence_turns": [1,4], "rationale": "..." }
  ],
  "signals": [ { "name": "operational maturity", "description": "...", "evidence_turns": [5,6] } ],
  "risks": [
    { "competency": "security", "category": "missing", "severity": "high", "reason": "...", "evidence_turns": [] }
  ],
  "recommendation": {
    "decision": "hire_with_risks",          // strong_hire|hire|hire_with_risks|borderline|no_hire
    "confidence_level": 0.72,
    "reasoning": "...",
    "citations": [ { "competency": "idempotency", "turns": [1,4], "note": "..." } ],
    "personas": [
      { "persona": "Architect", "overall_take": "...", "endorsements": ["..."], "concerns": ["..."],
        "per_competency": [ { "competency": "idempotency", "score": 0.8, "reasoning": "..." } ] }
    ]
  }
}
```

Risk `category` is one of `missing` (never demonstrated), `weak` (attempted, low confidence),
`jd_risk` (role-required but insufficiently validated).

## Audio (Phase 9)

Only **measurable** signals are computed — speaking pace, filler stats, word count, and
(if provided) response latency. No inferred traits.

### `POST /api/interviews/{id}/transcript`
Supply a transcript; always available (no transcription engine needed).
```json
{ "turn": 1, "transcript": "Um, so I used a dedup key…", "duration_sec": 5, "latency_ms": 1200 }
```
Response `201`:
```json
{
  "turn": 1, "source": "provided", "text": "...",
  "duration_sec": 5, "word_count": 14, "wpm": 168,
  "filler_total": 4, "filler_rate": 28.57,
  "fillers": [ { "word": "um", "count": 1 }, { "word": "you know", "count": 1 } ],
  "latency_ms": 1200
}
```

### `POST /api/interviews/{id}/audio` (multipart)
Field `file` = audio; optional form fields `turn`, `duration_sec`, `latency_ms`.
Transcribes via whisper.cpp when `WHISPER_BIN` + `WHISPER_MODEL` are set; otherwise returns
`501` directing you to the transcript endpoint. (whisper.cpp expects 16kHz mono WAV.)

Transcripts appear in `GET /api/interviews/{id}` under `transcripts`.

## Error envelope

All errors: `{ "error": "<message>" }` with the appropriate 4xx/5xx status.

## Note on local-model robustness

Engines request strict JSON. `llm.CallJSON` strips code fences and trailing commas and
retries once with a stricter instruction if the first response will not parse — important
for smaller local models whose structured output is occasionally malformed.
