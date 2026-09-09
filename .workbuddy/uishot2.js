const { chromium } = require("playwright-core");
const path = require("path");

const EXE = "C:/Users/link.chen/.agent-browser/browsers/chrome-153.0.8010.36/chrome.exe";
const OUT = "F:/projects/RJDJ/.workbuddy/shots2";
const BASE = "http://localhost:5173";

(async () => {
  const fs = require("fs");
  fs.mkdirSync(OUT, { recursive: true });
  const browser = await chromium.launch({ executablePath: EXE, headless: true, args: ["--no-sandbox", "--disable-gpu"] });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
  const logs = [];
  try {
    await page.goto(BASE + "/login", { waitUntil: "networkidle" });
    await page.waitForTimeout(600);
    await page.fill('input[placeholder="请输入用户名"]', "admin");
    await page.fill('input[placeholder="请输入密码"]', "admin123");
    await page.click('button[type="submit"]');
    await page.waitForTimeout(1500);

    // Strategies page (no config state)
    await page.goto(BASE + "/strategies", { waitUntil: "networkidle" });
    await page.waitForTimeout(800);
    await page.screenshot({ path: path.join(OUT, "strategies-empty.png") });
    logs.push("strategies-empty.png: OK");

    // AI settings dialog from analyze page
    await page.goto(BASE + "/analyze", { waitUntil: "networkidle" });
    await page.waitForTimeout(800);
    await page.click('text=AI 模型配置');
    await page.waitForTimeout(600);
    await page.screenshot({ path: path.join(OUT, "ai-settings-dialog.png") });
    logs.push("ai-settings-dialog.png: OK");

    // Fill quick preset deepseek
    await page.click('text=DeepSeek（推荐国内）').catch(()=>{});
    await page.waitForTimeout(400);
    await page.screenshot({ path: path.join(OUT, "ai-settings-deepseek.png") });
    logs.push("ai-settings-deepseek.png: OK");

    console.log("SHOTS2_OK\n" + logs.join("\n"));
  } catch (e) {
    console.error("ERR:", e.message);
  } finally {
    await browser.close();
  }
})();
