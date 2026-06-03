"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api, type Report, type InterviewView, type ConfidenceSnapshot } from "@/lib/api";
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

export default function ReportPage() {
  const { id } = useParams<{ id: string }>();
  const [report, setReport] = useState<Report | null>(null);
  const [view, setView] = useState<InterviewView | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [needsGen, setNeedsGen] = useState(false);

  const loadAll = useCallback(async () => {
    try {
      setView(await api.getInterview(id));
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    }
    try {
      setReport(await api.getReport(id));
      setNeedsGen(false);
    } catch {
      setNeedsGen(true);
    }
  }, [id]);

  useEffect(() => {
    loadAll();
  }, [loadAll]);

  async function generate() {
    setBusy(true);
    setError("");
    try {
      setReport(await api.generateReport(id));
      setNeedsGen(false);
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    } finally {
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

      {error && <div className="error">{error}</div>}

      {needsGen && !report && (
        <div className="panel">
          <p>No report generated yet.</p>
          <button onClick={generate} disabled={busy}>
            {busy ? "Generating… (multiple LLM calls, slow on CPU)" : "Generate report"}
          </button>
        </div>
      )}

      {report?.recommendation && (
        <div className="panel">
          <h2>Recommendation</h2>
          <div className="flex" style={{ gap: 16 }}>
            <span className={`decision d-${report.recommendation.decision}`}>
              {DECISION_LABEL[report.recommendation.decision] || report.recommendation.decision}
            </span>
            <span className="muted">confidence {(report.recommendation.confidence_level * 100).toFixed(0)}%</span>
          </div>
          <p style={{ marginTop: 10 }}>{report.recommendation.reasoning}</p>
          {report.recommendation.citations?.length > 0 && (
            <div>
              <div className="small muted" style={{ marginTop: 8 }}>Evidence citations</div>
              {report.recommendation.citations.map((c, i) => (
                <div key={i} className="small">
                  • <b>{c.competency}</b> {c.turns?.length ? `(turns ${c.turns.join(", ")})` : ""} — {c.note}
                </div>
              ))}
            </div>
          )}
        </div>
      )}

      {report && report.competency_scores?.length > 0 && (
        <div className="panel">
          <h2>Competency breakdown (cool / normal / hot)</h2>
          <div className="grid two">
            {report.competency_scores.map((s) => (
              <div key={s.competency} className="card">
                <div className="flex between">
                  <b>{s.competency}</b>
                  <span className="muted small">turns {s.evidence_turns?.join(", ") || "—"}</span>
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

      {report && (
        <div className="grid two">
          <div className="panel">
            <h2>Strongest signals</h2>
            {report.signals?.length ? (
              report.signals.map((s, i) => (
                <div key={i} style={{ marginBottom: 10 }}>
                  <b>{s.name}</b> <span className="muted small">turns {s.evidence_turns?.join(", ") || "—"}</span>
                  <div className="small muted">{s.description}</div>
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
              <div className="quote">&ldquo;{e.supporting_quote}&rdquo; <span className="muted">— {e.competency}, turn {e.turn}</span></div>
              {e.revisions?.map((r, i) => (
                <div key={i} className="rev">
                  ↑ revised at turn {r.at_turn}: {r.old_strength.toFixed(2)} → {r.new_strength.toFixed(2)} — {r.note}
                </div>
              ))}
            </div>
          ))}
        </div>
      )}

      <div className="panel">
        <h2>Transcript & evidence</h2>
        {view?.turns.filter((t) => t.answered).map((t) => {
          const ev = (view.evidence || []).filter((e) => e.turn === t.turn);
          return (
            <div className="turn" key={t.id}>
              <div className="q">T{t.turn} {t.kind === "followup" ? "↳ " : ""}{t.question}</div>
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
    </>
  );
}
