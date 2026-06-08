"use client";

import { useEffect, useMemo, useState, type CSSProperties } from "react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { api, type Dashboard } from "@/lib/api";
import ChartCard from "@/components/charts/ChartCard";
import KpiTiles from "@/components/charts/KpiTiles";
import { sampleDashboard } from "@/components/charts/sample";
import {
  CompetencyProfileChart,
  CompetencyTrendsChart,
  ConfidenceOverTimeChart,
  CoverageBarChart,
  PersonaCompetencyChart,
  RigorScatterChart,
  RisksChart,
  ScoreEvolutionChart,
  TopSignalsChart,
  VerdictMixChart,
} from "@/components/charts/Charts";
import type { TrendDirection } from "@/lib/api";

const DIRECTION_META: Record<TrendDirection, { label: string; color: string; icon: string }> = {
  improving: { label: "improving", color: "var(--good)", icon: "▲" },
  declining: { label: "declining", color: "var(--bad)", icon: "▼" },
  stable: { label: "stable", color: "var(--muted)", icon: "▬" },
  new: { label: "new", color: "var(--accent)", icon: "✦" },
};

// Shared styling for the header filter controls (scope / period / date inputs).
const ctrlStyle: CSSProperties = {
  background: "var(--panel-2)",
  color: "var(--text)",
  border: "1px solid var(--border)",
  borderRadius: 8,
  padding: "8px 10px",
  font: "inherit",
  fontSize: 13,
};

