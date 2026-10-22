// 真实 Chrome 加载实际 UVue/UTS 编译产物，验证问答面板的浏览器交互；不同替代 HBuilderX 官方 H5 包。
// 用法：node scripts/dev/browser-qa-e2e.cjs <隔离依赖目录>
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const http = require('node:http');
const os = require('node:os');
const { spawn } = require('node:child_process');
const { createRequire } = require('node:module');
const root = path.resolve(__dirname, '../..');
if (!process.argv[2]) throw new Error('必须提供与业务工程隔离的依赖目录');
const dependency = createRequire(path.join(path.resolve(process.argv[2]), 'package.json'));
const ts = dependency('typescript');
const compiler = dependency('@vue/compiler-sfc');

const chromeCandidates = [
  process.env.TM_CHROME_PATH,
  'C:\\Program Files\\Google\\Chrome\\Application\\chrome.exe',
  'C:\\Program Files (x86)\\Microsoft\\Edge\\Application\\msedge.exe',
].filter(Boolean);
const chromePath = chromeCandidates.find(p => fs.existsSync(p));
if (!chromePath) throw new Error('未找到 Chrome/Edge，无法执行浏览器验收');

// 与前端源码测试相同的固定令牌载荷：仅用于本地分区，不授予服务端权限。
const token = 'test.' + Buffer.from(JSON.stringify({ ID: 1, UserType: 'client' })).toString('base64url') + '.test';
const now = 1792281600000;
const excerpt = '测试示例题材：此处为自动化测试引文，非实际资料。';
const excerptBytes = Buffer.byteLength(excerpt, 'utf8');
const question = '该题材的公开定义是什么？';

const status = (over = {}) => ({ seq: 1, state: 'queued', subjectType: 'theme', subjectId: 7, skill: 'material_qa', question, createdAt: now, updatedAt: now, ...over });
const answer = () => ({ seq: 1, state: 'answered', template: 'AI 资料问答：以下为所选授权资料中经核验的原文引文，不是新的报告。', subjectName: '测试示例题材', aiGenerated: true, modelName: '测试模型（非真实调用）', filingNo: '测试编号（非备案证明）', quotes: [{ snapshotId: 1, startByte: 0, endByte: excerptBytes, excerpt, sourceType: '公告', sourceUrl: 'https://example.test/source', collectedAt: now }] });
const conversation = { id: 3, title: '测试会话', archived: false, version: 5, lastSeq: 4, createdAt: now, updatedAt: now };

