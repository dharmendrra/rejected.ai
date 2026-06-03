"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { useParams } from "next/navigation";
import Link from "next/link";
import { api, type InterviewView, type ConfidenceSnapshot } from "@/lib/api";
import { Bar } from "@/components/Bars";

function latestByCompetency(snaps: ConfidenceSnapshot[]): Record<string, ConfidenceSnapshot> {
  const out: Record<string, ConfidenceSnapshot> = {};
  [...snaps].sort((a, b) => a.turn - b.turn).forEach((s) => (out[s.competency] = s));
  return out;
}

export default function InterviewRunner() {
  const { id } = useParams<{ id: string }>();
  const [view, setView] = useState<InterviewView | null>(null);
  const [answer, setAnswer] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  const load = useCallback(async () => {
    try {
      setView(await api.getInterview(id));
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    }
  }, [id]);

  useEffect(() => {
    load();
  }, [load]);

  const openTurn = useMemo(() => view?.turns.find((t) => !t.answered), [view]);
  const completed = view?.interview.status === "completed";
  const latest = useMemo(() => latestByCompetency(view?.confidence || []), [view]);

  async function submit() {
    if (!answer.trim()) return;
    setBusy(true);
    setError("");
    try {
      await api.submitAnswer(id, answer);
      setAnswer("");
      await load();
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    } finally {
      setBusy(false);
    }
  }

  if (!view) return <p className="spin">{error ? <span className="error">{error}</span> : "Loading…"}</p>;

  const competencies = view.interview.competencies || [];

  return (
    <div className="grid sidebar">
      <div>
        <div className="panel">
          <div className="flex between">
            <h2>
              {view.interview.level} · {view.interview.type}
            </h2>
            <span className="tag">{completed ? "completed" : "active"}</span>
          </div>

          {completed ? (
            <div>
              <p>Interview complete. {view.turns.filter((t) => t.answered).length} answers recorded.</p>
              <Link href={`/interview/${id}/report`}>
                <button>View report & replay →</button>
              </Link>
            </div>
          ) : openTurn ? (
            <div>
              <p className="small muted">
                Turn {openTurn.turn} · {openTurn.kind}
                {openTurn.target_competencies?.length ? ` · targets: ${openTurn.target_competencies.join(", ")}` : ""}
              </p>
              <p style={{ fontSize: 17, fontWeight: 600 }}>{openTurn.question}</p>
              <textarea value={answer} onChange={(e) => setAnswer(e.target.value)} placeholder="Type the candidate's answer…" />
              {error && <div className="error">{error}</div>}
              <div style={{ marginTop: 12 }}>
                <button onClick={submit} disabled={busy || !answer.trim()}>
                  {busy ? "Evaluating… (LLM)" : "Submit answer"}
                </button>
                {busy && <span className="note" style={{ marginLeft: 10 }}>extracting evidence & re-scoring…</span>}
              </div>
            </div>
          ) : (
            <p>No open question.</p>
          )}
        </div>

        <div className="panel">
          <h2>Conversation</h2>
          {view.turns.filter((t) => t.answered).length === 0 && <p className="muted small">No answers yet.</p>}
          {view.turns
            .filter((t) => t.answered)
            .map((t) => (
              <div className="turn" key={t.id}>
                <div className="q">
                  T{t.turn} {t.kind === "followup" ? "↳ " : ""}
                  {t.question}
                </div>
                <div className="a">{t.answer}</div>
                {t.response_type && t.response_type !== "answer" && (
                  <span className="tag">{t.response_type}</span>
                )}
              </div>
            ))}
        </div>
      </div>

      <div className="panel">
        <h2>Live confidence</h2>
        {competencies.length === 0 && <p className="muted small">No competencies yet.</p>}
        {competencies.map((c) => {
          const s = latest[c];
          return (
            <div key={c} style={{ marginBottom: 12 }}>
              <div className="flex between small">
                <span>{c}</span>
                <span className="muted">{s ? s.normal.toFixed(2) : "—"}</span>
              </div>
              <Bar value={s?.normal ?? 0} />
            </div>
          );
        })}
        <p className="note">Scores recompute over the full evidence ledger after every answer.</p>
      </div>
    </div>
  );
}
