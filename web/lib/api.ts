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
  rigor_percent: number;
  status: string;
  competencies: string[];
  created_at: string;
  updated_at: string;
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
  candidate_name: string;
  job_title: string;
  jd_raw: string;
  completed_at?: string;
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

export interface IdealResponse {
  question: string;
  competency: string;
  key_points: string[];
  sample_answer: string;
}

export interface CoachingItem {
  title: string;
  category: string;
  severity: string;
  description: string;
  target_level?: string;
  observed_level?: string;
  action_points?: string[];
}

export interface ReportStep {
  name: string;
  status: string;
}

export interface ReportProgress {
  id: string;
  interview_id: string;
  status: string;
  total_steps: number;
  completed_steps: number;
  current_step: string;
  steps: ReportStep[];
  error?: string;
  created_at: string;
  updated_at: string;
}

export interface Report {
  interview: Interview;
  competency_scores: CompetencyScore[];
  signals: StrongestSignal[];
  risks: RiskItem[];
  recommendation: Recommendation | null;
  ideal_responses?: IdealResponse[];
  coaching_items?: CoachingItem[];
  status?: string;
  progress?: ReportProgress;
}

// ─── Calls ───────────────────────────────────────────────────────────────────

async function req<T>(path: string, init?: RequestInit): Promise<T> {
  const isFormData = init?.body instanceof FormData;
  const headers = { ...(init?.headers || {}) } as Record<string, string>;
  if (!isFormData && !headers["Content-Type"]) {
    headers["Content-Type"] = "application/json";
  }
  const res = await fetch(`${BASE}${path}`, {
    ...init,
    headers,
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
  ingestResume: (rawOrFile: string | File) => {
    if (rawOrFile instanceof File) {
      const fd = new FormData();
      fd.append("file", rawOrFile);
      return req<CandidateProfile>("/api/resumes", { method: "POST", body: fd });
    }
    return req<CandidateProfile>("/api/resumes", { method: "POST", body: JSON.stringify({ raw: rawOrFile }) });
  },
  createInterview: (body: {
    job_description_id: string;
    candidate_profile_id: string;
    level: string;
    type: string;
    duration_min: number;
    rigor_percent: number;
  }) => req<CreateResult>("/api/interviews", { method: "POST", body: JSON.stringify(body) }),
  submitAnswer: (id: string, answer: string) =>
    req<AnswerResult>(`/api/interviews/${id}/answer`, { method: "POST", body: JSON.stringify({ answer }) }),
  uploadAudio: (id: string, turn: number, file: Blob, durationSec: number, latencyMs: number) => {
    const fd = new FormData();
    fd.append("file", file, "audio.webm");
    fd.append("turn", String(turn));
    fd.append("duration_sec", String(durationSec));
    fd.append("latency_ms", String(latencyMs));
    return req<any>(`/api/interviews/${id}/audio`, { method: "POST", body: fd });
  },
  uploadVideo: (id: string, turn: number, file: Blob, latencyMs: number) => {
    const fd = new FormData();
    fd.append("file", file, "video.webm");
    fd.append("turn", String(turn));
    fd.append("latency_ms", String(latencyMs));
    return req<any>(`/api/interviews/${id}/video`, { method: "POST", body: fd });
  },
  getInterview: (id: string) => req<InterviewView>(`/api/interviews/${id}`),
  generateReport: (id: string) => req<Report>(`/api/interviews/${id}/report`, { method: "POST" }),
  getReport: (id: string) => req<Report>(`/api/interviews/${id}/report`),
  listInterviews: () => req<any[]>("/api/interviews"),
  deleteInterview: (id: string) => req<{ status: string }>(`/api/interviews/${id}`, { method: "DELETE" }),
};
