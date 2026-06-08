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

## `GET /api/interviews`

Lists all interviews for the History dashboard. Returns an array of summary objects (each
hydrated with the linked résumé/JD and its turns so the dashboard can render a round without
extra fetches):
```json
[
  {
    "id": "...",
    "level": "Senior Engineer",
    "type": "Mixed",
    "status": "completed",                 // interview status
    "report_status": "completed",          // "" if no report progress doc yet
    "created_at": "...",
    "updated_at": "...",
    "candidate_name": "...",
    "resume_id": "...", "resume_raw": "...", "resume_tech": ["..."],
    "job_title": "...", "jd_id": "...", "jd_raw": "...",
    "questions": [ { "turn": 1, "question": "...", "answer": "..." } ]
  }
]
```

## `DELETE /api/interviews/{id}`

Cascade-deletes the interview session and its documents across the questions, transcripts,
video-metadata, capability-graph, confidence-score, competency-score, evidence-ledger,
signal, risk-area, recommendation, and ideal-response collections.
```json
{ "status": "deleted" }
```

## `POST /api/interviews/{id}/report` (Phase 6–7)

Generates the final assessment: finalizes competency scores, runs the evaluator persona
panel, derives strongest signals and categorized risks, and produces the explainable
recommendation. Makes several LLM calls (slow).

**Generation is asynchronous.** `POST` starts a background goroutine and returns
immediately — it does **not** return the report. Idempotent: if a report already exists it
returns it; if one is already `generating` it returns the in-flight progress; otherwise it
clears prior failed/completed progress and starts fresh.

Response `200` (generation started or already running):
```json
{
  "status": "generating",
  "progress": {
    "id": "...",
    "interview_id": "...",
    "status": "generating",          // generating | completed | failed
    "total_steps": 5,
    "completed_steps": 2,
    "current_step": "Scoring competencies",
    "steps": [
      { "name": "Finalizing scores", "status": "completed" },   // pending | running | completed
      { "name": "Scoring competencies", "status": "running" }
    ],
    "error": ""                       // populated only when status == "failed"
  }
}
```

Poll `GET /api/interviews/{id}/report` until `.status` becomes `completed` (full report
below) or `failed`. While generating, the `GET` returns the same record carrying `status`
and `progress`; it `404`s only if generation has never been started.

`GET` response `200` (completed):
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

## Video (Phase 10)

Only **measurable** signals are computed — engagement, attention, participation, and timing,
all derived from raw per-frame counts. **Never** inferred: honesty, intelligence, personality,
mood, or any other trait. Every output is a count, a percentage of analyzed frames, or a
duration. See [VIDEO_METADATA_FORMAT.md](VIDEO_METADATA_FORMAT.md) for the full contract.

### `POST /api/interviews/{id}/video-metadata`
Supply per-frame metrics; always available (no detection engine needed). Requires either
`metrics.frames_analyzed` or `metrics.duration_sec` to be > 0.
```json
{
  "turn": 1,
  "metrics": {
    "frames_analyzed": 1800, "frames_face_present": 1710,
    "frames_gaze_on_screen": 1520, "frames_multi_face": 0,
    "on_camera_sec": 60.0, "duration_sec": 62.0
  },
  "latency_ms": 1200
}
```
Response `201`:
```json
{
  "turn": 1, "source": "provided", "frames_analyzed": 1800,
  "face_present_pct": 95, "gaze_on_screen_pct": 84.44,
  "on_camera_pct": 96.77, "multi_face_pct": 0,
  "duration_sec": 62, "latency_ms": 1200
}
```
Percentages clamp to `[0,100]`; a percentage is `0` when its denominator
(`frames_analyzed`, or `duration_sec` for `on_camera_pct`) is 0.

### `POST /api/interviews/{id}/video` (multipart)
Field `file` = video; optional form fields `turn`, `latency_ms`.
Runs the external detector when `VIDEO_DETECTOR_BIN` is set (the CLI prints `FrameMetrics`
JSON for the clip); otherwise returns `501` directing you to the video-metadata endpoint.

Video metadata appears in `GET /api/interviews/{id}` under `video`.

## Cross-interview learning (Phase 11)

Tracks how a candidate's measured competency scores move across **their own**
interviews over time. Fully **deterministic and explainable** — arithmetic over
stored `competency_scores`, ordered by interview time. No LLM, no inferred traits.
The headline metric is the balanced (`normal`) lens. See
[TRENDS_FORMAT.md](TRENDS_FORMAT.md) for the full contract.

A competency's `direction` is `improving` / `declining` when the latest score
differs from the first by more than ±0.05, `stable` within that band, and `new`
when only one interview exists. One trend document is kept per
`(candidate_id, competency)`.

### `POST /api/candidates/{id}/trends`
(Re)computes and persists the candidate's trends. `{id}` is a candidate profile id.
Returns `200` with the trends and a pattern summary:
```json
{
  "candidate_id": "...",
  "trends": [
    {
      "candidate_id": "...", "competency": "architecture",
      "interviews": 3, "first": 0.5, "latest": 0.9, "delta": 0.4,
      "direction": "improving",
      "points": [
        { "interview_id": "...", "normal": 0.5, "confidence": 0.6, "at": "2026-01-01T00:00:00Z" },
        { "interview_id": "...", "normal": 0.7, "confidence": 0.7, "at": "2026-02-01T00:00:00Z" },
        { "interview_id": "...", "normal": 0.9, "confidence": 0.8, "at": "2026-03-01T00:00:00Z" }
      ]
    }
  ],
  "improving": ["architecture"], "declining": ["delivery"], "stable": ["comms"]
}
```
Interviews without a generated report (no finalized competency scores)
contribute nothing.

### `GET /api/candidates/{id}/trends`
Returns previously computed trends in the same shape. `404` if none have been
computed yet (POST to compute).

## Error envelope

All errors: `{ "error": "<message>" }` with the appropriate 4xx/5xx status.

## Note on local-model robustness

Engines request strict JSON. `llm.CallJSON` strips code fences and trailing commas and
retries once with a stricter instruction if the first response will not parse — important
for smaller local models whose structured output is occasionally malformed.
