"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { api, type PondQuestion } from "@/lib/api";

const LEVEL_DEFAULTS = [
  "Senior Engineer",
  "Staff Engineer",
  "Lead Engineer",
  "Engineering Manager",
  "Principal Engineer",
  "Architect",
];

const TYPE_DEFAULTS = [
  "Mixed",
  "Technical Screening",
  "System Design",
  "Architecture Review",
  "Engineering Leadership",
  "AI Engineering",
  "HR Round",
];

export default function PondPage() {
  const [questions, setQuestions] = useState<PondQuestion[]>([]);
  const [roles, setRoles] = useState<string[]>(LEVEL_DEFAULTS);
  const [types, setTypes] = useState<string[]>(TYPE_DEFAULTS);
  const [selectedRole, setSelectedRole] = useState<string>("all");
  const [selectedType, setSelectedType] = useState<string>("all");
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    loadQuestions();
  }, [selectedRole, selectedType]);

  const loadQuestions = async () => {
    setLoading(true);
    setError(null);
    try {
      const data = await api.listQuestionsPond({
        role: selectedRole,
        type: selectedType,
      });
      setQuestions(data.questions);

      // Merge backend facets with our defaults for complete filter menus
      if (data.roles && data.roles.length > 0) {
        const uniqueRoles = Array.from(new Set([...LEVEL_DEFAULTS, ...data.roles])).sort();
        setRoles(uniqueRoles);
      }
      if (data.types && data.types.length > 0) {
        const uniqueTypes = Array.from(new Set([...TYPE_DEFAULTS, ...data.types])).sort();
        setTypes(uniqueTypes);
      }
    } catch (err: any) {
      setError(err.message || "Failed to load questions from the pond");
    } finally {
      setLoading(false);
    }
  };

  const fmtDate = (s: string) => {
    if (!s) return "";
    const d = new Date(s);
    if (isNaN(d.getTime())) return s;
    return d.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
  };

  return (
    <div style={{ paddingBottom: "64px" }}>
      <div className="flex between" style={{ marginBottom: "24px", flexWrap: "wrap", gap: "12px" }}>
        <div>
          <h1 style={{ margin: "0 0 4px 0", fontSize: "24px", fontWeight: "700" }}>🗂️ Question Pond</h1>
          <p className="muted small" style={{ margin: 0 }}>
            Browse and filter cached questions generated across previous AI sessions.
          </p>
        </div>
        <Link href="/">
          <button className="ghost" style={{ fontSize: "13px", padding: "8px 16px" }}>
            ✨ Start Fresh Session
          </button>
        </Link>
      </div>

      {/* Filter panel */}
      <div className="panel" style={{ padding: "16px", marginBottom: "20px", display: "flex", gap: "16px", flexWrap: "wrap" }}>
        <div style={{ minWidth: "200px", flex: 1 }}>
          <label style={{ fontSize: "12px", marginBottom: "6px" }} htmlFor="filter-role">Level / Role</label>
          <select
            id="filter-role"
            value={selectedRole}
            onChange={(e) => setSelectedRole(e.target.value)}
            style={{ width: "100%" }}
          >
            <option value="all">All Levels</option>
            {roles.map((r) => (
              <option key={r} value={r}>
                {r}
              </option>
            ))}
          </select>
        </div>

        <div style={{ minWidth: "200px", flex: 1 }}>
          <label style={{ fontSize: "12px", marginBottom: "6px" }} htmlFor="filter-type">Interview Type</label>
          <select
            id="filter-type"
            value={selectedType}
            onChange={(e) => setSelectedType(e.target.value)}
            style={{ width: "100%" }}
          >
            <option value="all">All Types</option>
            {types.map((t) => (
              <option key={t} value={t}>
                {t}
              </option>
            ))}
          </select>
        </div>
      </div>

      {error && <div className="error">{error}</div>}

      {loading ? (
        <div style={{ textAlign: "center", padding: "64px 0" }}>
          <div className="spin-timer" style={{ fontSize: "32px", marginBottom: "16px" }}>⏳</div>
          <div className="muted">Loading question pond...</div>
        </div>
      ) : questions.length === 0 ? (
        <div className="panel" style={{ textAlign: "center", padding: "48px 24px" }}>
          <div style={{ fontSize: "48px", marginBottom: "16px" }}>🗂️</div>
          <h3 style={{ margin: "0 0 8px 0" }}>No questions in the pond</h3>
          <p className="muted small" style={{ maxWidth: "420px", margin: "0 auto 24px" }}>
            Pond is empty for the selected filters. Run a session in AI mode for this Level & Type to generate and cache questions here!
          </p>
          <Link href="/">
            <button>Generate with AI</button>
          </Link>
        </div>
      ) : (
        <div style={{ display: "flex", flexDirection: "column", gap: "12px" }}>
          <div className="muted small" style={{ display: "flex", justifyContent: "space-between" }}>
            <span>Showing {questions.length} question{questions.length === 1 ? "" : "s"}</span>
          </div>

          {questions.map((q) => (
            <div className="panel" key={q.id} style={{ margin: 0, padding: "16px 20px" }}>
              <div style={{ fontSize: "15px", fontWeight: "500", lineHeight: "1.5", marginBottom: "12px" }}>
                {q.question}
              </div>

              {/* Tags block */}
              <div className="flex" style={{ gap: "6px", flexWrap: "wrap", marginBottom: "12px" }}>
                <span className="tag" style={{ fontSize: "11px", color: "var(--accent)", borderColor: "var(--accent)" }}>
                  {q.role}
                </span>
                <span className="tag" style={{ fontSize: "11px", color: "var(--good)", borderColor: "var(--good)" }}>
                  {q.type}
                </span>
                <span className="tag" style={{ fontSize: "11px", color: "var(--warn)", borderColor: "var(--warn)" }}>
                  🎯 Rigor: {q.rigor_percent}%
                </span>
                {q.target_competencies.map((comp) => (
                  <span key={comp} className="tag" style={{ fontSize: "11px" }}>
                    {comp}
                  </span>
                ))}
              </div>

              {/* Metadata strip */}
              <div className="flex between" style={{ alignItems: "center", flexWrap: "wrap", gap: "8px", paddingTop: "8px", borderTop: "1px solid var(--border)", fontSize: "11px" }}>
                <div className="muted" style={{ display: "flex", gap: "16px" }}>
                  {q.job_title && (
                    <span>
                      💼 Context: <strong>{q.job_title}</strong>
                    </span>
                  )}
                  <span>
                    🤖 Model: <strong>{q.model || "—"}</strong>
                  </span>
                  {q.source_interview_id && (
                    <span>
                      🔗 Source:{" "}
                      <Link href={`/interview/${q.source_interview_id}/report`} className="muted" style={{ textDecoration: "underline" }}>
                        View Report
                      </Link>
                    </span>
                  )}
                </div>
                <div className="muted">
                  <span>Reused: <strong>{q.used_count}</strong> time{q.used_count === 1 ? "" : "s"}</span>
                  <span style={{ margin: "0 8px" }}>·</span>
                  <span>Added: {fmtDate(q.created_at)}</span>
                </div>
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