export default function DashboardPage() {
  const router = useRouter();
  const [data, setData] = useState<Dashboard | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [usingSample, setUsingSample] = useState(false);
  const [candidate, setCandidate] = useState("all");
  // Distinct candidate names are captured once (from the unscoped fetch) so the
  // dropdown does not collapse to a single name after scoping.
  const [candidateNames, setCandidateNames] = useState<string[]>([]);
  const [selectedEvolution, setSelectedEvolution] = useState<string>("");

  // Date filter: "all" | "month" | "year" | "range".
  const [period, setPeriod] = useState<"all" | "month" | "year" | "range">("all");
  const [monthVal, setMonthVal] = useState<string>(""); // YYYY-MM
  const [yearVal, setYearVal] = useState<string>(""); // YYYY
  const [fromVal, setFromVal] = useState<string>(""); // YYYY-MM-DD
  const [toVal, setToVal] = useState<string>(""); // YYYY-MM-DD

  // Resolve the selected period to RFC3339 from/to bounds for the API.
  const { from, to } = useMemo<{ from?: string; to?: string }>(() => {
    const startOf = (d: string) => (d ? new Date(`${d}T00:00:00`).toISOString() : undefined);
    const endOf = (d: string) => (d ? new Date(`${d}T23:59:59.999`).toISOString() : undefined);
    if (period === "month" && /^\d{4}-\d{2}$/.test(monthVal)) {
      const [y, m] = monthVal.split("-").map(Number);
      const last = new Date(y, m, 0).getDate(); // last day of month
      return { from: startOf(`${monthVal}-01`), to: endOf(`${monthVal}-${String(last).padStart(2, "0")}`) };
    }
    if (period === "year" && /^\d{4}$/.test(yearVal)) {
      return { from: startOf(`${yearVal}-01-01`), to: endOf(`${yearVal}-12-31`) };
    }
    if (period === "range") {
      return { from: startOf(fromVal), to: endOf(toVal) };
    }
    return {};
  }, [period, monthVal, yearVal, fromVal, toVal]);

  useEffect(() => {
    let cancelled = false;
    const load = async () => {
      setLoading(true);
      setError(null);
      try {
        const d = await api.getDashboard({ candidate, from, to });
        if (cancelled) return;
        setData(d);
        setUsingSample(false);
      } catch (err: any) {
        if (cancelled) return;
        // Backend may not be available in this environment; fall back to the
        // local fixture so the dashboard still renders during development.
        setData(sampleDashboard);
        setUsingSample(true);
        setError(err?.message || "Could not reach /api/dashboard");
      } finally {
        if (!cancelled) setLoading(false);
      }
    };
    load();
    return () => {
      cancelled = true;
    };
  }, [candidate, from, to]);

  // Capture the full distinct-name list from an unscoped, unfiltered load so the
  // dropdown keeps everyone even after scoping or date-filtering.
  useEffect(() => {
    if (!data || candidate !== "all" || period !== "all") return;
    const names = Array.from(
      new Set(data.confidence_over_time.map((p) => p.candidate_name).filter((n): n is string => !!n))
    ).sort();
    setCandidateNames(names);
  }, [data, candidate, period]);

  // Default the evolution selector to the first available interview.
  useEffect(() => {
    if (!data) return;
    if (data.score_evolution.length === 0) {
      setSelectedEvolution("");
    } else if (!data.score_evolution.some((e) => e.interview_id === selectedEvolution)) {
      setSelectedEvolution(data.score_evolution[0].interview_id);
    }
  }, [data, selectedEvolution]);

  const goToReport = (id: string) => {
    if (id) router.push(`/interview/${id}/report`);
  };

  const selectedEvo = useMemo(
    () => data?.score_evolution.find((e) => e.interview_id === selectedEvolution) ?? null,
    [data, selectedEvolution]
  );

  const interviewCount = data?.kpis.total_interviews ?? 0;
  const completedCount = data?.kpis.completed_reports ?? 0;
  const notEnoughForTrends = completedCount < 2 ? "Not enough data yet — complete at least 2 interviews." : null;

  if (loading && !data) {
    return (
      <div style={{ textAlign: "center", padding: "64px 0" }}>
        <div className="spin-timer" style={{ fontSize: "32px", marginBottom: "16px" }}>⏳</div>
        <div className="muted">Loading progress dashboard...</div>
      </div>
    );
  }

  if (data && interviewCount === 0) {
    return (
      <div style={{ paddingBottom: "64px" }}>
        <h1 style={{ margin: "0 0 4px 0", fontSize: 24, fontWeight: 700 }}>📈 Progress Dashboard</h1>
        <p className="muted small" style={{ margin: "0 0 24px 0" }}>
          Trends and signals across all your practice interviews.
        </p>
        <div className="panel" style={{ textAlign: "center", padding: "48px 24px" }}>
          <div style={{ fontSize: 48, marginBottom: 16 }}>📈</div>
          <h3 style={{ margin: "0 0 8px 0" }}>Nothing to chart yet</h3>
          <p className="muted small" style={{ maxWidth: 420, margin: "0 auto 24px" }}>
            Run a few practice interviews and generate their reports — your verdicts, confidence,
            competencies, and risks will show up here as charts.
          </p>
          <Link href="/">
            <button>Start fresh session</button>
          </Link>
        </div>
      </div>
    );
  }

  if (!data) {
    return (
      <div style={{ paddingBottom: "64px" }}>
        <h1 style={{ margin: "0 0 16px 0", fontSize: 24, fontWeight: 700 }}>📈 Progress Dashboard</h1>
        <div className="error">{error || "Failed to load dashboard."}</div>
      </div>
    );
  }

  const trendsSorted = [...data.competency_trends].sort((a, b) => b.delta - a.delta);

  return (
    <div style={{ paddingBottom: "64px" }}>
      <div className="flex between" style={{ marginBottom: 16, flexWrap: "wrap", gap: 12, alignItems: "flex-end" }}>
        <div>
          <h1 style={{ margin: "0 0 4px 0", fontSize: 24, fontWeight: 700 }}>📈 Progress Dashboard</h1>
          <p className="muted small" style={{ margin: 0 }}>
            Portfolio view across {interviewCount} interview{interviewCount === 1 ? "" : "s"}
            {data.kpis.candidates > 0 && ` · ${data.kpis.candidates} candidate${data.kpis.candidates === 1 ? "" : "s"}`}
          </p>
        </div>
        <div className="flex" style={{ gap: 8, alignItems: "center", flexWrap: "wrap" }}>
          <label style={{ margin: 0, fontSize: 12 }} htmlFor="scope-candidate">
            Scope
          </label>
          <select
            id="scope-candidate"
            value={candidate}
            onChange={(e) => setCandidate(e.target.value)}
            style={ctrlStyle}
          >
            <option value="all">All candidates</option>
            {candidateNames.map((n) => (
              <option key={n} value={n}>
                {n}
              </option>
            ))}
          </select>

          <label style={{ margin: "0 0 0 8px", fontSize: 12 }} htmlFor="scope-period">
            Period
          </label>
          <select
            id="scope-period"
            value={period}
            onChange={(e) => setPeriod(e.target.value as typeof period)}
            style={ctrlStyle}
          >
            <option value="all">All time</option>
            <option value="month">Month</option>
            <option value="year">Year</option>
            <option value="range">Custom range</option>
          </select>

          {period === "month" && (
            <input
              type="month"
              aria-label="Month"
              value={monthVal}
              onChange={(e) => setMonthVal(e.target.value)}
              style={ctrlStyle}
            />
          )}
          {period === "year" && (
            <input
              type="number"
              aria-label="Year"
              placeholder="YYYY"
              min={2000}
              max={2100}
              value={yearVal}
              onChange={(e) => setYearVal(e.target.value)}
              style={{ ...ctrlStyle, width: 90 }}
            />
          )}
          {period === "range" && (
            <span className="flex" style={{ gap: 6, alignItems: "center" }}>
              <input
                type="date"
                aria-label="From date"
                value={fromVal}
                onChange={(e) => setFromVal(e.target.value)}
                style={ctrlStyle}
              />
              <span className="muted small">to</span>
              <input
                type="date"
                aria-label="To date"
                value={toVal}
                onChange={(e) => setToVal(e.target.value)}
                style={ctrlStyle}
              />
            </span>
          )}
        </div>
      </div>

      {usingSample && (
        <div
          className="note"
          style={{
            background: "rgba(245,166,35,0.08)",
            border: "1px solid var(--warn)",
            color: "var(--warn)",
            borderRadius: 8,
            padding: "8px 12px",
            marginBottom: 16,
            fontSize: 12,
          }}
        >
          ⚠️ Showing sample data — could not reach <code>/api/dashboard</code>
          {error ? ` (${error})` : ""}. Charts below are illustrative.
        </div>
      )}

      <KpiTiles kpis={data.kpis} />

      <div
        style={{
          display: "grid",
          gridTemplateColumns: "repeat(auto-fit, minmax(360px, 1fr))",
          gap: 16,
        }}
      >
        <ChartCard
          title="Verdict mix"
          subtitle="Recommendation decisions across completed reports"
          empty={completedCount === 0 ? "No completed reports yet." : null}
        >
          {(h) => <VerdictMixChart data={data.verdict_mix} height={h} />}
        </ChartCard>

        <ChartCard
          title="Confidence over time"
          subtitle="Click a point to open that interview's report"
          empty={data.confidence_over_time.length === 0 ? "No completed reports yet." : null}
        >
          {(h) => <ConfidenceOverTimeChart data={data.confidence_over_time} height={h} onSelect={goToReport} />}
        </ChartCard>

        <ChartCard
          title="Competency trends"
          subtitle="Normalized score per competency over interviews"
          empty={notEnoughForTrends || (data.competency_trends.length === 0 ? "No competency data yet." : null)}
          headerExtra={
            !notEnoughForTrends && trendsSorted.length > 0 ? (
              <div className="flex" style={{ gap: 6, flexWrap: "wrap", maxWidth: 320, justifyContent: "flex-end" }}>
                {trendsSorted.slice(0, 4).map((t) => {
                  const m = DIRECTION_META[t.direction] ?? DIRECTION_META.stable;
                  return (
                    <span
                      key={t.competency}
                      className="tag"
                      title={`${t.competency}: ${m.label} (${t.delta >= 0 ? "+" : ""}${Math.round(t.delta * 100)}%)`}
                      style={{ color: m.color, borderColor: m.color, fontSize: 10, padding: "1px 8px" }}
                    >
                      {m.icon} {t.competency}
                    </span>
                  );
                })}
              </div>
            ) : undefined
          }
        >
          {(h) => <CompetencyTrendsChart data={data.competency_trends} height={h} />}
        </ChartCard>

        <ChartCard
          title="Current competency profile"
          subtitle={completedCount >= 2 ? "Latest vs first interview" : "Latest interview"}
          empty={data.competency_profile.length === 0 ? "No competency data yet." : null}
        >
          {(h) => <CompetencyProfileChart data={data.competency_profile} height={h} showFirst={completedCount >= 2} />}
        </ChartCard>

        <ChartCard
          title="Rigor vs performance"
          subtitle="Interview rigor against resulting confidence · click a point for the report"
          empty={data.rigor_vs_confidence.length === 0 ? "No completed reports yet." : null}
        >
          {(h) => <RigorScatterChart data={data.rigor_vs_confidence} height={h} onSelect={goToReport} />}
        </ChartCard>

        <ChartCard
          title="Coverage by type"
          subtitle="Interviews per round type"
          empty={data.coverage.by_type.length === 0 ? "No interviews yet." : null}
        >
          {(h) => <CoverageBarChart data={data.coverage.by_type} height={h} />}
        </ChartCard>

        <ChartCard
          title="Coverage by level"
          subtitle="Interviews per seniority level"
          empty={data.coverage.by_level.length === 0 ? "No interviews yet." : null}
        >
          {(h) => <CoverageBarChart data={data.coverage.by_level} height={h} />}
        </ChartCard>

        <ChartCard
          title="Recurring risks"
          subtitle="Risk counts by category, stacked by severity"
          empty={data.risks.every((r) => r.count === 0) ? "No risks recorded yet." : null}
        >
          {(h) => <RisksChart data={data.risks} height={h} />}
        </ChartCard>

        <ChartCard
          title="Consistent strengths"
          subtitle="Most frequent strong signals"
          empty={data.top_signals.length === 0 ? "No signals recorded yet." : null}
        >
          {(h) => <TopSignalsChart data={data.top_signals} height={h} />}
        </ChartCard>

        <ChartCard
          title="Evaluator lens comparison"
          subtitle="Average per-competency score by evaluator persona"
          empty={data.persona_competency.length === 0 ? "No persona data yet." : null}
        >
          {(h) => <PersonaCompetencyChart data={data.persona_competency} height={h} />}
        </ChartCard>

        <ChartCard
          title="Within-interview evolution"
          subtitle="Average competency score by question turn"
          empty={data.score_evolution.length === 0 ? "No per-turn data yet." : null}
          headerExtra={
            data.score_evolution.length > 0 ? (
              <select
                value={selectedEvolution}
                onChange={(e) => setSelectedEvolution(e.target.value)}
                aria-label="Select interview"
                style={{
                  background: "var(--panel-2)",
                  color: "var(--text)",
                  border: "1px solid var(--border)",
                  borderRadius: 8,
                  padding: "4px 8px",
                  font: "inherit",
                  fontSize: 12,
                  maxWidth: 200,
                }}
              >
                {data.score_evolution.map((e) => (
                  <option key={e.interview_id} value={e.interview_id}>
                    {(e.candidate_name || "Unknown")}{e.type ? ` · ${e.type}` : ""}
                  </option>
                ))}
              </select>
            ) : undefined
          }
        >
          {(h) => <ScoreEvolutionChart data={selectedEvo} height={h} />}
        </ChartCard>
      </div>
    </div>
  );
}
