// Refine the record-modal capture: wait for the fake camera feed to attach,
// then also capture the active "recording" state.
import { createRequire } from "module";
const require = createRequire(import.meta.url);
const { chromium } = require("/usr/local/lib/node_modules/playwright");

const BASE = "http://localhost:3000";
const OUT = "screenshots";
const OPEN_INTERVIEW = "6a254bf46c11519439e99998";
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const browser = await chromium.launch({
  args: [
    "--use-fake-device-for-media-stream",
    "--use-fake-ui-for-media-stream",
    "--autoplay-policy=no-user-gesture-required",
  ],
});
const ctx = await browser.newContext({
  viewport: { width: 1440, height: 900 },
  deviceScaleFactor: 2,
  permissions: ["camera", "microphone"],
});
const page = await ctx.newPage();

await page.goto(`${BASE}/interview/${OPEN_INTERVIEW}`, { waitUntil: "domcontentloaded" });
await sleep(3000);
await page.getByRole("button", { name: /Record your answer/i }).click({ timeout: 8000 });

// Wait until the <video> actually has frames (fake feed attached), up to ~12s.
try {
  await page.waitForFunction(() => {
    const v = document.querySelector("video");
    return v && v.readyState >= 2 && v.videoWidth > 0;
  }, { timeout: 12000 });
  console.log("camera feed attached");
} catch { console.log("camera feed not detected; capturing anyway"); }
await sleep(800);

await page.locator(".modal-content").first().screenshot({ path: `${OUT}/record_modal.png` });
console.log("section: record_modal.png (camera preview)");

// Active recording state.
try {
  await page.getByRole("button", { name: /Start Recording/i }).click({ timeout: 5000 });
  await sleep(2500);
  await page.locator(".modal-content").first().screenshot({ path: `${OUT}/record_modal_recording.png` });
  console.log("section: record_modal_recording.png (REC state)");
} catch (e) { console.log("MISS recording state:", e.message.split("\n")[0]); }

await browser.close();
console.log("done");
