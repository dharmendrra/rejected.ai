"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api } from "../../lib/api";

interface QuestionTurn {
  turn: number;
  question: string;
  answer?: string;
  answered: boolean;
}

interface InterviewItem {
  id: string;
  level: string;
  type: string;
  status: string;
  report_status?: string;
  created_at: string;
  updated_at: string;
  candidate_name?: string;
  resume_id?: string;
  resume_raw?: string;
  resume_tech?: string[];
  job_title?: string;
  jd_id?: string;
  jd_raw?: string;
  questions?: QuestionTurn[];
}

export default function HistoryPage() {
  const [interviews, setInterviews] = useState<InterviewItem[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadInterviews();
  }, []);

  const loadInterviews = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listInterviews();
      setInterviews(data);
    } catch (err: any) {
      setError(err.message || "Failed to load past interviews");
    } finally {
      setLoading(false);
    }
  };

  const handleDelete = async (id: string, candidateName: string) => {
    if (!confirm(`Are you sure you want to delete the interview round for ${candidateName || "Unknown Candidate"}? This will permanently delete the session, questions, evaluation ledger, and report recommendations.`)) {
      return;
    }
    try {
      await api.deleteInterview(id);
      setInterviews(prev => prev.filter(item => item.id !== id));
    } catch (err: any) {
      alert(`Failed to delete interview: ${err.message}`);
    }
  };

  if (loading) {
    return (
      <div style={{ textAlign: "center", padding: "64px 0" }}>
        <div className="spin-timer" style={{ fontSize: "32px", marginBottom: "16px" }}>⏳</div>
        <div className="muted">Loading past interviews...</div>
      </div>
    );
  }

  return (
    <div style={{ paddingBottom: "64px" }}>
      <div className="flex between" style={{ marginBottom: "24px" }}>
        <div>
          <h1 style={{ margin: "0 0 4px 0", fontSize: "24px", fontWeight: "700" }}>Past Interview Dashboard</h1>
          <p className="muted small" style={{ margin: 0 }}>Review, resume, or clean up your past evidence-accumulating sessions.</p>
        </div>
        <Link href="/">
          <button className="ghost" style={{ fontSize: "13px", padding: "8px 16px" }}>
            + New Interview
          </button>
        </Link>
      </div>

      {error && <div className="error">{error}</div>}

      {interviews.length === 0 ? (
        <div className="panel" style={{ textAlign: "center", padding: "48px 24px" }}>
          <div style={{ fontSize: "48px", marginBottom: "16px" }}>📁</div>
          <h3 style={{ margin: "0 0 8px 0" }}>No interviews found</h3>
          <p className="muted small" style={{ maxWidth: "400px", margin: "0 auto 24px" }}>
            You haven't run any interview sessions yet. Upload a job description and resume to get started!
          </p>
          <Link href="/">
            <button>Start fresh session</button>
          </Link>
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "20px" }}>
          {interviews.map(iv => {
            const dateStr = new Date(iv.created_at).toLocaleDateString(undefined, {
              dateStyle: "medium",
            });
            const timeStr = new Date(iv.created_at).toLocaleTimeString(undefined, {
              timeStyle: "short",
            });

            let displayStatus = iv.status;
            let statusColor = "var(--warn)";
            let statusBg = "rgba(245,166,35,0.08)";

            if (iv.report_status === "generating") {
              displayStatus = "evaluating";
              statusColor = "var(--accent)";
              statusBg = "rgba(91, 157, 255, 0.08)";
            } else if (iv.report_status === "failed") {
              displayStatus = "failed";
              statusColor = "var(--bad)";
              statusBg = "rgba(255, 107, 107, 0.08)";
            } else if (iv.status === "completed") {
              displayStatus = "completed";
              statusColor = "var(--good)";
              statusBg = "rgba(62, 207, 142, 0.08)";
            }

            return (
              <div key={iv.id} className="panel" style={{ margin: 0, display: "flex", flexDirection: "column", gap: "16px" }}>
                {/* Header Row */}
                <div className="flex between" style={{ flexWrap: "wrap", gap: "12px" }}>
                  <div>
                    <div className="flex" style={{ gap: "8px", marginBottom: "4px" }}>
                      <h3 style={{ margin: 0, fontSize: "18px", fontWeight: 700 }}>
                        {iv.candidate_name || "Unknown Candidate"}
                      </h3>
                      <span className={`tag cat`} style={{ 
                        color: statusColor,
                        borderColor: statusColor,
                        background: statusBg,
                        fontSize: "10px",
                        padding: "1px 8px"
                      }}>
                        {displayStatus}
                      </span>
                    </div>
                    <div className="muted small">
                      Role: <strong>{iv.job_title || "Unknown Title"}</strong> • {iv.level} ({iv.type})
                    </div>
                  </div>
                  <div className="flex" style={{ gap: "8px" }}>
                    {iv.status === "completed" || iv.report_status === "generating" || iv.report_status === "failed" ? (
                      <Link href={`/interview/${iv.id}/report`}>
                        <button style={{ padding: "8px 14px", fontSize: "13px" }}>
                          {iv.report_status === "generating" ? "⏳ Evaluating..." : "📊 View Report"}
                        </button>
                      </Link>
                    ) : (
                      <Link href={`/interview/${iv.id}`}>
                        <button style={{ padding: "8px 14px", fontSize: "13px" }}>⚡ Resume Round</button>
                      </Link>
                    )}
                    <button 
                      onClick={() => handleDelete(iv.id, iv.candidate_name || "Unknown")}
                      style={{ 
                        background: "rgba(255,107,107,0.12)", 
                        color: "var(--bad)", 
                        border: "1px solid var(--bad)", 
                        padding: "8px 12px", 
                        fontSize: "13px" 
                      }}
                    >
                      🗑️ Delete
                    </button>
                  </div>
                </div>

                {/* Meta details */}
                <div className="muted small" style={{ borderTop: "1px solid var(--border)", paddingTop: "12px", display: "flex", gap: "24px", flexWrap: "wrap" }}>
                  <div>Started: <strong>{dateStr} at {timeStr}</strong></div>
                  <div>Questions Askable: <strong>{iv.questions?.length || 0}</strong></div>
                  <div>Answered: <strong>{iv.questions?.filter(q => q.answered).length || 0}</strong></div>
                </div>

                {/* Job Description & Resume Details */}
                <div className="grid two" style={{ gap: "12px" }}>
                  <details className="card" style={{ background: "var(--panel-2)" }}>
                    <summary className="small" style={{ cursor: "pointer", color: "var(--accent)", fontWeight: 600 }}>
                      📋 Job Description (Click to view)
                    </summary>
                    <div style={{ marginTop: "8px", fontSize: "12px", maxHeight: "150px", overflowY: "auto", whiteSpace: "pre-wrap", color: "var(--muted)", fontFamily: "ui-monospace, monospace" }}>
                      {iv.jd_raw || "No job description text found."}
                    </div>
                  </details>

                  <details className="card" style={{ background: "var(--panel-2)" }}>
                    <summary className="small" style={{ cursor: "pointer", color: "var(--accent)", fontWeight: 600 }}>
                      📄 Candidate Resume (Click to view)
                    </summary>
                    <div style={{ marginTop: "8px", fontSize: "12px", maxHeight: "150px", overflowY: "auto", whiteSpace: "pre-wrap", color: "var(--muted)", fontFamily: "ui-monospace, monospace" }}>
                      {iv.resume_raw || "No resume text found."}
                    </div>
                  </details>
                </div>

                {/* Questions asked collapsible */}
                {iv.questions && iv.questions.length > 0 && (
                  <details className="card" style={{ background: "var(--panel-2)" }}>
                    <summary className="small" style={{ cursor: "pointer", color: "var(--text)", fontWeight: 600 }}>
                      💬 Interview Transcript & Questions ({iv.questions.filter(q => q.answered).length} answered)
                    </summary>
                    <div style={{ marginTop: "12px", display: "flex", flexDirection: "column", gap: "10px" }}>
                      {iv.questions.map(q => (
                        <div key={q.turn} className="turn" style={{ paddingBottom: "8px", marginBottom: 0 }}>
                          <div className="small q">Q{q.turn}: {q.question}</div>
                          {q.answered ? (
                            <div className="small a" style={{ marginTop: "4px", color: "var(--muted)" }}>
                              A: {q.answer}
                            </div>
                          ) : (
                            <div className="small muted" style={{ marginTop: "4px", fontStyle: "italic" }}>
                              (Unanswered)
                            </div>
                          )}
                        </div>
                      ))}
                    </div>
                  </details>
                )}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}
