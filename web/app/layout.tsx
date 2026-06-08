import "./globals.css";
import type { Metadata } from "next";
import Header from "@/components/Header";

export const metadata: Metadata = {
  title: "rejected.ai — Interview Intelligence",
  description: "Local-first, evidence-accumulating interview intelligence.",
};

export default function RootLayout({ children }: { children: React.ReactNode }) {
  return (
    <html lang="en" suppressHydrationWarning>
      <body suppressHydrationWarning>
        <div className="container" style={{ position: "relative" }}>
          <Header />
          {children}
        </div>
      </body>
    </html>
  );
}
