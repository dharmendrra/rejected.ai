# Video Metadata Format (Phase 10)

Measurable-only video signals for one interview answer. The platform computes
**engagement, attention, participation, and timing** from raw per-frame counts.
It **never** infers honesty, intelligence, personality, mood, or any other trait.
Every stored field is a count, a percentage of analyzed frames, or a duration.

There are two ingest paths, mirroring the Phase 9 audio design:

1. **Provided metrics** — the caller POSTs already-extracted `FrameMetrics` JSON.
   Always available; no engine required.
2. **Detector upload** — the caller uploads a video clip; an external detector CLI
   inspects it and emits `FrameMetrics` JSON, which the platform then aggregates.
   Available only when `VIDEO_DETECTOR_BIN` is configured.

## FrameMetrics (input)

The raw per-turn observations. These are the **only** inputs to video analysis —
nothing is inferred, everything is counted.

| Field | Type | Meaning |
|---|---|---|
| `frames_analyzed` | int | Frames the detector inspected at all. Denominator for the `*_pct` fields below. |
| `frames_face_present` | int | Frames in which a (single) candidate face was detected. |
| `frames_gaze_on_screen` | int | Frames in which the candidate's gaze was directed at the screen. |
| `frames_multi_face` | int | Frames in which more than one face was detected. |
| `on_camera_sec` | float | Seconds the candidate was on camera. |
| `duration_sec` | float | Total clip length in seconds. Denominator for `on_camera_pct`. |

```json
{
  "frames_analyzed": 1800,
  "frames_face_present": 1710,
  "frames_gaze_on_screen": 1520,
  "frames_multi_face": 0,
  "on_camera_sec": 60.0,
  "duration_sec": 62.0
}
```

## VideoMetadata (stored / returned)

Computed by `media.AnalyzeVideo`. Each `*_pct` is a direct share of its
denominator, clamped to `[0,100]`, and `0` when the denominator is 0.

| Field | Derivation | Signal class |
|---|---|---|
| `face_present_pct` | `frames_face_present / frames_analyzed * 100` | engagement |
| `gaze_on_screen_pct` | `frames_gaze_on_screen / frames_analyzed * 100` | attention |
| `on_camera_pct` | `on_camera_sec / duration_sec * 100` | participation |
| `multi_face_pct` | `frames_multi_face / frames_analyzed * 100` | observation (extra faces in frame) |
| `duration_sec` | passthrough | timing |
| `latency_ms` | caller-provided | timing (response latency) |
| `frames_analyzed` | passthrough | sample size / confidence in the above |

`source` is `"provided"` (metrics path) or `"detector"` (upload path). One document
is kept per `(interview_id, turn)` — re-ingesting a turn replaces the prior doc.

## External detector contract

`ExternalDetector` (configured via `VIDEO_DETECTOR_BIN`, optional
`VIDEO_DETECTOR_MODEL`) is invoked as:

```
<VIDEO_DETECTOR_BIN> -i <videoPath> [-m <VIDEO_DETECTOR_MODEL>]
```

It must print a single `FrameMetrics` JSON object to **stdout** and exit `0`.
On non-zero exit, stderr is surfaced in the error. The platform intentionally
does not bundle a computer-vision stack — the detector is the user's pluggable
binary, keeping the Go service dependency-light and the signals honest (counted,
not inferred).

## Edge cases

- **No detector configured** → `POST /video` returns `501` with guidance to use
  `POST /video-metadata`.
- **Empty metrics** (`frames_analyzed` and `duration_sec` both ≤ 0) → `400`.
- **Malformed counts** (e.g. `frames_face_present > frames_analyzed`, or
  `on_camera_sec > duration_sec`) → the percentage clamps to `100` rather than
  exceeding it.
- **Zero frames / zero duration** → the corresponding percentage is `0` (no
  division by zero).
