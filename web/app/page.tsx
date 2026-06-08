"use client";

import { useState, useRef } from "react";
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
  const fileInputRef = useRef<HTMLInputElement>(null);
  const [jd, setJd] = useState("");
  const [resume, setResume] = useState("");
  const [resumeFile, setResumeFile] = useState<File | null>(null);
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
      const cvRes = await api.ingestResume(resumeFile || resume);
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
              setResumeFile(null);
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
            {resumeFile ? (
              <div className="card" style={{ display: "flex", flexDirection: "column", justifyContent: "center", alignItems: "center", minHeight: 140, gap: 12 }}>
                <div style={{ textAlign: "center" }}>
                  <p style={{ margin: 0, fontWeight: 600 }}>📄 {resumeFile.name}</p>
                  <p style={{ margin: "4px 0 0", fontSize: "12px", color: "var(--muted)" }}>
                    {(resumeFile.size / 1024).toFixed(1)} KB
                  </p>
                </div>
                <button
                  className="ghost"
                  type="button"
                  onClick={() => setResumeFile(null)}
                  style={{ padding: "6px 12px", fontSize: "13px" }}
                >
                  Remove File
                </button>
              </div>
            ) : (
              <div style={{ display: "flex", flexDirection: "column", gap: "8px" }}>
                <textarea
                  value={resume}
                  onChange={(e) => setResume(e.target.value)}
                  placeholder="Paste resume text…"
                />
                <div style={{ display: "flex", alignItems: "center", gap: "8px" }}>
                  <span style={{ fontSize: "12px", color: "var(--muted)" }}>Or upload file:</span>
                  <input
                    type="file"
                    ref={fileInputRef}
                    accept=".pdf,.docx,.txt,.md"
                    onChange={(e) => {
                      const file = e.target.files?.[0];
                      if (file) {
                        setResumeFile(file);
                        setResume("");
                      }
                    }}
                    style={{ display: "none" }}
                  />
                  <button
                    type="button"
                    className="ghost"
                    onClick={() => fileInputRef.current?.click()}
                    style={{ fontSize: "12px", padding: "6px 12px", border: "1px solid var(--border)", background: "var(--panel-2)", cursor: "pointer", color: "var(--text)" }}
                  >
                    Choose file…
                  </button>
                </div>
              </div>
            )}
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
          <button onClick={start} disabled={busy || !jd.trim() || (!resumeFile && !resume.trim())}>
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
