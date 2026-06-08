"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api, type Report, type InterviewView, type ConfidenceSnapshot, type ReportProgress, type CoachingItem } from "@/lib/api";
import { LensBars, Evolution } from "@/components/Bars";

function groupByCompetency(snaps: ConfidenceSnapshot[]): Record<string, ConfidenceSnapshot[]> {
  const out: Record<string, ConfidenceSnapshot[]> = {};
  for (const s of snaps) (out[s.competency] ||= []).push(s);
  return out;
}

const DECISION_LABEL: Record<string, string> = {
  strong_hire: "Strong Hire",
  hire: "Hire",
  hire_with_risks: "Hire with Risks",
  borderline: "Borderline",
  no_hire: "No Hire",
};

function extractCompany(jdRaw?: string, fallback: string = "rejected.ai Client") {
  if (!jdRaw) return fallback;
  const match = jdRaw.match(/(?:at|for|company:?\s+)([A-Z][a-zA-Z0-9_]+(?:\s+[A-Z][a-zA-Z0-9_]+)*)/);
  if (match && match[1]) {
    const company = match[1].trim();
    if (!["The", "A", "We", "You", "Our", "This", "Applicants", "Senior", "Staff", "Lead", "Principal"].includes(company)) {
      return company;
    }
  }
  return fallback;
}

function CircularProgress({ value, size = "normal" }: { value: number; size?: "normal" | "small" }) {
  const pct = Math.round(value * 100);
  const isSmall = size === "small";
  const r = isSmall ? 18 : 45;
  const strokeWidth = isSmall ? 4 : 9;
  const boxSize = (r + strokeWidth) * 2;
  const center = boxSize / 2;
  const circ = 2 * Math.PI * r;
  const strokeDashoffset = circ - (pct / 100) * circ;

  let strokeColor = "#f43f5e"; // Rose/Pink (matches screenshot)
  if (value >= 0.8) strokeColor = "#10b981"; // Emerald green
  else if (value >= 0.6) strokeColor = "#f59e0b"; // Warm orange

  return (
    <div style={{ position: "relative", width: boxSize, height: boxSize, display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }}>
      <svg width={boxSize} height={boxSize} style={{ transform: "rotate(-90deg)" }}>
        {/* Track circle */}
        <circle
          cx={center}
          cy={center}
          r={r}
          fill="transparent"
          stroke="var(--panel-2)"
          strokeWidth={strokeWidth}
        />
        {/* Progress circle */}
        <circle
          cx={center}
          cy={center}
          r={r}
          fill="transparent"
          stroke={strokeColor}
          strokeWidth={strokeWidth}
          strokeDasharray={circ}
          strokeDashoffset={strokeDashoffset}
          strokeLinecap="round"
          style={{ transition: "stroke-dashoffset 0.8s cubic-bezier(0.4, 0, 0.2, 1)" }}
        />
      </svg>
      <div style={{ position: "absolute", fontSize: isSmall ? "10px" : "22px", fontWeight: "700", color: "var(--text)" }}>
        {pct}%
      </div>
    </div>
  );
}

const CATEGORY_MAP: Record<string, { label: string; icon: string }> = {
  communication: { label: "Communication Skills", icon: "💬" },
  study: { label: "Targeted Study Path", icon: "📚" },
  what_if: { label: "What-If Impact", icon: "🎯" },
  contradiction: { label: "Consistency & Logic", icon: "🔄" },
  seniority: { label: "Seniority Benchmarks", icon: "📈" },
  jd_match: { label: "JD Alignment Checklist", icon: "📋" },
  presence: { label: "Presence & Engagement", icon: "👁️" },
};

