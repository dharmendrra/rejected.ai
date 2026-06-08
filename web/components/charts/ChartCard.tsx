"use client";

import { useEffect, useState, type ReactNode } from "react";

interface ChartCardProps {
  title: string;
  subtitle?: string;
  // The chart to render. Receives the height it should fill so the same chart
  // can render compact inline and large inside the zoom modal.
  children: (height: number) => ReactNode;
  // Optional extra controls rendered in the card header (e.g. a selector).
  headerExtra?: ReactNode;
  // Inline chart height in px.
  height?: number;
  // Empty-state message; when set the chart body is replaced by this.
  empty?: string | null;
}

export default function ChartCard({
  title,
  subtitle,
  children,
  headerExtra,
  height = 240,
  empty = null,
}: ChartCardProps) {
  const [zoomed, setZoomed] = useState(false);

  useEffect(() => {
    if (!zoomed) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") setZoomed(false);
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [zoomed]);

  const Header = (
    <div className="flex between" style={{ alignItems: "flex-start", gap: 8, marginBottom: 12 }}>
      <div>
        <h2 style={{ margin: 0 }}>{title}</h2>
        {subtitle && (
          <div className="muted" style={{ fontSize: 12, marginTop: 4, textTransform: "none", letterSpacing: 0 }}>
            {subtitle}
          </div>
        )}
      </div>
      <div className="flex" style={{ gap: 8, alignItems: "center" }}>
        {headerExtra}
        <button
          className="ghost"
          aria-label={`Expand ${title}`}
          title="Expand to fullscreen"
          onClick={() => setZoomed(true)}
          style={{ fontSize: 14, padding: "4px 10px", lineHeight: 1 }}
        >
          ⤢
        </button>
      </div>
    </div>
  );

  return (
    <div className="panel" style={{ margin: 0, display: "flex", flexDirection: "column" }}>
      {Header}
      {empty ? (
        <div
          className="muted small"
          style={{
            height,
            display: "flex",
            alignItems: "center",
            justifyContent: "center",
            textAlign: "center",
            padding: "0 12px",
          }}
        >
          {empty}
        </div>
      ) : (
        children(height)
      )}

      {zoomed && (
        <div className="modal-overlay" onClick={() => setZoomed(false)}>
          <div
            className="modal-content"
            onClick={(e) => e.stopPropagation()}
            style={{ maxWidth: "min(1100px, 95vw)", textAlign: "left", padding: 20 }}
          >
            <div className="flex between" style={{ alignItems: "flex-start", gap: 8, marginBottom: 16 }}>
              <div>
                <h2 style={{ margin: 0 }}>{title}</h2>
                {subtitle && (
                  <div className="muted" style={{ fontSize: 12, marginTop: 4, textTransform: "none", letterSpacing: 0 }}>
                    {subtitle}
                  </div>
                )}
              </div>
              <button
                className="ghost"
                aria-label="Close"
                title="Close (Esc)"
                onClick={() => setZoomed(false)}
                style={{ fontSize: 16, padding: "4px 12px", lineHeight: 1 }}
              >
                ✕
              </button>
            </div>
            {empty ? (
              <div
                className="muted small"
                style={{ height: 480, display: "flex", alignItems: "center", justifyContent: "center" }}
              >
                {empty}
              </div>
            ) : (
              children(Math.min(560, Math.round(window.innerHeight * 0.7)))
            )}
          </div>
        </div>
      )}
    </div>
  );
}
