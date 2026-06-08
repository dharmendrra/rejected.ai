"use client";

import type { DashboardKpis } from "@/lib/api";
import { C } from "./theme";

function Tile({ label, value, hint }: { label: string; value: string; hint?: string }) {
  return (
    <div
      className="card"
      style={{ display: "flex", flexDirection: "column", gap: 4, minWidth: 0 }}
    >
      <div className="muted" style={{ fontSize: 11, textTransform: "uppercase", letterSpacing: "0.05em" }}>
        {label}
      </div>
      <div style={{ fontSize: 24, fontWeight: 700, color: C.text, overflow: "hidden", textOverflow: "ellipsis" }}>
        {value}
      </div>
      {hint && (
        <div className="muted" style={{ fontSize: 11 }}>
          {hint}
        </div>
      )}
    </div>
  );
}

export default function KpiTiles({ kpis }: { kpis: DashboardKpis }) {
  const answeredPct =
    kpis.questions_asked > 0
      ? Math.round((kpis.questions_answered / kpis.questions_asked) * 100)
      : 0;

  return (
    <div
      style={{
        display: "grid",
        gridTemplateColumns: "repeat(auto-fit, minmax(150px, 1fr))",
        gap: 12,
        marginBottom: 16,
      }}
    >
      <Tile label="Interviews" value={String(kpis.total_interviews)} hint={`${kpis.candidates} candidate${kpis.candidates === 1 ? "" : "s"}`} />
      <Tile label="Reports" value={String(kpis.completed_reports)} hint={`${kpis.pending_reports} pending`} />
      <Tile
        label="Questions"
        value={`${kpis.questions_answered}/${kpis.questions_asked}`}
        hint={`${answeredPct}% answered`}
      />
      <Tile label="Avg confidence" value={`${Math.round(kpis.avg_confidence * 100)}%`} hint="over completed reports" />
      <Tile
        label="Most improved"
        value={kpis.most_improved_competency || "—"}
        hint={kpis.most_improved_competency ? "biggest positive trend" : "needs ≥2 interviews"}
      />
    </div>
  );
}
