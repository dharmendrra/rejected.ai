// Local fixture matching the GET /api/dashboard contract. Used only as a
// development/fallback aid so the page renders when the backend endpoint is
// unavailable. The live page always calls api.getDashboard() first.

import type { Dashboard } from "@/lib/api";

export const sampleDashboard: Dashboard = {
  generated_at: new Date().toISOString(),
  scope: { candidate: "all", from: null, to: null },
  kpis: {
    total_interviews: 6,
    completed_reports: 5,
    pending_reports: 1,
    questions_asked: 48,
    questions_answered: 41,
    avg_confidence: 0.68,
    most_improved_competency: "System Design",
    candidates: 3,
  },
  verdict_mix: [
    { decision: "strong_hire", count: 1 },
    { decision: "hire", count: 2 },
    { decision: "hire_with_risks", count: 1 },
    { decision: "borderline", count: 1 },
    { decision: "no_hire", count: 0 },
  ],
  confidence_over_time: [
    { interview_id: "i1", at: "2026-04-02T10:00:00Z", confidence: 0.55, decision: "borderline", level: "Senior Engineer", type: "System Design", rigor_percent: 60, candidate_name: "Ada Lovelace", job_title: "Backend Engineer" },
    { interview_id: "i2", at: "2026-04-18T10:00:00Z", confidence: 0.62, decision: "hire_with_risks", level: "Senior Engineer", type: "Coding", rigor_percent: 70, candidate_name: "Ada Lovelace", job_title: "Backend Engineer" },
    { interview_id: "i3", at: "2026-05-04T10:00:00Z", confidence: 0.71, decision: "hire", level: "Senior Engineer", type: "System Design", rigor_percent: 75, candidate_name: "Ada Lovelace", job_title: "Backend Engineer" },
    { interview_id: "i4", at: "2026-05-20T10:00:00Z", confidence: 0.66, decision: "hire", level: "Staff Engineer", type: "Behavioral", rigor_percent: 65, candidate_name: "Grace Hopper", job_title: "Platform Lead" },
    { interview_id: "i5", at: "2026-06-01T10:00:00Z", confidence: 0.82, decision: "strong_hire", level: "Staff Engineer", type: "System Design", rigor_percent: 80, candidate_name: "Grace Hopper", job_title: "Platform Lead" },
  ],
  competency_trends: [
    {
      competency: "System Design",
      direction: "improving",
      first: 0.5,
      latest: 0.8,
      delta: 0.3,
      points: [
        { interview_id: "i1", at: "2026-04-02T10:00:00Z", normal: 0.5, confidence: 0.55 },
        { interview_id: "i3", at: "2026-05-04T10:00:00Z", normal: 0.68, confidence: 0.71 },
        { interview_id: "i5", at: "2026-06-01T10:00:00Z", normal: 0.8, confidence: 0.82 },
      ],
    },
    {
      competency: "Communication",
      direction: "stable",
      first: 0.7,
      latest: 0.72,
      delta: 0.02,
      points: [
        { interview_id: "i1", at: "2026-04-02T10:00:00Z", normal: 0.7, confidence: 0.55 },
        { interview_id: "i3", at: "2026-05-04T10:00:00Z", normal: 0.71, confidence: 0.71 },
        { interview_id: "i5", at: "2026-06-01T10:00:00Z", normal: 0.72, confidence: 0.82 },
      ],
    },
    {
      competency: "Concurrency",
      direction: "declining",
      first: 0.65,
      latest: 0.5,
      delta: -0.15,
      points: [
        { interview_id: "i1", at: "2026-04-02T10:00:00Z", normal: 0.65, confidence: 0.55 },
        { interview_id: "i3", at: "2026-05-04T10:00:00Z", normal: 0.58, confidence: 0.71 },
        { interview_id: "i5", at: "2026-06-01T10:00:00Z", normal: 0.5, confidence: 0.82 },
      ],
    },
  ],
  competency_profile: [
    { competency: "System Design", cool: 0.4, normal: 0.8, hot: 0.9, confidence: 0.82, first_normal: 0.5 },
    { competency: "Communication", cool: 0.5, normal: 0.72, hot: 0.85, confidence: 0.8, first_normal: 0.7 },
    { competency: "Concurrency", cool: 0.3, normal: 0.5, hot: 0.6, confidence: 0.6, first_normal: 0.65 },
    { competency: "Databases", cool: 0.45, normal: 0.66, hot: 0.78, confidence: 0.7, first_normal: 0.6 },
    { competency: "Testing", cool: 0.5, normal: 0.7, hot: 0.8, confidence: 0.72, first_normal: 0.55 },
  ],
  rigor_vs_confidence: [
    { interview_id: "i1", rigor_percent: 60, confidence: 0.55, decision: "borderline", type: "System Design" },
    { interview_id: "i2", rigor_percent: 70, confidence: 0.62, decision: "hire_with_risks", type: "Coding" },
    { interview_id: "i3", rigor_percent: 75, confidence: 0.71, decision: "hire", type: "System Design" },
    { interview_id: "i4", rigor_percent: 65, confidence: 0.66, decision: "hire", type: "Behavioral" },
    { interview_id: "i5", rigor_percent: 80, confidence: 0.82, decision: "strong_hire", type: "System Design" },
  ],
  coverage: {
    by_type: [
      { key: "System Design", count: 3 },
      { key: "Coding", count: 1 },
      { key: "Behavioral", count: 1 },
      { key: "HR Round", count: 1 },
    ],
    by_level: [
      { key: "Senior Engineer", count: 3 },
      { key: "Staff Engineer", count: 2 },
      { key: "Engineer", count: 1 },
    ],
  },
  risks: [
    { category: "missing", severity: "high", count: 2 },
    { category: "missing", severity: "medium", count: 3 },
    { category: "missing", severity: "low", count: 1 },
    { category: "weak", severity: "high", count: 1 },
    { category: "weak", severity: "medium", count: 4 },
    { category: "weak", severity: "low", count: 2 },
    { category: "jd_risk", severity: "medium", count: 1 },
    { category: "jd_risk", severity: "low", count: 2 },
  ],
  top_signals: [
    { name: "Clear tradeoff reasoning", count: 5 },
    { name: "Strong fundamentals", count: 4 },
    { name: "Structured communication", count: 3 },
    { name: "Production mindset", count: 2 },
    { name: "Curiosity", count: 2 },
  ],
  persona_competency: [
    {
      persona: "System Architect",
      competencies: [
        { competency: "System Design", avg_score: 0.82 },
        { competency: "Concurrency", avg_score: 0.55 },
        { competency: "Databases", avg_score: 0.7 },
      ],
    },
    {
      persona: "Pragmatist",
      competencies: [
        { competency: "System Design", avg_score: 0.7 },
        { competency: "Concurrency", avg_score: 0.6 },
        { competency: "Databases", avg_score: 0.65 },
      ],
    },
  ],
  score_evolution: [
    {
      interview_id: "i5",
      candidate_name: "Grace Hopper",
      type: "System Design",
      series: [
        { turn: 1, avg_normal: 0.6 },
        { turn: 2, avg_normal: 0.66 },
        { turn: 3, avg_normal: 0.72 },
        { turn: 4, avg_normal: 0.78 },
        { turn: 5, avg_normal: 0.8 },
      ],
    },
    {
      interview_id: "i3",
      candidate_name: "Ada Lovelace",
      type: "System Design",
      series: [
        { turn: 1, avg_normal: 0.5 },
        { turn: 2, avg_normal: 0.58 },
        { turn: 3, avg_normal: 0.64 },
        { turn: 4, avg_normal: 0.68 },
      ],
    },
  ],
};
