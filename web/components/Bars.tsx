"use client";

import type { ConfidenceSnapshot } from "@/lib/api";

export function Bar({ value, kind = "normal" }: { value: number; kind?: "cool" | "normal" | "hot" }) {
  const pct = Math.round(Math.max(0, Math.min(1, value)) * 100);
  return (
    <div className={`bar ${kind}`} title={`${kind}: ${value.toFixed(2)}`}>
      <span style={{ width: `${pct}%` }} />
    </div>
  );
}

// LensBars shows the cool / normal / hot triad for one competency.
export function LensBars({ cool, normal, hot }: { cool: number; normal: number; hot: number }) {
  return (
    <div style={{ display: "grid", gap: 4 }}>
      <div className="flex"><span className="small muted" style={{ width: 52 }}>cool</span><Bar value={cool} kind="cool" /></div>
      <div className="flex"><span className="small muted" style={{ width: 52 }}>normal</span><Bar value={normal} kind="normal" /></div>
      <div className="flex"><span className="small muted" style={{ width: 52 }}>hot</span><Bar value={hot} kind="hot" /></div>
    </div>
  );
}

// Evolution renders the per-turn normal-confidence timeline for one competency
// as a sparkline — the visual proof of confidence evolving (and rising) over turns.
export function Evolution({ snaps }: { snaps: ConfidenceSnapshot[] }) {
  const sorted = [...snaps].sort((a, b) => a.turn - b.turn);
  return (
    <div>
      <div className="spark">
        {sorted.map((s) => (
          <div key={s.turn} style={{ height: `${Math.max(4, s.normal * 100)}%` }} title={`turn ${s.turn}: ${s.normal.toFixed(2)}`} />
        ))}
      </div>
      <div className="small muted">
        {sorted.map((s) => `t${s.turn}=${s.normal.toFixed(2)}`).join("  ")}
      </div>
    </div>
  );
}
