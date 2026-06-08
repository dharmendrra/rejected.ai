# Question Pond — Implementation Plan

Reuse LLM-generated questions instead of always paying for a fresh generation.

**Idea (as requested):**
1. Whenever the LLM generates an interview's questions, **also append** each one to a new
   `questions_pond` collection, tagged with `role` + `type`. Append-only; duplicates are fine.
2. On a new interview, offer **"Choose from the Question Pond"**. If picked, **skip the
   question-generation LLM call** and instead select `N` pond questions (filtered by role/type),
   where `N` is the usual time-based budget.
3. When the candidate answers (typed or recorded), write into the existing `questions`
   collection **exactly as today** — the question text is duplicated in; **no foreign key** to
   the pond (redundant data is acceptable).
4. New top-menu page **"Question Pond"** listing `questions_pond` with role/type filters.
5. Existing schema/flow otherwise unchanged — only the *source* of an interview's questions
   becomes one of two: **AI-generated** (current) or **Pond**.

---

## Assumptions (please confirm — see end)

- **`role` = the interview "Level"** dropdown (Senior Engineer, Staff, EM, …) and **`type` =
  the "Type"** dropdown (System Design, HR Round, …). These already exist on the new-interview
  form, so Pond mode reuses them as the filter (no new form fields needed).
- **Pond mode still builds the capability graph** (that's a *separate* LLM call from question
  generation, and downstream confidence-scoring/reports need the competencies). Pond mode only
  skips the **question-generation** LLM call. (Flagged below — confirm if you also want to skip
  the graph.)
- **No dedup** in the pond (you said redundancy is fine).
- **Reused questions keep their original `target_competencies`** from when they were generated.

---

## Backend

### 1. New collection + index
- `internal/store/store.go`: add `CollQuestionsPond = "questions_pond"`.
- `internal/store/indexes.go`: index `{ role: 1, type: 1 }` (for the filtered pulls + the Pond page).

### 2. New domain type
`internal/domain/domain.go`:
```go
type PondQuestion struct {
    ID                 bson.ObjectID `bson:"_id,omitempty" json:"id"`
    Question           string        `bson:"question" json:"question"`
    TargetCompetencies []string      `bson:"target_competencies" json:"target_competencies"`

    // Filter/category fields.
    Role         string `bson:"role" json:"role"`                   // interview Level
    Type         string `bson:"type" json:"type"`                   // interview Type
    RigorPercent int    `bson:"rigor_percent" json:"rigor_percent"` // difficulty when generated

    // Provenance / future-proofing meta (captured at insert; cheap to store, useful later
    // for filtering, analytics, dedup, and tracing where a question came from).
    Model             string        `bson:"model" json:"model"`                             // generating model, e.g. gemma4:e4b / claude-sonnet-4-6
    SourceInterviewID bson.ObjectID `bson:"source_interview_id" json:"source_interview_id"` // interview it was generated for (provenance only, NOT a reuse link)
    JobTitle          string        `bson:"job_title,omitempty" json:"job_title,omitempty"` // JD title at generation time (context)
    UsedCount         int           `bson:"used_count" json:"used_count"`                   // times reused from the pond; drives least-used rotation (incremented on reuse)
    CreatedAt         time.Time     `bson:"created_at" json:"created_at"`                   // when added to the pond
}
```
All meta is set at insert time. `created_at` drives newest-first ordering on the Pond page and any
future cleanup/TTL; `role`/`type`/`rigor_percent` enable filtering; `model`/`source_interview_id`/
`job_title` give provenance; `used_count` is incremented each time a question is reused and drives the
least-used rotation (so the same questions aren't shown again and again). Add more fields here freely as
needs emerge — the collection is standalone and append-only, so extending it is low-risk.

### 3. Write to the pond on generation
In `internal/interview/session.go` `CreateSession`, after `generateAllQuestions` succeeds (AI
mode only), insert the generated questions into `questions_pond` tagged with `iv.Level` (role)
and `iv.Type`. Best-effort: a failed pond insert must NOT fail the interview (log + continue).

### 4. Pond-sourced interview creation
- `internal/interview/session.go` — extend `CreateRequest` with a source flag:
  ```go
  Source string `json:"source"` // "" / "ai" (default) or "pond"
  ```
- In `CreateSession`, branch:
  - **AI (default):** unchanged — `generateAllQuestions` + write-to-pond (step 3).
  - **Pond:** skip `generateAllQuestions`. Pull `n = questionBudget(iv.DurationMin)` questions
    from `questions_pond` where `role == iv.Level && type == iv.Type`, then map them into `Turn`
    docs (interview_id, turn 1..n, question, target_competencies) and `InsertMany` into
    `questions` — **identical to today's persistence path**.
  - **Selection (DECIDED — avoid repeats):** don't use naive `$sample` (it can show the same
    popular questions over and over). Instead **rotate by least-used**: among the role/type
    matches, take the `n` with the lowest `used_count`, breaking ties **randomly**, then
    **increment `used_count`** on the chosen ones. This guarantees the whole pond is cycled
    through roughly evenly before any question repeats, while staying varied via the random
    tiebreak. Pipeline: `match(role,type) → sort(used_count asc) → take a low-usage window →
    $sample n from that window → $inc used_count`.
  - **Fallback (DECIDED):** if the pond has **0** questions for that role/type, fall back to
    `generateAllQuestions` (the normal AI path) and write the new questions into the pond. Return
    a flag in the create response, e.g. `question_source: "pond" | "ai" | "ai_fallback"`, so the
    UI can show the "no pond questions found — generated with the LLM instead" tip.
  - **Capability graph:** see the dedicated section below — in **Pond mode it's built in the
    background** so the candidate can start immediately; in **AI mode it stays synchronous**
    (question generation depends on `graphs.ValidationTargets`).

