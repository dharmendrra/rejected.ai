// Re-capture the live answering screen + record modal (fake camera) after the
// interview's timer was reset so the question UI renders.
import { createRequire } from "module";
const require = createRequire(import.meta.url);
const { chromium } = require("/usr/local/lib/node_modules/playwright");

const BASE = "http://localhost:3000";
const OUT = "screenshots";
const OPEN_INTERVIEW = "6a254bf46c11519439e99998";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const browser = await chromium.launch({
  args: ["--use-fake-device-for-media-stream", "--use-fake-ui-for-media-stream"],
});
const ctx = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
  permissions: ["camera", "microphone"],
});
const page = await ctx.newPage();

await page.goto(`${BASE}/interview/${OPEN_INTERVIEW}`, { waitUntil: "domcontentloaded" });
await sleep(3500);
await page.screenshot({ path: `${OUT}/interview_answering.png`, fullPage: true });
console.log("full: interview_answering.png");

// Big crop of the question/answer panel.
try {
  await page.locator(".panel").first().screenshot({ path: `${OUT}/sec_question_panel.png` });
  console.log("section: sec_question_panel.png");
} catch (e) { console.log("MISS question panel:", e.message.split("\n")[0]); }

// Live confidence sidebar crop.
try {
  const h = page.locator('h2:has-text("Live confidence")').first();
  const panel = h.locator('xpath=ancestor::div[contains(concat(" ",normalize-space(@class)," ")," panel ")][1]');
  await panel.screenshot({ path: `${OUT}/sec_live_confidence_runtime.png` });
  console.log("section: sec_live_confidence_runtime.png");
} catch (e) { console.log("MISS live confidence:", e.message.split("\n")[0]); }

// Record modal with fake camera.
try {
  await page.getByRole("button", { name: /Record your answer/i }).click({ timeout: 8000 });
  await sleep(3000); // fake getUserMedia connects
  await page.screenshot({ path: `${OUT}/record_modal_full.png`, fullPage: true });
  console.log("full: record_modal_full.png");
  await page.locator(".modal-content").first().screenshot({ path: `${OUT}/record_modal.png` });
  console.log("section: record_modal.png");
} catch (e) { console.log("MISS record modal:", e.message.split("\n")[0]); }

await browser.close();
console.log("done");
