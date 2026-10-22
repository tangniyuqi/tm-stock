// 官方 H5 + 会话意图 + 报告 + 反馈的整链浏览器验收（配合 H5 网关 feedback 模式）。
// 用法：node scripts/dev/browser-h5-intent-feedback.cjs http://127.0.0.1:<port>/__test__/
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const entry = process.argv[2];
if (!entry) throw new Error('需要网关 __test__ 地址');
const outDir = path.join(os.tmpdir(), 'tm-stock-h5-intent-feedback');
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
function ui(cdp) {
  const evaluate = async expression => { const r = await cdp.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true }); if (r.exceptionDetails) throw new Error('页面脚本异常：' + JSON.stringify(r.exceptionDetails).slice(0, 400)); return r.result.value; };
  const text = () => evaluate('document.body ? document.body.innerText : ""');
  const until = async (fn, name, timeout = 60000) => { const start = Date.now(); for (;;) { if (await fn()) return; if (Date.now() - start > timeout) throw new Error('等待超时：' + name + '；正文=' + String(await text()).slice(0, 900)); await sleep(250); } };
  const clickText = async label => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(label) + '); if (!leaf) return "missing"; const target = leaf.closest("button, uni-button, [role=button]") || leaf; target.click(); return "clicked"; })()');
  const fill = async (placeholder, value) => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const label = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(placeholder) + '); if (!label) return "missing-label"; const root = label.closest("uni-input, uni-textarea") || label.parentElement; const input = root ? root.querySelector("input,textarea") : null; if (!input) return "missing-input"; const proto = input.tagName === "TEXTAREA" ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype; const setter = Object.getOwnPropertyDescriptor(proto, "value").set; setter.call(input, ' + JSON.stringify(value) + '); input.dispatchEvent(new Event("input", { bubbles: true })); return "filled"; })()');
  return { evaluate, text, until, clickText, fill };
}

