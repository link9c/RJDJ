const { chromium } = require("playwright-core");
const path = require("path");
const chromePath = "C:/Users/link.chen/.agent-browser/browsers/chrome-153.0.8010.36/chrome.exe";
const shotDir = "F:/projects/RJDJ/.workbuddy/shots3";
const fs = require("fs");
if (!fs.existsSync(shotDir)) fs.mkdirSync(shotDir, { recursive: true });

(async () => {
  const browser = await chromium.launch({ executablePath: chromePath, headless: true });
  const ctx = await browser.newContext({ viewport: { width: 1440, height: 900 } });
  const page = await ctx.newPage();
  page.on("pageerror", (e) => console.log("PAGEERROR:", e.message));

  await page.goto("http://localhost:5173/login", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(600);

  await page.locator('input[placeholder*="用户名"]').first().fill("admin");
  await page.locator('input[placeholder*="密码"]').first().fill("admin123");

  // submit via Enter on password field (form submit)
  await page.locator('input[placeholder*="密码"]').first().press("Enter");

  await page.waitForFunction(() => !location.pathname.startsWith("/login"), { timeout: 10000 });
  await page.waitForTimeout(1200);
  console.log("after login, path:", page.url());

  await page.goto("http://localhost:5173/strategies", { waitUntil: "domcontentloaded" });
  await page.waitForTimeout(1500);
  await page.screenshot({ path: path.join(shotDir, "strategies-default-dca.png") });
  console.log("shot strategies-default-dca");

  const allTab = page.locator("button", { hasText: "全部策略" }).first();
  if (await allTab.count()) { await allTab.click(); await page.waitForTimeout(600); }
  await page.screenshot({ path: path.join(shotDir, "strategies-all-tabs.png") });
  console.log("shot strategies-all-tabs");

  const dcaTab = page.locator("button", { hasText: "马丁格尔" }).first();
  if (await dcaTab.count()) { await dcaTab.click(); await page.waitForTimeout(600); }
  await page.screenshot({ path: path.join(shotDir, "strategies-dca-tab.png") });
  console.log("shot strategies-dca-tab");

  await browser.close();
  console.log("DONE");
})().catch((e) => { console.error("ERR", e.message); process.exit(1); });