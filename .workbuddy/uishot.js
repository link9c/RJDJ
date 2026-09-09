const { chromium } = require("playwright-core");
const path = require("path");

const EXE = "C:/Users/link.chen/.agent-browser/browsers/chrome-153.0.8010.36/chrome.exe";
const OUT = "F:/projects/RJDJ/.workbuddy/shots";
const BASE = "http://localhost:5173";

(async () => {
  const fs = require("fs");
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({
    executablePath: EXE,
    headless: true,
    args: ["--no-sandbox", "--disable-gpu"],
  });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const logs = [];

  try {
    // 1. Login page
    await page.goto(BASE + "/login", { waitUntil: "networkidle" });
    await page.waitForTimeout(800);
    await page.screenshot({ path: path.join(OUT, "01-login.png"), fullPage: false });
    logs.push("01-login.png: OK");

    // 2. Login flow
    await page.fill('input[placeholder="请输入用户名"]', "admin");
    await page.fill('input[placeholder="请输入密码"]', "admin123");
    await page.click('button[type="submit"]');
    await page.waitForNavigation({ waitUntil: "networkidle" }).catch(() => {});
    await page.waitForTimeout(1500);

    // Dashboard empty state (no config)
    const url = page.url();
    await page.screenshot({ path: path.join(OUT, "02-dashboard-empty.png"), fullPage: false });
    logs.push(`02-dashboard-empty.png (url=${url}): OK`);

    // 3. Config page
    await page.goto(BASE + "/config", { waitUntil: "networkidle" });
    await page.waitForTimeout(800);
    await page.screenshot({ path: path.join(OUT, "03-config-empty.png"), fullPage: false });
    logs.push("03-config-empty.png: OK");

    // 4. Open "新增配置" dialog
    await page.click('text=新增配置');
    await page.waitForTimeout(500);
    await page.screenshot({ path: path.join(OUT, "04-config-form.png"), fullPage: false });
    logs.push("04-config-form.png: OK");

    // 5. AI analyze page
    await page.goto(BASE + "/analyze", { waitUntil: "networkidle" });
    await page.waitForTimeout(800);
    await page.screenshot({ path: path.join(OUT, "05-analyze.png"), fullPage: false });
    logs.push("05-analyze.png: OK");

    // Fill a config to see dashboard populated? Needs valid OKX creds — skip live data, just note.
    console.log("ALL_SHOTS_OK\n" + logs.join("\n"));
  } catch (e) {
    console.error("UI_TEST_ERROR:", e.message);
    await page.screenshot({ path: path.join(OUT, "error.png"), fullPage: false }).catch(() => {});
  } finally {
    await browser.close();
  }
})();
