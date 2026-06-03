"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { api } from "@/lib/api";

const SAMPLE_JD = `Senior Backend Engineer — Payments Platform
Own backend services for payment capture, settlement, and reconciliation.
Ensure exactly-once processing and safe handling of retries and duplicate requests.
Lead technical design reviews and mentor mid-level engineers.
Define SLOs, drive incident response, and improve operational maturity.
Required: distributed systems fundamentals (consistency, idempotency, concurrency),
production cloud + relational DBs + queues, operating at scale (observability, on-call).`;

const SAMPLE_RESUME = `Priya Nair — Senior Software Engineer
8 years building distributed systems, last 4 in payments.
Owned payment capture and ledger services; led migration to a new settlement pipeline; mentored 3 engineers.
Designed payment capture handling millions of txns/day. Introduced structured logging + tracing.
Tech: Go, Java, PostgreSQL, Kafka, Redis, AWS, Kubernetes, gRPC.`;

export default function Home() {
  const router = useRouter();
  const [jd, setJd] = useState("");
  const [resume, setResume] = useState("");
  const [level, setLevel] = useState("Senior Engineer");
  const [type, setType] = useState("Mixed");
  const [duration, setDuration] = useState(20);
  const [busy, setBusy] = useState(false);
  const [step, setStep] = useState("");
  const [error, setError] = useState("");

  async function start() {
    setError("");
    setBusy(true);
    try {
      setStep("Structuring job description… (LLM, may take a few minutes locally)");
      const jdRes = await api.ingestJD(jd);
      setStep("Structuring resume…");
      const cvRes = await api.ingestResume(resume);
      setStep("Building capability graphs & first question…");
      const created = await api.createInterview({
        job_description_id: jdRes.id,
        candidate_profile_id: cvRes.id,
        level,
        type,
        duration_min: duration,
      });
      router.push(`/interview/${created.interview.id}`);
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
      setBusy(false);
      setStep("");
    }
  }

  return (
    <>
      <p className="sub">
        Paste a job description and a resume, then run an adaptive interview whose confidence
        scores evolve as evidence accumulates.
      </p>

      <div className="panel">
        <div className="flex between">
          <h2>New interview</h2>
          <button
            className="ghost"
            type="button"
            disabled={busy}
            onClick={() => {
              setJd(SAMPLE_JD);
              setResume(SAMPLE_RESUME);
            }}
          >
            Load sample
          </button>
        </div>

        <div className="grid two">
          <div>
            <label>Job description</label>
            <textarea value={jd} onChange={(e) => setJd(e.target.value)} placeholder="Paste JD text…" />
          </div>
          <div>
            <label>Resume</label>
            <textarea value={resume} onChange={(e) => setResume(e.target.value)} placeholder="Paste resume text…" />
          </div>
        </div>

        <div className="row" style={{ marginTop: 12 }}>
          <div>
            <label>Level</label>
            <select value={level} onChange={(e) => setLevel(e.target.value)}>
              {["Senior Engineer", "Staff Engineer", "Lead Engineer", "Engineering Manager", "Principal Engineer", "Architect"].map(
                (l) => (
                  <option key={l}>{l}</option>
                )
              )}
            </select>
          </div>
          <div>
            <label>Type</label>
            <select value={type} onChange={(e) => setType(e.target.value)}>
              {["Mixed", "Technical Screening", "System Design", "Architecture Review", "Engineering Leadership", "AI Engineering"].map(
                (t) => (
                  <option key={t}>{t}</option>
                )
              )}
            </select>
          </div>
          <div>
            <label>Duration (min)</label>
            <input type="number" value={duration} min={5} max={90} onChange={(e) => setDuration(Number(e.target.value))} />
          </div>
        </div>

        {error && <div className="error">{error}</div>}
        {busy && <p className="spin">⏳ {step}</p>}

        <div style={{ marginTop: 16 }}>
          <button onClick={start} disabled={busy || !jd.trim() || !resume.trim()}>
            {busy ? "Working…" : "Start interview"}
          </button>
        </div>
        <p className="note" style={{ marginTop: 8 }}>
          Note: with the local Ollama model on CPU each step can take a few minutes.
        </p>
      </div>
    </>
  );
}
