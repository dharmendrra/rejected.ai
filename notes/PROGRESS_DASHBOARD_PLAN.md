# Progress Dashboard — Implementation Plan

A new top-level screen (`📈 Progress Dashboard`) that visualizes everything the
platform already stores across interviews — verdicts, confidence, competencies,
risks, signals, evaluator personas, and trends — as charts, with every chart
zoomable to fullscreen.

**Locked decisions (confirmed with the user):**
- **Data source:** a new server-side aggregation endpoint `GET /api/dashboard`.
- **Scope:** global "portfolio" view across all interviews, time-series grouped by candidate **name** (ship now; stable per-candidate identity is a noted follow-up).
- **Charts:** **Recharts** (React/SVG) as the one new frontend dependency.

---

## 1. Placement & routing

- **Nav:** add a third link in `web/components/Header.tsx` after `📁 Past interviews`:
  `📈 Progress Dashboard` → `/dashboard`.
- **Route:** new `web/app/dashboard/page.tsx` (client component, like `history/page.tsx`).
- **API client:** add `getDashboard()` to `web/lib/api.ts` plus TypeScript types.

---

## 2. Data model caveat (must be acknowledged, not blocking)

"Progress" implies tracking one person over time, but the current home flow
(`web/app/page.tsx → start()`) **re-ingests the résumé on every interview**, creating a
**new `candidate_profiles` document each time**. Therefore `candidate_id` is *not* stable
across interviews, and `learning.BuildTrends` / `/api/candidates/{id}/trends` only ever see
one interview per candidate.

**Decision:** ship the **global portfolio** view. The aggregator groups time-series by
**candidate name** (normalized, trimmed, lowercased) as a best-effort identity. This works
with existing data and needs no flow change.

**Follow-up (out of scope for v1, but document it):** add a "reuse existing résumé/candidate"
path so `candidate_id` persists; then real per-candidate trends light up. Tracked as a future
task in `TODO.md`.

---

## 3. Backend — `GET /api/dashboard`

### 3.1 Route
Register in `internal/api/server.go`:
```
mux.HandleFunc("GET /api/dashboard", s.handleDashboard)
```
Optional query params:
- `candidate=<name>` — scope to one candidate name (default: all).
- `from=<RFC3339>` / `to=<RFC3339>` — date range filter (default: all time).

### 3.2 New package `internal/dashboard`
A pure, testable aggregator that takes already-loaded documents and returns the payload
struct. The handler does the batched Mongo reads (mirroring the N+1 fix in
`handleListInterviews`), then calls the aggregator.

**Batched reads (one query each, by `$in` over interview IDs):**
- `interviews` (all, sorted by `created_at`)
- `recommendations` (decision, confidence_level, personas, created_at)
- `competency_scores` (competency, cool/normal/hot, confidence)
- `risk_areas` (risks[].category, severity, competency)
- `signals` (signals[].name)
- `confidence_scores` (per-turn snapshots, for score-evolution)
- `candidate_profiles` + `job_descriptions` (names/titles for labels)

Reuse `learning.BuildTrends` by constructing `ScoredInterview{InterviewID, At, Scores}` per
interview and grouping by candidate name.

### 3.3 Response payload (every field maps to real stored data)

```jsonc
{
  "generated_at": "<RFC3339>",
  "scope": { "candidate": "all", "from": null, "to": null },

  "kpis": {
    "total_interviews": 0,
    "completed_reports": 0,
    "pending_reports": 0,
    "questions_asked": 0,
    "questions_answered": 0,
    "avg_confidence": 0.0,          // mean recommendation.confidence_level over completed
    "most_improved_competency": "", // largest positive trend delta (or "" if <2 interviews)
    "candidates": 0                 // distinct candidate names
  },

  "verdict_mix": [                   // recommendation.decision counts
    { "decision": "strong_hire", "count": 0 },
    { "decision": "hire", "count": 0 },
    { "decision": "hire_with_risks", "count": 0 },
    { "decision": "borderline", "count": 0 },
    { "decision": "no_hire", "count": 0 }
  ],

  "confidence_over_time": [          // one point per completed interview, time-ordered
    { "interview_id": "", "at": "<RFC3339>", "confidence": 0.0,
      "decision": "", "level": "", "type": "", "rigor_percent": 0,
      "candidate_name": "", "job_title": "" }
  ],

  "competency_trends": [             // from learning.BuildTrends (per candidate name)
    { "competency": "", "direction": "improving|declining|stable|new",
      "first": 0.0, "latest": 0.0, "delta": 0.0,
      "points": [ { "interview_id": "", "at": "<RFC3339>", "normal": 0.0, "confidence": 0.0 } ] }
  ],

  "competency_profile": [           // latest score per competency (radar)
    { "competency": "", "cool": 0.0, "normal": 0.0, "hot": 0.0, "confidence": 0.0,
      "first_normal": 0.0 }        // earliest value, for first-vs-latest overlay
  ],

  "rigor_vs_confidence": [          // scatter
    { "interview_id": "", "rigor_percent": 0, "confidence": 0.0, "decision": "", "type": "" }
  ],

  "coverage": {
    "by_type":  [ { "key": "System Design", "count": 0 } ],
    "by_level": [ { "key": "Senior Engineer", "count": 0 } ]
  },

  "risks": [                        // category x severity totals
    { "category": "missing|weak|jd_risk", "severity": "low|medium|high", "count": 0 }
  ],

  "top_signals": [                  // signal name frequency (desc)
    { "name": "", "count": 0 }
  ],

  "persona_competency": [           // avg per-competency score by persona
    { "persona": "System Architect",
      "competencies": [ { "competency": "", "avg_score": 0.0 } ] }
  ],

  "score_evolution": [              // per interview, normal-by-turn (for the within-interview line)
    { "interview_id": "", "candidate_name": "", "type": "",
      "series": [ { "turn": 0, "avg_normal": 0.0 } ] }
  ]
}
```

