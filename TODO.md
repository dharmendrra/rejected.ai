# Roadmap & Todo List

This document tracks future features, design iterations, and planned improvements for **rejected.ai**.

## 📌 Active / Upcoming Tasks

### 1. 🌊 Server-Sent Events (SSE) for Real-Time Status Updates
*   **Concept:** Replace the current 3-second polling mechanism (`api.getReport`) with a persistent `EventSource` connection to stream progress updates instantly.
*   **Behavior:**
    *   *Backend:* Add a new streaming route `GET /api/interviews/{id}/report/stream` that sets `Content-Type: text/event-stream` and writes progress updates to the connection as they happen.
    *   *Frontend:* Use a native browser `EventSource` subscription inside React `useEffect` to listen to the status stream and automatically close when the state becomes `"completed"` or `"failed"`.

---

## Completed Features (Recent)
*   **🎛️ Interview Rigor / Difficulty Slider (0–100%):** Slider on the config page; `rigor_percent` flows through the API and is now injected into the batch question generator (`generateAllQuestions`), so question depth/hardness tracks the 0–100% scale. Verified end-to-end on the same JD/résumé: 5% → basic conceptual checks, 95% → multi-region distributed system design with CAP trade-offs.
*   **📝 LLM Call Audit Logging:** Optional `LLM_LOG_LEVEL` (`off` / `info` / `debug`) wraps every Ollama/Anthropic call via a decorator and appends JSON Lines traces (model, prompt/output sizes, latency, errors; full prompts + raw responses at `debug`) to `logs/llm_calls.log`. Documented in `notes/LLM_AUDIT_LOG_FORMAT.md`.
*   **📱 Simplified Navigation Header:** Removed the hamburger dropdown details menu and laid out direct buttons ("✨ Start fresh" and "📁 Past interviews") in the navbar to make navigation cleaner and faster.
*   **🔄 Incremental Pipeline Resumption:** Enabled true resumption for both turn evaluations and report-level compilation steps (Evaluator Panel, Strongest Signals, Risks, Recommendations, Ideal Response Guide). Interrupted generations will resume exactly from the last successful step, skipping already-completed LLM calls.
*   **📹 Live Webcam & Mic Recording:** Added an interactive "Record your answer" popup modal. Captures candidate video/audio responses directly in a square camera viewport with recording timer tracking.
*   **💾 Permanent Media Storage & Fallbacks:** Backend saves uploaded media files permanently under `uploads/{interview_id}/`. Implemented automatic fallback mock transcripts and gaze metrics when Whisper or video detector binaries are not configured locally.
*   **⏳ Asynchronous Report Generation:** Avoids gateway/connection timeouts by running evaluation in a background goroutine and exposing active status.
*   **📊 Pipeline Progress Bar & Step Tracker:** Displays a smooth progress percentage bar and shows exactly which step in the execution pipeline is `✅ done`, `⏳ running`, or `⚪ pending`.
*   **📁 Past Interviews Dashboard:** A full rounds dashboard page at `/history` supporting reviewing past transcripts/JDs/resumes and deleting rounds cascade.