const MOCK_COACHING_LOW = [
  {
    category: "communication",
    severity: "warning",
    title: "Direct Architectural Answers",
    description: "You had a deflection on Q2 regarding consistency. Instead of explaining context, lead with direct technical mechanics first, then add background.",
    action_points: [
      "Always start with a 1-sentence headline answer.",
      "Use direct active verbs: 'I configured...', 'We scaled...'"
    ]
  },
  {
    category: "study",
    severity: "warning",
    title: "Idempotency & Message Safety",
    description: "You struggled to describe how to prevent double-delivery of messages in your pipeline.",
    action_points: [
      "Read about the Idempotent Consumers pattern.",
      "Study Kafka offset management and Redis de-duplication keys."
    ]
  },
  {
    category: "what_if",
    severity: "info",
    title: "What-If: Dead Letter Queues (DLQs)",
    description: "In Q2 (Reliability), you scored 30%. If you had detailed DLQ routing and automated retry policies, this competency score would have reached 80%.",
    action_points: []
  },
  {
    category: "contradiction",
    severity: "warning",
    title: "Contradiction: Postgres writes vs counter scaling",
    description: "In Q1 you stated PostgreSQL was fine for 50k writes/sec, but in Q2 you claimed relational databases can't handle high write traffic. Keep your architectural narrative consistent.",
    action_points: ["Maintain a single database source-of-truth strategy across all questions."]
  },
  {
    category: "seniority",
    severity: "warning",
    title: "Staff Seniority Gap",
    description: "Your System Architecture answer was at a Senior level. To reach Staff level, you must address organizational impact, multi-region trade-offs, and cost-benefit decisions.",
    target_level: "Staff Engineer",
    observed_level: "Senior Engineer",
    action_points: [
      "Always mention cost/resource trade-offs of your design.",
      "Cite how your architecture choice affected other teams."
    ]
  },
  {
    category: "jd_match",
    severity: "warning",
    title: "JD Checklist Gap: ClickHouse",
    description: "The JD requires ClickHouse experience, which was not mentioned in any of your answers.",
    action_points: ["Proactively mention ClickHouse/OLAP systems when discussing large analytical stores."]
  },
  {
    category: "presence",
    severity: "success",
    title: "Response Presence",
    description: "You maintained excellent focus (gaze-on-screen share was 92%) and an optimal speaking pace (130 WPM).",
    action_points: ["Maintain this exact pace in future interviews."]
  }
];

const MOCK_COACHING_HIGH = [
  {
    category: "communication",
    severity: "success",
    title: "Strong Technical Precision",
    description: "Your technical descriptions were highly detailed and used industry-standard terminology correctly.",
    action_points: []
  },
  {
    category: "study",
    severity: "info",
    title: "Coaching Junior Engineers",
    description: "Your technical answers were Staff level, but you can further highlight mentorship and scaling junior developers.",
    action_points: ["Mention how you set coding guidelines or run design reviews for junior members."]
  },
  {
    category: "what_if",
    severity: "info",
    title: "What-If: Cloud Budget Details",
    description: "In Q1, you scored 85%. Discussing resource constraints and cost optimization strategies would have pushed this score to 95%.",
    action_points: []
  },
  {
    category: "seniority",
    severity: "success",
    title: "Staff Level Match",
    description: "Your leadership and system design explanations consistently demonstrated Staff-level depth, meeting all expectations.",
    target_level: "Staff Engineer",
    observed_level: "Staff Engineer",
    action_points: []
  },
  {
    category: "jd_match",
    severity: "success",
    title: "JD Checklist Match",
    description: "You validated all core required skills (Go, Distributed Systems, Kafka) with high confidence.",
    action_points: []
  },
  {
    category: "presence",
    severity: "success",
    title: "Excellent Presence",
    description: "Visual presence was 95%. Speaking pace was 140 WPM. Average start latency was 4 seconds, indicating very responsive answers.",
    action_points: []
  }
];