### 5. New endpoint(s) for the Pond page
- `internal/api/pond_handlers.go`: `handleListQuestionsPond` → `GET /api/questions-pond`
  with optional `?role=&type=` filters (batched single query; sorted newest-first; reasonable
  cap/limit). Returns `[]PondQuestion` + the distinct `role`/`type` values for the filter
  dropdowns (or a tiny separate `GET /api/questions-pond/facets`).
- Register routes in `internal/api/server.go`.

### 6. (Optional) availability count
A lightweight `GET /api/questions-pond/count?role=&type=` so the new-interview screen can show
"42 pond questions available for Senior Engineer · System Design" and disable Pond mode when 0.

---

## Frontend

### 7. New-interview screen (`web/app/page.tsx`)
- Add a **"Questions source"** control: `🤖 Generate with AI (default)` vs
  `🗂️ Use the Question Pond`.
- When **Pond** is selected, the existing **Level** + **Type** selections act as the role/type
  filter; optionally show the availability count and disable the Start button (or auto-fallback
  to AI) if the pond has 0 for that role/type.
- `web/lib/api.ts` `createInterview(...)`: pass `source: "pond" | "ai"`.

### 8. New "Question Pond" menu + page
- `web/components/Header.tsx`: add a 4th nav item `🗂️ Question Pond` → `/pond`.
- `web/app/pond/page.tsx`: list `questions_pond` via `api.listQuestionsPond({role, type})`, with
  two filter dropdowns (role, type) populated from the facets/distinct values. Show question text
  + target competencies + tags; paginate or cap for large ponds. Reuse the dark-UI `.panel` /
  card styling.

---

## Background capability-graph build (Pond mode only)

Because Pond questions come from Mongo (fast) and don't need the graph, we don't make the
candidate wait on the graph's LLM call. The graph is still required later (confidence scoring,
risk, report), so we build it **in the background** and only block at report time if it isn't
ready yet.

**Why Pond-only:** AI mode's `generateAllQuestions` reads `graphs.ValidationTargets`, so the
graph must exist before questions are generated — AI mode keeps the synchronous build. Pond
selection needs none of that.

**At interview creation (Pond mode):**
1. Build the capability graph **after** returning — launch a goroutine (same pattern as the
   report-generation goroutine) that runs `capability.Build`, persists the `capability_graphs`
   doc, and sets `iv.Competencies` (via `deriveCompetencies`).