(async () => {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'tm-stock-intent-chrome-'));
  const chrome = spawn(chromePath, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--disable-extensions', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { stdio: 'ignore' });
  const results = [];
  const check = (ok, name, detail) => { results.push({ ok, name, detail }); if (!ok) throw new Error(name + (detail ? '：' + detail : '')); };
  let cdp;
  try {
    const port = (await waitFile(path.join(profile, 'DevToolsActivePort'))).split(/\r?\n/)[0].trim();
    const base = 'http://127.0.0.1:' + port;
    const target = await (await fetch(base + '/json/new?about:blank', { method: 'PUT' })).json();
    cdp = new CDP(target.webSocketDebuggerUrl);
    await cdp.send('Page.enable'); await cdp.send('Runtime.enable');
    const page = ui(cdp);

    await cdp.send('Page.navigate', { url: entry });
    await page.until(async () => String(await page.text()).includes('进入测试会员一'), '网关测试页');
    check((await page.clickText('进入测试会员一')) === 'clicked', '进入测试会员一');
    await page.until(async () => String(await page.text()).includes('AI 资料助手'), '官方 H5 首页', 40000);

    check((await page.fill('新会话标题，1至40字', '浏览器整链会话')) === 'filled', '填写会话标题');
    check((await page.clickText('新建会话')) === 'clicked', '新建会话');
    await page.until(async () => String(await page.text()).includes('当前会话：浏览器整链会话'), '会话创建完成', 30000);
    check((await page.clickText('使用该技能')) === 'clicked', '切换到轻度资料整理');
    await sleep(600);
    check((await page.fill('输入题材名称，仅用于查询目录', '示例')) === 'filled', '填写题材');
    check((await page.clickText('查询题材')) === 'clicked', '查询题材');
    await page.until(async () => String(await page.text()).includes('核对该题材报价'), '题材报价', 20000);
    check((await page.clickText('核对该题材报价')) === 'clicked', '核对报价');
    await page.until(async () => String(await page.text()).includes('保存会话报价意图（不冻结积分）'), '会话报价意图', 20000);
    check((await page.clickText('保存会话报价意图（不冻结积分）')) === 'clicked', '保存会话报价意图');
    await page.until(async () => String(await page.text()).includes('重新核对报价与资料范围'), '意图已准备', 30000);
    check((await page.clickText('重新核对报价与资料范围')) === 'clicked', '重新核对报价');
    await page.until(async () => String(await page.text()).includes('确认会话报价并创建'), '确认创建入口', 20000);
    check((await page.clickText('确认会话报价并创建')) === 'clicked', '确认会话报价');
    await page.until(async () => String(await page.text()).includes('确认冻结'), '二次确认冻结', 10000);
    check((await page.clickText('确认冻结')) === 'clicked', '确认冻结');
    await page.until(async () => /查看会话关联任务 #/.test(String(await page.text())), '意图已关联任务', 60000);
    const linked = String(await page.text()).match(/查看会话关联任务 #(\d+)/);
    check(linked != null, '读取关联任务编号');
    check((await page.clickText('查看会话关联任务 #' + linked[1])) === 'clicked', '查看关联任务');
    await page.until(async () => String(await page.text()).includes('核对并查看报告'), '任务完成且报告可用', 90000);
    check((await page.clickText('核对并查看报告')) === 'clicked', '核对并查看报告');
    await page.until(async () => /题材定义|事件时间线|环节说明/.test(String(await page.text())), '报告正文', 30000);
    check(String(await page.text()).includes('AI 生成'), '报告含 AI 标识');

    await page.until(async () => String(await page.text()).includes('报告追问 · 任务 #'), '报告追问面板', 20000);
    const reportQuestion = '报告的题材定义引用了什么分类？';
    check((await page.fill('输入针对本报告的事实问题，最多1000字', reportQuestion)) === 'filled', '填写报告追问');
    await sleep(300);
    check((await page.clickText('提交报告追问（不扣积分）')) === 'clicked', '提交报告追问');
    await page.until(async () => String(await page.text()).includes('核对当前答案'), '报告追问已作答', 40000);
    check((await page.clickText('核对当前答案')) === 'clicked', '读取报告追问答案');
    await page.until(async () => String(await page.text()).includes('qa-report-test'), '报告追问答案模型', 20000);
    const reportAnswer = String(await page.text());
    check(reportAnswer.includes('AI 资料问答：'), '报告追问固定模板');
    check(reportAnswer.includes('年报'), '报告追问出处类型');
    await page.evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.includes("AI 资料问答：")); if (leaf) leaf.scrollIntoView({ block: "center" }); return true; })()');
    await sleep(400);
    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(400);
    const qaShot = await cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
    fs.writeFileSync(path.join(outDir, 'reportqa-375.png'), Buffer.from(qaShot.data, 'base64'));
    check((await page.clickText('核实终态并清理本地原键')) === 'clicked', '清理报告追问原键');

    check((await page.clickText('举报与反馈')) === 'clicked', '展开报告反馈');
    await page.until(async () => String(await page.text()).includes('本报告暂无反馈记录'), '反馈状态已读取', 30000);
    check((await page.clickText('事实或归属依据')) === 'clicked', '选择反馈分类');
    check((await page.clickText('确认反馈分类')) === 'clicked', '确认反馈分类');
    await page.until(async () => String(await page.text()).includes('提交反馈'), '反馈二次确认', 10000);
    check((await page.clickText('提交反馈')) === 'clicked', '提交反馈');
    await page.until(async () => /受理单 #\d+ · 已受理/.test(String(await page.text())), '反馈已受理', 30000);

    // 不能切走标签页：uni-app 的 onHide 会清理当前任务/报告状态，改为当前页同源控制请求。
    const resolved = await page.evaluate('fetch("/__test__/resolve-feedback", { method: "POST" }).then(r => r.status)');
    check(resolved === 204, '内部处理反馈', String(resolved));
    check((await page.clickText('查询反馈状态')) === 'clicked', '回读反馈状态');
    await page.until(async () => /受理单 #\d+ · 已处理/.test(String(await page.text())), '反馈已处理', 30000);
    check(String(await page.text()).includes('已完成核查记录'), '显示处理记录');
    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(400);
    const shot = await cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
    fs.writeFileSync(path.join(outDir, 'intent-feedback-375.png'), Buffer.from(shot.data, 'base64'));

    const finished = await page.evaluate('fetch("/__test__/finish", { method: "POST" }).then(r => r.status)');
    check(finished === 204, '结束并核对', String(finished));
    fs.writeFileSync(path.join(outDir, 'result.json'), JSON.stringify({ ok: true, results }, null, 2));
    for (const row of results) console.log((row.ok ? '通过：' : '失败：') + row.name + (row.detail ? ' — ' + row.detail : ''));
    console.log('截图：' + outDir);
    console.log('通过：官方H5会话意图→确认→worker→报告→反馈受理/处理→结束核对整链完成。');
  } catch (error) {
    console.error(error); process.exitCode = 1;
    try { if (cdp) { const r = await cdp.send('Runtime.evaluate', { expression: 'document.body.innerText', returnByValue: true }); fs.writeFileSync(path.join(outDir, 'failure-body.txt'), String(r.result.value || '')); const shot = await cdp.send('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(outDir, 'intent-feedback-failure.png'), Buffer.from(shot.data, 'base64')); } } catch (_) {}
  } finally { chrome.kill(); if (cdp) cdp.close(); }
})();
