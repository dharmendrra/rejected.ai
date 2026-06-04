# Cross-Interview Trends Format (Phase 11)

How a candidate's measured competency scores move across **their own** interviews
over time. This layer is deterministic and explainable: every value is arithmetic
over previously stored `CompetencyScore` documents, ordered by interview time.
**No LLM is involved and no trait is inferred** — it is a trajectory of prior,
already-explainable results.

## What is tracked

For each candidate (a `candidate_profile_id`), the service:

1. Loads every interview with that `candidate_profile_id`, oldest-first.
2. Loads each interview's finalized `competency_scores` (present only after a
   report has been generated for that interview).
3. Groups scores by competency and builds one trajectory per competency.
4. Persists one `HistoricalTrend` document per `(candidate_id, competency)`,
   replacing any prior set (recomputation is idempotent).

The **headline metric** is the balanced `normal` lens of each `CompetencyScore`.
The `confidence` (evidence confidence behind that score) is carried alongside
each point for context.

## TrendPoint

One competency measurement at one interview, in time order.

| Field | Type | Meaning |
|---|---|---|
| `interview_id` | ObjectID | The interview this point came from. |
| `normal` | float (0..1) | Balanced-lens competency score — the headline metric. |
| `confidence` | float (0..1) | Evidence confidence behind the score. |
| `at` | time | Interview `created_at`. |

## HistoricalTrend

One candidate's trajectory on one competency.

| Field | Type | Derivation |
|---|---|---|
| `candidate_id` | ObjectID | The candidate profile. |
| `competency` | string | Competency name. |
| `points` | []TrendPoint | Oldest-first measurements. |
| `interviews` | int | Number of points (interviews that scored this competency). |
| `first` | float | `normal` of the earliest point. |
| `latest` | float | `normal` of the most recent point. |
| `delta` | float | `latest - first`. |
| `direction` | string | `improving` / `declining` / `stable` / `new` (see below). |

### Direction

With `delta = latest - first` and an epsilon of **0.05** on the 0..1 scale:

- `new` — only one interview so far; no trajectory yet.
- `improving` — `delta > +0.05`.
- `declining` — `delta < -0.05`.
- `stable` — `|delta| <= 0.05` (movement within noise).

## API read model

`POST /api/candidates/{id}/trends` (compute) and `GET /api/candidates/{id}/trends`
(load) both return the trends plus an at-a-glance pattern summary derived from the
`direction` of each trend:

```json
{
  "candidate_id": "...",
  "trends": [ /* HistoricalTrend, sorted by competency */ ],
  "improving": ["architecture"],
  "declining": ["delivery"],
  "stable": ["comms"]
}
```

`new` competencies appear in `trends` but in none of the summary buckets.

## Edge cases

- **No scored interviews** → compute returns an empty `trends` list and clears any
  prior trend docs; load returns `404`.
- **Single interview** → each competency is `new` with one point; `first == latest`,
  `delta == 0`.
- **Competency missing from some interviews** → only the interviews that scored it
  contribute points; `interviews` reflects that count, not the candidate's total.
- **Interviews fed/stored out of order** → points are always sorted oldest-first
  before `first`/`latest`/`delta` are computed.
