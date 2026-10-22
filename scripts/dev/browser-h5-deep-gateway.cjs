// 官方 H5 + 网关深度任务浏览器验收（配合 TM_TEST_AI_H5_ORIGIN 交互网关）。
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');
const entry = process.argv[2];
if (!entry) throw new Error('需要网关 __test__ 地址');
const outDir = path.join(os.tmpdir(), 'tm-stock-h5-deep-gateway');
fs.mkdirSync(outDir, { recursive: true });
const chromePath = [process.env.TM_CHROME_PATH, 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe', 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'].filter(Boolean).find(p => fs.existsSync(p));
if (!chromePath) throw new Error('未找到 Chrome/Edge');
const sleep = ms => new Promise(r => setTimeout(r, ms));
async function waitFile(file, timeout = 20000) { const start = Date.now(); while (Date.now() - start < timeout) { if (fs.existsSync(file)) return fs.readFileSync(file, 'utf8'); await sleep(100); } throw new Error('等待文件超时'); }
class CDP {
  constructor(url) { this.ws = new WebSocket(url); this.id = 0; this.pending = new Map(); this.ready = new Promise((res, rej) => { this.ws.addEventListener('open', res); this.ws.addEventListener('error', rej); }); this.ws.addEventListener('message', e => { const m = JSON.parse(e.data); if (m.id && this.pending.has(m.id)) { const j = this.pending.get(m.id); this.pending.delete(m.id); if (m.error) j.reject(new Error(m.error.message)); else j.resolve(m.result); } }); }
  async send(method, params = {}) { await this.ready; const id = ++this.id; return new Promise((res, rej) => { this.pending.set(id, { resolve: res, reject: rej }); this.ws.send(JSON.stringify({ id, method, params })); }); }
  close() { try { this.ws.close(); } catch (_) {} }
}
function ui(cdp) {
  const evaluate = async expression => { const r = await cdp.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }); if (r.exceptionDetails) throw new Error('页面脚本异常：' + JSON.stringify(r.exceptionDetails).slice(0, 500)); return r.result.value; };
  const text = () => evaluate('document.body ? document.body.innerText : ""');
  const until = async (fn, name, timeout = 60000) => { const start = Date.now(); for (;;) { if (await fn()) return; if (Date.now() - start > timeout) throw new Error('等待超时：' + name + '；正文=' + String(await text()).slice(0, 600)); await sleep(250); } };
  const clickText = async label => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(label) + '); if (!leaf) return "missing"; const target = leaf.closest("button, uni-button, [role=button]") || leaf; target.click(); return "clicked"; })()');
  const fill = async (placeholder, value) => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const label = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(placeholder) + '); if (!label) return "missing-label"; const root = label.closest("uni-input, uni-textarea") || label.parentElement; const input = root ? root.querySelector("input,textarea") : null; if (!input) return "missing-input"; const proto = input.tagName === "TEXTAREA" ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype; const setter = Object.getOwnPropertyDescriptor(proto, "value").set; setter.call(input, ' + JSON.stringify(value) + '); input.dispatchEvent(new Event("input", { bubbles: true })); return "filled"; })()');
  const clickSkill = async title => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const heading = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(title) + '); if (!heading) return "missing"; let card = heading.parentElement; for (let i = 0; i < 6 && card; i++) { const leaf = Array.from(card.querySelectorAll("*")).find(el => el.children.length === 0 && el.textContent.trim() === "使用该技能"); if (leaf) { (leaf.closest("button, uni-button, [role=button]") || leaf).click(); return "clicked"; } card = card.parentElement; } return "no-button"; })()');
  return { evaluate, text, until, clickText, fill, clickSkill };
}
(async () => {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'tm-stock-deep-chrome-'));
  const chrome = spawn(chromePath, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--disable-extensions', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { stdio: 'ignore' });
  const results = [];
  const check = (ok, name, detail) => { results.push({ ok, name, detail }); if (!ok) throw new Error(name + (detail ? '：' + detail : '')); };
  let cdp;
  try {
    const port = (await waitFile(path.join(profile, 'DevToolsActivePort'))).split(/\r?\n/)[0].trim();
    const target = await (await fetch('http://127.0.0.1:' + port + '/json/new?about:blank', { method: 'PUT' })).json();
    cdp = new CDP(target.webSocketDebuggerUrl);
    await cdp.send('Page.enable'); await cdp.send('Runtime.enable');
    const page = ui(cdp);
    await cdp.send('Page.navigate', { url: entry });
    await page.until(async () => String(await page.text()).includes('进入测试会员一'), '网关测试页');
    check((await page.clickText('进入测试会员一')) === 'clicked', '进入测试会员一');
    await page.until(async () => String(await page.text()).includes('AI 资料助手'), '官方 H5 首页', 40000);
    check((await page.fill('新会话标题，1至40字', '深度网关会话')) === 'filled', '填写会话标题');
    check((await page.clickText('新建会话')) === 'clicked', '新建会话');
    await page.until(async () => String(await page.text()).includes('当前会话：深度网关会话'), '会话创建完成', 30000);
    check((await page.clickSkill('深度资料整理')) === 'clicked', '切换深度技能');
    await sleep(600);
    check((await page.fill('输入题材名称，仅用于查询目录', '示例')) === 'filled', '填写题材');
    check((await page.clickText('查询题材')) === 'clicked', '查询题材');
    await page.until(async () => String(await page.text()).includes('核对该题材报价'), '题材报价', 20000);
    check((await page.clickText('核对该题材报价')) === 'clicked', '核对深度报价');
    await page.until(async () => String(await page.text()).includes('保存会话报价意图（不冻结积分）'), '深度报价完成', 30000);
    check((await page.clickText('保存会话报价意图（不冻结积分）')) === 'clicked', '保存深度意图');
    await page.until(async () => String(await page.text()).includes('重新核对报价与资料范围'), '深度意图已准备', 30000);
    check((await page.clickText('重新核对报价与资料范围')) === 'clicked', '重新核对深度报价');
    await page.until(async () => String(await page.text()).includes('确认会话报价并创建'), '深度确认入口', 20000);
    check((await page.clickText('确认会话报价并创建')) === 'clicked', '确认深度报价');
    await page.until(async () => String(await page.text()).includes('确认冻结'), '深度二次确认', 10000);
    check((await page.clickText('确认冻结')) === 'clicked', '确认冻结');
    await page.until(async () => /查看会话关联任务 #/.test(String(await page.text())), '深度任务已关联', 60000);
    const linked = String(await page.text()).match(/查看会话关联任务 #(\d+)/);
    check(linked != null, '读取深度任务号');
    check((await page.clickText('查看会话关联任务 #' + linked[1])) === 'clicked', '查看深度任务');
    await page.until(async () => String(await page.text()).includes('核对并查看报告'), '深度任务完成', 120000);
    check((await page.clickText('核对并查看报告')) === 'clicked', '核对深度报告');
    await page.until(async () => String(await page.text()).includes('题材定义'), '深度报告正文', 30000);
    const body = String(await page.text());
    check(body.includes('事件时间线') && body.includes('环节说明') && body.includes('披露公司'), '深度四节报告');
    check((body.match(/示例题材/g) || []).length >= 6, '深度报告至少六条引文');
    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(500);
    try {
      const shot = await cdp.send('Page.captureScreenshot', { format: 'png' });
      fs.writeFileSync(path.join(outDir, 'deep-gateway-375.png'), Buffer.from(shot.data, 'base64'));
    } catch (e) { console.error('截图失败（不判定业务失败）:', e.message); }
    for (const row of results) console.log((row.ok ? '通过：' : '失败：') + row.name + (row.detail ? ' — ' + row.detail : ''));
    console.log('截图：' + outDir);
    console.log('通过：官方 H5 + 真实网关，深度报价→确认→执行→四节报告整链。');
    cdp.close();
    process.exit(0);
  } finally { chrome.kill(); }
})().catch(e => { console.error(e); process.exitCode = 1; });