# Roadmap & Todo List

This document tracks future features, design iterations, and planned improvements for **rejected.ai**.

## 📌 Active / Upcoming Tasks

### 1. 🎛️ Interview Rigor / Difficulty Slider (0–100%)
*   **Concept:** Introduce a `Rigor` slider (0% to 100%) in the interview configuration page.
*   **Behavior:** Scale the depth, hardness, and complexity of questions dynamically based on this parameter combined with the selected Role (IC/EM) and Phase (Tech Screening/System Design).
    *   *Low Rigor (0–20%):* Basic conceptual checks, syntax, simple behavioral recall.
    *   *Medium Rigor (40–60%):* Standard technical screenings, standard optimizations, structured behavioral analysis.
    *   *High Rigor (80–100%):* Bare-metal system design, low-level compilers/runtimes from scratch, deep socio-technical org restructuring scenarios (EM).
*   **Implementation Steps:**
    *   Add `rigor_percent` field (int) to `domain.Interview` struct.
    *   Wire the slider UI to the interview configuration page (`web/app/page.tsx`).
    *   Pass the value through the API to the backend and inject it into the question generator system/user prompts in `internal/interview/engine.go`.

### 2. 🌊 Server-Sent Events (SSE) for Real-Time Status Updates
*   **Concept:** Replace the current 3-second polling mechanism (`api.getReport`) with a persistent `EventSource` connection to stream progress updates instantly.
*   **Behavior:**
    *   *Backend:* Add a new streaming route `GET /api/interviews/{id}/report/stream` that sets `Content-Type: text/event-stream` and writes progress updates to the connection as they happen.
    *   *Frontend:* Use a native browser `EventSource` subscription inside React `useEffect` to listen to the status stream and automatically close when the state becomes `"completed"` or `"failed"`.

### 3. 📝 LLM Call Logging & Evaluation Tracing Audit Log
*   **Concept:** Add comprehensive logging of LLM input prompts and raw output responses to a local log file or dedicated DB collection (`llm_audit_logs`) to easily diagnose failures, latency, and parser errors.
*   **Behavior:**
    *   *Debug/Audit Mode:* Control LLM logging through a config flag or environment variable (e.g., `LLM_LOG_LEVEL=debug`).
    *   *Backend:* Capture raw inputs (system prompts, user prompts) and raw outputs/errors for every Ollama or Anthropic invocation and log them, optionally masking sensitive candidate info.
    *   *Developer/Admin Logs:* Expose a dev route or output structured traces to a file (`logs/llm_calls.log`) to easily troubleshoot formatting or JSON parser failures.

---

## Completed Features (Recent)
*   **📱 Simplified Navigation Header:** Removed the hamburger dropdown details menu and laid out direct buttons ("✨ Start fresh" and "📁 Past interviews") in the navbar to make navigation cleaner and faster.
*   **🔄 Incremental Pipeline Resumption:** Enabled true resumption for both turn evaluations and report-level compilation steps (Evaluator Panel, Strongest Signals, Risks, Recommendations, Ideal Response Guide). Interrupted generations will resume exactly from the last successful step, skipping already-completed LLM calls.
*   **📹 Live Webcam & Mic Recording:** Added an interactive "Record your answer" popup modal. Captures candidate video/audio responses directly in a square camera viewport with recording timer tracking.
*   **💾 Permanent Media Storage & Fallbacks:** Backend saves uploaded media files permanently under `uploads/{interview_id}/`. Implemented automatic fallback mock transcripts and gaze metrics when Whisper or video detector binaries are not configured locally.
*   **⏳ Asynchronous Report Generation:** Avoids gateway/connection timeouts by running evaluation in a background goroutine and exposing active status.
*   **📊 Pipeline Progress Bar & Step Tracker:** Displays a smooth progress percentage bar and shows exactly which step in the execution pipeline is `✅ done`, `⏳ running`, or `⚪ pending`.
*   **📁 Past Interviews Dashboard:** A full rounds dashboard page at `/history` supporting reviewing past transcripts/JDs/resumes and deleting rounds cascade.
