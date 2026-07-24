// Second capture pass: report-section crops for no-hire & HR, big coaching cards,
// home config variants, the live answering screen, and the record modal (fake camera).
// Run:  node web/scripts/shots2.mjs   (backend :8090 + frontend :3000 must be up)
import { createRequire } from "module";
const require = createRequire(import.meta.url);
const { chromium } = require("/usr/local/lib/node_modules/playwright");

const BASE = "http://localhost:3000";
const OUT = "screenshots";

const NO_HIRE = "6a266fdf42cce3ef133a9205";   // John Smith
const HR_ROUND = "6a266fdf42cce3ef133a921d";  // Sarah Connor
const STRONG_HIRE = "6a266fdf42cce3ef133a91ed"; // Alex Mercer
const OPEN_INTERVIEW = "6a254bf46c11519439e99998"; // DHARMENDRA, has an open question

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

async function full(name) { await page.screenshot({ path: `${OUT}/${name}`, fullPage: true }); console.log("full:", name); }

async function sectionShot(headingText, name) {
  try {
    const h = page.locator(`h2:has-text("${headingText}")`).first();
    const panel = h.locator('xpath=ancestor::div[contains(concat(" ",normalize-space(@class)," ")," panel ")][1]');
    await panel.scrollIntoViewIfNeeded(); await sleep(250);
    await panel.screenshot({ path: `${OUT}/${name}` });
    console.log("section:", name);
  } catch (e) { console.log("MISS", headingText, e.message.split("\n")[0]); }
}

// A) Section crops for the No-Hire and HR reports (high-vs-low contrast).
const reportSections = [
  ["Recommendation", "rec"], ["Strongest signals", "signals"], ["Risk areas", "risks"],
  ["Evaluator panel", "evaluator"], ["Transcript & evidence", "transcript"],
];
for (const [id, tag] of [[NO_HIRE, "nohire"], [HR_ROUND, "hr"]]) {
  await page.goto(`${BASE}/interview/${id}/report`, { waitUntil: "domcontentloaded" });
  await sleep(4500);
  for (const [h, n] of reportSections) await sectionShot(h, `sec_${tag}_${n}.png`);
}

// B) Big coaching cards (each card on its own, readable).
for (const [id, tag] of [[STRONG_HIRE, "strong"], [HR_ROUND, "hr"]]) {
  await page.goto(`${BASE}/interview/${id}/report`, { waitUntil: "domcontentloaded" });
  await sleep(4000);
  await page.getByText("Candidate Coaching", { exact: false }).first().click();
  await sleep(1500);
  const cards = page.locator(".grid.two .card");
  const n = Math.min(await cards.count(), 4);
  for (let i = 0; i < n; i++) {
    await cards.nth(i).scrollIntoViewIfNeeded(); await sleep(200);
    await cards.nth(i).screenshot({ path: `${OUT}/card_coaching_${tag}_${i + 1}.png` });
  }
  console.log(`coaching cards (${tag}): ${n}`);
}

// C) Home config variants — populate the form, then set Level / Type / Rigor.
const variants = [
  { level: "Senior Engineer", type: "System Design", rigor: "90", file: "home_systemdesign_high.png" },
  { level: "Engineering Manager", type: "Engineering Leadership", rigor: "50", file: "home_emleadership_med.png" },
  { level: "Staff Engineer", type: "HR Round", rigor: "15", file: "home_hr_low.png" },
  { level: "Senior Engineer", type: "AI Engineering", rigor: "70", file: "home_aieng.png" },
];
for (const v of variants) {
  await page.goto(`${BASE}/`, { waitUntil: "domcontentloaded" });
  await sleep(1200);
  await page.getByText("Load sample", { exact: false }).first().click();
  await sleep(400);
  await page.locator("select").nth(0).selectOption({ label: v.level });
  await page.locator("select").nth(1).selectOption({ label: v.type });
  const slider = page.locator('input[type=range]');
  await slider.evaluate((el, val) => {
    const setter = Object.getOwnPropertyDescriptor(window.HTMLInputElement.prototype, "value").set;
    setter.call(el, val);
    el.dispatchEvent(new Event("input", { bubbles: true }));
    el.dispatchEvent(new Event("change", { bubbles: true }));
  }, v.rigor);
  await sleep(400);
  await full(v.file);
}

// D) Live answering screen + record modal.
await page.goto(`${BASE}/interview/${OPEN_INTERVIEW}`, { waitUntil: "domcontentloaded" });
await sleep(3500);
await full("interview_answering.png");
await sectionShot("Live confidence", "sec_live_confidence_runtime.png");
// the question panel is the first .panel; crop it big
try {
  const qpanel = page.locator(".panel").first();
  await qpanel.screenshot({ path: `${OUT}/sec_question_panel.png` });
  console.log("section: sec_question_panel.png");
} catch (e) { console.log("MISS question panel", e.message.split("\n")[0]); }

// Record modal (fake camera feed)
try {
  await page.getByText("Record your answer", { exact: false }).first().click();
  await sleep(2800); // let getUserMedia (fake) connect
  await full("record_modal_full.png");
  const modal = page.locator(".modal-content").first();
  await modal.screenshot({ path: `${OUT}/record_modal.png` });
  console.log("section: record_modal.png");
} catch (e) { console.log("MISS record modal", e.message.split("\n")[0]); }

await browser.close();
console.log("done");
