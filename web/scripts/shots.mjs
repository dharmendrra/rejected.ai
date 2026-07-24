// Screenshot capture for the rejected.ai video brief.
// Run:  node web/scripts/shots.mjs
// Requires backend (:8090) and frontend (:3000) running, and the global
// playwright module (resolved by absolute path below).
import { createRequire } from "module";
const require = createRequire(import.meta.url);
const { chromium } = require("/usr/local/lib/node_modules/playwright");

const BASE = "http://localhost:3000";
const API = "http://localhost:8090";
const OUT = "screenshots";

const STRONG_HIRE = "6a266fdf42cce3ef133a91ed"; // Alex Mercer
const HR_ROUND = "6a266fdf42cce3ef133a921d";    // Sarah Connor
const PRIYA = "6a2433305d177ad4a7afe989";       // active, 4 answered turns -> real eval

const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const browser = await chromium.launch();
const ctx = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2, // retina-sharp text for a 1080p+ video
});
const page = await ctx.newPage();

async function fullShot(name) {
  await page.screenshot({ path: `${OUT}/${name}`, fullPage: true });
  console.log("full:", name);
}

// Capture a single report section by its <h2> heading, as a big readable image.
async function sectionShot(headingText, name) {
  try {
    const h = page.locator(`h2:has-text("${headingText}")`).first();
    const panel = h.locator(
      'xpath=ancestor::div[contains(concat(" ",normalize-space(@class)," ")," panel ")][1]'
    );
    await panel.scrollIntoViewIfNeeded();
    await sleep(300);
    await panel.screenshot({ path: `${OUT}/${name}` });
    console.log("section:", name);
  } catch (e) {
    console.log("MISS section", headingText, "->", e.message.split("\n")[0]);
  }
}

// 1) Strong-hire report: full page + each section big.
await page.goto(`${BASE}/interview/${STRONG_HIRE}/report`, { waitUntil: "domcontentloaded" });
await sleep(4500);
await fullShot("report_strong_hire.png");
const sections = [
  ["Recommendation", "sec_recommendation.png"],
  ["Live confidence", "sec_live_confidence.png"],
  ["Strongest signals", "sec_signals.png"],
  ["Risk areas", "sec_risks.png"],
  ["Evaluator panel", "sec_evaluator_panel.png"],
  ["Score evolution", "sec_score_evolution.png"],
  ["Ideal Response Guide", "sec_ideal_response.png"],
  ["Transcript & evidence", "sec_transcript_evidence.png"],
  ["Competency breakdown", "sec_competency_breakdown.png"],
];
for (const [h, n] of sections) await sectionShot(h, n);

// 2) Candidate Coaching tab (strong hire + HR round), full + section.
for (const [id, full, sec] of [
  [STRONG_HIRE, "coaching_strong_hire.png", "sec_coaching_strong.png"],
  [HR_ROUND, "coaching_hr_round.png", "sec_coaching_hr.png"],
]) {
  await page.goto(`${BASE}/interview/${id}/report`, { waitUntil: "domcontentloaded" });
  await sleep(4000);
  await page.getByText("Candidate Coaching", { exact: false }).first().click();
  await sleep(1500);
  await fullShot(full);
  await sectionShot("Candidate Coaching Guide", sec);
}

// 3) Evaluation progress screen — trigger a real generation, capture while the LLM works.
console.log("triggering report generation on Priya Nair...");
await ctx.request.post(`${API}/api/interviews/${PRIYA}/report`);
await page.goto(`${BASE}/interview/${PRIYA}/report`, { waitUntil: "domcontentloaded" });
try {
  await page.waitForSelector("text=Generating Hiring Intelligence", { timeout: 20000 });
  await sleep(2500);
  await fullShot("evaluation_progress_1.png");
  // Big crop of just the progress panel.
  try {
    const p = page.locator("text=Generating Hiring Intelligence").locator(
      'xpath=ancestor::div[contains(concat(" ",normalize-space(@class)," ")," panel ")][1]'
    );
    await p.screenshot({ path: `${OUT}/sec_evaluation_progress.png` });
    console.log("section: sec_evaluation_progress.png");
  } catch (e) {
    console.log("MISS progress panel:", e.message.split("\n")[0]);
  }
  await sleep(18000); // let the pipeline advance a few steps
  await fullShot("evaluation_progress_2.png");
} catch (e) {
  console.log("progress screen not shown:", e.message.split("\n")[0]);
  await fullShot("evaluation_progress_fallback.png");
}

await browser.close();
console.log("done");
