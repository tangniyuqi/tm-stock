// 驱动既有 H5 网关夹具：官方 H5 + 真实 Go/MySQL/worker/报告/结算。
// 用法：node scripts/dev/browser-h5-gateway.cjs http://127.0.0.1:<port>/__test__/
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const entry = process.argv[2];
if (!entry) throw new Error('需要网关 __test__ 地址');
const origin = new URL(entry).origin;
const outDir = path.join(os.tmpdir(), 'tm-stock-h5-gateway');
fs.mkdirSync(outDir, { recursive: true });
const chromePath = [process.env.TM_CHROME_PATH, 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe', 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'].filter(Boolean).find(p => fs.existsSync(p));
if (!chromePath) throw new Error('未找到 Chrome/Edge');
const sleep = ms => new Promise(r => setTimeout(r, ms));
async function waitFile(file, timeout = 20000) { const start = Date.now(); while (Date.now() - start < timeout) { if (fs.existsSync(file)) return fs.readFileSync(file, 'utf8'); await sleep(100); } throw new Error('等待文件超时：' + file); }
class CDP {
  constructor(url) {
    this.ws = new WebSocket(url); this.id = 0; this.pending = new Map();
    this.ready = new Promise((resolve, reject) => { this.ws.addEventListener('open', resolve); this.ws.addEventListener('error', reject); });
    this.ws.addEventListener('message', event => { const m = JSON.parse(event.data); if (m.id && this.pending.has(m.id)) { const j = this.pending.get(m.id); this.pending.delete(m.id); if (m.error) j.reject(new Error(m.error.message)); else j.resolve(m.result); } });
  }
  async send(method, params = {}) { await this.ready; const id = ++this.id; return new Promise((resolve, reject) => { this.pending.set(id, { resolve, reject }); this.ws.send(JSON.stringify({ id, method, params })); }); }
  close() { try { this.ws.close(); } catch (_) {} }
}
(async () => {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'tm-stock-gateway-chrome-'));
  const chrome = spawn(chromePath, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--disable-extensions', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { stdio: 'ignore' });
  const results = [];
  const check = (ok, name, detail) => { results.push({ ok, name, detail }); if (!ok) throw new Error(name + (detail ? '：' + detail : '')); };
  let cdp;
  try {
    const port = (await waitFile(path.join(profile, 'DevToolsActivePort'))).split(/\r?\n/)[0].trim();
    const target = await (await fetch('http://127.0.0.1:' + port + '/json/new?about:blank', { method: 'PUT' })).json();
    cdp = new CDP(target.webSocketDebuggerUrl);
    await cdp.send('Page.enable'); await cdp.send('Runtime.enable');
    const evaluate = async expression => { const r = await cdp.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }); if (r.exceptionDetails) throw new Error('页面脚本异常：' + JSON.stringify(r.exceptionDetails).slice(0, 400)); return r.result.value; };
    const text = () => evaluate('document.body ? document.body.innerText : ""');
    const until = async (fn, name, timeout = 60000) => { const start = Date.now(); for (;;) { if (await fn()) return; if (Date.now() - start > timeout) throw new Error('等待超时：' + name + '；正文=' + String(await text()).slice(0, 800)); await sleep(250); } };
    const clickText = async label => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(label) + '); if (!leaf) return "missing"; const target = leaf.closest("button, uni-button, [role=button]") || leaf; target.click(); return "clicked"; })()');
    const fill = async (placeholder, value) => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const label = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(placeholder) + '); if (!label) return "missing-label"; const root = label.closest("uni-input, uni-textarea") || label.parentElement; const input = root ? root.querySelector("input,textarea") : null; if (!input) return "missing-input"; const proto = input.tagName === "TEXTAREA" ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype; const setter = Object.getOwnPropertyDescriptor(proto, "value").set; setter.call(input, ' + JSON.stringify(value) + '); input.dispatchEvent(new Event("input", { bubbles: true })); return "filled"; })()');

    await cdp.send('Page.navigate', { url: entry });
    await until(async () => String(await text()).includes('进入测试会员一'), '网关测试页');
    check((await clickText('进入测试会员一')) === 'clicked', '进入测试会员一');
    await until(async () => String(await text()).includes('AI 资料助手'), '官方 H5 首页', 40000);
    check((await clickText('全部任务')) === 'clicked', '切到全部任务');
    await sleep(500);
    check((await fill('输入题材名称，仅用于查询目录', '示例')) === 'filled', '填写题材');
    check((await clickText('查询题材')) === 'clicked', '查询题材');
    await until(async () => String(await text()).includes('核对该题材报价'), '题材报价入口', 20000);
    check((await clickText('核对该题材报价')) === 'clicked', '核对报价');
    await until(async () => String(await text()).includes('确认报价并创建'), '报价确认入口', 20000);
    check((await clickText('确认报价并创建')) === 'clicked', '确认报价');
    await until(async () => String(await text()).includes('确认冻结'), '创建二次确认', 10000);
    check((await clickText('确认冻结')) === 'clicked', '确认冻结');
    await until(async () => String(await text()).includes('查看已创建任务'), '任务创建回执', 60000);
    check(String(await text()).includes('已确认创建任务'), '显示创建收据');
    check((await clickText('查看已创建任务')) === 'clicked', '查看已创建任务');
    await until(async () => String(await text()).includes('核对并查看报告'), '报告可用', 30000);
    check((await clickText('核对并查看报告')) === 'clicked', '核对并查看报告');
    await until(async () => /题材定义|事件时间线|环节说明/.test(String(await text())), '报告正文', 30000);
    const reportText = String(await text());
    check(reportText.includes('AI 生成'), '报告含AI标识');
    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(400);
    const shot = await cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
    fs.writeFileSync(path.join(outDir, 'gateway-375-report.png'), Buffer.from(shot.data, 'base64'));
    await cdp.send('Page.navigate', { url: entry });
    await until(async () => String(await text()).includes('结束并核对测试'), '网关控制页');
    check((await clickText('结束并核对测试')) === 'clicked', '结束并核对');
    await until(async () => String(await text()).includes('测试操作已确认'), '网关核对完成', 30000);
    fs.writeFileSync(path.join(outDir, 'result.json'), JSON.stringify({ ok: true, results }, null, 2));
    for (const row of results) console.log((row.ok ? '通过：' : '失败：') + row.name + (row.detail ? ' — ' + row.detail : ''));
    console.log('截图：' + outDir);
    console.log('通过：官方H5→真实网关→报价/冻结/创建/worker/报告/结算整链已在真实Chrome中完成。');
  } catch (error) {
    console.error(error); process.exitCode = 1;
    try { if (cdp) { const shot = await cdp.send('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(outDir, 'gateway-failure.png'), Buffer.from(shot.data, 'base64')); fs.writeFileSync(path.join(outDir, 'failure-body.txt'), String(await evaluateText(cdp))); } } catch (_) {}
  } finally { chrome.kill(); if (cdp) cdp.close(); }
})();
async function evaluateText(cdp) { try { return (await cdp.send('Runtime.evaluate', { expression: 'document.body.innerText', returnByValue: true })).result.value || ''; } catch (_) { return ''; } }