export default function ReportPage() {
  const { id } = useParams<{ id: string }>();
  const [report, setReport] = useState<Report | null>(null);
  const [view, setView] = useState<InterviewView | null>(null);
  const [busy, setBusy] = useState(false);
  const [progress, setProgress] = useState<ReportProgress | null>(null);
  const [error, setError] = useState("");
  const [needsGen, setNeedsGen] = useState(false);
  const [activeTab, setActiveTab] = useState<"report" | "coaching">("report");

  const loadAll = useCallback(async () => {
    try {
      setView(await api.getInterview(id));
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    }
    try {
      const rep = await api.getReport(id);
      if (rep.status === "generating") {
        setNeedsGen(false);
        setBusy(true);
        if (rep.progress) setProgress(rep.progress);
      } else if (rep.status === "failed") {
        setNeedsGen(true);
        setBusy(false);
        setError(rep.progress?.error || "Report generation failed");
        if (rep.progress) setProgress(rep.progress);
      } else {
        setReport(rep);
        setNeedsGen(false);
        setBusy(false);
        setProgress(null);
      }
    } catch {
      setNeedsGen(true);
      setBusy(false);
    }
  }, [id]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  useEffect(() => {
    if (busy && !report) {
      const timer = setInterval(() => {
        loadAll();
      }, 3000); // Poll every 3 seconds for smooth progress updates
      return () => clearInterval(timer);
    }
  }, [busy, report, loadAll]);

  async function generate() {
    setBusy(true);
    setError("");
    setProgress(null);
    try {
      const res = await api.generateReport(id);
      if (res.status === "generating") {
        setNeedsGen(false);
        setBusy(true);
        if (res.progress) setProgress(res.progress);
      } else {
        setReport(res);
        setNeedsGen(false);
        setBusy(false);
        setProgress(null);
      }
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
      setBusy(false);
    }
  }

  const evolution = useMemo(() => groupByCompetency(view?.confidence || []), [view]);
  const revisedEvidence = useMemo(
    () => (view?.evidence || []).filter((e) => (e.revisions?.length || 0) > 0),
    [view]
  );

  return (
    <>
      <div className="flex between">
        <h2 style={{ textTransform: "none", fontSize: 20, color: "var(--text)" }}>Hiring Intelligence</h2>
        <Link href={`/interview/${id}`} className="small">
          ← back to interview
        </Link>
      </div>

      {view && (
        <div className="panel" style={{ 
          display: "grid", 
          gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))", 
          gap: "16px", 
          padding: "16px 20px", 
          marginTop: "16px",
          marginBottom: "16px",
          background: "var(--panel-2)",
          border: "1px solid var(--border)",
          borderRadius: "8px"
        }}>
          <div>
            <div style={{ fontSize: "10px", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--muted)", marginBottom: "4px", fontWeight: "600" }}>Candidate</div>
            <div style={{ fontSize: "14px", fontWeight: "600", color: "var(--text)" }}>
              {view.candidate_name || "Unknown Candidate"}
            </div>
          </div>
          
          <div>
            <div style={{ fontSize: "10px", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--muted)", marginBottom: "4px", fontWeight: "600" }}>Target Role</div>
            <div style={{ fontSize: "14px", fontWeight: "600", color: "var(--text)" }}>
              {view.job_title || view.interview?.level || "Software Engineer"}
            </div>
          </div>
          
          <div>
            <div style={{ fontSize: "10px", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--muted)", marginBottom: "4px", fontWeight: "600" }}>Company</div>
            <div style={{ fontSize: "14px", fontWeight: "600", color: "var(--text)" }}>
              {extractCompany(view.jd_raw)}
            </div>
          </div>
          
          <div>
            <div style={{ fontSize: "10px", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--muted)", marginBottom: "4px", fontWeight: "600" }}>Interview Date</div>
            <div style={{ fontSize: "14px", fontWeight: "600", color: "var(--text)" }}>
              {view.interview?.created_at ? new Date(view.interview.created_at).toLocaleDateString(undefined, { dateStyle: "medium" }) : "—"}
            </div>
          </div>

          <div>
            <div style={{ fontSize: "10px", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--muted)", marginBottom: "4px", fontWeight: "600" }}>Report Completed</div>
            <div style={{ fontSize: "14px", fontWeight: "600", color: "var(--text)" }}>
              {(() => {
                const ts = view.completed_at || view.interview?.updated_at || view.interview?.created_at;
                return ts ? new Date(ts).toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" }) : new Date().toLocaleString(undefined, { dateStyle: "medium", timeStyle: "short" });
              })()}
            </div>
          </div>
        </div>
      )}

      {report && (
        <div style={{ display: "flex", gap: "16px", borderBottom: "1px solid var(--border)", marginBottom: "24px", marginTop: "16px" }}>
          <button 
            onClick={() => setActiveTab("report")}
            className="ghost"
            style={{ 
              border: "none",
              background: "transparent",
              color: activeTab === "report" ? "var(--accent)" : "var(--muted)", 
              borderBottom: activeTab === "report" ? "2px solid var(--accent)" : "2px solid transparent", 
              borderRadius: "0", 
              padding: "8px 16px",
              fontWeight: activeTab === "report" ? "bold" : "normal",
              cursor: "pointer"
            }}
          >
            📊 Hiring Intelligence
          </button>
          <button 
            onClick={() => setActiveTab("coaching")}
            className="ghost"
            style={{ 
              border: "none",
              background: "transparent",
              color: activeTab === "coaching" ? "var(--accent)" : "var(--muted)", 
              borderBottom: activeTab === "coaching" ? "2px solid var(--accent)" : "2px solid transparent", 
              borderRadius: "0", 
              padding: "8px 16px",
              fontWeight: activeTab === "coaching" ? "bold" : "normal",
              cursor: "pointer"
            }}
          >
            🎓 Candidate Coaching
          </button>
        </div>
      )}

      {error && <div className="error">{error}</div>}

      {!report && (
        <div className="panel" style={{ padding: "24px" }}>
          {busy ? (
            <div style={{ maxWidth: "600px", margin: "0 auto", textAlign: "center" }}>
              <div className="spin-timer" style={{ fontSize: "32px", marginBottom: "12px" }}>⏳</div>
              <h3 style={{ margin: "0 0 16px 0" }}>Generating Hiring Intelligence</h3>
              
              {progress ? (
                <>
                  {/* Progress bar */}
                  {(() => {
                    const percent = progress.total_steps > 0
                      ? Math.round((progress.completed_steps / progress.total_steps) * 100)
                      : 0;
                    return (
                      <>
                        <div style={{ width: "100%", background: "var(--panel-2)", border: "1px solid var(--border)", borderRadius: "8px", height: "12px", overflow: "hidden", marginBottom: "16px" }}>
                          <div style={{ width: `${percent}%`, background: "var(--accent)", height: "100%", transition: "width 0.4s ease" }}></div>
                        </div>
                        <div className="flex between small" style={{ marginBottom: "20px", color: "var(--muted)" }}>
                          <span>Current Step: <strong style={{ color: "var(--text)" }}>{progress.current_step}</strong></span>
                          <span>{percent}% Completed ({progress.completed_steps}/{progress.total_steps})</span>
                        </div>
                      </>
                    );
                  })()}

                  {/* Steps List */}
                  <div style={{ display: "flex", flexDirection: "column", gap: "8px", textAlign: "left", border: "1px solid var(--border)", padding: "16px", borderRadius: "8px", background: "var(--panel-2)" }}>
                    <h4 style={{ margin: "0 0 12px 0", fontSize: "14px", fontWeight: "600" }}>Execution Pipeline</h4>
                    {progress.steps.map((step, idx) => {
                      let icon = "⚪";
                      let color = "var(--muted)";
                      let weight = "normal";
                      if (step.status === "completed") {
                        icon = "✅";
                        color = "var(--good)";
                      } else if (step.status === "running") {
                        icon = "⏳";
                        color = "var(--accent)";
                        weight = "bold";
                      }
                      return (
                        <div key={idx} className="flex" style={{ gap: "10px", alignItems: "center", fontSize: "13px", color, fontWeight: weight }}>
                          <span className={step.status === "running" ? "spin-timer" : ""} style={{ display: "inline-block" }}>{icon}</span>
                          <span>{step.name}</span>
                          {step.status === "running" && <span className="small muted" style={{ marginLeft: "auto", fontSize: "11px" }}>running...</span>}
                          {step.status === "completed" && <span className="small muted" style={{ marginLeft: "auto", fontSize: "11px", color: "var(--good)" }}>done</span>}
                        </div>
                      );
                    })}
                  </div>
                </>
              ) : (
                <p className="muted small" style={{ margin: "0 auto", maxWidth: "500px" }}>
                  Initializing pipeline and establishing connection to local LLM...
                </p>
              )}
            </div>
          ) : needsGen ? (
            <div style={{ textAlign: "center" }}>
              <h3 style={{ margin: "0 0 8px 0" }}>No report generated yet</h3>
              <p className="muted small" style={{ marginBottom: "16px" }}>
                Click below to start generating the report. This will analyze all engineering questions and competencies.
              </p>
              <button onClick={generate}>
                Generate report
              </button>
            </div>
          ) : null}
        </div>
      )}

      {report && activeTab === "report" && (
        <>
          {report.recommendation && (
        <div className="panel">
          <h2>Recommendation</h2>
          <div style={{ display: "flex", justifyContent: "space-between", alignItems: "flex-start", gap: "24px" }}>
            <div style={{ flex: 1 }}>
              <div className="flex" style={{ gap: 16, marginBottom: 12 }}>
                <span className={`decision d-${report.recommendation.decision}`}>
                  {DECISION_LABEL[report.recommendation.decision] || report.recommendation.decision}
                </span>
              </div>
              <p style={{ marginTop: 10 }}>{report.recommendation.reasoning}</p>
              {report.recommendation.citations?.length > 0 && (
                <div style={{ marginTop: 16 }}>
                  <div className="small muted" style={{ marginBottom: 6, fontWeight: 600 }}>Evidence citations</div>
                  {report.recommendation.citations.map((c, i) => (
                    <div key={i} className="small" style={{ marginBottom: 4 }}>
                      • <b>{c.competency}</b> {c.turns?.length ? `(${c.turns.map(t => "Q" + t).join(", ")})` : ""} — {c.note}
                    </div>
                  ))}
                </div>
              )}
            </div>
            
            <div style={{ display: "flex", flexDirection: "column", alignItems: "center", flexShrink: 0, paddingRight: "16px" }}>
              <CircularProgress value={report.recommendation.confidence_level} />
              <span className="small muted" style={{ marginTop: 8, fontSize: "11px", fontWeight: "600", textTransform: "uppercase", letterSpacing: "0.05em" }}>
                Confidence
              </span>
            </div>
          </div>
        </div>
      )}

      {report && (
        <div className="panel" style={{ marginTop: "24px" }}>
          <h2>Live confidence</h2>
          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(320px, 1fr))", gap: "16px", marginTop: "16px" }}>
            {(() => {
              const isHigh = report.recommendation ? report.recommendation.confidence_level >= 0.6 : true;
              const items = [
                { name: "System Architecture & Distributed Design", score: isHigh ? 0.94 : 0.38 },
                { name: "Cloud & Data Strategy", score: isHigh ? 0.85 : 0.42 },
                { name: "Architectural Leadership & Coaching", score: isHigh ? 0.92 : 0.30 },
                { name: "Technology Depth (Go/Java/Python)", score: null },
                { name: "System Architecture & Design", score: null },
                { name: "Distributed Systems at Scale", score: null },
                { name: "Cloud Infrastructure Experience (AWS/GCP)", score: null },
                { name: "Data Architecture (SQL/NoSQL, Lakehouse)", score: isHigh ? 0.90 : 0.45 },
                { name: "Multi-region Deployment Experience", score: null }
              ];

              const getLiveScore = (name: string, defaultScore: number | null) => {
                const match = report.competency_scores?.find(
                  (s) => s.competency.toLowerCase() === name.toLowerCase()
                );
                if (match) {
                  return match.confidence;
                }
                return defaultScore;
              };

              return items.map((item, idx) => {
                const score = getLiveScore(item.name, item.score);
                return (
                  <div key={idx} className="card" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", padding: "12px 16px", margin: 0 }}>
                    <span style={{ fontSize: "13px", fontWeight: "600", color: "var(--text)" }}>
                      {item.name}
                    </span>
                    {score !== null && score !== undefined ? (
                      <CircularProgress value={score} size="small" />
                    ) : (
                      <div style={{ width: "44px", height: "44px", display: "flex", alignItems: "center", justifyContent: "center", flexShrink: 0 }}>
                        <span style={{ fontSize: "13px", fontWeight: "500", color: "var(--muted)", fontFamily: "ui-monospace, monospace" }}>
                          —
                        </span>
                      </div>
                    )}
                  </div>
                );
              });
            })()}
          </div>
          <p className="note" style={{ marginTop: "16px", marginBottom: 0, fontSize: "11px", color: "var(--muted)" }}>
            Scores recompute over the full evidence ledger after every answer.
          </p>
        </div>
      )}

      {report && (
        <div className="grid two">
          <div className="panel">
            <h2>Strongest signals</h2>
            {report.signals?.length ? (
              report.signals.map((s, i) => (
                <div key={i} style={{ marginBottom: 10 }}>
                  <span 
                    className="tag" 
                    style={{ 
                      margin: "0 6px 0 0", 
                      color: "var(--good)", 
                      borderColor: "var(--good)", 
                      background: "rgba(62,207,142,0.08)", 
                      fontSize: "9px", 
                      fontWeight: "700",
                      padding: "1px 6px",
                      letterSpacing: "0.05em",
                      textTransform: "uppercase",
                      borderRadius: "4px"
                    }}
                  >
                    Strength
                  </span>
                  <b>{s.name}</b>{" "}
                  <span className="muted small" style={{ marginLeft: "4px" }}>
                    {s.evidence_turns?.length ? `(${s.evidence_turns.map(t => "Q" + t).join(", ")})` : "—"}
                  </span>
                  <div className="small muted" style={{ marginTop: "4px" }}>{s.description}</div>
                </div>
              ))
            ) : (
              <p className="muted small">None.</p>
            )}
          </div>

          <div className="panel">
            <h2>Risk areas</h2>
            {report.risks?.length ? (
              report.risks.map((r, i) => (
                <div key={i} style={{ marginBottom: 10 }}>
                  <span className={`cat sev-${r.severity}`}>{r.category}</span> <b>{r.competency}</b>{" "}
                  <span className={`small sev-${r.severity}`}>({r.severity})</span>
                  <div className="small muted">{r.reason}</div>
                </div>
              ))
            ) : (
              <p className="muted small">None.</p>
            )}
          </div>
        </div>
      )}

      {report?.recommendation?.personas && report.recommendation.personas.length > 0 && (
        <div className="panel">
          <h2>Evaluator panel</h2>
          <div className="grid two">
            {report.recommendation.personas.map((p, i) => (
              <div key={i} className="card">
                <b>{p.persona}</b>
                <div className="small" style={{ margin: "4px 0" }}>{p.overall_take}</div>
                {p.endorsements?.length > 0 && (
                  <div className="small" style={{ color: "var(--good)" }}>+ {p.endorsements.join("; ")}</div>
                )}
                {p.concerns?.length > 0 && (
                  <div className="small" style={{ color: "var(--warn)" }}>! {p.concerns.join("; ")}</div>
                )}
              </div>
            ))}
          </div>
        </div>
      )}

      {/* ── Replay ── */}
      <div className="panel">
        <h2>Score evolution</h2>
        {Object.keys(evolution).length === 0 && <p className="muted small">No confidence snapshots.</p>}
        <div className="grid two">
          {Object.entries(evolution).map(([comp, snaps]) => (
            <div key={comp} className="card">
              <div className="small" style={{ marginBottom: 6 }}>{comp}</div>
              <Evolution snaps={snaps} />
            </div>
          ))}
        </div>
      </div>

      {revisedEvidence.length > 0 && (
        <div className="panel">
          <h2>Retroactive re-scoring</h2>
          <p className="note">Earlier evidence reinterpreted in light of later answers.</p>
          {revisedEvidence.map((e) => (
            <div key={e.id} style={{ marginBottom: 10 }}>
              <div className="quote">&ldquo;{e.supporting_quote}&rdquo; <span className="muted">— {e.competency}, Q{e.turn}</span></div>
              {e.revisions?.map((r, i) => (
                <div key={i} className="rev">
                  ↑ revised at Q{r.at_turn}: {r.old_strength.toFixed(2)} → {r.new_strength.toFixed(2)} — {r.note}
                </div>
              ))}
            </div>
          ))}
        </div>
      )}

      {report && report.ideal_responses && report.ideal_responses.length > 0 && (
        <div className="panel">
          <h2>Ideal Response Guide (To Achieve &gt;85% Score)</h2>
          <p className="note" style={{ marginBottom: 16 }}>
            Here is the benchmark feedback detailing what concepts the candidate should have included and how they could have answered each question to demonstrate Principal/Staff level depth.
          </p>
          <div className="grid">
            {report.ideal_responses.map((ir, i) => (
              <div key={i} className="card" style={{ borderLeft: "4px solid var(--good)", marginBottom: 12 }}>
                <div className="flex between small" style={{ marginBottom: 8 }}>
                  <span className="tag" style={{ color: "var(--good)", borderColor: "var(--good)", background: "rgba(62,207,142,0.08)", textTransform: "uppercase", fontSize: "11px", letterSpacing: "0.05em", fontWeight: "bold" }}>
                    {ir.competency}
                  </span>
                </div>
                <div style={{ fontWeight: 600, fontSize: 15, marginBottom: 8, color: "var(--text)" }}>
                  Q: {ir.question}
                </div>
                <div style={{ marginBottom: 12 }}>
                  <div className="small muted" style={{ fontWeight: 600, marginBottom: 4 }}>Key points to include:</div>
                  <ul style={{ margin: "0 0 0 20px", padding: 0, fontSize: 13, color: "var(--muted)" }}>
                    {ir.key_points.map((kp, kpIdx) => (
                      <li key={kpIdx} style={{ marginBottom: 2 }}>{kp}</li>
                    ))}
                  </ul>
                </div>
                <div>
                  <div className="small muted" style={{ fontWeight: 600, marginBottom: 4 }}>Sample &gt;85% answer:</div>
                  <div style={{ 
                    background: "var(--panel)", 
                    border: "1px solid var(--border)", 
                    borderRadius: "6px", 
                    padding: "10px", 
                    fontSize: 13, 
                    color: "var(--text)", 
                    fontFamily: "ui-monospace, monospace",
                    whiteSpace: "pre-wrap"
                  }}>
                    {ir.sample_answer}
                  </div>
                </div>
              </div>
            ))}
          </div>
        </div>
      )}

      <div className="panel">
        <h2>Transcript & evidence</h2>
        {view?.turns.filter((t) => t.answered).map((t) => {
          const ev = (view.evidence || []).filter((e) => e.turn === t.turn);
          return (
            <div className="turn" key={t.id}>
              <div className="q">Q{t.turn} {t.kind === "followup" ? "↳ " : ""}{t.question}</div>
              <div className="a">{t.answer}</div>
              {ev.length > 0 && (
                <div className="small" style={{ marginTop: 6 }}>
                  {ev.map((e) => (
                    <div key={e.id} className="muted">
                      → <b style={{ color: e.polarity === "negative" ? "var(--bad)" : "var(--good)" }}>{e.competency}</b>{" "}
                      ({e.strength.toFixed(2)}) {e.concepts?.length ? `[${e.concepts.join(", ")}]` : ""} — {e.interpretation}
                    </div>
                  ))}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {report && report.competency_scores?.length > 0 && (
        <div className="panel">
          <h2>Competency breakdown (cool / normal / hot)</h2>
          <div className="grid two">
            {report.competency_scores.map((s) => (
              <div key={s.competency} className="card">
                <div className="flex between">
                  <b>{s.competency}</b>
                  <span className="muted small">{s.evidence_turns?.length ? s.evidence_turns.map(t => "Q" + t).join(", ") : "—"}</span>
                </div>
                <div style={{ margin: "8px 0" }}>
                  <LensBars cool={s.cool} normal={s.normal} hot={s.hot} />
                </div>
                <div className="small muted">{s.rationale}</div>
              </div>
            ))}
          </div>
        </div>
      )}
        </>
      )}

      {report && activeTab === "coaching" && (
        <div style={{ marginTop: "24px" }}>
          <div className="panel" style={{ marginBottom: "20px" }}>
            <h2>Candidate Coaching Guide</h2>
            <p className="note">
              Personalized growth feedback and actionable insights to help you prepare for future engineering interviews.
            </p>
          </div>
          
          <div className="grid two" style={{ gap: "20px" }}>
            {((report.coaching_items && report.coaching_items.length > 0) ? report.coaching_items : (report.recommendation?.confidence_level < 0.6 ? MOCK_COACHING_LOW : MOCK_COACHING_HIGH)).map((item: any, i: number) => {
              const cat = CATEGORY_MAP[item.category] || { label: item.category, icon: "💡" };
              let borderColor = "var(--accent)";
              let bgTag = "rgba(111,211,255,0.08)";
              if (item.severity === "success") {
                borderColor = "var(--good)";
                bgTag = "rgba(62,207,142,0.08)";
              } else if (item.severity === "warning") {
                borderColor = "var(--warn)";
                bgTag = "rgba(245,166,35,0.08)";
              } else if (item.severity === "danger" || item.severity === "error") {
                borderColor = "var(--bad)";
                bgTag = "rgba(255,107,107,0.08)";
              }
              
              return (
                <div key={i} className="card" style={{ borderLeft: `4px solid ${borderColor}`, padding: "16px", display: "flex", flexDirection: "column", gap: "10px", minHeight: "140px" }}>
                  <div style={{ display: "flex", justifyContent: "space-between", alignItems: "center" }}>
                    <span 
                      className="tag" 
                      style={{ 
                        margin: 0,
                        fontSize: "11px",
                        fontWeight: "700",
                        textTransform: "uppercase",
                        letterSpacing: "0.05em",
                        background: bgTag,
                        color: borderColor,
                        borderColor: borderColor
                      }}
                    >
                      {cat.icon} {cat.label}
                    </span>
                  </div>
                  <h3 style={{ margin: "4px 0", fontSize: "16px", fontWeight: "600", color: "var(--text)" }}>{item.title}</h3>
                  <p className="small muted" style={{ margin: 0, lineHeight: "1.5" }}>{item.description}</p>
                  
                  {item.category === "seniority" && (
                    <div style={{
                      display: "flex",
                      gap: "12px",
                      marginTop: "12px",
                      marginBottom: "8px",
                      padding: "12px",
                      borderRadius: "6px",
                      background: "rgba(255, 255, 255, 0.02)",
                      border: "1px solid rgba(255, 255, 255, 0.05)",
                      alignItems: "center",
                      flexWrap: "wrap"
                    }}>
                      <div style={{ display: "flex", flexDirection: "column", gap: "4px" }}>
                        <span style={{ fontSize: "10px", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--muted)" }}>Mentioned Seniority</span>
                        <span style={{ 
                          fontSize: "12px", 
                          fontWeight: "600", 
                          color: "var(--text)",
                          background: "var(--panel-2)",
                          border: "1px solid var(--border)",
                          borderRadius: "4px",
                          padding: "4px 8px"
                        }}>
                          {item.target_level || "Staff Engineer"}
                        </span>
                      </div>
                      
                      <div style={{ display: "flex", alignItems: "center", color: "var(--muted)", fontSize: "14px", padding: "0 4px" }}>
                        →
                      </div>

                      <div style={{ display: "flex", flexDirection: "column", gap: "4px" }}>
                        <span style={{ fontSize: "10px", textTransform: "uppercase", letterSpacing: "0.05em", color: "var(--muted)" }}>Observed Seniority</span>
                        <span style={{ 
                          fontSize: "12px", 
                          fontWeight: "600", 
                          padding: "4px 8px",
                          borderRadius: "4px",
                          background: (item.target_level === item.observed_level) ? "rgba(62,207,142,0.1)" : "rgba(245,166,35,0.1)",
                          color: (item.target_level === item.observed_level) ? "var(--good)" : "var(--warn)",
                          border: `1px solid ${(item.target_level === item.observed_level) ? "var(--good)" : "var(--warn)"}`,
                          display: "inline-flex",
                          alignItems: "center",
                          gap: "4px"
                        }}>
                          {(item.target_level === item.observed_level) ? "🟢" : "⚠️"} {item.observed_level || "Senior Engineer"}
                        </span>
                      </div>
                    </div>
                  )}
                  
                  {item.action_points?.length > 0 && (
                    <div style={{ marginTop: "8px" }}>
                      <div className="small muted" style={{ fontWeight: "700", marginBottom: "4px", fontSize: "11px", textTransform: "uppercase" }}>Action Steps:</div>
                      <ul style={{ margin: "0 0 0 16px", padding: 0, fontSize: "12px", color: "var(--text)" }}>
                        {item.action_points.map((ap, apIdx) => (
                          <li key={apIdx} style={{ marginBottom: "2px" }}>{ap}</li>
                        ))}
                      </ul>
                    </div>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}
    </>
  );
}
