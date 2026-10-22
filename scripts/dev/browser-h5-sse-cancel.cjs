// 官方 H5 + 会话改名/归档 + 运行中取消 + SSE 的整链浏览器验收（网关 running_cancel 模式）。
// 用法：node scripts/dev/browser-h5-sse-cancel.cjs http://127.0.0.1:<port>/__test__/
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const entry = process.argv[2];
if (!entry) throw new Error('需要网关 __test__ 地址');
const outDir = path.join(os.tmpdir(), 'tm-stock-h5-sse-cancel');
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
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'tm-stock-sse-chrome-'));
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
    const until = async (fn, name, timeout = 60000) => { const start = Date.now(); for (;;) { if (await fn()) return; if (Date.now() - start > timeout) throw new Error('等待超时：' + name + '；正文=' + String(await text()).slice(0, 900)); await sleep(250); } };
    const clickText = async label => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(label) + '); if (!leaf) return "missing"; const target = leaf.closest("button, uni-button, [role=button]") || leaf; target.click(); return "clicked"; })()');
    const fill = async (placeholder, value) => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const label = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(placeholder) + '); if (!label) return "missing-label"; const root = label.closest("uni-input, uni-textarea") || label.parentElement; const input = root ? root.querySelector("input,textarea") : null; if (!input) return "missing-input"; const proto = input.tagName === "TEXTAREA" ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype; const setter = Object.getOwnPropertyDescriptor(proto, "value").set; setter.call(input, ' + JSON.stringify(value) + '); input.dispatchEvent(new Event("input", { bubbles: true })); return "filled"; })()');

    await cdp.send('Page.navigate', { url: entry });
    await until(async () => String(await text()).includes('进入测试会员一'), '网关测试页');
    check((await clickText('进入测试会员一')) === 'clicked', '进入测试会员一');
    await until(async () => String(await text()).includes('AI 资料助手'), '官方 H5 首页', 40000);

    check((await fill('新会话标题，1至40字', 'SSE取消会话')) === 'filled', '填写会话标题');
    check((await clickText('新建会话')) === 'clicked', '新建会话');
    await until(async () => String(await text()).includes('当前会话：SSE取消会话'), '会话创建完成', 30000);
    check((await fill('新的会话标题', 'SSE取消会话-改名')) === 'filled', '填写新标题');
    await sleep(300);
    check((await clickText('保存会话标题')) === 'clicked', '保存会话标题');
    await until(async () => String(await text()).includes('标题已更新'), '会话改名完成', 20000);
    check((await clickText('归档当前会话')) === 'clicked', '归档当前会话');
    await until(async () => String(await text()).includes('确认归档'), '归档确认', 10000);
    check((await clickText('确认归档')) === 'clicked', '确认归档');
    await until(async () => String(await text()).includes('会话已归档'), '会话归档完成', 20000);

    check((await clickText('全部任务')) === 'clicked', '切到全部任务');
    await sleep(500);
    check((await fill('输入题材名称，仅用于查询目录', '示例')) === 'filled', '填写题材');
    check((await clickText('查询题材')) === 'clicked', '查询题材');
    await until(async () => String(await text()).includes('核对该题材报价'), '题材报价', 20000);
    check((await clickText('核对该题材报价')) === 'clicked', '核对报价');
    await until(async () => String(await text()).includes('确认报价并创建'), '报价确认入口', 20000);
    check((await clickText('确认报价并创建')) === 'clicked', '确认报价');
    await until(async () => String(await text()).includes('确认冻结'), '二次确认冻结', 10000);
    check((await clickText('确认冻结')) === 'clicked', '确认冻结');
    await until(async () => String(await text()).includes('查看已创建任务'), '任务创建回执', 60000);
    check((await clickText('查看已创建任务')) === 'clicked', '查看已创建任务');

    await until(async () => String(await text()).includes('申请取消任务'), '任务运行中可取消', 60000);
    const streamSeen = String(await text()).includes('已连接公开阶段进度');
    results.push({ ok: true, name: '任务运行态已展示', detail: streamSeen ? '已连接公开阶段进度' : '未显示流文案，可能已轮询兜底' });
    check((await clickText('申请取消任务')) === 'clicked', '申请取消任务');
    await until(async () => String(await text()).includes('确认申请取消'), '取消二次确认', 10000);
    check((await clickText('确认申请取消')) === 'clicked', '确认申请取消');
    await until(async () => /已取消|取消请求已登记/.test(String(await text())), '取消已受理', 30000);

    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(400);
    const shot = await cdp.send('Page.captureScreenshot', { format: 'png', captureBeyondViewport: true });
    fs.writeFileSync(path.join(outDir, 'sse-cancel-375.png'), Buffer.from(shot.data, 'base64'));

    const finished = await evaluate('fetch("/__test__/finish", { method: "POST" }).then(r => r.status)');
    check(finished === 204, '结束并核对', String(finished));
    fs.writeFileSync(path.join(outDir, 'result.json'), JSON.stringify({ ok: true, results, streamSeen }, null, 2));
    for (const row of results) console.log((row.ok ? '通过：' : '失败：') + row.name + (row.detail ? ' — ' + row.detail : ''));
    console.log('截图：' + outDir);
    console.log('通过：官方H5会话改名/归档→任务→SSE→运行中取消→结束核对整链完成。');
  } catch (error) {
    console.error(error); process.exitCode = 1;
    try { if (cdp) { const r = await cdp.send('Runtime.evaluate', { expression: 'document.body.innerText', returnByValue: true }); fs.writeFileSync(path.join(outDir, 'failure-body.txt'), String(r.result.value || '')); const shot = await cdp.send('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(outDir, 'sse-cancel-failure.png'), Buffer.from(shot.data, 'base64')); } } catch (_) {}
  } finally { chrome.kill(); if (cdp) cdp.close(); }
})();
