"use client";

import {
  Bar,
  BarChart,
  Brush,
  CartesianGrid,
  Cell,
  Legend,
  Line,
  LineChart,
  Pie,
  PieChart,
  PolarAngleAxis,
  PolarGrid,
  PolarRadiusAxis,
  Radar,
  RadarChart,
  ResponsiveContainer,
  Scatter,
  ScatterChart,
  Tooltip,
  XAxis,
  YAxis,
  ZAxis,
} from "recharts";

import type {
  CompetencyProfileItem,
  CompetencyTrend,
  ConfidencePoint,
  CoverageItem,
  PersonaCompetency,
  RigorVsConfidenceItem,
  RiskBucket,
  ScoreEvolution,
  TopSignal,
  VerdictMixItem,
  DecisionValue,
} from "@/lib/api";

import {
  AXIS_PROPS,
  C,
  DECISION_COLOR,
  DECISION_LABEL,
  GRID_STROKE,
  HEX,
  RISK_CATEGORY_LABEL,
  SEVERITY_COLOR,
  seriesColor,
  TOOLTIP_STYLE,
} from "./theme";

type Nav = (interviewId: string) => void;

const fmtDate = (s: string) => {
  if (!s) return "";
  const d = new Date(s);
  if (isNaN(d.getTime())) return s;
  return d.toLocaleDateString(undefined, { month: "short", day: "numeric" });
};

const pct = (v: number) => `${Math.round(v * 100)}%`;

// ─── Verdict mix donut ────────────────────────────────────────────────────────

export function VerdictMixChart({ data, height }: { data: VerdictMixItem[]; height: number }) {
  const rows = data.filter((d) => d.count > 0);
  return (
    <ResponsiveContainer width="100%" height={height}>
      <PieChart>
        <Pie
          data={rows}
          dataKey="count"
          nameKey="decision"
          innerRadius="55%"
          outerRadius="80%"
          paddingAngle={2}
          stroke={HEX.panel}
        >
          {rows.map((d) => (
            <Cell key={d.decision} fill={DECISION_COLOR[d.decision as DecisionValue] ?? HEX.muted} />
          ))}
        </Pie>
        <Tooltip
          {...TOOLTIP_STYLE}
          formatter={(value: number, name: string) => [value, DECISION_LABEL[name as DecisionValue] ?? name]}
        />
        <Legend
          formatter={(value: string) => DECISION_LABEL[value as DecisionValue] ?? value}
          wrapperStyle={{ fontSize: 12, color: HEX.muted }}
        />
      </PieChart>
    </ResponsiveContainer>
  );
}

// ─── Confidence over time (line + brush) ──────────────────────────────────────

export function ConfidenceOverTimeChart({
  data,
  height,
  onSelect,
}: {
  data: ConfidencePoint[];
  height: number;
  onSelect?: Nav;
}) {
  const rows = data.map((d, i) => ({ ...d, idx: i, label: fmtDate(d.at) }));
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={rows} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
        <CartesianGrid stroke={GRID_STROKE} />
        <XAxis dataKey="label" {...AXIS_PROPS} />
        <YAxis domain={[0, 1]} tickFormatter={pct} {...AXIS_PROPS} />
        <Tooltip
          {...TOOLTIP_STYLE}
          formatter={(value: number) => [pct(value), "Confidence"]}
          labelFormatter={((_l: unknown, p: { payload?: ConfidencePoint }[]) => {
            const row = p?.[0]?.payload;
            if (!row) return "";
            return `${row.candidate_name || "Unknown"} · ${row.type || ""} · ${fmtDate(row.at)}`;
          }) as never}
        />
        <Line
          type="monotone"
          dataKey="confidence"
          stroke={HEX.accent}
          strokeWidth={2}
          dot={{ r: 3, fill: HEX.accent, cursor: onSelect ? "pointer" : "default" }}
          activeDot={
            {
              r: 5,
              cursor: onSelect ? "pointer" : "default",
              onClick: (_e: unknown, p: { payload?: ConfidencePoint }) => {
                if (onSelect && p?.payload?.interview_id) onSelect(p.payload.interview_id);
              },
              // Recharts' activeDot prop typing does not model the payload-carrying
              // click handler, so widen it here.
            } as never
          }
        />
        {rows.length > 6 && <Brush dataKey="label" height={20} stroke={HEX.accent} fill={HEX.panel2} travellerWidth={8} />}
      </LineChart>
    </ResponsiveContainer>
  );
}

// ─── Competency trends (multi-line) ───────────────────────────────────────────

