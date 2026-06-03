import "./globals.css";
import type { Metadata } from "next";
import Link from "next/link";

export const metadata: Metadata = {
  title: "rejected.ai — Interview Intelligence",
  description: "Local-first, evidence-accumulating interview intelligence.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en">
      <body>
        <div className="container">
          <div className="header">
            <Link href="/" className="brand" style={{ color: "var(--text)" }}>
              rejected<span className="dim">.ai</span>
            </Link>
            <span className="note">evidence-accumulating interview intelligence</span>
          </div>
          {children}
        </div>
      </body>
    </html>
  );
}