Notes:
- `avg_normal` per turn = mean of `confidence_scores.normal` across competencies at that turn.
- Decisions/severities/categories come straight from `domain` constants — don't invent labels.
- Video/transcript signals (gaze %, WPM) are intentionally **excluded from v1** because they're
  placeholder unless whisper.cpp / a detector are configured; can be added later behind a
  "requires recording setup" label.

### 3.4 Tests
`internal/dashboard/*_test.go` — feed synthetic interviews/reports into the aggregator and assert:
- verdict counts, avg_confidence, KPI math
- trend grouping by candidate name + direction
- risk category×severity bucketing
- signal frequency ordering
No DB or LLM needed (pure functions).

---

## 4. Frontend — `/dashboard`

### 4.1 Dependency
Add **Recharts** to `web/package.json`. Charts used: `LineChart` (+`Brush`), `BarChart`
(stacked + grouped), `PieChart`/donut, `RadarChart`, `ScatterChart`, `ResponsiveContainer`,
`Tooltip`, `Legend`.

### 4.2 Page structure
- Header: title + **scope selector** (candidate-name dropdown: "All" + distinct names) and an
  optional date-range control. Re-fetch `getDashboard()` on change.
- **KPI strip** (stat tiles) across the top.
- Responsive **card grid** below; each card = one chart in a `.panel`, with a title and a
  **⤢ expand** button (top-right).
- Loading + empty states (see §6).

### 4.3 Card → chart inventory
| Card | Recharts type | Payload field |
|---|---|---|
| KPI tiles | (plain) | `kpis` |
| Verdict mix | donut Pie | `verdict_mix` (color per decision) |
| Confidence over time | Line + Brush | `confidence_over_time` |
| Competency trends | multi-Line (+ direction badges) | `competency_trends` |
| Current competency profile | Radar (latest vs first overlay) | `competency_profile` |
| Rigor vs performance | Scatter | `rigor_vs_confidence` |
| Interview coverage | two Bar charts | `coverage.by_type`, `coverage.by_level` |
| Recurring risks | stacked Bar | `risks` (stack by severity, group by category) |
| Consistent strengths | horizontal Bar | `top_signals` |
| Evaluator lens comparison | grouped Bar or Radar | `persona_competency` |
| Within-interview evolution | Line (per selected interview) | `score_evolution` |

### 4.4 Color & style
- Reuse existing CSS variables (`--accent`, `--good`, `--warn`, `--bad`, `--muted`) so it
  matches the dark UI. Fixed color map for the 5 verdicts (green→red gradient like the report dial).

---

## 5. Zoom / interaction

- Each card has a **⤢ expand** button → opens a **fullscreen modal** (reuse the
  `.modal-overlay` / `.modal-content` pattern from `interview/[id]/page.tsx`) that re-renders the
  same chart at large size.
- **Time-series** charts include a Recharts `<Brush>` for drag-to-zoom a date/turn window
  (works inline and in the modal).
- **Tooltips** on hover everywhere; **click a data point** → navigate to that interview's report
  (`/interview/{id}/report`).
- `Esc` / backdrop click closes the modal.

---

## 6. Edge cases & empty states

- **0 interviews:** friendly empty panel with a link to "Start fresh".
- **1 interview:** trends render "not enough data yet" (matches `TrendNew`); single-point
  snapshots still shown; `most_improved_competency` is empty.
- **Reports not generated:** such interviews are excluded from verdict/confidence/competency
  charts and counted under `kpis.pending_reports`.
- **Mixed candidate names / "(Mock)" data:** scope selector lets the user isolate a name.

---

## 7. Build order (phased)

1. **Backend:** `internal/dashboard` aggregator + `handleDashboard` + route + unit tests.
2. **Frontend scaffold:** nav item, `/dashboard` route, `getDashboard()` + types, add Recharts.
3. **Phase-A charts** (need only list + reports): KPI strip, verdict donut, confidence-over-time,
   coverage bars, risk stacked bar.
4. **Phase-B charts:** competency trends, radar profile, persona comparison, scatter, score
   evolution.
5. **Zoom modal + brush + empty states.**
6. **(Follow-up)** stable candidate identity so trends are truly per-person.

---

## 8. Delivery

- Branch: `feat/progress-dashboard` off the updated `main`.
- Suggested split into two PRs for reviewability:
  - **PR A — backend:** `internal/dashboard`, handler, route, tests.
  - **PR B — frontend:** route, nav, Recharts, charts, zoom.
- No AI attribution in commits/PRs (per saved preference).

---

## 9. Rough effort

- Backend endpoint + aggregator + tests: ~0.5 day
- Frontend scaffold + Phase-A: ~1 day
- Phase-B + zoom modal: ~1–1.5 days
- (Optional) stable candidate identity: ~0.5 day

---

## 10. Files touched (anticipated)

**New**
- `internal/dashboard/aggregate.go`
- `internal/dashboard/aggregate_test.go`
- `internal/api/dashboard_handlers.go`
- `web/app/dashboard/page.tsx`
- `web/components/charts/*` (small wrappers around Recharts + the zoom modal)

**Modified**
- `internal/api/server.go` (route)
- `web/components/Header.tsx` (nav item)
- `web/lib/api.ts` (client method + types)
- `web/package.json` (Recharts)
- `TODO.md` (mark dashboard in progress; add "stable candidate identity" follow-up)
