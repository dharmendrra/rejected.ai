// Typed client for the rejected.ai Go backend.
const BASE = process.env.NEXT_PUBLIC_API_BASE || "http://localhost:8080";

// ─── Types (mirror internal/domain) ──────────────────────────────────────────

export interface JobDescription {
  id: string;
  title: string;
  required_skills: string[];
  technical_expectations: string[];
}

export interface CandidateProfile {
  id: string;
  name: string;
  technologies: string[];
}

export interface ValidationTarget {
  competency: string;
  reason: string;
  priority: number;
}

export interface CapabilityGraphSet {
  candidate: { name: string; category: string; evidence: string[]; strength: number }[];
  target: { name: string; category: string; importance: string; weight: number }[];
  strengths: string[];
  gaps: string[];
  unknowns: string[];
  risk_areas: string[];
  validation_targets: ValidationTarget[];
}

export interface Interview {
  id: string;
  level: string;
  type: string;
  duration_min: number;
  status: string;
  competencies: string[];
}

export interface Turn {
  id: string;
  turn: number;
  kind: string;
  question: string;
  target_competencies: string[];
  answer: string;
  answered: boolean;
  compression_ratio: number;
  assumptions?: string[];
  response_type?: string;
  response_reasoning?: string;
}

export interface Revision {
  at_turn: number;
  old_strength: number;
  new_strength: number;
  note: string;
}

export interface EvidenceItem {
  id: string;
  turn: number;
  competency: string;
  concepts: string[];
  polarity: string;
  strength: number;
  supporting_quote: string;
  interpretation: string;
  revisions?: Revision[];
}

export interface ConfidenceSnapshot {
  competency: string;
  turn: number;
  confidence: number;
  cool: number;
  normal: number;
  hot: number;
  evidence_count: number;
  evidence_turns: number[];
  rationale: string;
}

export interface CreateResult {
  interview: Interview;
  graphs: CapabilityGraphSet;
  question: Turn;
}

export interface AnswerResult {
  turn: Turn;
  evidence: EvidenceItem[];
  snapshots: ConfidenceSnapshot[];
  next?: Turn;
  completed: boolean;
}

export interface InterviewView {
  interview: Interview;
  graphs: CapabilityGraphSet | null;
  turns: Turn[];
  evidence: EvidenceItem[];
  confidence: ConfidenceSnapshot[];
}

export interface CompetencyScore {
  competency: string;
  confidence: number;
  cool: number;
  normal: number;
  hot: number;
  evidence_turns: number[];
  rationale: string;
}

export interface StrongestSignal {
  name: string;
  description: string;
  evidence_turns: number[];
}

export interface RiskItem {
  competency: string;
  category: string;
  severity: string;
  reason: string;
  evidence_turns: number[];
}

export interface PersonaView {
  persona: string;
  overall_take: string;
  endorsements: string[];
  concerns: string[];
  per_competency: { competency: string; score: number; reasoning: string }[];
}

export interface Recommendation {
  decision: string;
  confidence_level: number;
  reasoning: string;
  citations: { competency: string; turns: number[]; note: string }[];
  personas: PersonaView[];
}

export interface Report {
  interview: Interview;
  competency_scores: CompetencyScore[];
  signals: StrongestSignal[];
  risks: RiskItem[];
  recommendation: Recommendation | null;
}

// ─── Calls ───────────────────────────────────────────────────────────────────

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers: { "Content-Type": "application/json", ...(init?.headers || {}) },
  });
  if (!res.ok) {
    let msg = `${res.status} ${res.statusText}`;
    try {
      const body = await res.json();
      if (body?.error) msg = body.error;
    } catch {
      /* ignore */
    }
    throw new Error(msg);
  }
  return res.json() as Promise<T>;
}

export const api = {
  ingestJD: (raw: string) =>
    req<JobDescription>("/api/job-descriptions", { method: "POST", body: JSON.stringify({ raw }) }),
  ingestResume: (raw: string) =>
    req<CandidateProfile>("/api/resumes", { method: "POST", body: JSON.stringify({ raw }) }),
  createInterview: (body: {
    job_description_id: string;
    candidate_profile_id: string;
    level: string;
    type: string;
    duration_min: number;
  }) => req<CreateResult>("/api/interviews", { method: "POST", body: JSON.stringify(body) }),
  submitAnswer: (id: string, answer: string) =>
    req<AnswerResult>(`/api/interviews/${id}/answer`, { method: "POST", body: JSON.stringify({ answer }) }),
  getInterview: (id: string) => req<InterviewView>(`/api/interviews/${id}`),
  generateReport: (id: string) => req<Report>(`/api/interviews/${id}/report`, { method: "POST" }),
  getReport: (id: string) => req<Report>(`/api/interviews/${id}/report`),
};
