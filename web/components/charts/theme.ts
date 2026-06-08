// Shared chart theme constants for the Progress Dashboard.
// These resolve to the dark-UI CSS variables defined in app/globals.css.

import type { DecisionValue } from "@/lib/api";

// CSS variable references (usable directly as SVG fill/stroke).
export const C = {
  accent: "var(--accent)",
  good: "var(--good)",
  warn: "var(--warn)",
  bad: "var(--bad)",
  muted: "var(--muted)",
  text: "var(--text)",
  border: "var(--border)",
  panel: "var(--panel)",
  panel2: "var(--panel-2)",
} as const;

// Concrete hex values that mirror the CSS variables. Recharts sometimes needs a
// real color string (e.g. for Pie cell fills, gradients) rather than a var().
export const HEX = {
  accent: "#5b9dff",
  good: "#3ecf8e",
  warn: "#f5a623",
  bad: "#ff6b6b",
  muted: "#8b96a8",
  text: "#e6ebf2",
  border: "#2a3344",
  panel: "#141923",
  panel2: "#1b2230",
} as const;

// Fixed color per verdict, green→red.
export const DECISION_COLOR: Record<DecisionValue, string> = {
  strong_hire: HEX.good,
  hire: "#7fd8a8",
  hire_with_risks: HEX.warn,
  borderline: "#d79a52",
  no_hire: HEX.bad,
};

export const DECISION_LABEL: Record<DecisionValue, string> = {
  strong_hire: "Strong hire",
  hire: "Hire",
  hire_with_risks: "Hire w/ risks",
  borderline: "Borderline",
  no_hire: "No hire",
};

export const SEVERITY_COLOR: Record<string, string> = {
  low: HEX.muted,
  medium: HEX.warn,
  high: HEX.bad,
};

export const RISK_CATEGORY_LABEL: Record<string, string> = {
  missing: "Missing",
  weak: "Weak",
  jd_risk: "JD risk",
};

// A small categorical palette for multi-series charts (competency trends, etc).
export const SERIES_PALETTE = [
  "#5b9dff",
  "#3ecf8e",
  "#f5a623",
  "#ff6b6b",
  "#b388ff",
  "#4dd0e1",
  "#ffd166",
  "#ef79b3",
  "#9ccc65",
  "#ff8a65",
];

export function seriesColor(i: number): string {
  return SERIES_PALETTE[i % SERIES_PALETTE.length];
}

// Shared Recharts styling props.
export const AXIS_PROPS = {
  stroke: HEX.muted,
  tick: { fill: HEX.muted, fontSize: 11 },
};

export const GRID_STROKE = "rgba(138,150,168,0.15)";

export const TOOLTIP_STYLE = {
  contentStyle: {
    background: HEX.panel2,
    border: `1px solid ${HEX.border}`,
    borderRadius: 8,
    color: HEX.text,
    fontSize: 12,
  },
  labelStyle: { color: HEX.text },
  itemStyle: { color: HEX.text },
};
