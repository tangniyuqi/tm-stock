// 官方 H5 + 真实 Go/MySQL/回环模型的浏览器纵向验收。
// 用法：node scripts/dev/browser-h5-realstack.cjs <ready.json> <done 文件> [baseUrl]
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

if (!process.argv[2] || !process.argv[3]) throw new Error('需要 ready.json 与 done 文件路径');
const ready = JSON.parse(fs.readFileSync(process.argv[2], 'utf8'));
const donePath = process.argv[3];
const base = (process.argv[4] || 'http://127.0.0.1:5173').replace(/\/$/, '');
const outDir = path.join(os.tmpdir(), 'tm-stock-h5-realstack');
fs.mkdirSync(outDir, { recursive: true });

const chromePath = [process.env.TM_CHROME_PATH, 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe', 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'].filter(Boolean).find(p => fs.existsSync(p));
if (!chromePath) throw new Error('未找到 Chrome/Edge');
const sleep = ms => new Promise(r => setTimeout(r, ms));
async function waitFile(file, timeout = 20000) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    if (fs.existsSync(file)) return fs.readFileSync(file, 'utf8');
    await sleep(100);
  }
  throw new Error('等待文件超时：' + file);
}
class CDP {
  constructor(url) {
    this.ws = new WebSocket(url); this.id = 0; this.pending = new Map();
    this.ready = new Promise((resolve, reject) => { this.ws.addEventListener('open', resolve); this.ws.addEventListener('error', reject); });
    this.ws.addEventListener('message', event => {
      const message = JSON.parse(event.data);
      if (message.id && this.pending.has(message.id)) {
        const job = this.pending.get(message.id); this.pending.delete(message.id);
        if (message.error) job.reject(new Error(message.error.message)); else job.resolve(message.result);
      }
    });
  }
  async send(method, params = {}) {
    await this.ready; const id = ++this.id;
    return new Promise((resolve, reject) => { this.pending.set(id, { resolve, reject }); this.ws.send(JSON.stringify({ id, method, params })); });
  }
  close() { try { this.ws.close(); } catch (_) {} }
}

