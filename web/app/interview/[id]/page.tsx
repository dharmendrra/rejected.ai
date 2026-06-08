"use client";

import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useParams, useRouter } from "next/navigation";
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
  const router = useRouter();
  const [view, setView] = useState<InterviewView | null>(null);
  const [answer, setAnswer] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");

  // 🎥 Video/Audio Recording states
  const [recordModalOpen, setRecordModalOpen] = useState(false);
  const [recording, setRecording] = useState(false);
  const [mediaStream, setMediaStream] = useState<MediaStream | null>(null);
  const [mediaRecorder, setMediaRecorder] = useState<MediaRecorder | null>(null);
  const [recordError, setRecordError] = useState("");
  const [recordBusy, setRecordBusy] = useState(false);
  const [recordDuration, setRecordDuration] = useState(0);
  const [recordStartTime, setRecordStartTime] = useState<number>(0);
  
  const recordStartTimeRef = useRef<number>(0);

  const load = useCallback(async () => {
    try {
      setView(await api.getInterview(id));
    } catch (e) {
      setError(String(e instanceof Error ? e.message : e));
    }
  }, [id]);

  async function triggerReportAndNavigate() {
    setBusy(true);
    setError("");
    try {
      await api.generateReport(id);
      router.push(`/interview/${id}/report`);
    } catch (e) {
      setError(e instanceof Error ? e.message : String(e));
      setBusy(false);
    }
  }

  useEffect(() => {
    load();
  }, [load]);

  const completed = view?.interview.status === "completed";
  const [timeLeft, setTimeLeft] = useState<number | null>(null);

  useEffect(() => {
    if (!view || completed) return;

    const startTime = new Date(view.interview.created_at).getTime();
    const durationMs = view.interview.duration_min * 60 * 1000;
    const endTime = startTime + durationMs;

    const updateTimer = () => {
      const remaining = Math.max(0, Math.ceil((endTime - Date.now()) / 1000));
      setTimeLeft(remaining);
    };

    updateTimer();
    const timerId = setInterval(updateTimer, 1000);

    return () => clearInterval(timerId);
  }, [view, completed]);

  const openTurn = useMemo(() => view?.turns.find((t) => !t.answered), [view]);
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

  // 🎥 Video/Audio Recording Logics & Hooks
  useEffect(() => {
    let activeStream: MediaStream | null = null;

    if (recordModalOpen) {
      navigator.mediaDevices.getUserMedia({
        video: { width: 480, height: 480, aspectRatio: 1 },
        audio: true
      })
      .then((stream) => {
        activeStream = stream;
        setMediaStream(stream);
      })
      .catch((err) => {
        setRecordError("Could not access camera/microphone: " + err.message);
      });
    }

    return () => {
      if (activeStream) {
        activeStream.getTracks().forEach((track) => track.stop());
      }
      setMediaStream(null);
      setRecording(false);
      setRecordError("");
      setRecordDuration(0);
    };
  }, [recordModalOpen]);

  // Update duration timer
  useEffect(() => {
    if (!recording) return;
    const interval = setInterval(() => {
      setRecordDuration(Math.round((Date.now() - recordStartTimeRef.current) / 1000));
    }, 500);
    return () => clearInterval(interval);
  }, [recording]);

  const startRecording = useCallback(() => {
    if (!mediaStream || !openTurn) return;
    try {
      setRecordError("");
      const chunks: Blob[] = [];
      const recorder = new MediaRecorder(mediaStream, { mimeType: "video/webm" });
      
      recorder.ondataavailable = (e) => {
        if (e.data && e.data.size > 0) {
          chunks.push(e.data);
        }
      };

      recorder.onstop = async () => {
        setRecordBusy(true);
        const durationSec = Math.max(1, Math.round((Date.now() - recordStartTimeRef.current) / 1000));
        const videoBlob = new Blob(chunks, { type: "video/webm" });
        
        try {
          // 1. Upload video metrics
          await api.uploadVideo(id, openTurn.turn, videoBlob, 1000);
          
          // 2. Ingest audio transcript
          const audioRes = await api.uploadAudio(id, openTurn.turn, videoBlob, durationSec, 1000);
          
          // 3. Submit answer with transcript text
          const transcriptText = audioRes.text || `[Recorded Audio/Video Answer - Turn ${openTurn.turn}]`;
          await api.submitAnswer(id, transcriptText);
          
          setRecordModalOpen(false);
          await load();
        } catch (err) {
          setRecordError(String(err instanceof Error ? err.message : err));
        } finally {
          setRecordBusy(false);
        }
      };

      recorder.start(100);
      setMediaRecorder(recorder);
      setRecording(true);
      const now = Date.now();
      setRecordStartTime(now);
      recordStartTimeRef.current = now;
      setRecordDuration(0);
    } catch (err) {
      setRecordError("Failed to start MediaRecorder: " + (err instanceof Error ? err.message : err));
    }
  }, [mediaStream, id, openTurn, load]);

  const stopAndSubmit = useCallback(() => {
    if (mediaRecorder && mediaRecorder.state !== "inactive") {
      mediaRecorder.stop();
      setRecording(false);
    }
  }, [mediaRecorder]);

  const cancelRecording = useCallback(() => {
    if (mediaRecorder && mediaRecorder.state !== "inactive") {
      mediaRecorder.onstop = null;
      mediaRecorder.stop();
    }
    setRecordModalOpen(false);
  }, [mediaRecorder]);

  const videoRef = useCallback((node: HTMLVideoElement | null) => {
    if (node && mediaStream) {
      node.srcObject = mediaStream;
    }
  }, [mediaStream]);

  if (!view) return <p className="spin">{error ? <span className="error">{error}</span> : "Loading…"}</p>;

  const competencies = view.interview.competencies || [];

  const answeredCount = view.turns.filter((t) => t.answered).length;

  return (
    <div className="grid sidebar">
      <div>
        {!completed && (
          answeredCount > 0 ? (
            <button 
              onClick={triggerReportAndNavigate}
              disabled={busy}
              className="ghost" 
              style={{ 
                position: "absolute", 
                top: 24, 
                right: 140, 
                fontSize: "13px", 
                padding: "6px 12px", 
                border: "1px solid var(--border)", 
                background: "transparent", 
                cursor: "pointer",
                color: "var(--text)"
              }}
            >
              {busy ? "Ending..." : "🏁 End interview"}
            </button>
          ) : (
            <Link href="/">
              <button 
                className="ghost" 
                style={{ 
                  position: "absolute", 
                  top: 24, 
                  right: 140, 
                  fontSize: "13px", 
                  padding: "6px 12px", 
                  border: "1px solid var(--border)", 
                  background: "transparent", 
                  cursor: "pointer",
                  color: "var(--muted)"
                }}
              >
                no answers, please start fresh
              </button>
            </Link>
          )
        )}
        <div className="panel">
          <div className="flex between" style={{ alignItems: "center" }}>
            <h2>
              {view.interview.level} · {view.interview.type}
            </h2>
            <div className="flex" style={{ gap: 8, alignItems: "center" }}>
              {timeLeft !== null && !completed && (
                <span 
                  className="tag" 
                  style={{ 
                    color: timeLeft < 60 ? "var(--bad)" : timeLeft < 180 ? "var(--warn)" : "var(--accent)",
                    borderColor: timeLeft < 60 ? "var(--bad)" : timeLeft < 180 ? "var(--warn)" : "var(--border)",
                    fontWeight: "bold",
                    background: timeLeft < 60 ? "rgba(255,107,107,0.1)" : timeLeft < 180 ? "rgba(245,166,35,0.1)" : "var(--panel-2)"
                  }}
                >
                  <span className="spin-timer" style={{ marginRight: 6 }}>⏳</span>
                  {Math.floor(timeLeft / 60)}:{(timeLeft % 60).toString().padStart(2, '0')}
                </span>
              )}
              <span className="tag">{completed ? "completed" : "active"}</span>
            </div>
          </div>
          {timeLeft !== null && !completed && timeLeft > 0 && (
            <div className="bar" style={{ marginTop: 8, height: 4, background: "var(--border)" }}>
              <span 
                style={{ 
                  width: `${(timeLeft / (view.interview.duration_min * 60)) * 100}%`, 
                  backgroundColor: timeLeft < 60 ? "var(--bad)" : timeLeft < 180 ? "var(--warn)" : "var(--accent)", 
                  transition: "width 1s linear" 
                }} 
              />
            </div>
          )}

          <div style={{ marginTop: 16 }}>
            {completed ? (
              <div>
                <p>Interview complete. {view.turns.filter((t) => t.answered).length} answers recorded.</p>
                <button onClick={triggerReportAndNavigate} disabled={busy}>
                  {busy ? "Starting..." : "View report & replay →"}
                </button>
                {error && <div className="error" style={{ marginTop: 8 }}>{error}</div>}
              </div>
            ) : timeLeft !== null && timeLeft <= 0 ? (
              <div style={{ borderLeft: "4px solid var(--bad)", paddingLeft: 16, marginTop: 12 }}>
                <h3 style={{ color: "var(--bad)", marginTop: 0 }}>⏱️ Time Limit Reached</h3>
                <p>The configured allowed time of {view.interview.duration_min} minutes for this session has run out.</p>
                <p className="muted small">
                  You can still proceed to generate the candidate report with the {view.turns.filter((t) => t.answered).length} answers collected so far.
                </p>
                <button onClick={triggerReportAndNavigate} disabled={busy} style={{ background: "var(--bad)", color: "#06101f" }}>
                  {busy ? "Starting..." : "Generate Report →"}
                </button>
                {error && <div className="error" style={{ marginTop: 8 }}>{error}</div>}
              </div>
            ) : openTurn ? (
              <div>
                <p className="small muted">
                  Question {openTurn.turn} ({openTurn.turn} / {view.turns.length}) · {openTurn.kind}
                  {openTurn.target_competencies?.length ? ` · targets: ${openTurn.target_competencies.join(", ")}` : ""}
                </p>
                <p style={{ fontSize: 17, fontWeight: 600 }}>{openTurn.question}</p>
                <textarea value={answer} onChange={(e) => setAnswer(e.target.value)} placeholder="Type the candidate's answer…" />
                {error && <div className="error">{error}</div>}
                <div className="flex" style={{ marginTop: 12, gap: 12, alignItems: "center" }}>
                  <button onClick={submit} disabled={busy || !answer.trim()}>
                    {busy ? "Saving…" : "Submit text answer"}
                  </button>
                  <span className="muted" style={{ fontSize: "13px" }}>or</span>
                  <button 
                    onClick={() => setRecordModalOpen(true)} 
                    disabled={busy}
                    className="ghost"
                    style={{ borderColor: "var(--accent)", color: "var(--accent)" }}
                  >
                    📹 Record your answer
                  </button>
                  {busy && <span className="note" style={{ marginLeft: 10 }}>recording answer…</span>}
                </div>
              </div>
            ) : (
              <div>
                <p>No open question.</p>
                {error && <div className="error" style={{ marginTop: 8 }}>{error}</div>}
              </div>
            )}
          </div>
        </div>

        <div className="panel">
          <h2>Conversation</h2>
          {view.turns.filter((t) => t.answered).length === 0 && <p className="muted small">No answers yet.</p>}
          {view.turns
            .filter((t) => t.answered)
            .map((t) => (
              <div className="turn" key={t.id}>
                <div className="q">
                  Q{t.turn} {t.kind === "followup" ? "↳ " : ""}
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

      {recordModalOpen && (
        <div className="modal-overlay">
          <div className="modal-content">
            <h3 style={{ margin: "0 0 10px 0" }}>📹 Answer via Video/Audio</h3>
            <p className="small muted" style={{ margin: "0 0 16px 0" }}>
              Question {openTurn?.turn} · Targets: {openTurn?.target_competencies?.join(", ")}
            </p>
            <p style={{ fontWeight: 600, fontSize: "14px", marginBottom: "16px", color: "var(--text)" }}>
              &ldquo;{openTurn?.question}&rdquo;
            </p>

            <div className="video-container">
              {recording && (
                <div className="recording-indicator">
                  <div className="recording-dot" />
                  <span>REC {Math.floor(recordDuration / 60)}:{(recordDuration % 60).toString().padStart(2, '0')}</span>
                </div>
              )}
              {mediaStream ? (
                <video ref={videoRef} className="video-element" autoPlay playsInline muted />
              ) : (
                <div style={{ display: "flex", alignItems: "center", justifyContent: "center", height: "100%", color: "var(--muted)", fontSize: "12px", padding: "20px" }}>
                  {recordError || "Connecting to camera..."}
                </div>
              )}
            </div>

            {recordError && <div className="error" style={{ fontSize: "12px", margin: "10px 0" }}>{recordError}</div>}

            <div className="modal-actions">
              {!recording ? (
                <button 
                  onClick={startRecording} 
                  disabled={!mediaStream || recordBusy}
                  style={{ background: "var(--accent)", color: "#06101f" }}
                >
                  🔴 Start Recording
                </button>
              ) : (
                <button 
                  onClick={stopAndSubmit} 
                  disabled={recordBusy}
                  style={{ background: "var(--bad)", color: "#fff" }}
                >
                  ⏹️ {recordBusy ? "Uploading & Processing..." : "Stop & Submit"}
                </button>
              )}
              <button 
                onClick={cancelRecording} 
                disabled={recordBusy}
                className="ghost"
              >
                Cancel
              </button>
            </div>
            
            {timeLeft !== null && (
              <div style={{ marginTop: "16px", fontSize: "12px", color: "var(--muted)" }}>
                Interview Timer: ⏳ {Math.floor(timeLeft / 60)}:{(timeLeft % 60).toString().padStart(2, '0')}
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}
