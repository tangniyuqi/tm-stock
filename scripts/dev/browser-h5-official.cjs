// 用真实 Chrome DevTools 协议驱动官方 HBuilderX H5 运行页面；接口由本脚本拦截并回固定响应。
// 用法：node scripts/dev/browser-h5-official.cjs [baseUrl]
const fs = require('node:fs');
const os = require('node:os');
const path = require('node:path');
const { spawn } = require('node:child_process');

const base = (process.argv[2] || 'http://127.0.0.1:5173').replace(/\/$/, '');
const outDir = path.join(os.tmpdir(), 'tm-stock-h5-official');
fs.mkdirSync(outDir, { recursive: true });
const token = 'test.' + Buffer.from(JSON.stringify({ ID: 42, UserType: 'client' })).toString('base64url') + '.test';
const now = 1792281600000;
const emptyPage = { items: [], hasMore: false, nextBeforeId: 0 };
const historyPage = { items: [{ id: 9, analysisType: 1, subjectType: 'theme', subjectId: 7, status: 3, deadlineAt: now, reportAvailable: false }], hasMore: false, nextBeforeId: 0 };
const conversation = { id: 1, title: '官方H5测试会话', archived: false, version: 5, lastSeq: 0, createdAt: now, updatedAt: now };
const conversationList = { items: [{ id: 1, title: conversation.title, archived: false, version: 5, updatedAt: now }], hasMore: false, nextBeforeId: 0 };
const messages = { conversationId: 1, items: [], upperSeq: 0, nextAfterSeq: 0, hasMore: false };
const theme = { id: 7, name: '测试示例题材', description: '测试占位' };
const excerpt = '测试示例题材：此处为自动化测试引文，非实际资料。';
const excerptBytes = Buffer.byteLength(excerpt, 'utf8');
const question = '该题材的公开定义是什么？';

const chromePath = [process.env.TM_CHROME_PATH, 'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe', 'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe'].filter(Boolean).find(p => fs.existsSync(p));
if (!chromePath) throw new Error('未找到 Chrome/Edge');

const sleep = ms => new Promise(r => setTimeout(r, ms));
async function waitFile(file, timeout = 15000) {
  const start = Date.now();
  while (Date.now() - start < timeout) {
    if (fs.existsSync(file)) { const text = fs.readFileSync(file, 'utf8').trim(); if (text) return text; }
    await sleep(100);
  }
  throw new Error('等待文件超时：' + file);
}

class CDP {
  constructor(url) {
    this.ws = new WebSocket(url);
    this.id = 0;
    this.pending = new Map();
    this.handlers = new Map();
    this.ready = new Promise((resolve, reject) => {
      this.ws.addEventListener('open', resolve);
      this.ws.addEventListener('error', reject);
    });
    this.ws.addEventListener('message', event => {
      const message = JSON.parse(event.data);
      if (message.id && this.pending.has(message.id)) {
        const job = this.pending.get(message.id);
        this.pending.delete(message.id);
        if (message.error) job.reject(new Error(message.error.message)); else job.resolve(message.result);
        return;
      }
      if (message.method && this.handlers.has(message.method)) this.handlers.get(message.method)(message.params);
    });
  }
  async send(method, params = {}) {
    await this.ready;
    const id = ++this.id;
    return new Promise((resolve, reject) => {
      this.pending.set(id, { resolve, reject });
      this.ws.send(JSON.stringify({ id, method, params }));
    });
  }
  on(method, handler) { this.handlers.set(method, handler); }
  close() { try { this.ws.close(); } catch (_) {} }
}

function jsonBody(value) { return Buffer.from(JSON.stringify({ code: 0, msg: '', data: value }), 'utf8').toString('base64'); }