export function CompetencyTrendsChart({ data, height }: { data: CompetencyTrend[]; height: number }) {
  // Merge all trend points onto a shared x-axis keyed by date.
  const dateSet = new Set<string>();
  data.forEach((t) => t.points.forEach((p) => dateSet.add(p.at)));
  const dates = Array.from(dateSet).sort();
  const rows = dates.map((at) => {
    const row: Record<string, number | string> = { label: fmtDate(at) };
    data.forEach((t) => {
      const p = t.points.find((pp) => pp.at === at);
      if (p) row[t.competency] = p.normal;
    });
    return row;
  });

  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={rows} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
        <CartesianGrid stroke={GRID_STROKE} />
        <XAxis dataKey="label" {...AXIS_PROPS} />
        <YAxis domain={[0, 1]} tickFormatter={pct} {...AXIS_PROPS} />
        <Tooltip {...TOOLTIP_STYLE} formatter={(value: number) => pct(value)} />
        <Legend wrapperStyle={{ fontSize: 11, color: HEX.muted }} />
        {data.map((t, i) => (
          <Line
            key={t.competency}
            type="monotone"
            dataKey={t.competency}
            stroke={seriesColor(i)}
            strokeWidth={2}
            dot={{ r: 2 }}
            connectNulls
          />
        ))}
      </LineChart>
    </ResponsiveContainer>
  );
}

// ─── Current competency profile (radar, latest vs first) ──────────────────────

export function CompetencyProfileChart({
  data,
  height,
  showFirst,
}: {
  data: CompetencyProfileItem[];
  height: number;
  showFirst: boolean;
}) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <RadarChart data={data} outerRadius="72%">
        <PolarGrid stroke={GRID_STROKE} />
        <PolarAngleAxis dataKey="competency" tick={{ fill: HEX.muted, fontSize: 10 }} />
        <PolarRadiusAxis domain={[0, 1]} tick={{ fill: HEX.muted, fontSize: 9 }} tickFormatter={pct} />
        {showFirst && (
          <Radar name="First" dataKey="first_normal" stroke={HEX.muted} fill={HEX.muted} fillOpacity={0.15} />
        )}
        <Radar name="Latest" dataKey="normal" stroke={HEX.accent} fill={HEX.accent} fillOpacity={0.35} />
        <Tooltip {...TOOLTIP_STYLE} formatter={(value: number) => pct(value)} />
        <Legend wrapperStyle={{ fontSize: 12, color: HEX.muted }} />
      </RadarChart>
    </ResponsiveContainer>
  );
}

// ─── Rigor vs confidence (scatter) ────────────────────────────────────────────

export function RigorScatterChart({
  data,
  height,
  onSelect,
}: {
  data: RigorVsConfidenceItem[];
  height: number;
  onSelect?: Nav;
}) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <ScatterChart margin={{ top: 8, right: 16, bottom: 8, left: -16 }}>
        <CartesianGrid stroke={GRID_STROKE} />
        <XAxis
          type="number"
          dataKey="rigor_percent"
          name="Rigor"
          domain={[0, 100]}
          unit="%"
          {...AXIS_PROPS}
        />
        <YAxis type="number" dataKey="confidence" name="Confidence" domain={[0, 1]} tickFormatter={pct} {...AXIS_PROPS} />
        <ZAxis range={[60, 60]} />
        <Tooltip
          {...TOOLTIP_STYLE}
          cursor={{ stroke: GRID_STROKE }}
          formatter={(value: number, name: string) => (name === "Confidence" ? pct(value) : `${value}%`)}
        />
        <Scatter
          data={data}
          fill={HEX.accent}
          onClick={((node: { payload?: RigorVsConfidenceItem }) => {
            const id = node?.payload?.interview_id;
            if (onSelect && id) onSelect(id);
          }) as never}
          cursor={onSelect ? "pointer" : "default"}
        >
          {data.map((d, i) => (
            <Cell key={i} fill={DECISION_COLOR[d.decision as DecisionValue] ?? HEX.accent} />
          ))}
        </Scatter>
      </ScatterChart>
    </ResponsiveContainer>
  );
}

// ─── Coverage bars ────────────────────────────────────────────────────────────

export function CoverageBarChart({ data, height }: { data: CoverageItem[]; height: number }) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <BarChart data={data} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
        <CartesianGrid stroke={GRID_STROKE} vertical={false} />
        <XAxis dataKey="key" {...AXIS_PROPS} interval={0} angle={-15} textAnchor="end" height={50} />
        <YAxis allowDecimals={false} {...AXIS_PROPS} />
        <Tooltip {...TOOLTIP_STYLE} cursor={{ fill: "rgba(138,150,168,0.08)" }} />
        <Bar dataKey="count" fill={HEX.accent} radius={[4, 4, 0, 0]} />
      </BarChart>
    </ResponsiveContainer>
  );
}

// ─── Recurring risks (stacked bar by severity, grouped by category) ───────────