function bundle(entry) {
  const ids = new Map(), modules = new Map(), queue = [path.resolve(entry)];
  const idFor = file => {
    file = path.resolve(file);
    if (!ids.has(file)) ids.set(file, 'm' + ids.size);
    return ids.get(file);
  };
  const entryId = idFor(entry);
  while (queue.length) {
    const file = path.resolve(queue.shift());
    if (modules.has(file)) continue;
    const source = fs.readFileSync(file, 'utf8');
    let code;
    if (file.endsWith('.uvue')) {
      const parsed = compiler.parse(source, { filename: file });
      assert.deepEqual(parsed.errors, [], file + ' SFC 解析失败');
      const script = compiler.compileScript(parsed.descriptor, { id: file });
      const template = compiler.compileTemplate({ source: parsed.descriptor.template.content, filename: file, id: file, compilerOptions: { isCustomElement: () => false, bindingMetadata: script.bindings } });
      assert.deepEqual(template.errors, [], file + ' 模板编译失败');
      const scriptJS = ts.transpileModule(script.content, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText;
      const templateJS = ts.transpileModule(template.code, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText;
      code = scriptJS + '\n;(function(){\n' + templateJS + '\nexports.default.render = exports.render;\n})();\n';
    } else {
      code = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText;
    }
    code = code.replace(/require\((["'])([^"']+)\1\)/g, (all, quote, spec) => {
      if (spec === 'vue') return '__req("vue")';
      if (!spec.startsWith('.')) throw new Error('浏览器验收不接受非相对依赖：' + spec);
      const target = path.resolve(path.dirname(file), spec);
      if (!fs.existsSync(target)) throw new Error('找不到模块：' + target);
      queue.push(target);
      return '__req(' + JSON.stringify(idFor(target)) + ')';
    });
    modules.set(file, { id: idFor(file), code });
  }
  const table = {};
  for (const [, mod] of modules) table[mod.id] = mod.code;
  const bodies = Object.entries(table).map(([id, source]) => JSON.stringify(id) + ':function(module,exports,__req){\n' + source + '\n}').join(',\n');
  const code = 'window.__modules={' + bodies + '};\n' +
    'window.__cache={};window.__req=function(id){if(id==="vue")return window.Vue;if(window.__cache[id])return window.__cache[id].exports;' +
    'var f=window.__modules[id];if(typeof f!=="function")throw new Error("module missing: "+id+" type="+typeof f+" count="+Object.keys(window.__modules).length+" keys="+Object.keys(window.__modules).slice(0,3).join(","));' +
    'var m={exports:{}};window.__cache[id]=m;f(m,m.exports,window.__req);return m.exports;};window.__reqLoaded=typeof window.__req;';

  return { code, entryId };
}

// 浏览器侧驱动：全部通过真实 DOM 事件操作，不直接改组件内部状态。
const driver = String.raw`
(function () {
  const report = (ok, name, detail) => fetch('/__result', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ ok, name, detail }) });
  window.uni = {
    getStorageSync: key => localStorage.getItem(key) || '',
    setStorageSync: (key, value) => localStorage.setItem(key, value),
    removeStorageSync: key => localStorage.removeItem(key),
    request: options => {
      const init = { method: options.method || 'GET', headers: options.header || {} };
      if (init.method !== 'GET' && options.data != null) init.body = JSON.stringify(options.data);
      fetch(options.url, init).then(async response => {
        const text = await response.text();
        if (options.success) options.success({ statusCode: response.status, data: text });
      }).catch(error => { if (options.fail) options.fail(error); });
    },
    showModal: options => { window.__modals = window.__modals || []; window.__modals.push(options); },
    setClipboardData: () => {}, navigateTo: () => {},
  };
  window.__logs = [];
  const keep = console.error.bind(console), keepWarn = console.warn.bind(console);
  console.error = (...args) => { window.__logs.push(args.map(String).join(' ')); keep(...args); };
  console.warn = (...args) => { window.__logs.push(args.map(String).join(' ')); keepWarn(...args); };
  window.onerror = (message) => { report(false, 'uncaught', String(message) + ' | ' + window.__logs.slice(-3).join(' / ')); };
  window.addEventListener('unhandledrejection', e => report(false, 'unhandled', String(e.reason)));
  const results = [];
  const check = (ok, name, detail) => { results.push({ ok, name, detail }); if (!ok) throw new Error(name + ': ' + (detail || '')); };
  const sleep = ms => new Promise(r => setTimeout(r, ms));
  const text = () => document.getElementById('app').innerText;
  async function until(fn, name, timeout = 8000) {
    const start = Date.now();
    for (;;) {
      const value = fn();
      if (value) return value;
      if (Date.now() - start > timeout) throw new Error('等待超时：' + name + '；当前文本：' + text().slice(0, 400));
      await sleep(50);
    }
  }
  const buttons = () => Array.from(document.querySelectorAll('button'));
  const button = label => buttons().find(b => b.textContent.trim() === label && !b.disabled);
  async function click(label) { const b = await until(() => button(label), '可点击按钮 ' + label); b.click(); }
  function fill(placeholder, value) {
    const input = document.querySelector('[placeholder="' + placeholder + '"]');
    if (!input) throw new Error('找不到输入框：' + placeholder + '；输入框数=' + document.querySelectorAll('input,textarea').length + '；正文=' + text().slice(0, 300));
    input.value = value;
    input.dispatchEvent(new Event('input', { bubbles: true }));
  }
  async function run() {
    if (typeof window.__req !== 'function') {
      const fetched = await fetch('/bundle.js');
      const body = await fetched.text();
      throw new Error('打包运行时缺失：__req=' + typeof window.__req + ' modules=' + typeof window.__modules + ' status=' + fetched.status + ' len=' + body.length + ' head=' + body.slice(0, 60));
    }
    localStorage.setItem('token', TOKEN);
    const panel = window.__req(window.ENTRY).default;
    const app = window.Vue.createApp({ render: () => window.Vue.h(panel, { ref: value => { window.__panelRef = value; }, sessionToken: TOKEN, conversationId: 3, archived: false, questionSeq: 0 }) });
    app.component('view', { render() { return window.Vue.h('div', {}, this.$slots.default ? this.$slots.default() : []); } });
    app.component('text', { render() { return window.Vue.h('span', { style: 'display:block' }, this.$slots.default ? this.$slots.default() : []); } });
    app.component('scroll-view', { render() { return window.Vue.h('div', {}, this.$slots.default ? this.$slots.default() : []); } });
    app.component('x-sheet', { render() { return window.Vue.h('section', {}, this.$slots.default ? this.$slots.default() : []); } });
    app.component('x-navbar', { props: ['title'], render() { return window.Vue.h('header', {}, this.title); } });
    app.component('x-button', { props: ['disabled', 'loading'], emits: ['click'], render() { return window.Vue.h('button', { disabled: this.disabled, onClick: () => { if (!this.disabled) this.$emit('click'); } }, this.$slots.default ? this.$slots.default() : []); } });
    app.component('x-input', { props: ['modelValue', 'disabled', 'placeholder', 'maxlength', 'type'], emits: ['update:modelValue'], render() {
      const self = this;
      return window.Vue.h(this.type === 'textarea' ? 'textarea' : 'input', { value: this.modelValue, disabled: this.disabled, placeholder: this.placeholder, maxlength: this.maxlength, onInput: e => self.$emit('update:modelValue', e.target.value) });
    } });
    app.mount('#app');
    await until(() => text().includes('会话资料问答'), '问答面板挂载');
    await sleep(100);
    check(window.__panelRef != null, '组件实例已挂载');
    check(!text().includes('测试模型'), '初始不展示未经读取的模型信息');

    fill('查找本次问答的题材', '示例题材');
    await click('查询问答题材');
    await until(() => text().includes('测试示例题材'), '题材搜索结果');
    await click('选择问答题材 #7');
    fill('输入所选资料的事实问题，最多1000字', QUESTION);
    await sleep(50);
    await click('提交本次资料问题（不扣积分）');
    await until(() => text().includes('本机保留有原问题'), '提交后待核查状态');
    check(!button('提交本次资料问题（不扣积分）'), '待核查期间没有第二个提交入口');
    check(!!localStorage.getItem('tm_ai_question_v1_1'), '发请求前已持久化原键');

    await click('核查本机原问题');
    await until(() => text().includes('核对当前答案'), '状态变为 answered');
    await click('核对当前答案');
    await until(() => text().includes(EXCERPT), '答案引文展示');
    check(text().includes('AI 生成'), '展示 AI 标识');
    check(text().includes('测试模型（非真实调用）'), '展示模型名');
    check(text().includes('https://example.test/source'), '展示出处链接');

    await click('核实终态并清理本地原键');
    await until(() => !localStorage.getItem('tm_ai_question_v1_1'), '终态后清理原键');
    check(text().includes('本地原键已清理'), '清理提示');
    return results;
  }
  run().then(async rows => {
    const bad = rows.find(r => !r.ok);
    await report(!bad, 'browser-qa', rows);
  }).catch(async error => { await report(false, 'browser-qa', [].concat(results, [{ ok: false, name: String(error && error.message || error) }])); });
})();
`;

const html = String.raw`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8"><title>问答面板浏览器验收</title></head>
<body><main id="app"></main>
<script src="/vendor/vue.global.prod.js"></script>
<script src="/bundle.js"></script>
<script src="/driver.js"></script>
</body></html>`;

function main() {
  const built = bundle('pages/ai/components/AIQuestionPanel.uvue');
  const bundleJS = built.code;
  const requests = [];
  let finish;
  const done = new Promise(resolve => { finish = resolve; });
  const json = (res, code, data, statusCode = 200) => { res.writeHead(statusCode, { 'Content-Type': 'application/json' }); res.end(JSON.stringify({ code, msg: '', data })); };
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, 'http://127.0.0.1');
    if (url.pathname === '/vendor/vue.global.prod.js') { res.writeHead(200, { 'Content-Type': 'text/javascript' }); res.end(fs.readFileSync(path.join(process.argv[2], 'node_modules/vue/dist/vue.global.prod.js'))); return; }
    if (url.pathname === '/bundle.js') { res.writeHead(200, { 'Content-Type': 'text/javascript' }); res.end(bundleJS); return; }
    if (url.pathname === '/driver.js') { res.writeHead(200, { 'Content-Type': 'text/javascript' }); res.end('window.ENTRY=' + JSON.stringify(built.entryId) + ';window.TOKEN=' + JSON.stringify(token) + ';window.QUESTION=' + JSON.stringify(question) + ';window.EXCERPT=' + JSON.stringify(excerpt) + ';\n' + driver); return; }
    if (url.pathname === '/') { res.writeHead(200, { 'Content-Type': 'text/html; charset=utf-8' }); res.end(html); return; }
    if (url.pathname === '/__result' && req.method === 'POST') {
      let body = '';
      req.on('data', chunk => { body += chunk; });
      req.on('end', () => { finish(JSON.parse(body)); res.writeHead(204); res.end(); });
      return;
    }
    let body = '';
    req.on('data', chunk => { body += chunk; });
    req.on('end', () => {
      requests.push({ method: req.method, path: url.pathname, search: url.search, header: req.headers, body });
      if (url.pathname === '/api/v1/theme/search') return json(res, 0, [{ id: 7, name: '测试示例题材', description: '测试占位' }]);
      if (url.pathname === '/api/v1/ai/conversations/3' && req.method === 'GET') return json(res, 0, conversation);
      if (url.pathname === '/api/v1/ai/conversations/3/questions' && req.method === 'POST') return json(res, 0, status());
      if (url.pathname === '/api/v1/ai/conversations/3/question-receipt') return json(res, 0, status({ state: 'answered', updatedAt: now + 1000 }));
      if (url.pathname === '/api/v1/ai/conversations/3/questions/1/answer') return json(res, 0, answer());
      return json(res, 40403, null, 404);
    });
  });
  server.listen(0, '127.0.0.1', async () => {
    const origin = 'http://127.0.0.1:' + server.address().port;
    const profile = fs.mkdtempSync(path.join(os.tmpdir(), 'tm-stock-chrome-'));
    const chrome = spawn(chromePath, ['--headless=new', '--disable-gpu', '--no-first-run', '--no-default-browser-check', '--disable-extensions', '--user-data-dir=' + profile, origin + '/'], { stdio: 'ignore' });
    let result;
    try {
      result = await Promise.race([done, new Promise((_, reject) => setTimeout(() => reject(new Error('浏览器验收超时（30秒）')), 30000))]);
    } finally {
      chrome.kill();
      server.close();
    }
    const rows = Array.isArray(result.detail) ? result.detail : [{ ok: false, name: String(result.detail) }];
    for (const row of rows) console.log((row.ok ? '通过：' : '失败：') + row.name + (row.detail ? ' — ' + JSON.stringify(row.detail) : ''));
    assert.equal(result.ok, true, '浏览器内断言失败');
    assert.equal(rows.every(r => r.ok), true);
    const search = requests.find(r => r.path === '/api/v1/theme/search');
    assert.ok(search); assert.equal(search.method, 'GET');
    const detail = requests.filter(r => r.path === '/api/v1/ai/conversations/3' && r.method === 'GET');
    assert.equal(detail.length, 1);
    const submit = requests.filter(r => r.path === '/api/v1/ai/conversations/3/questions' && r.method === 'POST');
    assert.equal(submit.length, 1, '只能提交一次');
    assert.match(submit[0].header['idempotency-key'] || '', /^question_[a-f0-9]{48}$/);
    const submitted = JSON.parse(submit[0].body);
    assert.deepEqual(submitted, { version: 5, subjectType: 'theme', subjectId: 7, skill: 'material_qa', question });
    assert.equal(submit[0].header.authorization, 'Bearer ' + token);
    assert.equal(requests.filter(r => r.path === '/api/v1/ai/conversations/3/question-receipt').length, 2, '核查与终态各一次');
    assert.equal(requests.filter(r => r.path === '/api/v1/ai/conversations/3/questions/1/answer').length, 1);
    console.log('通过：真实 Chrome 加载实际问答面板，完成选题→提交→状态→答案→清理原键；服务端断言一次提交与请求顺序。');
    console.log('本验收不覆盖：官方 HBuilderX H5 包、真实 Go/MySQL/模型联调、报告追问与移动端真机。');
  });
}
main();