(async () => {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'tm-stock-realstack-chrome-'));
  const chrome = spawn(chromePath, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--disable-extensions', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { stdio: 'ignore' });
  const results = [];
  const check = (ok, name, detail) => { results.push({ ok, name, detail }); if (!ok) throw new Error(name + (detail ? '：' + detail : '')); };
  let cdp;
  try {
    const port = (await waitFile(path.join(profile, 'DevToolsActivePort'))).split(/\r?\n/)[0].trim();
    const target = await (await fetch('http://127.0.0.1:' + port + '/json/new?about:blank', { method: 'PUT' })).json();
    cdp = new CDP(target.webSocketDebuggerUrl);
    await cdp.send('Page.enable'); await cdp.send('Runtime.enable');
    const evaluate = async expression => {
      const result = await cdp.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
      if (result.exceptionDetails) throw new Error('页面脚本异常：' + JSON.stringify(result.exceptionDetails).slice(0, 400));
      return result.result.value;
    };
    const text = () => evaluate('document.body ? document.body.innerText : ""');
    const until = async (fn, name, timeout = 40000) => {
      const start = Date.now();
      for (;;) {
        if (await fn()) return;
        if (Date.now() - start > timeout) throw new Error('等待超时：' + name + '；正文=' + String(await text()).slice(0, 700));
        await sleep(250);
      }
    };
    const clickText = async label => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(label) + '); if (!leaf) return "missing"; const target = leaf.closest("button, uni-button, [role=button]") || leaf; target.click(); return "clicked"; })()');
    const fill = async (placeholder, value) => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const label = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(placeholder) + '); if (!label) return "missing-label"; const root = label.closest("uni-input, uni-textarea") || label.parentElement; const input = root ? root.querySelector("input,textarea") : null; if (!input) return "missing-input"; const proto = input.tagName === "TEXTAREA" ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype; const setter = Object.getOwnPropertyDescriptor(proto, "value").set; setter.call(input, ' + JSON.stringify(value) + '); input.dispatchEvent(new Event("input", { bubbles: true })); return "filled"; })()');

    await cdp.send('Page.navigate', { url: base + '/' });
    await sleep(1500);
    await evaluate('localStorage.setItem("token", ' + JSON.stringify(ready.token) + '); location.hash = "#/pages/ai/index"; location.reload();');
    await until(async () => String(await text()).includes('我的会话'), '官方首页');
    const openLabel = '打开会话 #' + ready.conversation;
    check((await clickText(openLabel)) === 'clicked', '打开真实会话', openLabel);
    await until(async () => String(await text()).includes('会话资料问答'), '问答面板');
    check((await fill('查找本次问答的题材', ready.subjectName)) === 'filled', '填写题材');
    check((await clickText('查询问答题材')) === 'clicked', '查询题材');
    await until(async () => String(await text()).includes('选择问答题材 #' + ready.subjectId), '题材结果');
    check((await clickText('选择问答题材 #' + ready.subjectId)) === 'clicked', '选择题材');
    await sleep(400);
    check((await fill('输入所选资料的事实问题，最多1000字', ready.question)) === 'filled', '填写问题');
    await sleep(300);
    await evaluate('(() => { if (window.__net) return true; window.__net = []; const original = window.fetch; window.fetch = async function () { const response = await original.apply(this, arguments); try { const copy = response.clone(); copy.text().then(body => window.__net.push({ url: String(arguments[0] && arguments[0].url || arguments[0]), status: response.status, body: body.slice(0, 600) })); } catch (_) {} return response; }; return true; })()');
    check((await clickText('提交本次资料问题（不扣积分）')) === 'clicked', '提交问题');
    await until(async () => String(await text()).includes('刷新问题状态'), '真实问答执行完成', 60000);
    check(String(await text()).includes('本机保留有原问题'), '待核查记录仍在');
    const stored = await evaluate('localStorage.getItem("tm_ai_question_v1_1")');
    check(typeof stored === 'string' && stored.includes('question_'), '本地原键存在');
    check((await clickText('核对当前答案')) === 'clicked', '核对答案');
    await until(async () => String(await text()).includes(ready.excerpt), '真实答案引文', 20000);
    const body = String(await text());
    check(body.includes('AI 生成'), '真实答案含AI标识');
    check(body.includes('qa-sql-test'), '真实答案含模型名');
    await evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.includes(' + JSON.stringify(ready.excerpt) + ')); if (leaf) leaf.scrollIntoView({ block: "center" }); return true; })()');
    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(400);
    const shot = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outDir, 'realstack-375-answer.png'), Buffer.from(shot.data, 'base64'));
    check((await clickText('核实终态并清理本地原键')) === 'clicked', '清理本地原键');
    await until(async () => (await evaluate('localStorage.getItem("tm_ai_question_v1_1")')) == null, '原键清理');
    fs.writeFileSync(donePath, 'done');
    fs.writeFileSync(path.join(outDir, 'result.json'), JSON.stringify({ ok: true, results, ready: { conversation: ready.conversation, subjectId: ready.subjectId } }, null, 2));
    for (const row of results) console.log((row.ok ? '通过：' : '失败：') + row.name + (row.detail ? ' — ' + row.detail : ''));
    console.log('截图：' + outDir);
    console.log('通过：官方H5→Vite代理→真实Go/MySQL→回环模型，问答恰好一次执行并交付核验引文。');
  } catch (error) {
    try { if (cdp) { const shot = await cdp.send('Page.captureScreenshot', { format: 'png' }); fs.writeFileSync(path.join(outDir, 'realstack-failure.png'), Buffer.from(shot.data, 'base64')); } } catch (_) {}
    console.error(error); process.exitCode = 1;
    let failureText = '';
    if (cdp) {
      try {
        const evaluated = await cdp.send('Runtime.evaluate', { expression: 'document.body.innerText + "\\n---NET---\\n" + JSON.stringify(window.__net || [])', returnByValue: true });
        failureText = String(evaluated.result.value || '');
      } catch (_) { failureText = ''; }
    }
    try { fs.writeFileSync(path.join(outDir, 'failure-body.txt'), failureText); } catch (_) {}
    try { fs.writeFileSync(donePath, 'failed'); } catch (_) {}
  } finally { chrome.kill(); if (cdp) cdp.close(); }
})();