export function RisksChart({ data, height }: { data: RiskBucket[]; height: number }) {
  const cats = ["missing", "weak", "jd_risk"];
  const sevs = ["high", "medium", "low"];
  const rows = cats.map((cat) => {
    const row: Record<string, number | string> = { category: RISK_CATEGORY_LABEL[cat] ?? cat };
    sevs.forEach((sev) => {
      row[sev] = data
        .filter((d) => d.category === cat && d.severity === sev)
        .reduce((a, b) => a + b.count, 0);
    });
    return row;
  });

  return (
    <ResponsiveContainer width="100%" height={height}>
      <BarChart data={rows} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
        <CartesianGrid stroke={GRID_STROKE} vertical={false} />
        <XAxis dataKey="category" {...AXIS_PROPS} />
        <YAxis allowDecimals={false} {...AXIS_PROPS} />
        <Tooltip {...TOOLTIP_STYLE} cursor={{ fill: "rgba(138,150,168,0.08)" }} />
        <Legend wrapperStyle={{ fontSize: 12, color: HEX.muted }} />
        {sevs.map((sev) => (
          <Bar key={sev} dataKey={sev} stackId="risk" fill={SEVERITY_COLOR[sev]} name={sev} />
        ))}
      </BarChart>
    </ResponsiveContainer>
  );
}

// ─── Consistent strengths (horizontal bar) ────────────────────────────────────

export function TopSignalsChart({ data, height }: { data: TopSignal[]; height: number }) {
  return (
    <ResponsiveContainer width="100%" height={height}>
      <BarChart data={data} layout="vertical" margin={{ top: 8, right: 16, bottom: 4, left: 8 }}>
        <CartesianGrid stroke={GRID_STROKE} horizontal={false} />
        <XAxis type="number" allowDecimals={false} {...AXIS_PROPS} />
        <YAxis type="category" dataKey="name" width={140} tick={{ fill: HEX.muted, fontSize: 11 }} />
        <Tooltip {...TOOLTIP_STYLE} cursor={{ fill: "rgba(138,150,168,0.08)" }} />
        <Bar dataKey="count" fill={HEX.good} radius={[0, 4, 4, 0]} />
      </BarChart>
    </ResponsiveContainer>
  );
}

// ─── Evaluator lens comparison (grouped bar) ──────────────────────────────────

export function PersonaCompetencyChart({ data, height }: { data: PersonaCompetency[]; height: number }) {
  // Pivot to: one row per competency, one bar per persona.
  const compSet = new Set<string>();
  data.forEach((p) => p.competencies.forEach((c) => compSet.add(c.competency)));
  const comps = Array.from(compSet);
  const rows = comps.map((comp) => {
    const row: Record<string, number | string> = { competency: comp };
    data.forEach((p) => {
      const c = p.competencies.find((cc) => cc.competency === comp);
      if (c) row[p.persona] = c.avg_score;
    });
    return row;
  });

  return (
    <ResponsiveContainer width="100%" height={height}>
      <BarChart data={rows} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
        <CartesianGrid stroke={GRID_STROKE} vertical={false} />
        <XAxis dataKey="competency" {...AXIS_PROPS} interval={0} angle={-15} textAnchor="end" height={50} />
        <YAxis domain={[0, 1]} tickFormatter={pct} {...AXIS_PROPS} />
        <Tooltip {...TOOLTIP_STYLE} formatter={(value: number) => pct(value)} cursor={{ fill: "rgba(138,150,168,0.08)" }} />
        <Legend wrapperStyle={{ fontSize: 11, color: HEX.muted }} />
        {data.map((p, i) => (
          <Bar key={p.persona} dataKey={p.persona} fill={seriesColor(i)} radius={[3, 3, 0, 0]} />
        ))}
      </BarChart>
    </ResponsiveContainer>
  );
}

// ─── Within-interview evolution (line for one selected interview) ─────────────

export function ScoreEvolutionChart({ data, height }: { data: ScoreEvolution | null; height: number }) {
  if (!data) {
    return (
      <div
        className="muted small"
        style={{ height, display: "flex", alignItems: "center", justifyContent: "center" }}
      >
        No evolution data for this interview.
      </div>
    );
  }
  return (
    <ResponsiveContainer width="100%" height={height}>
      <LineChart data={data.series} margin={{ top: 8, right: 12, bottom: 4, left: -16 }}>
        <CartesianGrid stroke={GRID_STROKE} />
        <XAxis dataKey="turn" {...AXIS_PROPS} tickFormatter={(t: number) => `Q${t}`} />
        <YAxis domain={[0, 1]} tickFormatter={pct} {...AXIS_PROPS} />
        <Tooltip
          {...TOOLTIP_STYLE}
          formatter={(value: number) => [pct(value), "Avg score"]}
          labelFormatter={(t) => `Question ${t}`}
        />
        <Line type="monotone" dataKey="avg_normal" stroke={C.accent} strokeWidth={2} dot={{ r: 3 }} />
      </LineChart>
    </ResponsiveContainer>
  );
}
