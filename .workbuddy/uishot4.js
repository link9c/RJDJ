const { chromium } = require("playwright-core");
const path = require("path");
const chromePath = "C:/Users/link.chen/.agent-browser/browsers/chrome-153.0.8010.36/chrome.exe";
const shotDir = "F:/projects/RJDJ/.workbuddy/shots4";
const fs = require("fs");
if (!fs.existsSync(shotDir)) fs.mkdirSync(shotDir, { recursive: true });

(async () => {
  const browser = await chromium.launch({ executablePath: chromePath, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 1600 } });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log("PAGEERROR:", e.message));

  await page.goto("http://localhost:5173/login", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(500);
  await page.locator('input[placeholder*="用户名"]').first().fill("admin");
  await page.locator('input[placeholder*="密码"]').first().fill("admin123");
  await page.locator('input[placeholder*="密码"]').first().press("Enter");
  await page.waitForFunction(() => !location.pathname.startsWith("/login"), { timeout: 10000 });
  await page.waitForTimeout(800);
  console.log("logged in:", page.url());

  // Dashboard (no config => empty state, confirms no crash)
  await page.goto("http://localhost:5173/", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1200);
  await page.screenshot({ path: path.join(shotDir, "dashboard-empty.png") });
  console.log("shot dashboard-empty");

  await browser.close();
  console.log("DONE");
})().catch((e) => { console.error("ERR", e.message); process.exit(1); });