2. Track state on the interview with a `graph_status` field: `building` → `ready` / `failed`.
3. Return the interview + pond questions immediately so the candidate can start.
4. *(Optional nicety)* seed `iv.Competencies` right away from the union of the selected pond
   questions' `target_competencies`, so the live-confidence sidebar isn't empty while the graph
   builds; refine it when the real graph lands.

**During the interview:** nothing needs the graph (answering just records answers; evaluation is
deferred), so there's no blocker. The live-confidence sidebar is simply sparse until the graph
finishes (cosmetic).

**At report generation — "ensure graph ready":** replace `report.go`'s current hard-fail on a
missing graph with:
- `ready` → load and proceed.
- `building` → poll/wait with a sane timeout (the graph's one LLM call is usually done long
  before the candidate finishes, so this rarely waits).
- `failed` / missing / still building after timeout (e.g. server restarted mid-build) →
  **build it synchronously now** as a fallback, then proceed.

This makes evaluation non-blocking for the candidate and guaranteed-correct for the report
(the graph always exists by the time the recommendation is written).

**Edge / robustness:**
- Background build **failure** is fine — the report-time fallback rebuilds it synchronously.
- **Server restart** mid-build leaves `graph_status=building` with no goroutine; the report-time
  timeout→sync-fallback covers it. (Optionally, a startup sweep can reset stale `building`
  statuses, like the existing report-progress cleanup in `cmd/server/main.go`.)
- Concurrency: the background goroutine writes Mongo after the request returns — same model as
  the report goroutine (mind the data-race lesson: don't share mutable state with the handler).

---

## Edge cases

- **Empty pond** for a role/type (DECIDED): **fall back to AI generation** and show an info
  tip — e.g. *"No questions found in the pond for this role/type — generating with the LLM
  instead."* So Pond mode never dead-ends; it silently degrades to the normal AI path (and those
  freshly-generated questions get written into the pond, so next time there will be some).
- **Partial pond** (some, but fewer than `n`): use what's available (show fewer). *(Optional
  future: top up the remainder with an LLM call.)*
- **Competency alignment:** reused questions carry the `target_competencies` they were generated
  with, which may not match the *current* interview's capability graph. Downstream confidence
  scoring still works, but is less precisely targeted than freshly-generated questions. Acceptable
  per the "redundancy is fine" stance; noting it for transparency.
- **Pond growth:** append-only with duplicates means it grows steadily. Fine for now; a dedup or
  TTL/cleanup is a possible future task.

---

## Build order

1. Backend: collection + index + `PondQuestion` type.
2. Backend: write-to-pond on AI generation (step 3) — pond starts filling immediately.
3. Backend: `source: "pond"` branch in `CreateSession` (step 4).
4. Backend: `GET /api/questions-pond` (+ facets/count) endpoints.
5. Frontend: new-interview source toggle.
6. Frontend: "Question Pond" nav + `/pond` page.

(2 can ship first on its own — it just starts accumulating questions with zero behavior change.)

---

## Files touched (anticipated)

**New:** `internal/api/pond_handlers.go`, `web/app/pond/page.tsx`.
**Modified:** `internal/store/store.go`, `internal/store/indexes.go`, `internal/domain/domain.go`
(add `PondQuestion`; add `graph_status` to `Interview`), `internal/interview/session.go`
(pond branch + background graph build), `internal/interview/engine.go` and/or
`internal/report/report.go` (the report-time "ensure graph ready" wait/sync-fallback),
`internal/api/server.go`, `web/app/page.tsx`, `web/components/Header.tsx`, `web/lib/api.ts`.
Optionally `cmd/server/main.go` (startup sweep of stale `graph_status=building`).

---

## Decisions locked
- **role = Level**, **type = Type** (confirmed).
- **Selection = least-used rotation with random tiebreak** (+ increment `used_count`), so the
  pond cycles evenly and questions don't repeat until the pool is exhausted.
- **Empty pond** for a role/type → fall back to AI generation + show an info tip; the fresh
  questions are still added to the pond.
- Pond stores **`created_at`** plus provenance/meta (role, type, rigor, model, source interview,
  job title, `used_count`).
- **Capability graph:** Pond mode builds it **in the background** (candidate starts immediately);
  report generation waits-if-building, with a synchronous build as fallback. AI mode keeps the
  synchronous build (question generation depends on the graph). See the dedicated section.
