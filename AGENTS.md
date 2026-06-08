# AGENTS.md — working notes for AI agents on rejected.ai

Portable, in-repo context for any AI coding agent (Claude Code, etc.). Read this first.
It complements per-machine memory (which does not travel between machines).

## What this is

**rejected.ai** is a **local-first interview-practice tool**. You give it a job description and
a résumé; it builds capability graphs, runs a mock technical interview, and produces an
evidence-backed report (verdict, signals, risks, evaluator personas, coaching) where every score
traces to something the candidate said.

**Audience framing:** the local/self-hosted build is for an **individual job seeker** to practice
for free on their own machine. Companies would use a hosted website, not this local install. The
report verdict is a *practice signal*, not a real hiring decision.

## Tech stack

- **Backend:** Go (standard-library `net/http`, Go 1.22+ method+path routes). No web framework.
- **Frontend:** Next.js + React + TypeScript (App Router), vanilla CSS, hand-rolled SVG dials.
- **Database:** MongoDB (driver v2).
- **LLM:** Ollama locally (default model `gemma4:e4b`) or Anthropic Claude (`claude-sonnet-4-6`).
- **Optional media:** whisper.cpp (audio→text) and an external video-metrics CLI — off unless configured.

## Repo layout

- `cmd/server` — backend entrypoint. `cmd/insert_dummy` — inserts mock interviews (no LLM, pure DB inserts). `cmd/check_db` — DB inspection.
- `internal/` — domain packages: `api`, `interview`, `evidence`, `confidence`, `capability`,
  `assumptions`, `evaluators`, `signals`, `risk`, `recommendation`, `report`, `learning`,
  `media`, `documents`, `llm`, `store`, `config`, `domain`.
- `web/` — Next.js app (`app/`, `components/`, `lib/api.ts`). `web/scripts/shots*.mjs` capture UI screenshots via Playwright.
- `notes/` — **internal technical docs** (see conventions). `docs/` — **GitHub Pages site** (do not put internal docs here).
- `scripts/` — `seed_demo.sh` end-to-end demo; sample JD/résumé.

## Run / build / test

```bash
# prereqs: mongod on :27017, ollama on :11434 (ollama pull gemma4:e4b)
cp config.example.json config.json            # first time
go build -o bin/server ./cmd/server && ./bin/server   # backend :8080
cd web && npm install && npm run dev                  # frontend :3000
go test ./...                                          # backend tests
go build ./... && go vet ./...                         # build + vet
```

Seed realistic data without the LLM: `go run ./cmd/insert_dummy` (inserts 3 complete mock
interviews + reports; insert-only, safe).

## Conventions (IMPORTANT — these are real preferences, follow them)

- **Docs location:** internal technical docs / format specs / plans go in **`notes/`**
  (e.g. `notes/API_FORMAT.md`, `notes/TRENDS_FORMAT.md`). **`docs/` is the published GitHub Pages
  site** — do not put internal docs there.
- **No AI attribution:** do **not** add `Co-Authored-By: Claude` to commits or
  "Generated with Claude Code" to PR bodies. Keep commits/PRs attribution-free.
- **Git workflow:** branch off `main`, open focused PRs (one concern each), let the user merge.
  Don't push/commit unless asked.
- **Logging style:** stdlib `log` with bracketed prefixes — `[BOOT]`, `[API]`, `[OLLAMA]`,
  `[ANTHROPIC]`, `[REPORT]`, `[LLM]`. Match this when adding logs.
- **Config:** `config.json` (gitignored) read at startup via `internal/config`; template is
  `config.example.json`. `Temperature` is a `*float64` so an explicit `0` is honored.
- **Scope discipline:** stay on the asked task; ask before out-of-scope edits (don't "while I'm
  here" refactor unrelated code or rewrite existing docs).

## Architecture facts (don't contradict these — they bit us before)

- **Questions are generated UPFRONT in one batch** (`interview.generateAllQuestions`), not as
  per-turn adaptive follow-ups. Rigor (`rigor_percent`, 0–100) IS injected into that prompt.
- **Evaluation is DEFERRED to report time:** evidence extraction + confidence rescoring run as a
  batch when the report is generated (`interview.EvaluateAllTurns`), not after each answer.
- **No embeddings / vector search** in the running product — capability matching is LLM-driven
  text reasoning. (`nomic-embed-text` was dead code and was removed.)
- **Recording answers** needs whisper.cpp to actually transcribe; without it the audio endpoint
  stores a placeholder. Video presence metrics need an external detector or they're placeholder.
  **Typing an answer always works.**
- **Verdict labels** (exact): `strong_hire`, `hire`, `hire_with_risks`, `borderline`, `no_hire`.
- **LLM audit logging:** `LLM_LOG_LEVEL` = `off` (default) / `info` (metadata, PII-safe) /
  `debug` (full prompts+responses) → JSON Lines at `logs/llm_calls.log`. See `notes/LLM_AUDIT_LOG_FORMAT.md`.
- **Candidate identity caveat:** the home flow re-ingests the résumé each interview, so
  `candidate_id` is NOT stable across interviews (affects cross-interview trends).

## Pointers

- Architecture & data flow: `README.md` (kept accurate), `notes/ARCHITECTURE.md`.
- Formats: `notes/API_FORMAT.md`, `notes/TRENDS_FORMAT.md`, `notes/VIDEO_METADATA_FORMAT.md`, `notes/LLM_AUDIT_LOG_FORMAT.md`.
- Roadmap: `TODO.md`. Plans: `notes/*_PLAN.md`.
