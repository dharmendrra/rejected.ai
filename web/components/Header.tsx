"use client";

import Link from "next/link";

export default function Header() {
  return (
    <div className="header" style={{ display: "flex", justifyContent: "space-between", alignItems: "center", marginBottom: 16 }}>
      <div style={{ display: "flex", alignItems: "baseline", gap: 12 }}>
        <Link href="/" className="brand" style={{ color: "var(--text)" }}>
          rejected<span className="dim">.ai</span>
        </Link>
        <span className="note">evidence-accumulating interview intelligence</span>
      </div>
      <div style={{ display: "flex", gap: 8 }}>
        <Link href="/" style={{ textDecoration: "none" }}>
          <button className="ghost" style={{ fontSize: "13px", padding: "6px 12px", border: "1px solid var(--border)", background: "transparent", cursor: "pointer", borderRadius: "8px" }}>
            ✨ Start fresh
          </button>
        </Link>
        <Link href="/history" style={{ textDecoration: "none" }}>
          <button className="ghost" style={{ fontSize: "13px", padding: "6px 12px", border: "1px solid var(--border)", background: "transparent", cursor: "pointer", borderRadius: "8px" }}>
            📁 Past interviews
          </button>
        </Link>
      </div>
    </div>
  );
}
