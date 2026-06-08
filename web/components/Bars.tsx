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
    <div style={{ display: "grid", gap: 6 }}>
      <Bar value={cool} kind="cool" />
      <Bar value={normal} kind="normal" />
      <Bar value={hot} kind="hot" />
    </div>
  );
}

// Evolution renders the turn-by-turn competency score progression as a beautiful timeline path.
export function Evolution({ snaps }: { snaps: ConfidenceSnapshot[] }) {
  const sorted = [...snaps].sort((a, b) => a.turn - b.turn);
  
  const getScoreColors = (val: number) => {
    if (val >= 0.8) return { bg: "rgba(62,207,142,0.08)", border: "rgba(62,207,142,0.25)", color: "var(--good)" };
    if (val >= 0.6) return { bg: "rgba(245,166,35,0.08)", border: "rgba(245,166,35,0.25)", color: "var(--warn)" };
    return { bg: "rgba(255,107,107,0.08)", border: "rgba(255,107,107,0.25)", color: "var(--bad)" };
  };

  return (
    <div style={{ display: "flex", alignItems: "center", gap: "6px", flexWrap: "wrap", margin: "8px 0" }}>
      {sorted.map((s, idx) => {
        const pct = Math.round(s.normal * 100);
        const { bg, border, color } = getScoreColors(s.normal);
        
        return (
          <div key={s.turn} style={{ display: "flex", alignItems: "center" }}>
            <div 
              style={{ 
                background: bg,
                border: `1px solid ${border}`,
                borderRadius: "6px", 
                padding: "4px 10px", 
                display: "flex", 
                alignItems: "center", 
                gap: "8px",
                boxShadow: "0 1px 2px rgba(0,0,0,0.02)"
              }}
              title={`Question ${s.turn} evaluation score`}
            >
              <span style={{ fontSize: "11px", color: "var(--muted)", fontWeight: "600" }}>Q{s.turn}</span>
              <span style={{ fontSize: "13px", fontWeight: "700", color: color }}>{pct}%</span>
            </div>
            {idx < sorted.length - 1 && (
              <span style={{ color: "var(--border)", margin: "0 4px", fontSize: "14px", fontWeight: "normal" }}>→</span>
            )}
          </div>
        );
      })}
    </div>
  );
}