(async () => {
  const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'tm-stock-h5-chrome-'));
  const chrome = spawn(chromePath, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--disable-extensions', '--remote-debugging-port=0', '--user-data-dir=' + profile, 'about:blank'], { stdio: 'ignore' });
  const results = [];
  const check = (ok, name, detail) => { results.push({ ok, name, detail }); if (!ok) throw new Error(name + (detail ? '：' + detail : '')); };
  try {
    const port = (await waitFile(path.join(profile, 'DevToolsActivePort'))).split(/\r?\n/)[0].trim();
    const created = await fetch('http://127.0.0.1:' + port + '/json/new?about:blank', { method: 'PUT' });
    const target = await created.json();
    const cdp = new CDP(target.webSocketDebuggerUrl);
    await cdp.send('Page.enable');
    await cdp.send('Runtime.enable');
    await cdp.send('Fetch.enable', { patterns: [{ urlPattern: base + '/api/v1/*', requestStage: 'Request' }] });
    const apiCalls = [];
    cdp.on('Fetch.requestPaused', async event => {
      const url = new URL(event.request.url);
      let posted = null;
      try { posted = event.request.postData ? JSON.parse(event.request.postData) : null; } catch (_) {}
      apiCalls.push({ method: event.request.method, path: url.pathname, search: url.search, posted });
      const reply = (data, status = 200) => cdp.send('Fetch.fulfillRequest', {
        requestId: event.requestId, responseCode: status,
        responseHeaders: [{ name: 'Content-Type', value: 'application/json; charset=utf-8' }],
        body: jsonBody(data),
      }).catch(() => {});
      if (url.pathname === '/api/v1/ai/tasks') return reply(historyPage);
      if (url.pathname === '/api/v1/ai/conversations' && event.request.method === 'GET') return reply(conversationList);
      if (url.pathname === '/api/v1/ai/conversations/1' && event.request.method === 'GET') return reply(conversation);
      if (url.pathname === '/api/v1/ai/conversations/1/messages') return reply(messages);
      if (url.pathname === '/api/v1/ai/conversations/1/task-intents' && event.request.method === 'GET') return reply(emptyPage);
      if (url.pathname === '/api/v1/ai/tasks/quote' && event.request.method === 'POST') {
        return reply({ analysisType: 'deep', subjectType: 'theme', subjectId: 7, subjectName: '示例题材', pricePoints: 30, priceVersion: 'v1', availablePoints: 100, quotedAt: now, holdExpireSec: 90, materialScope: [{ sourceType: 'annual_report', sourceUrl: 'https://example.test/source', collectedAt: now }] });
      }
      if (url.pathname === '/api/v1/ai/conversations/1/task-intents' && event.request.method === 'POST') {
        return reply({ id: 3, conversationId: 1, analysisType: 'deep', subjectType: 'theme', subjectId: 7, priceVersion: 'v1', pricePoints: 30, state: 'prepared', rejectionCode: '', taskId: 0, messageSeq: 0, createdAt: now, updatedAt: now });
      }
      if (url.pathname === '/api/v1/ai/conversations/1/task-intents/3/confirm' && event.request.method === 'POST') {
        return reply({ id: 3, conversationId: 1, analysisType: 'deep', subjectType: 'theme', subjectId: 7, priceVersion: 'v1', pricePoints: 30, state: 'linked', rejectionCode: '', taskId: 9, messageSeq: 1, createdAt: now, updatedAt: now });
      }
      if (url.pathname === '/api/v1/ai/tasks/9' && event.request.method === 'GET') {
        return reply({ id: 9, analysisType: 2, subjectType: 'theme', subjectId: 7, status: 3, currentStep: 5, eventSeq: 5, cancelRequested: false, errorCode: '', deadlineAt: now + 600000, reportAvailable: true });
      }
      if (url.pathname === '/api/v1/ai/tasks/9/report' && event.request.method === 'GET') {
        const q = (text, i) => ({ text, sourceType: '年报', sourceUrl: 'https://example.test/source', collectedAt: now + i });
        return reply({ taskId: 9, subjectType: 'theme', subjectId: 7, subjectName: '示例题材', generatedAt: now, disclosure: { aiGenerated: true, modelName: '测试模型（非真实调用）', filingNo: '测试编号（非备案证明）' }, sections: [
          { key: 'definition', title: '题材定义', aiGenerated: true, quotes: [q('示例题材是本测试资料的主题。', 0), q('示例题材的第二段第一句。', 1)] },
          { key: 'timeline', title: '事件时间线', aiGenerated: true, quotes: [q('示例题材的事项发生于资料记载日期。', 2), q('示例题材的第二段第二句。', 3)] },
          { key: 'segment', title: '环节说明', aiGenerated: true, quotes: [q('示例题材包含资料所述环节。', 4)] },
          { key: 'companies', title: '披露公司', aiGenerated: true, quotes: [q('示例题材的公司信息见本段。', 5)] }
        ] });
      }
      if (url.pathname === '/api/v1/theme/search') return reply([theme]);
      if (url.pathname === '/api/v1/ai/conversations/1/questions' && event.request.method === 'POST') {
        return reply({ seq: 1, state: 'queued', subjectType: 'theme', subjectId: 7, skill: 'material_qa', question: (posted && posted.question) || question, createdAt: now, updatedAt: now });
      }
      if (url.pathname === '/api/v1/ai/conversations/1/question-receipt') {
        return reply({ seq: 1, state: 'answered', subjectType: 'theme', subjectId: 7, skill: 'material_qa', question: (apiCalls.filter(c => c.path.endsWith('/questions') && c.posted).pop() || { posted: { question } }).posted.question, createdAt: now, updatedAt: now + 1000 });
      }
      if (url.pathname === '/api/v1/ai/conversations/1/questions/1/answer') {
        return reply({ seq: 1, state: 'answered', template: 'AI 资料问答：以下为所选授权资料中经核验的原文引文，不是新的报告。', subjectName: '测试示例题材', aiGenerated: true, modelName: '测试模型（非真实调用）', filingNo: '测试编号（非备案证明）', quotes: [{ snapshotId: 1, startByte: 0, endByte: excerptBytes, excerpt, sourceType: '公告', sourceUrl: 'https://example.test/source', collectedAt: now }] });
      }
      return reply(null, 404);
    });

    const evaluate = async expression => {
      const result = await cdp.send('Runtime.evaluate', { expression, awaitPromise: true, returnByValue: true });
      if (result.exceptionDetails) throw new Error('页面脚本异常：' + JSON.stringify(result.exceptionDetails).slice(0, 500));
      return result.result.value;
    };
    const text = () => evaluate('document.body ? document.body.innerText : ""');
    const clickSkill = async title => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const heading = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(title) + '); if (!heading) return "missing"; let card = heading.parentElement; for (let i = 0; i < 6 && card; i++) { const button = Array.from(card.querySelectorAll("*")).find(el => el.children.length === 0 && el.textContent.trim() === "使用该技能"); if (button) { button.click(); return "clicked"; } card = card.parentElement; } return "no-button"; })()');
    const until = async (fn, name, timeout = 30000) => {
      const start = Date.now();
      for (;;) {
        if (await fn()) return;
        if (Date.now() - start > timeout) throw new Error('等待超时：' + name + '；正文=' + String(await text()).slice(0, 600));
        await sleep(250);
      }
    };

    await cdp.send('Page.navigate', { url: base + '/' });
    await sleep(1500);
    await evaluate('localStorage.setItem("token", ' + JSON.stringify(token) + '); location.hash = "#/pages/ai/index"; location.reload();');
    await until(async () => { const body = String(await text()); return body.includes('会话') && body.includes('全部任务'); }, '首页渲染');

    const body = String(await text());
    check(body.includes('AI 资料助手'), '官方页面标题', body.slice(0, 200));
    check(body.includes('从会话开始'), '首屏会话优先文案');
    check(body.includes('我的会话'), '会话面板已挂载');

    const clickText = async label => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(label) + '); if (!leaf) return "missing"; const target = leaf.closest("button, uni-button, [role=button]") || leaf; target.click(); return "clicked"; })()');
    const isHidden = async label => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(label) + '); if (!leaf) return "missing"; let node = leaf; while (node) { if (getComputedStyle(node).display === "none") return true; node = node.parentElement; } return false; })()');

    check((await isHidden('我的会话')) === false, '会话区默认可见');
    check((await isHidden('全部资料任务（历史）')) === true, '历史任务区默认隐藏');
    check((await clickText('全部任务')) === 'clicked', '点击全部任务');
    await sleep(700);
    check((await isHidden('全部资料任务（历史）')) === false, '切到全部任务后历史区可见');
    check((await isHidden('我的会话')) === true, '切到全部任务后会话区隐藏');
    check((await clickText('会话')) === 'clicked', '切回会话');
    await sleep(700);
    check((await isHidden('我的会话')) === false, '切回后会话区可见');

    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(800);
    const shot1 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outDir, 'official-h5-375-conversation.png'), Buffer.from(shot1.data, 'base64'));
    await clickText('全部任务'); await sleep(700);
    const shot2 = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outDir, 'official-h5-375-tasks.png'), Buffer.from(shot2.data, 'base64'));

    // 官方 H5 运行时的问答纵向：打开会话→选题→提交→状态→答案→清理本地原键。
    await clickText('会话'); await sleep(600);
    check((await clickText('打开会话 #1')) === 'clicked', '打开会话 #1');
    await until(async () => String(await text()).includes('会话资料问答'), '问答面板挂载', 20000);
    await until(async () => String(await text()).includes('选择技能'), '技能卡渲染', 15000);
    check((await isHidden('会话资料问答')) === false, '默认资料问答技能可见');
    check(!String(await text()).includes('为当前会话选择轻度题材资料'), '未选择轻度技能时任务面板未挂载');
    check((await clickText('使用该技能')) === 'clicked', '切换到轻度资料整理');
    await sleep(600);
    check((await isHidden('为当前会话选择轻度题材资料')) === false, '轻度技能显示任务面板');
    check((await isHidden('会话资料问答')) === true, '切到轻度后问答面板隐藏');
    check((await clickText('使用该技能')) === 'clicked', '切回资料问答');
    await sleep(600);
    check((await isHidden('会话资料问答')) === false, '切回后问答面板恢复');
    const fill = async (placeholder, value) => evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const label = all.find(el => el.children.length === 0 && el.textContent.trim() === ' + JSON.stringify(placeholder) + '); if (!label) return "missing-label"; const root = label.closest("uni-input, uni-textarea") || label.parentElement; const input = root ? root.querySelector("input,textarea") : null; if (!input) return "missing-input"; const proto = input.tagName === "TEXTAREA" ? window.HTMLTextAreaElement.prototype : window.HTMLInputElement.prototype; const setter = Object.getOwnPropertyDescriptor(proto, "value").set; setter.call(input, ' + JSON.stringify(value) + '); input.dispatchEvent(new Event("input", { bubbles: true })); return "filled"; })()');
    const filledKeyword = await fill('查找本次问答的题材', '示例题材');
    check(filledKeyword === 'filled', '填写问答题材', filledKeyword);
    check((await clickText('查询问答题材')) === 'clicked', '查询问答题材');
    await until(async () => String(await text()).includes('选择问答题材 #7'), '题材搜索结果', 15000);
    check((await clickText('选择问答题材 #7')) === 'clicked', '选择问答题材');
    await sleep(400);
    const filledQuestion = await fill('输入所选资料的事实问题，最多1000字', question);
    check(filledQuestion === 'filled', '填写问题', filledQuestion);
    await sleep(300);
    check((await clickText('提交本次资料问题（不扣积分）')) === 'clicked', '提交问题');
    await until(async () => String(await text()).includes('本机保留有原问题'), '提交后待核查状态', 20000);
    const stored = await evaluate('localStorage.getItem("tm_ai_question_v1_42")');
    check(typeof stored === 'string' && stored.includes('question_'), '提交后本地原键存在');
    check((await clickText('核查本机原问题')) === 'clicked', '核查本机原问题');
    await until(async () => String(await text()).includes('核对当前答案'), '状态变为 answered', 15000);
    check((await clickText('核对当前答案')) === 'clicked', '核对当前答案');
    await until(async () => String(await text()).includes(excerpt), '答案引文展示', 15000);
    const answered = String(await text());
    check(answered.includes('AI 生成'), '展示 AI 标识');
    check(answered.includes('测试模型（非真实调用）'), '展示模型名');
    check(answered.includes('https://example.test/source'), '展示出处链接');
    await evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.includes("测试示例题材：此处为自动化测试引文")); if (leaf) leaf.scrollIntoView({ block: "center" }); return true; })()');
    await sleep(400);
    await cdp.send('Emulation.setDeviceMetricsOverride', { width: 375, height: 812, deviceScaleFactor: 2, mobile: true });
    await sleep(300);
    const shotAnswer = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outDir, 'official-h5-375-qa-answer.png'), Buffer.from(shotAnswer.data, 'base64'));
    const posted = apiCalls.filter(call => call.path.endsWith('/questions') && call.posted).pop();
    check(posted != null, '问答提交实际发出');
    check(posted.posted.skill === 'material_qa' && posted.posted.subjectId === 7 && posted.posted.version === 5, '提交体绑定会话版本与对象', JSON.stringify(posted.posted));
    check(apiCalls.filter(call => call.path.endsWith('/questions') && call.method === 'POST').length === 1, '只提交一次');
    check((await clickText('核实终态并清理本地原键')) === 'clicked', '核实终态并清理原键');
    await until(async () => (await evaluate('localStorage.getItem("tm_ai_question_v1_42")')) == null, '终态后清理原键', 15000);

    // 深度技能：受控 30 积分报价→深度意图→确认→关联任务；后端执行由真库测试覆盖。
    check((await clickSkill('深度资料整理')) === 'clicked', '切换到深度技能');
    await until(async () => String(await text()).includes('为当前会话选择深度题材资料'), '深度任务面板', 15000);
    check((await fill('输入题材名称，仅用于查询目录', '示例题材')) === 'filled', '填写深度题材');
    check((await clickText('查询题材')) === 'clicked', '查询深度题材');
    await until(async () => String(await text()).includes('核对该题材报价'), '深度题材结果', 15000);
    check((await clickText('核对该题材报价')) === 'clicked', '深度报价');
    await until(async () => String(await text()).includes('30 积分'), '深度30积分报价', 15000);
    check((await clickText('保存会话报价意图（不冻结积分）')) === 'clicked', '保存深度意图');
    await until(async () => String(await text()).includes('重新核对报价与资料范围'), '深度意图已准备', 15000);
    const deepQuote = apiCalls.filter(call => call.path === '/api/v1/ai/tasks/quote' && call.posted).pop();
    check(deepQuote != null && deepQuote.posted.analysis_type === 'deep' && deepQuote.posted.subject_id === 7, '深度报价请求体', JSON.stringify(deepQuote && deepQuote.posted));
    const deepPrepare = apiCalls.filter(call => call.path.endsWith('/task-intents') && call.method === 'POST' && call.posted).pop();
    check(deepPrepare != null && deepPrepare.posted.analysis_type === 'deep' && deepPrepare.posted.price_version === 'v1', '深度意图请求体', JSON.stringify(deepPrepare && deepPrepare.posted));
    check((await clickText('重新核对报价与资料范围')) === 'clicked', '重新核对深度报价');
    await until(async () => String(await text()).includes('确认会话报价并创建'), '深度确认入口', 15000);
    check((await clickText('确认会话报价并创建')) === 'clicked', '确认深度报价');
    await sleep(500);
    check((await clickText('确认冻结')) === 'clicked', '确认冻结深度报价');
    await until(async () => String(await text()).includes('查看会话关联任务 #9'), '深度任务已关联', 15000);
    check(apiCalls.filter(call => call.path.endsWith('/task-intents/3/confirm') && call.method === 'POST').length === 1, '深度确认只提交一次');
    const deepAnswered = String(await text());
    check(deepAnswered.includes('深度'), '深度状态展示');
    check((await clickText('查看会话关联任务 #9')) === 'clicked', '查看深度关联任务');
    await until(async () => String(await text()).includes('任务已完成'), '深度任务完成态', 15000);
    check((await clickText('核对并查看报告')) === 'clicked', '核对深度报告');
    await until(async () => String(await text()).includes('题材定义'), '深度报告章节', 15000);
    const deepReport = String(await text());
    check(deepReport.includes('事件时间线') && deepReport.includes('环节说明') && deepReport.includes('披露公司'), '深度四节报告');
    check((deepReport.match(/示例题材/g) || []).length >= 6, '深度报告至少六条引文');
    await evaluate('(() => { const all = Array.from(document.querySelectorAll("*")); const leaf = all.find(el => el.children.length === 0 && el.textContent.trim() === "题材定义"); if (leaf) leaf.scrollIntoView({ block: "center" }); return true; })()');
    await sleep(300);
    const deepShot = await cdp.send('Page.captureScreenshot', { format: 'png' });
    fs.writeFileSync(path.join(outDir, 'official-h5-375-deep.png'), Buffer.from(deepShot.data, 'base64'));

    check(apiCalls.some(call => call.path === '/api/v1/ai/tasks'), '官方页面实际请求任务历史');
    check(apiCalls.some(call => call.path === '/api/v1/ai/conversations'), '官方页面实际请求会话列表');
    fs.writeFileSync(path.join(outDir, 'result.json'), JSON.stringify({ ok: true, results, apiCalls }, null, 2));
    for (const row of results) console.log((row.ok ? '通过：' : '失败：') + row.name + (row.detail ? ' — ' + row.detail : ''));
    console.log('截图：' + outDir);
    console.log('通过：官方 HBuilderX H5 运行页面 + 真实 Chrome，默认会话入口、双页签切换与 375px 布局已实际渲染。');
    console.log('边界：接口为受控固定响应，未接 Go/MySQL/真实模型，不替代整链联调。');
    cdp.close();
  } finally {
    chrome.kill();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
