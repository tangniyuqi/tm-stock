// 直接执行实际 UTS 模块与 UVue 页面的模拟接口验收；不替代 HBuilderX 或真实浏览器验收。
// 用法：node scripts/dev/verify-ai-report-ui.cjs <隔离依赖目录>
// 固定测试依赖：vue/@vue/compiler-sfc/@vue/server-renderer 3.5.13、typescript 5.7.2。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const root = path.resolve(__dirname, '../..');
if (!process.argv[2]) throw new Error('必须提供与业务工程隔离的依赖目录');
const dependency = createRequire(path.join(path.resolve(process.argv[2]), 'package.json'));
const ts = dependency('typescript');
const compiler = dependency('@vue/compiler-sfc');
const vue = dependency('vue');
const { renderToString } = dependency('@vue/server-renderer');
const { webcrypto } = require('node:crypto');
const native = tag => ['view', 'text', 'scroll-view', 'x-sheet', 'x-button', 'x-navbar', 'x-input'].includes(tag);
const clone = value => JSON.parse(JSON.stringify(value));
const now = 1792281600000;
const memberToken = id => 'test.' + Buffer.from(JSON.stringify({ ID: id, UserType: 'client' })).toString('base64url') + '.test';
const quote = (over = {}) => ({ analysisType: 'light', subjectType: 'theme', subjectId: 7, subjectName: '测试示例题材', pricePoints: 7, priceVersion: 'test-v1', availablePoints: 20, quotedAt: now, holdExpireSec: 90, ...over,
  materialScope: [{ sourceType: 'announcement', sourceUrl: 'https://example.test/source', collectedAt: now }] });
const receipt = () => ({ taskId: 9, pricePoints: 7, priceVersion: 'test-v1' });
const summary = (id = 9, status = 3) => ({ id, analysisType: 1, subjectType: 'theme', subjectId: 7, status, deadlineAt: now, reportAvailable: false });
const progress = (id = 9, status = 3) => ({ ...summary(id, status), currentStep: 4, eventSeq: 8, cancelRequested: false, errorCode: '', reportAvailable: status === 3 });
const report = (id = 9) => ({
  taskId: id, subjectType: 'theme', subjectId: 7, subjectName: '测试示例题材', generatedAt: now,
  disclosure: { aiGenerated: true, modelName: '测试模型（非真实调用）', filingNo: '测试编号（非备案证明）' },
  sections: ['definition', 'timeline', 'segment', 'companies'].map((key, i) => ({
    key, title: ['题材定义', '事件时间线', '环节说明', '披露公司'][i], aiGenerated: true,
    quotes: [{ text: '测试示例题材：本段是自动化测试示例，非实际资料。', sourceType: '公告', sourceUrl: 'https://example.test/source', collectedAt: now }],
  })),
});

// 明确的测试公司及示例出处，不进入产品默认数据。
const companyReport = (id = 9) => {
  const value = report(id);
  value.sections[3] = { key: 'companies', title: '披露公司', aiGenerated: false, quotes: [], companies: [1, 2].map(i => ({
    name: `测试示例公司${i}`, code: `60000${i}`, node: '测试示例环节',
    evidence: [{ text: `测试示例公司${i}：自动化测试资料，非真实公司披露。`, sourceType: '年报', sourceUrl: 'https://example.test/source', collectedAt: now, trimmed: true }],
  })) };
  return value;
};

function environment(sharedStorage = new Map(), conversations = false, conversationTasks = false) {
  const cache = new Map();
  const requests = [], timers = new Map(), intervals = new Map(), hooks = { show: [], hide: [], unload: [] };
  const storage = sharedStorage, modals = [], opened = [], navigation = [], clipboard = [], lockQueues = new Map();
  let timerID = 0;
  const uni = {
    getStorageSync: key => storage.get(key), request: options => {
      // 旧任务专项明确保持新会话服务关闭；会话专项另行启用并捕获全部请求。
      if (!conversations && /\/api\/v1\/ai\/conversation/.test(options.url)) { options.success({statusCode:404,data:JSON.stringify({code:40403,msg:'测试会话服务关闭',data:null})}); return; }
      if (!conversationTasks && /\/task-intent/.test(options.url)) { options.success({statusCode:404,data:JSON.stringify({code:40403,msg:'测试会话意图服务关闭',data:null})}); return; }
      requests.push(options);
    },
    setStorageSync: (key, value) => storage.set(key, value), removeStorageSync: key => storage.delete(key),
    showModal: options => modals.push(options), showToast: () => {},
    setClipboardData: options => clipboard.push(options), navigateTo: options => navigation.push(options.url),
  };
  const globals = {
    uni, window: { atob, crypto: webcrypto, navigator: { locks: { request: (name, callback) => {
      const result = (lockQueues.get(name) || Promise.resolve()).then(callback);
      lockQueues.set(name, result.catch(() => {})); return result;
    } } }, open: url => { const target = { opener: '测试占位' }; opened.push({ url, target }); return target; } },
    onShow: fn => hooks.show.push(fn), onHide: fn => hooks.hide.push(fn), onUnload: fn => hooks.unload.push(fn),
    setTimeout: (fn, ms) => { const id = ++timerID; timers.set(id, { fn, ms }); return id; },
    clearTimeout: id => timers.delete(id),
    setInterval: (fn, ms) => { const id = ++timerID; intervals.set(id, { fn, ms }); return id; },
    clearInterval: id => intervals.delete(id),
  };
  function execute(source, filename) {
    const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }, reportDiagnostics: true });
    assert.equal(output.diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error).length, 0, `${filename} 语法转换失败`);
    const sandbox = { exports: {}, ...globals, require: name => name.startsWith('.') ? load(path.resolve(path.dirname(filename), name)) : dependency(name) };
    vm.runInNewContext(output.outputText, sandbox, { timeout: 2000, filename });
    return sandbox.exports;
  }
  function load(filename) {
    filename = path.isAbsolute(filename) ? filename : path.join(root, filename);
    if (cache.has(filename)) return cache.get(filename);
    const source = fs.readFileSync(filename, 'utf8');
    let exports;
    if (filename.endsWith('.uvue')) {
      const parsed = compiler.parse(source, { filename });
      assert.deepEqual(parsed.errors, [], `${filename} SFC 解析失败`);
      const script = compiler.compileScript(parsed.descriptor, { id: filename });
      const template = compiler.compileTemplate({ source: parsed.descriptor.template.content, filename, id: filename, compilerOptions: { isCustomElement: native, bindingMetadata: script.bindings } });
      assert.deepEqual(template.errors, [], `${filename} 模板编译失败`);
      exports = execute(script.content, filename);
      exports.default.render = execute(template.code, filename).render;
    } else exports = execute(source, filename);
    cache.set(filename, exports);
    return exports;
  }
  const reply = (request, data, status = 200, code = 0) => request.success({ statusCode: status, data: JSON.stringify({ code, msg: '测试内部文字不得展示', data }) });
  return { load, requests, timers, intervals, hooks, storage, modals, opened, navigation, clipboard, reply, globals, uni };
}

function mount(component, props = {}) {
  const node = (tag, text = '') => ({ tag, text, children: [], props: {}, parent: null });
  const renderer = vue.createRenderer({
    createElement: tag => node(tag), createText: text => node('#text', text), createComment: text => node('#comment', text),
    insert(child, parent, anchor) {
      if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1);
      const index = anchor ? parent.children.indexOf(anchor) : -1;
      parent.children.splice(index < 0 ? parent.children.length : index, 0, child); child.parent = parent;
    },
    remove(child) { if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1); child.parent = null; },
    setText: (n, text) => { n.text = text; }, setElementText: (n, text) => { n.text = text; n.children = []; },
    parentNode: n => n.parent, nextSibling: n => n.parent?.children[n.parent.children.indexOf(n) + 1] || null,
    patchProp: (n, key, old, value) => { n.props[key] = value; },
  });
  const tree = node('root'), app = renderer.createApp(component, props);
  app.mount(tree);
  const text = n => (n.tag === '#comment' ? '' : n.text) + n.children.map(text).join('');
  const all = n => [n, ...n.children.flatMap(all)];
  return { app, tree, state: app._instance.setupState, text: () => text(tree), input(value) {
    const field = all(tree).find(n => n.tag === 'x-input' && !n.props.disabled); assert.ok(field, '找不到可编辑的实际输入绑定');
    assert.ok(!field.props.disabled); field.props['on:update:modelValue'](value);
  }, click(label, occurrence = 0) {
    const button = all(tree).filter(n => n.tag === 'x-button' && text(n) === label)[occurrence];
    assert.ok(button, `找不到实际模板按钮：${label}`);
    assert.ok(!button.props.disabled, `按钮被禁用：${label}`);
    button.props.onClick();
  } };
}
const flush = async () => { for (let i = 0; i < 8; i++) { await Promise.resolve(); await vue.nextTick(); } };

async function validators() {
  const env = environment(), rules = env.load('pages/libs/ai.uts');
  assert.equal(rules.isValidAIReport(report(), 9), true);
  const broken = [
    r => { r.taskId = '9'; }, r => { r.taskId = 9007199254740992; }, r => { r.subjectId = 0; },
    r => { r.subjectType = ['theme']; }, r => { r.generatedAt = 8640000000000001; },
    r => { delete r.disclosure; }, r => { r.disclosure.aiGenerated = 1; }, r => { r.disclosure.modelName = ''; },
    r => { r.disclosure.filingNo = ''; }, r => { r.sections.reverse(); }, r => { r.sections[0].aiGenerated = 'true'; },
    r => { r.sections[0].quotes = []; }, r => { r.sections[0].quotes[0].text = '缺失题材名'; },
    r => { r.sections[0].quotes[0].sourceType = ['公告']; }, r => { r.sections[0].quotes[0].sourceUrl = 'javascript:alert(1)'; },
    r => { r.sections[0].quotes[0].collectedAt = -1; }, r => { r.sections[0].quotes = Array(17).fill(r.sections[0].quotes[0]); },
  ];
  for (const mutate of broken) { const value = report(); mutate(value); assert.equal(rules.isValidAIReport(value, 9), false); }
  assert.equal(rules.isValidAIReport(null, 9), false);
  assert.equal(rules.isValidAIReport(companyReport(), 9), true);
  const companyBroken = [
    r => { r.sections[0].companies = r.sections[3].companies; }, r => { r.sections[3].aiGenerated = true; },
    r => { r.sections[3].quotes = report().sections[3].quotes; }, r => { r.sections[3].companies = []; },
    r => { r.sections[3].companies = Array(33).fill(r.sections[3].companies[0]); }, r => { r.sections[3].companies.reverse(); },
    r => { r.sections[3].companies[0] = null; }, r => { r.sections[3].companies[0].name = ''; },
    r => { r.sections[3].companies[0].code = '600001.SH'; }, r => { r.sections[3].companies[0].code = 600001; },
    r => { r.sections[3].companies[0].node = ''; }, r => { r.sections[3].companies[0].evidence = []; },
    r => { r.sections[3].companies[0].evidence = null; }, r => { r.sections[3].companies[0].evidence[0] = null; },
    r => { r.sections[3].companies[0].evidence[0].text = '缺失公司名称'; },
    r => { r.sections[3].companies[0].evidence[0].sourceUrl = 'javascript:alert(1)'; },
    r => { r.sections[3].companies[0].evidence[0].sourceType = '其他'; },
    r => { r.sections[3].companies[0].evidence[0].trimmed = 'true'; },
    r => { r.sections[3].companies[0].evidence[0].collectedAt = 9007199254740992; },
    r => { delete r.sections[3].companies; },
  ];
  for (const mutate of companyBroken) { const value = companyReport(); mutate(value); assert.equal(rules.isValidAIReport(value, 9), false); }
  assert.equal(rules.isValidAITaskHistory({ items: [summary(9007199254740991)], hasMore: false, nextBeforeId: 0 }, 0), true);
  for (const status of [1, 2, 3, 4, 5, 6]) {
    assert.equal(rules.isValidAITaskSummary(summary(9, status)), true);
    const invalid = { ...summary(9, status), reportAvailable: true };
    assert.equal(rules.isValidAITaskSummary(invalid), false, '历史摘要不能证明当前报告可读，成功状态也不能放行');
    assert.equal(rules.isValidAITaskHistory({ items: [summary(10), invalid], hasMore: true, nextBeforeId: 9 }, 0), false);
  }
  for (const items of [[summary(8), summary(9)], [summary(9), summary(9)], [summary(9007199254740992)]]) {
    assert.equal(rules.isValidAITaskHistory({ items, hasMore: false, nextBeforeId: 0 }, 0), false);
  }
  assert.equal(rules.isValidAITaskHistory({ items: [summary(8)], hasMore: true, nextBeforeId: 8 }, 9), true);
  assert.equal(rules.isValidAITaskHistory({ items: [], hasMore: true, nextBeforeId: 0 }, 0), false);
  assert.equal(rules.isValidAITaskProgress(progress(), 9), true);
  for (const bad of [{ id: '9' }, { reportAvailable: 1 }, { currentStep: 65 }, { errorCode: '内部消息' }, { status: 2, reportAvailable: true }]) {
    assert.equal(rules.isValidAITaskProgress({ ...progress(), ...bad }, 9), false);
  }
  console.log('通过：实际 DTO 验证器，安全整数、排序分页、状态闭集、四节结构、AI 标识及来源正反例。');
}

async function api() {
  const env = environment(), { AiApi } = env.load('pages/libs/ai-api.uts'), { MemberApi } = env.load('pages/libs/member-api.uts');
  env.storage.set('token', 'test-current-member'); env.storage.set('tm_member_token', 'test-old-placeholder');
  let promise = AiApi.history(0), request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/tasks?limit=20'); assert.equal(request.method, 'GET');
  assert.equal(request.header.Authorization, 'Bearer test-current-member'); assert.equal(request.timeout, 10000);
  assert.ok(!request.url.includes('token')); env.reply(request, { items: [summary()], hasMore: false, nextBeforeId: 0 });
  assert.equal((await promise).code, 0);
  for (const status of [1, 2, 3, 4, 5, 6]) {
    promise = AiApi.history(0); request = env.requests.shift();
    env.reply(request, { items: [{ ...summary(9, status), reportAvailable: true }], hasMore: false, nextBeforeId: 0 });
    const invalid = await promise;
    assert.equal(invalid.data, null, '非法历史报告许可须整页拒绝'); assert.notEqual(invalid.code, 0);
    assert.equal(env.requests.length, 0, '不能自动改查详情、读取报告或触发创建');
  }
  for (const bad of ['https://example.test/api/v1/ai/tasks', '/api/v1/ai/tasks/9?token=x', '/api/v1/ai/tasks/9/cancel', '/api/v1/ai/tasks?limit=20&before_id=0']) {
    assert.equal((await MemberApi.aiRead(bad)).data, null); assert.equal(env.requests.length, 0);
  }
  assert.equal((await AiApi.report(9007199254740992)).data, null); assert.equal(env.requests.length, 0);
  promise = AiApi.report(9); request = env.requests.shift(); env.reply(request, report(), 503, 0);
  assert.equal((await promise).data, null, 'HTTP 失败不得借 code=0 发布正文');
  promise = AiApi.report(9); request = env.requests.shift(); request.success({ statusCode: 200, data: '<html>测试错误</html>' });
  assert.equal((await promise).data, null);
  promise = AiApi.report(9); request = env.requests.shift(); env.reply(request, report(), 422, 42202);
  const rejected = await promise; assert.equal(rejected.data, null); assert.ok(!rejected.message.includes('内部'));
  promise = AiApi.report(9); request = env.requests.shift(); env.reply(request, report()); assert.equal((await promise).data.taskId, 9);
  promise = AiApi.progress(9); env.requests.shift().fail(); assert.equal((await promise).data, null); assert.equal(env.requests.length, 0);
  console.log('通过：实际请求封装复用现有会员令牌、GET 闭集、错误脱敏、无效 JSON/HTTP 拒绝、无自动重发。');
}

async function historyReportPermission() {
  // 首次读取与加载更多均须整页失败；正常成功详情仍可单独核验报告。
  for (const more of [false, true]) {
    const env = environment(); env.storage.set('token', memberToken(42));
    const ui = mount(env.load('pages/ai/index.uvue').default);
    env.hooks.show.forEach(fn => fn());
    if (more) {
      env.reply(env.requests.shift(), { items: [summary(10)], nextBeforeId: 10, hasMore: true }); await flush();
      ui.click('加载更多');
    }
    const request = env.requests.shift();
    assert.equal(request.method, 'GET');
    env.reply(request, { items: [summary(9), { ...summary(8), reportAvailable: true }], nextBeforeId: 8, hasMore: true }); await flush();
    assert.ok(ui.text().includes('资料服务暂不可用'));
    assert.deepEqual(Array.from(ui.state.history, value => value.id), more ? [10] : [], '不得接受整页中看似正常的前半部分');
    assert.equal(ui.state.beforeId, more ? 10 : 0); assert.equal(ui.state.hasMore, more);
    assert.equal(ui.state.progress, null); assert.equal(ui.state.report, null); assert.equal(env.requests.length, 0);
    // 用户主动查详情走原独立门禁，不根据被拒绝的历史字段获得许可。
    ui.state.loadProgress(9); env.reply(env.requests.shift(), progress()); await flush();
    ui.click('核对并查看报告'); env.reply(env.requests.shift(), report()); await flush();
    assert.equal(ui.state.report.taskId, 9); assert.equal(ui.state.history.length, more ? 1 : 0);
    env.hooks.unload.forEach(fn => fn()); ui.app.unmount(); assert.equal(env.timers.size, 0); assert.equal(env.intervals.size, 0);
  }
  console.log('通过：历史报告许可恒假，六种状态/API整页拒绝及实际页面首页/分页不混入、不改游标；独立详情和报告核验保持可用。');
}

async function taskOutcomeFeedback() {
  const rules = environment().load('pages/libs/ai.uts');
  assert.equal(typeof rules.aiTaskOutcomeMessage, 'function', '缺少经过验证的任务状态说明');
  const cases = [
    [4, 'provider_unavailable', '执行服务暂不可用'],
    [4, 'invalid_output', '返回内容未通过结构校验'],
    [4, 'guard_rejected', '资料未通过校验'],
    [4, 'budget_exhausted', '已达到执行限制'],
    [4, 'attempts_exhausted', '已达到处理次数限制'],
    [4, 'internal_failure', '任务处理未完成'],
    [5, 'cancelled', '不代表远端调用已经停止'],
    [6, 'expired', '任务已过期'],
  ];
  for (const [status, code, expected] of cases) {
    const value = { ...progress(9, status), errorCode: code, cancelRequested: status === 5 || status === 6 };
    assert.ok(rules.isValidAITaskProgress(value, 9));
    const note = rules.aiTaskOutcomeMessage(value);
    assert.ok(note.includes(expected));
    assert.ok(!note.includes(code) && !note.includes('已退款') && !note.includes('已释放'));
    const env = environment(); env.storage.set('token', memberToken(42));
    const ui = mount(env.load('pages/ai/index.uvue').default);
    env.hooks.show.forEach(fn => fn()); env.reply(env.requests.shift(), { items: [summary(9, status)], nextBeforeId: 0, hasMore: false }); await flush();
    ui.state.loadProgress(9); env.reply(env.requests.shift(), value); await flush();
    assert.ok(ui.text().includes(note), '实际模板必须展示固定说明');
    assert.ok(!ui.text().includes('取消请求已登记，等待后台确认'), '终态仍保留取消标志时不能继续显示等待');
    assert.equal(ui.state.report, null); assert.equal(env.requests.length, 0); assert.equal(env.timers.size, 0);
    ui.state.retryDetail(); await flush(); assert.ok(!ui.text().includes(note), '重新核对时清除旧任务说明');
    env.reply(env.requests.shift(), progress(9, 2)); await flush();
    assert.ok(ui.text().includes('任务尚未交付')); assert.ok(!ui.text().includes(note)); assert.equal(env.timers.size, 1);
    ui.state.retryDetail(); env.reply(env.requests.shift(), { ...value, errorCode: 'test-only-private-upstream-text' }); await flush();
    assert.equal(ui.state.progress, null); assert.ok(!ui.text().includes('test-only-private-upstream-text')); assert.ok(!ui.text().includes(note));
    env.hooks.unload.forEach(fn => fn()); ui.app.unmount();
  }
  assert.equal(rules.aiTaskOutcomeMessage(null), '');
  assert.equal(rules.aiTaskOutcomeMessage({ ...progress(9, 4), errorCode: 'test-only-private-upstream-text' }), '');
  assert.ok(rules.aiTaskOutcomeMessage({ ...progress(), reportAvailable: false }).includes('报告暂不可读取'));
  assert.ok(rules.aiTaskOutcomeMessage(progress()).includes('单独核对读取权限'));
  console.log('通过：实际页面展示六种固定失败原因与取消/过期说明，旧说明清理、串行查询、不泄漏错误原文、不承诺退款或远端停止。');
}

async function page() {
  const env = environment(), component = env.load('pages/ai/index.uvue').default, ui = mount(component);
  env.hooks.show.forEach(fn => fn()); await flush(); assert.ok(ui.text().includes('登录后查看本人任务')); assert.equal(env.requests.length, 0);
  ui.click('前往登录'); assert.equal(env.navigation[0], '/pages/member/login');
  env.storage.set('token', 'test-member-one'); ui.click('重新核对登录状态'); await flush();
  assert.ok(ui.text().includes('正在读取任务记录')); env.reply(env.requests.shift(), { items: [], nextBeforeId: 0, hasMore: false }); await flush();
  assert.ok(ui.text().includes('当前账号暂无任务记录'));
  ui.click('重新查询'); env.requests.shift().fail(); await flush(); assert.ok(ui.text().includes('资料服务暂不可用'));
  ui.click('重新查询'); env.reply(env.requests.shift(), { items: [summary()], nextBeforeId: 9, hasMore: true }); await flush();
  ui.click('加载更多'); assert.ok(env.requests[0].url.endsWith('&before_id=9'));
  env.reply(env.requests.shift(), { items: [summary(8)], nextBeforeId: 0, hasMore: false }); await flush(); assert.equal(ui.state.history.length, 2);
  ui.state.loadProgress(9); env.reply(env.requests.shift(), progress(9, 2)); await flush();
  assert.equal(env.timers.size, 1); const poll = [...env.timers.values()][0]; assert.equal(poll.ms, 3000); env.timers.clear(); poll.fn(); await flush();
  assert.equal(env.requests.length, 1); assert.equal(env.timers.size, 0, '前次未完成时不重叠轮询');
  env.reply(env.requests.shift(), progress()); await flush(); assert.equal(env.timers.size, 0);
  ui.click('核对并查看报告'); assert.equal(ui.state.report, null); env.reply(env.requests.shift(), report()); await flush();
  assert.ok(ui.text().includes('测试示例题材')); assert.ok(ui.text().includes('AI 生成')); assert.ok(ui.text().includes('非备案证明'));
  ui.click('查看原文'); assert.equal(env.opened.length, 0); assert.ok(env.modals[0].content.includes('example.test'));
  env.modals.shift().success({ confirm: true }); assert.equal(env.opened[0].url, 'https://example.test/source'); assert.equal(env.opened[0].target.opener, null);
  ui.click('核对并查看报告'); await flush(); assert.ok(!ui.text().includes('本段是自动化测试示例'));
  env.reply(env.requests.shift(), null, 422, 42202); await flush(); assert.equal(ui.state.report, null); assert.ok(ui.text().includes('暂不可读取'));
  ui.state.loadProgress(9); const stale = env.requests.shift(); ui.state.loadProgress(8); const current = env.requests.shift();
  env.reply(current, progress(8)); env.reply(stale, progress(9)); await flush(); assert.equal(ui.state.progress.id, 8);
  ui.click('核对并查看报告'); const privateRequest = env.requests.shift();
  env.storage.set('token', 'test-member-two'); env.reply(privateRequest, report(8)); await flush();
  assert.equal(ui.state.report, null); assert.equal(ui.state.history.length, 0); assert.ok(!ui.text().includes('测试示例题材'));
  env.hooks.show.forEach(fn => fn()); env.reply(env.requests.shift(), { items: [summary()], nextBeforeId: 0, hasMore: false }); await flush();
  ui.state.loadProgress(9); const hiddenRequest = env.requests.shift(); env.hooks.hide.forEach(fn => fn()); env.reply(hiddenRequest, progress()); await flush();
  assert.equal(ui.state.progress, null); assert.equal(env.timers.size, 0); assert.equal(env.intervals.size, 0);
  env.hooks.show.forEach(fn => fn()); const afterHide = env.requests.shift(); env.reply(afterHide, { items: [summary()], nextBeforeId: 0, hasMore: false }); await flush();
  env.storage.delete('token'); for (const interval of env.intervals.values()) interval.fn(); await flush(); assert.equal(ui.state.history.length, 0);
  env.storage.set('token', 'test-member-three'); env.hooks.show.forEach(fn => fn());
  env.reply(env.requests.shift(), { items: [summary()], nextBeforeId: 0, hasMore: false }); await flush();
  ui.state.loadProgress(9); env.reply(env.requests.shift(), null, 401, 40100); await flush();
  assert.equal(ui.state.history.length, 0); assert.equal(ui.state.progress, null); assert.ok(ui.text().includes('登录后查看本人任务'));
  env.hooks.unload.forEach(fn => fn()); ui.app.unmount(); assert.equal(env.intervals.size, 0); assert.equal(env.timers.size, 0);
  console.log('通过：实际页面加载/空/错/分页、串行轮询、报告/原文按钮、报告不可用、乱序与跨账号响应、离页清理。');
}

async function panel() {
  const env = environment(), component = env.load('pages/ai/components/AIReportPanel.uvue').default;
  const render = value => renderToString(vue.createSSRApp({ render: () => vue.h(component, { report: value }) }));
  const safe = report(); safe.sections[0].quotes[0].text += '<script>测试转义</script>';
  const html = await render(safe); assert.ok(html.includes('&lt;script&gt;测试转义&lt;/script&gt;')); assert.ok(!html.includes('<script>测试转义</script>'));
  const bad = report(); bad.disclosure.filingNo = ''; const closed = await render(bad);
  assert.ok(closed.includes('报告格式暂不可用')); assert.ok(!closed.includes('测试示例题材'));
  const library = companyReport();
  library.sections[3].companies[0].name = '<img src=x>';
  library.sections[3].companies[0].evidence[0].text = '<img src=x>：自动化测试示例文字。';
  const companyHTML = await render(library);
  assert.ok(companyHTML.includes('&lt;img src=x&gt;')); assert.ok(!companyHTML.includes('<img src=x>'));
  assert.ok(companyHTML.includes('资料库人工审核')); assert.ok(companyHTML.includes('原文节选')); assert.ok(companyHTML.includes('未接入行情数据'));
  assert.equal((companyHTML.match(/AI 整理 · 授权资料逐字摘录/g) || []).length, 3);
  const interactive = companyReport(); interactive.sections[3].companies[0].evidence[0].sourceUrl = 'https://example.test/company-source';
  const ui = mount(component, { report: interactive });
  assert.ok(ui.text().includes('测试示例公司1（600001）')); assert.ok(ui.text().includes('归属：测试示例环节'));
  ui.click('查看原文', 3); assert.equal(env.opened.length, 0); env.modals.shift().success({ confirm: true });
  assert.equal(env.opened.length, 1); assert.equal(env.opened[0].url, 'https://example.test/company-source');
  assert.equal(env.opened[0].target.opener, null); ui.app.unmount();
  const invalid = companyReport(); invalid.sections[3].companies[0].evidence[0].sourceUrl = 'javascript:alert(1)';
  const denied = await render(invalid); assert.ok(denied.includes('报告格式暂不可用')); assert.ok(!denied.includes('测试示例公司'));
  assert.ok(!fs.readFileSync(path.join(root, 'pages/ai/components/AIReportPanel.uvue'), 'utf8').includes('v-html'));
  console.log('通过：实际报告组件 SSR 转义与缺少标识整份拒绝，不渲染自由 HTML。');
  console.log('通过：公司事实DTO正反例、三节AI与资料库人工标识、代码顺序、节选及安全原文确认；缺依据整份拒绝，不提供个股详情或行情假值。');
}

async function reportPresentation() {
  const env = environment(), rules = env.load('pages/libs/ai-report-presentation.uts');
  for (const value of [report(), companyReport()]) {
    const original = JSON.stringify(value), sources = rules.aiReportSources(value);
    assert.equal(sources.length, value.sections[3].companies ? 2 : 1);
    assert.equal(sources.reduce((total, source) => total + source.references, 0), value.sections[3].companies ? 5 : 4);
    sources[0].references = 100; assert.equal(JSON.stringify(value), original, '归集结果不得修改原DTO');
    for (const category of ['fact', 'source', 'display', 'other']) {
      const text = rules.aiReportLocator(value, category);
      assert.ok(text.includes('任务编号：9')); assert.ok(text.includes('未提交反馈'));
      for (const privateValue of [value.subjectName, value.disclosure.modelName, value.sections[0].quotes[0].text, 'https://', 'Bearer', '600001']) assert.ok(!text.includes(privateValue));
    }
    assert.equal(rules.aiReportLocator(value, 'fact\n随意字段'), '');
  }
  for (const change of [
    value => { value.sections[1].quotes[0].sourceType = '年报'; },
    value => { value.sections[1].quotes[0].collectedAt -= 1; },
    value => { value.sections[1].quotes[0].sourceUrl += '?version=2'; },
  ]) { const value = report(); change(value); assert.equal(rules.aiReportSources(value).length, 2); }
  for (const change of [value => { value.disclosure.filingNo = ''; }, value => { value.sections[0].quotes[0].sourceUrl = 'javascript:alert(1)'; }]) {
    const value = report(); change(value); assert.equal(rules.aiReportSources(value).length, 0); assert.equal(rules.aiReportLocator(value, 'fact'), '');
  }
  const value = companyReport();
  value.sections.slice(0, 3).forEach((section, index) => { section.quotes[0].text += `章节测试${index}`; });
  const props = vue.reactive({ report: value, sessionToken: '' });
  const component = env.load('pages/ai/components/AIReportPanel.uvue').default;
  const ui = mount({ render: () => vue.h(component, props) });
  ui.click('事件时间线'); await flush();
  assert.ok(ui.text().includes(value.sections[1].quotes[0].text)); assert.ok(!ui.text().includes(value.sections[0].quotes[0].text));
  assert.ok(!ui.text().includes('测试示例公司1（600001）'));
  ui.click('披露公司'); await flush(); assert.ok(ui.text().includes('测试示例公司1（600001）'));
  ui.click('来源清单'); await flush(); assert.ok(ui.text().includes('出处条目 2 项')); assert.ok(ui.text().includes('本报告引用 3 处')); assert.ok(ui.text().includes('本报告引用 2 处'));
  assert.ok(!ui.text().includes(value.sections[0].quotes[0].text)); assert.equal(env.requests.length, 0);
  ui.click('查看该出处原文'); env.modals.shift().success({ confirm: true }); assert.equal(env.opened[0].url, 'https://example.test/source');
  ui.click('全部章节'); await flush(); assert.ok(ui.text().includes(value.sections[0].quotes[0].text));
  ui.click('来源清单'); props.report = report(10); await flush();
  assert.ok(!ui.text().includes('出处条目')); assert.ok(ui.text().includes(props.report.sections[0].quotes[0].text));
  props.report.disclosure.filingNo = ''; await flush(); assert.ok(ui.text().includes('报告格式暂不可用')); assert.ok(!ui.text().includes('报告操作与反馈'));
  ui.app.unmount();
  console.log('通过：报告章节切换与出处清单，精确去重/出现顺序/引用计数，非法DTO整份拒绝；不把出处条目冒充独立来源或触发生成。');
}

async function reportTools() {
  for (const mode of ['success', 'cancel', 'copy_fail', 'modal_fail', 'unmount', 'token_changed', 'prop_token_changed', 'report_changed', 'invalid', 'late_success']) {
    const env = environment(), token = memberToken(1); env.storage.set('token', token);
    const value = companyReport(), props = vue.reactive({ report: value, sessionToken: token });
    const component = env.load('pages/ai/components/AIReportTools.uvue').default;
    const ui = mount({ render: () => vue.h(component, props) });
    assert.throws(() => ui.click('复制报告正文（未开放）'), /按钮被禁用/);
    assert.throws(() => ui.click('下载报告（未开放）'), /按钮被禁用/);
    ui.click('准备报告定位信息'); await flush();
    assert.ok(ui.text().includes('以受理记录为准'));
    ui.click('来源或授权'); await flush(); assert.ok(ui.text().includes('当前分类：来源或授权'));
    ui.click('复制定位信息（不含正文）'); await flush();
    assert.equal(env.modals.length, 1); assert.equal(env.clipboard.length, 0);
    assert.throws(() => ui.click('复制定位信息（不含正文）'), /按钮被禁用/);
    const modal = env.modals.shift(); assert.ok(modal.content.includes('系统剪贴板'));
    if (mode === 'unmount') ui.app.unmount();
    if (mode === 'token_changed') env.storage.set('token', memberToken(2));
    if (mode === 'prop_token_changed') props.sessionToken = memberToken(2);
    if (mode === 'report_changed') props.report = report(10);
    if (mode === 'invalid') props.report.disclosure.filingNo = '';
    await flush();
    if (mode === 'modal_fail') modal.fail({ errMsg: '测试内部错误不得外泄' });
    else modal.success({ confirm: mode !== 'cancel' });
    await flush();
    if (['success', 'copy_fail', 'late_success'].includes(mode)) {
      assert.equal(env.clipboard.length, 1);
      const copy = env.clipboard[0]; assert.ok(copy.data.includes('反馈分类：来源或授权'));
      for (const secret of [value.subjectName, value.disclosure.modelName, value.sections[0].quotes[0].text, 'https://', token, '600001']) assert.ok(!copy.data.includes(secret));
      if (mode === 'late_success') { props.report = report(10); await flush(); }
      if (mode === 'copy_fail') copy.fail({ errMsg: '测试内部错误不得外泄' }); else copy.success();
      await flush();
      if (mode === 'success') assert.ok(ui.text().includes('定位信息已复制；反馈尚未提交。'));
      if (mode === 'copy_fail') assert.ok(ui.text().includes('复制失败'));
      if (mode === 'late_success') assert.ok(!ui.text().includes('定位信息已复制'));
    } else assert.equal(env.clipboard.length, 0, mode);
    assert.equal(env.requests.length, 0); assert.equal(env.storage.size, 1); assert.ok(!ui.text().includes('测试内部错误不得外泄'));
    if (mode !== 'unmount') ui.app.unmount();
  }
  const env = environment(), component = env.load('pages/ai/components/AIReportTools.uvue').default;
  const ui = mount(component, { report: report() }); ui.click('准备报告定位信息'); await flush();
  assert.throws(() => ui.click('复制定位信息（不含正文）'), /按钮被禁用/); ui.app.unmount();
  console.log('通过：反馈准备与元数据复制，明确无受理/无正文授权；二次确认、重复点击、失败、换报告/账号、离页与迟到回调隔离，不上传或持久保存草稿。');
}

const feedbackReceipt = (category = 'fact') => ({ id: 1, taskId: 9, category, status: 'received', resolution: '', version: 1, createdAt: now, updatedAt: now });
async function feedbackContracts() {
  const env = environment(), r = env.load('pages/libs/ai-feedback.uts'), { AiFeedbackApi } = env.load('pages/libs/ai-feedback-api.uts');
  assert.equal(r.isValidAIFeedback(feedbackReceipt(), 9), true);
  for (const mutate of [v => v.id = '1', v => v.taskId = '9', v => v.taskId = 8, v => v.version = '1', v => v.category = 'FACT', v => v.status = ['received'], v => v.resolution = 'reviewed', v => v.updatedAt--, v => v.updatedAt = 9007199254740992, v => v.status = 'deleted']) {
    const v = feedbackReceipt(); mutate(v); assert.equal(r.isValidAIFeedback(v, 9), false);
  }
  for (const [status, resolution, version] of [['processing', '', 2], ['resolved', 'reviewed', 3], ['resolved', 'referred', 3], ['dismissed', 'unconfirmed', 3], ['dismissed', 'not_applicable', 3]]) {
    const v = { ...feedbackReceipt(), status, resolution, version }; assert.equal(r.isValidAIFeedback(v, 9), true); assert.ok(r.aiFeedbackStatus(v)); assert.ok(r.aiFeedbackResolution(v));
  }
  for (const value of [null, { taskId: '9', items: [] }, { taskId: 9, items: [feedbackReceipt(), feedbackReceipt()] }, { taskId: 9, items: Array(5).fill(feedbackReceipt()) }]) assert.equal(r.isValidAIFeedbackList(value, 9), false);
  assert.equal(r.isValidAIFeedbackList({ taskId: 9, items: [] }, 9), true);
  for (const id of [0, -1, 1.1, 9007199254740992]) assert.notEqual((await AiFeedbackApi.list(id)).code, 0);
  assert.notEqual((await AiFeedbackApi.submit(9, 'arbitrary')).code, 0); assert.equal(env.requests.length, 0);
  env.storage.set('token', memberToken(1));
  const promise = AiFeedbackApi.submit(9, 'fact'), req = env.requests.shift();
  assert.equal(req.url, '/api/v1/ai/tasks/9/feedback'); assert.equal(req.method, 'POST'); assert.equal(JSON.stringify(req.data), '{"category":"fact"}');
  env.reply(req, feedbackReceipt('source')); assert.notEqual((await promise).code, 0, '回包分类必须匹配原提交');
  const list = AiFeedbackApi.list(9), get = env.requests.shift(); assert.equal(get.method, 'GET'); env.reply(get, { taskId: 9, items: [feedbackReceipt()] }); assert.equal((await list).code, 0);
  console.log('通过：反馈DTO闭集、归属/版本/时点/分类唯一性校验与固定GET/POST契约，不开放内部处理或自由文本。');
}

async function feedbackPanel() {
  for (const mode of ['success', 'unknown', 'malformed', 'cancel', '401', 'switch', 'unmount', 'late_get', 'get_error']) {
    const env = environment(), token = memberToken(1); env.storage.set('token', token);
    const props = vue.reactive({ taskId: 9, sessionToken: token }); const component = env.load('pages/ai/components/AIReportFeedbackPanel.uvue').default;
    const ui = mount({ render: () => vue.h(component, props) }); assert.equal(env.requests.length, 0);
    ui.click('举报与反馈'); await flush(); const get = env.requests.shift(); assert.equal(get.method, 'GET');
    if (mode === 'late_get') { props.taskId = 10; await flush(); }
    if (mode === 'get_error') env.reply(get, null, 503, 50306); else env.reply(get, { taskId: 9, items: [] }); await flush();
    if (['late_get', 'get_error'].includes(mode)) { assert.ok(!ui.text().includes('本报告暂无反馈记录')); ui.app.unmount(); continue; }
    ui.click('来源或授权'); await flush(); ui.click('确认反馈分类'); await flush(); assert.equal(env.requests.length, 0); assert.equal(env.modals.length, 1);
    assert.throws(() => ui.click('确认反馈分类'), /按钮被禁用/);
    env.modals.shift().success({ confirm: mode !== 'cancel' }); await flush();
    if (mode === 'cancel') { assert.equal(env.requests.length, 0); ui.app.unmount(); continue; }
    const post = env.requests.shift(); assert.equal(post.method, 'POST'); assert.equal(JSON.stringify(post.data), '{"category":"source"}');
    if (mode === 'switch') { env.storage.set('token', memberToken(2)); props.sessionToken = memberToken(2); await flush(); }
    if (mode === 'unmount') ui.app.unmount();
    if (mode === 'unknown') post.fail(); else if (mode === 'malformed') env.reply(post, feedbackReceipt('other')); else if (mode === '401') env.reply(post, null, 401, 40100); else env.reply(post, feedbackReceipt('source'));
    await flush(); assert.equal(env.requests.length, 0, '不得自动重发或查询'); assert.equal(env.storage.size, 1, '反馈草稿不得持久化');
    if (['unknown', 'malformed'].includes(mode)) {
      assert.ok(ui.text().includes('提交结果未确认')); assert.throws(() => ui.click('确认反馈分类'), /按钮被禁用/);
      ui.click('查询反馈状态'); const query = env.requests.shift(); assert.equal(query.method, 'GET'); env.reply(query, { taskId: 9, items: [feedbackReceipt('source')] }); await flush(); assert.ok(ui.text().includes('未重复提交'));
    }
    if (mode === 'success' || mode === 'unknown' || mode === 'malformed') {
      assert.ok(ui.text().includes('受理单 #1')); ui.click('确认反馈分类'); await flush(); assert.equal(env.requests.length, 0); assert.ok(ui.text().includes('无需重复提交'));
      ui.click('查询反馈状态'); env.reply(env.requests.shift(), { taskId: 9, items: [{ ...feedbackReceipt('source'), version: 3, status: 'resolved', resolution: 'reviewed' }] }); await flush();
      assert.ok(ui.text().includes('已处理')); assert.ok(ui.text().includes('不代表报告已更正'));
    }
    if (mode === 'switch' || mode === '401') assert.ok(!ui.text().includes('受理单 #1'));
    assert.ok(!ui.text().includes('测试内部文字不得展示'));
    if (mode !== 'unmount') ui.app.unmount();
  }
  console.log('通过：反馈确认提交→受理单→查询处理结果、自然幂等、未知先查、取消/401/换账号/离页/迟到回包隔离；无自动POST或存储。');
}

function entry() {
  const app = compiler.parse(fs.readFileSync(path.join(root, 'App.uvue'), 'utf8'), { filename: 'App.uvue' });
  assert.deepEqual(app.errors, []);
  const globalStyle = app.descriptor.styles.filter(style => !style.scoped).map(style => style.content).join('\n');
  assert.ok(globalStyle.includes('uni_modules/tmx-ui/scss/uvuePx.scss'), '实际页面工具类必须接入全局样式，不能由测试宿主补造');
  assert.ok(globalStyle.includes('uni_modules/tmx-ui/scss/remixicon.css'), '实际导航图标必须接入本地字体，不能只通过标签桩');
  for (const file of ['pages/ai/index.uvue', 'pages/ai/components/AICreatePanel.uvue', 'pages/ai/components/AICancelPanel.uvue']) {
    const source = fs.readFileSync(path.join(root, file), 'utf8');
    for (const button of source.matchAll(/<x-button\b[^>]*>/g)) {
      assert.ok(button[0].includes(':block="true"'), `${file} 的任务操作按钮不能沿用98px默认宽度截断确认文案`);
    }
  }
  const filename = 'pages/member/index.uvue';
  const parsed = compiler.parse(fs.readFileSync(path.join(root, filename), 'utf8'), { filename });
  assert.deepEqual(parsed.errors, []); const script = compiler.compileScript(parsed.descriptor, { id: 'ai-member-entry' });
  const template = compiler.compileTemplate({ source: parsed.descriptor.template.content, filename, id: 'ai-member-entry', compilerOptions: { bindingMetadata: script.bindings } });
  assert.deepEqual(template.errors, []);
  const walk = node => [node, ...(node.children || []).flatMap(walk)];
  const cell = walk(parsed.descriptor.template.ast).find(node => node.tag === 'x-cell' && node.props.some(prop => prop.name === 'url' && prop.value?.content === '/pages/ai/index'));
  assert.ok(cell, '会员页必须有实际资料入口');
  assert.ok(cell.props.some(prop => prop.name === 'min-height' && Number(prop.value?.content) >= 44));
  const pages = JSON.parse(fs.readFileSync(path.join(root, 'pages.json'), 'utf8')).pages;
  assert.ok(pages.some(p => p.path === 'pages/ai/index' && p.style.navigationStyle === 'custom'));
  const page = fs.readFileSync(path.join(root, 'pages/ai/index.uvue'), 'utf8');
  assert.ok(!page.includes(':show-nav-back="false"'), '非tab页面不得去掉返回导航');
  console.log('通过：全局工具类/本地图标接线、任务操作按钮宽度、会员入口SFC/路由注册/最小点击高度及返回导航契约；本脚本未执行x-cell实现。');
}

async function submissionContracts() {
  const env = environment(), rules = env.load('pages/libs/ai-submission.uts'), { AiSubmissionApi: api } = env.load('pages/libs/ai-submission-api.uts');
  const scope = { analysisType: 'light', subjectType: 'theme', subjectId: 7 }, token = memberToken(42);
  env.storage.set('token', token);
  assert.equal(rules.isValidAIQuote(quote(), scope), true);
  assert.equal(rules.isAIRequestKey('valid_test_key_001\n'), false); assert.equal(rules.isAIPriceVersion('test-v1\n'), false);
  for (const bad of [{ pricePoints: 0 }, { pricePoints: '7' }, { availablePoints: -1 }, { subjectId: 8 }, { analysisType: 'deep' }, { priceVersion: 'x'.repeat(33) }, { holdExpireSec: 0 }, { quotedAt: 9007199254740992 }, { materialScope: [] }, { materialScope: [{ ...quote().materialScope[0], sourceType: 'unknown' }] }, { materialScope: [{ ...quote().materialScope[0], sourceUrl: 'javascript:alert(1)' }] }, { materialScope: [{ ...quote().materialScope[0], collectedAt: now + 1 }] }]) {
    assert.equal(rules.isValidAIQuote({ ...quote(), ...bad }, scope), false, JSON.stringify(bad));
  }
  for (const category of ['announcement', 'annual_report', 'prospectus', 'official_catalog', 'interactive_qa']) {
    const value = quote(); value.materialScope[0].sourceType = category; assert.equal(rules.isValidAIQuote(value, scope), true);
  }
  for (const value of [null, { ...receipt(), taskId: '9' }, { ...receipt(), taskId: 9007199254740992 }, { ...receipt(), priceVersion: '' }]) assert.equal(rules.isValidAICreationReceipt(value), false);
  assert.equal(rules.aiPendingStorageKey(token), 'tm_ai_pending_v1_42');
  for (const value of ['', 'opaque-test-token', memberToken(0), memberToken(9007199254740992)]) assert.equal(rules.aiPendingStorageKey(value), '');
  const partition = rules.aiPendingStorageKey(token);
  const originals = await Promise.all([rules.reserveAIPending(partition, quote()), rules.reserveAIPending(partition, quote())]);
  assert.equal(originals.filter(Boolean).length, 1, '跨页签互斥只能生成一份待核查意图');
  const original = originals.find(Boolean);
  assert.equal(rules.isAIRequestKey(original.key), true); assert.equal(rules.readAIPending(partition).key, original.key);
  assert.ok(!JSON.stringify([...env.storage]).includes('Bearer'));
  let promise = api.quote(scope), request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/tasks/quote'); assert.equal(request.method, 'POST');
  assert.deepEqual(JSON.parse(JSON.stringify(request.data)), { analysis_type: 'light', subject_type: 'theme', subject_id: 7 });
  assert.equal(request.header['Idempotency-Key'], undefined); env.reply(request, quote()); assert.equal((await promise).code, 0);
  assert.equal((await api.quote({ ...scope, analysisType: 'deep' })).code, 40001); assert.equal(env.requests.length, 0);
  promise = api.create(original); request = env.requests.shift(); assert.equal(request.header.Authorization, 'Bearer ' + token);
  assert.equal(request.header['Idempotency-Key'], original.key); assert.equal(request.url, '/api/v1/ai/tasks');
  assert.deepEqual(JSON.parse(JSON.stringify(request.data)), { analysis_type: 'light', subject_type: 'theme', subject_id: 7, price_version: 'test-v1' });
  request.fail(); assert.equal((await promise).data, null); assert.equal(env.requests.length, 0, '未知不自动重发');
  promise = api.receipt(original.key); request = env.requests.shift(); assert.equal(request.method, 'GET'); assert.equal(request.url, '/api/v1/ai/task-receipt');
  assert.equal(request.header['Idempotency-Key'], original.key); assert.equal(JSON.stringify(request.data), '{}'); env.reply(request, receipt()); assert.equal((await promise).data.taskId, 9);
  assert.equal(await rules.clearConfirmedAIPending(partition, 'different_test_key_001'), false); assert.equal(rules.readAIPending(partition).key, original.key);
  assert.equal(await rules.clearConfirmedAIPending(partition, original.key), true);
  env.globals.window.crypto = null; assert.equal(await rules.reserveAIPending(partition, quote()), null); env.globals.window.crypto = webcrypto;
  env.globals.window.navigator.locks = null; assert.equal(await rules.reserveAIPending(partition, quote()), null);
  for (const invalid of ['损坏的原请求', 0, false, {}, 'null', '[]']) { env.storage.set(partition, invalid); assert.throws(() => rules.readAIPending(partition)); }
  console.log('通过：创建DTO/固定契约/来源闭集、仅头部原键、安全随机与跨页签互斥、持久保存及原键核查，缺能力失败关闭。');
}

async function submissionPage() {
  const env = environment(), token = memberToken(42), partition = 'tm_ai_pending_v1_42', created = [];
  env.storage.set('token', token);
  let ui = mount(env.load('pages/ai/components/AICreatePanel.uvue').default, { sessionToken: token, onCreated: id => created.push(id) });
  const prepareQuote = async value => {
    ui.input('测试题材'); ui.click('查询题材');
    const search = env.requests.shift(); assert.equal(search.url, '/api/v1/theme/search');
    env.reply(search, [{ id: 7, name: '测试示例题材', description: '自动化测试，非实际资料' }]); await flush();
    ui.click('核对该题材报价'); env.reply(env.requests.shift(), value); await flush();
  };
  await prepareQuote({ ...quote(), availablePoints: 6 }); assert.ok(ui.text().includes('可消费积分不足')); ui.state.confirmCreate(); assert.equal(env.modals.length, 0);
  await prepareQuote(quote()); ui.click('确认报价并创建'); assert.equal(env.requests.length, 0); assert.ok(env.modals[0].content.includes('冻结 7 积分'));
  ui.state.confirmCreate(); assert.equal(env.modals.length, 1); env.modals.shift().success({ confirm: false }); await flush(); assert.equal(env.storage.has(partition), false);
  ui.click('确认报价并创建'); env.modals.shift().success({ confirm: true }); await flush();
  const request = env.requests.shift(), original = JSON.parse(env.storage.get(partition));
  assert.equal(request.header['Idempotency-Key'], original.key); assert.equal(env.requests.length, 0); ui.state.confirmCreate(); assert.equal(env.modals.length, 0);
  request.fail(); await flush(); assert.equal(ui.state.state, 'unknown'); assert.ok(ui.text().includes('原请求已保留'));
  assert.equal(env.requests.length, 0); const saved = env.storage.get(partition); assert.ok(!saved.includes(token));
  ui.app.unmount();
  // 新模块/新页面模拟重载；同会员换JWT仍只能核查原键，没有隐式POST。
  const renewed = memberToken(42).replace(/^test/, 'renewed'); env.storage.set('token', renewed);
  ui = mount(env.load('pages/ai/components/AICreatePanel.uvue').default, { sessionToken: renewed, onCreated: id => created.push(id) });
  assert.equal(ui.state.state, 'unknown'); assert.equal(env.requests.length, 0);
  ui.click('核查原请求'); let check = env.requests.shift(); assert.equal(check.header['Idempotency-Key'], original.key);
  env.reply(check, null, 404, 40403); await flush(); assert.equal(env.storage.get(partition), saved); assert.equal(ui.state.state, 'unknown');
  assert.ok(ui.text().includes('不代表此前请求不会提交'));
  ui.click('核查原请求'); check = env.requests.shift(); env.reply(check, { ...receipt(), pricePoints: 8 }); await flush(); assert.equal(ui.state.receipt, null);
  ui.click('核查原请求'); env.reply(env.requests.shift(), receipt()); await flush(); assert.equal(ui.state.state, 'created');
  assert.ok(ui.text().includes('不代表报告已交付')); ui.click('查看已创建任务'); assert.deepEqual(created, [9]);
  ui.click('已记录任务，返回新建'); await flush(); assert.equal(env.storage.has(partition), false); assert.equal(ui.state.state, 'ready');
  await prepareQuote(quote()); ui.click('确认报价并创建'); env.modals.shift().success({ confirm: true }); await flush();
  env.reply(env.requests.shift(), null, 409, 40901); await flush(); assert.equal(ui.state.state, 'ready'); assert.equal(env.storage.has(partition), false);
  assert.ok(ui.text().includes('价格版本已变化')); assert.equal(env.requests.length, 0);
  await prepareQuote(quote()); ui.click('确认报价并创建'); const staleModal = env.modals.shift(); env.storage.set('token', memberToken(99)); staleModal.success({ confirm: true }); await flush();
  assert.equal(env.requests.length, 0); assert.equal(env.storage.has(partition), false); ui.app.unmount();
  // 他人待核查记录不进入当前页；存储配额故障不得发送创建。
  env.storage.set(partition, saved); const otherToken = memberToken(99); env.storage.set('token', otherToken);
  ui = mount(env.load('pages/ai/components/AICreatePanel.uvue').default, { sessionToken: otherToken });
  assert.equal(ui.state.pending, null); assert.ok(!ui.text().includes('任务 #9')); assert.equal(env.requests.length, 0);
  await prepareQuote(quote()); env.uni.setStorageSync = () => { throw new Error('测试存储配额故障'); };
  ui.click('确认报价并创建'); env.modals.shift().success({ confirm: true }); await flush(); assert.equal(env.requests.length, 0); assert.equal(env.storage.get(partition), saved); ui.app.unmount();
  console.log('通过：实际创建组件的搜索/报价/余额/二次确认、重复点击、超时保键、重载/续签核查、404不换键、收据不是交付、变价、跨账号与存储故障。');
}

async function memberFlow() {
  const env = environment(); env.storage.set('token', memberToken(42));
  const ui = mount(env.load('pages/ai/index.uvue').default);
  env.hooks.show.forEach(fn => fn()); env.reply(env.requests.shift(), { items: [], nextBeforeId: 0, hasMore: false }); await flush();
  ui.input('测试题材'); ui.click('查询题材'); env.reply(env.requests.shift(), [{ id: 7, name: '测试示例题材', description: '自动化测试示例' }]); await flush();
  ui.click('核对该题材报价'); env.reply(env.requests.shift(), quote()); await flush();
  ui.click('确认报价并创建'); env.modals.shift().success({ confirm: true }); await flush();
  env.reply(env.requests.shift(), receipt()); await flush(); ui.click('查看已创建任务');
  const createdReads = env.requests.splice(0), task = createdReads.find(r => r.url === '/api/v1/ai/tasks/9');
  const refreshedHistory = createdReads.find(r => r.url === '/api/v1/ai/tasks?limit=20');
  assert.ok(refreshedHistory, '确认创建后必须查询真实历史，不能继续显示暂无任务或用创建收据补造摘要');
  env.reply(refreshedHistory, { items: [summary(9, 1)], nextBeforeId: 0, hasMore: false });
  assert.ok(task); env.reply(task, progress(9, 2)); await flush();
  assert.equal(ui.state.state, 'ready'); assert.equal(ui.state.history[0].id, 9);
  assert.ok(!ui.text().includes('当前账号暂无任务记录'));
  assert.equal(ui.state.history[0].status, 1, '真实历史快照不能被详情或收据改写');
  assert.equal(ui.state.historyStatusLabel(summary(9, 1)), '资料处理中（当前核对）');
  assert.equal(ui.state.historyStatusLabel(summary(8, 1)), '排队中（历史查询）');
  assert.equal(ui.state.history[0].reportAvailable, false, '列表不得因详情而获得报告许可');
  const poll = [...env.timers.values()][0]; assert.ok(poll); env.timers.clear(); poll.fn(); await flush();
  env.reply(env.requests.shift(), progress()); await flush(); ui.click('核对并查看报告'); env.reply(env.requests.shift(), report()); await flush();
  assert.ok(ui.text().includes('本段是自动化测试示例')); assert.equal(ui.state.report.taskId, 9);
  assert.equal(env.requests.length, 0); assert.equal(env.timers.size, 0);
  env.hooks.hide.forEach(fn => fn()); await flush(); assert.equal(ui.state.report, null);
  assert.ok(env.storage.has('tm_ai_pending_v1_42'), '离页不能删除已发请求的原键');
  env.hooks.show.forEach(fn => fn()); env.reply(env.requests.shift(), { items: [summary()], nextBeforeId: 0, hasMore: false }); await flush();
  ui.click('核查原请求'); env.reply(env.requests.shift(), null, 401, 40100); await flush();
  assert.equal(ui.state.state, 'guest'); assert.equal(ui.state.history.length, 0); assert.equal(ui.state.report, null);
  assert.ok(env.storage.has('tm_ai_pending_v1_42'), '401清显示，不销毁仍需核查的原键');
  env.hooks.unload.forEach(fn => fn()); ui.app.unmount();
  console.log('通过：实际会员页嵌套创建组件→进度轮询→报告展示、离页保键及创建核查401清私有状态；接口均为显著测试响应。');
}

async function creationHistoryFlow() {
  for (const mode of ['late_initial', 'late_more', 'history_error', 'detail_error', 'history_401', 'account', 'hide', 'twice']) {
    const env = environment(); env.storage.set('token', memberToken(42));
    const ui = mount(env.load('pages/ai/index.uvue').default);
    env.hooks.show.forEach(fn => fn()); const initial = env.requests.shift(); let stale = null; await flush();
    if (mode === 'late_initial') stale = initial;
    else {
      env.reply(initial, mode === 'late_more' ? { items: [summary(8, 1)], nextBeforeId: 8, hasMore: true } : { items: [], nextBeforeId: 0, hasMore: false }); await flush();
      if (mode === 'late_more') { ui.click('加载更多'); stale = env.requests.shift(); }
    }
    ui.input('测试题材'); ui.click('查询题材'); env.reply(env.requests.shift(), [{ id: 7, name: '测试示例题材', description: '自动化测试示例' }]); await flush();
    ui.click('核对该题材报价'); env.reply(env.requests.shift(), quote()); await flush();
    ui.click('确认报价并创建'); env.modals.shift().success({ confirm: true }); await flush(); env.reply(env.requests.shift(), receipt()); await flush();
    ui.click('查看已创建任务'); let reads = env.requests.splice(0);
    assert.equal(reads.length, 2); assert.ok(reads.every(r => r.method === 'GET'), '接收创建收据后只允许读，不发送第二次创建');
    let list = reads.find(r => r.url === '/api/v1/ai/tasks?limit=20'), detail = reads.find(r => r.url === '/api/v1/ai/tasks/9');
    assert.ok(list && detail);
    if (mode === 'twice') {
      ui.click('查看已创建任务'); const newer = env.requests.splice(0);
      env.reply(list, { items: [], nextBeforeId: 0, hasMore: false }); env.reply(detail, progress(9, 2)); await flush();
      assert.equal(ui.state.progress, null); assert.equal(ui.state.state, 'loading');
      list = newer.find(r => r.url === '/api/v1/ai/tasks?limit=20'); detail = newer.find(r => r.url === '/api/v1/ai/tasks/9');
    }
    if (mode === 'account') env.storage.set('token', memberToken(99));
    if (mode === 'hide') env.hooks.hide.forEach(fn => fn());
    if (mode === 'history_401') env.reply(list, null, 401, 40100);
    else if (mode === 'history_error') list.fail();
    else env.reply(list, { items: [summary(9, 1)], nextBeforeId: 0, hasMore: false });
    if (mode === 'detail_error') detail.fail(); else env.reply(detail, progress(9, 2));
    if (stale) env.reply(stale, { items: [summary(7, 1)], nextBeforeId: 0, hasMore: false });
    await flush();
    if (['history_401', 'account', 'hide'].includes(mode)) {
      assert.equal(ui.state.history.length, 0); assert.equal(ui.state.progress, null); assert.equal(ui.state.report, null); assert.equal(env.timers.size, 0);
    } else {
      assert.equal(ui.state.selectedId, 9); assert.equal(ui.state.hasMore, false); assert.equal(ui.state.report, null);
      if (mode === 'history_error') { assert.equal(ui.state.state, 'error'); assert.equal(ui.state.history.length, 0); assert.equal(ui.state.progress.id, 9); }
      else { assert.equal(ui.state.state, 'ready'); assert.equal(ui.state.history.length, 1); assert.equal(ui.state.history[0].id, 9); }
      if (mode === 'detail_error') { assert.equal(ui.state.progress, null); assert.equal(env.timers.size, 0); }
      else { assert.equal(ui.state.progress.status, 2); assert.equal(ui.state.progress.reportAvailable, false); }
      assert.ok(!ui.text().includes('当前账号暂无任务记录'));
    }
    assert.ok(env.storage.has('tm_ai_pending_v1_42'), '历史或详情异常不能清除已发送请求的原键');
    assert.equal(env.requests.length, 0, '刷新不得触发创建、自动重试或取得报告正文');
    env.hooks.unload.forEach(fn => fn()); ui.app.unmount(); assert.equal(env.timers.size, 0);
  }
  console.log('通过：创建收据后刷新真实历史与独立详情，旧首页/分页/重复查看迟到丢弃，历史/详情故障、401/换账号/离页不补创建或报告许可。');
}

// 取消响应均为测试数据，既不冒充释放凭据，也不触发真实后台或模型。
const cancellation = (status = 2, extra = {}) => ({ id: 9, status, eventSeq: 9, cancelRequested: status === 2,
  errorCode: status === 5 ? 'cancelled' : status === 6 ? 'expired' : '', reportAvailable: false, ...extra });

async function cancellationContracts() {
  const env = environment(), rules = env.load('pages/libs/ai-cancel-api.uts');
  const { AiCancellationApi: api } = rules, { MemberApi } = env.load('pages/libs/member-api.uts');
  for (const value of [cancellation(), cancellation(3), cancellation(4, { errorCode: 'guard_rejected' }), cancellation(5), cancellation(6)]) {
    assert.equal(rules.isValidAITaskCancellation(value, 9), true);
  }
  for (const value of [null, cancellation(1), cancellation(2, { cancelRequested: false }), cancellation(4), cancellation(5, { errorCode: '' }),
    cancellation(3, { reportAvailable: true }), cancellation(3, { errorCode: '内部文字' }), cancellation(2, { id: '9' }), cancellation(2, { id: 8 }),
    cancellation(2, { eventSeq: 9007199254740992 }), cancellation(2, { cancelRequested: 'true' }), cancellation(2, { status: '2' })]) {
    assert.equal(rules.isValidAITaskCancellation(value, 9), false, JSON.stringify(value));
  }
  env.storage.set('token', 'test-cancellation-current'); env.storage.set('tm_member_token', 'test-cancellation-unused');
  for (const id of [0, -1, '9', NaN, 9007199254740992, 'https://example.test']) {
    assert.equal((await api.cancel(id)).data, null); assert.equal((await MemberApi.aiCancel(id)).data, null);
  }
  assert.equal(env.requests.length, 0);
  let promise = api.cancel(9), req = env.requests.shift();
  assert.equal(req.url, '/api/v1/ai/tasks/9/cancel'); assert.equal(req.method, 'POST');
  assert.equal(req.header.Authorization, 'Bearer test-cancellation-current'); assert.equal(req.header['Content-Type'], 'application/json');
  assert.equal(req.header['Idempotency-Key'], undefined); assert.equal(JSON.stringify(req.data), '{}'); assert.equal(req.timeout, 10000);
  env.reply(req, cancellation()); assert.equal((await promise).code, 0);
  for (const reply of [r => r.fail(), r => env.reply(r, cancellation(), 503, 50303),
    r => env.reply(r, cancellation(), 503, 0), r => env.reply(r, cancellation(3, { reportAvailable: true })),
    r => r.success({ statusCode: 200, data: '<html>测试内部故障</html>' })]) {
    promise = api.cancel(9); req = env.requests.shift(); reply(req);
    const result = await promise; assert.equal(result.data, null); assert.ok(!result.message.includes('内部'));
    await flush(); assert.equal(env.requests.length, 0, '取消错误不能自动重发');
  }
  promise = api.cancel(9); env.reply(env.requests.shift(), null, 401, 40100);
  assert.equal((await promise).code, 40100);
  console.log('通过：实际取消DTO闭集、固定空JSON契约、当前会员令牌、错误脱敏及无自动重试；回执不能授予报告或退款许可。');
}

async function cancellationPanel() {
  for (const mode of ['dismiss', 'running', 'queued', 'success_race', 'unknown', 'closed', 'invalid', '401', 'task_change', 'account_change', 'hidden', 'terminal', 'already_requested', 'no_progress']) {
    const env = environment(), token = memberToken(42), events = { pause: [], refresh: [], unauthorized: 0 };
    env.storage.set('token', token);
    const props = vue.reactive({ id: 9, sessionToken: token, task: progress(9, mode === 'terminal' ? 3 : mode === 'queued' ? 1 : 2) });
    if (mode === 'already_requested') props.task.cancelRequested = true;
    if (mode === 'no_progress') props.task = null;
    const component = env.load('pages/ai/components/AICancelPanel.uvue').default;
    // HBuilderX 5.26 的 Vue 声明不接受运行时类型数组中的 null，不能只验源码能渲染。
    assert.ok(!Array.isArray(component.props.task.type) || !component.props.task.type.includes(null), '取消组件不能生成 [Object, null] 属性类型');
    assert.equal(component.props.task.default, null, '暂无进度必须显式保留 null，不造任务或变成必填对象');
    assert.notEqual(component.props.task.required, true, '可空进度不能触发必填对象校验');
    const ui = mount({ render: () => vue.h(component, { ...props, onPause: x => events.pause.push(x), onRefresh: id => events.refresh.push(id), onUnauthorized: () => events.unauthorized++ }) });
    await flush();
    if (['terminal', 'already_requested', 'no_progress'].includes(mode)) {
      assert.ok(!ui.text().includes('申请取消任务')); assert.equal(env.requests.length, 0); ui.app.unmount(); continue;
    }
    ui.click('申请取消任务'); await flush();
    assert.ok(ui.text().includes('确认申请取消任务 #9')); assert.equal(env.requests.length, 0); assert.equal(events.pause.at(-1), true);
    ui.click('保留任务'); await flush(); assert.equal(env.requests.length, 0); assert.equal(events.pause.at(-1), false);
    if (mode === 'dismiss') { ui.app.unmount(); continue; }
    ui.click('申请取消任务'); await flush(); ui.click('确认申请取消');
    const child = ui.app._instance.subTree.component.setupState;
    child.confirmCancel(); child.beginCancel(); await flush(); assert.equal(env.requests.length, 1, '重复点击不得重复POST');
    const req = env.requests.shift(); assert.equal(req.method, 'POST');
    if (mode === 'task_change') { props.id = 8; props.task = progress(8, 2); await flush(); }
    if (mode === 'account_change') { env.storage.set('token', memberToken(43)); props.sessionToken = memberToken(43); await flush(); }
    if (mode === 'hidden') ui.app.unmount();
    if (mode === 'unknown') req.fail();
    else if (mode === 'closed') env.reply(req, null, 503, 50303);
    else if (mode === 'invalid') env.reply(req, cancellation(3, { reportAvailable: true }));
    else if (mode === '401') env.reply(req, null, 401, 40100);
    else env.reply(req, cancellation(mode === 'queued' ? 5 : mode === 'success_race' ? 3 : 2));
    await flush(); assert.equal(env.requests.length, 0);
    if (['task_change', 'account_change', 'hidden'].includes(mode)) {
      assert.equal(events.unauthorized, 0); assert.equal(events.refresh.length, 0);
      assert.ok(!ui.text().includes('取消请求已登记')); if (mode !== 'hidden') ui.app.unmount(); continue;
    }
    if (mode === '401') { assert.equal(events.unauthorized, 1); ui.app.unmount(); continue; }
    assert.equal(events.pause.at(-1), false);
    if (['unknown', 'closed', 'invalid'].includes(mode)) assert.ok(ui.text().includes('取消结果尚未确认'));
    else if (mode === 'running') assert.ok(ui.text().includes('尚不表示远端已停止或积分已释放'));
    else assert.ok(ui.text().includes('不以取消回执承诺退款或改写已有结果'));
    assert.ok(!ui.text().includes('确认申请取消任务 #9')); assert.ok(!ui.text().includes('申请取消任务'));
    ui.click('核对原任务状态'); assert.equal(events.refresh.at(-1), 9); assert.equal(env.requests.length, 0);
    props.task = { ...progress(9, mode === 'success_race' ? 3 : 5), errorCode: mode === 'success_race' ? '' : 'cancelled' }; await flush();
    assert.ok(ui.text().includes(mode === 'success_race' ? '任务已完成' : '任务已取消'));
    assert.ok(!ui.text().includes('取消结果尚未确认')); ui.app.unmount();
  }
  console.log('通过：实际取消组件二次确认、重复点击、排队/运行/成功竞争、未知/关闭/畸形/401与切任务/账号/离页的迟到响应隔离。');
}

async function cancellationMemberFlow() {
  for (const mode of ['running', 'unknown', '401', 'switch', 'hide']) {
    const env = environment(); env.storage.set('token', memberToken(42));
    const ui = mount(env.load('pages/ai/index.uvue').default);
    env.hooks.show.forEach(fn => fn()); env.reply(env.requests.shift(), { items: [summary(9, 2)], nextBeforeId: 0, hasMore: false }); await flush();
    ui.state.loadProgress(9); env.reply(env.requests.shift(), progress(9, 2)); await flush(); assert.equal(env.timers.size, 1);
    ui.click('申请取消任务'); await flush(); assert.equal(env.timers.size, 0); assert.equal(ui.state.cancelInteracting, true); assert.equal(ui.state.feedMode,'paused');
    ui.state.retryDetail(); assert.equal(env.requests.length, 0, '确认期间不能重入同一详情请求');
    ui.click('保留任务'); await flush(); assert.equal(env.timers.size, 1);
    ui.click('申请取消任务'); await flush(); ui.click('确认申请取消'); await flush();
    const req = env.requests.shift(); assert.equal(req.url, '/api/v1/ai/tasks/9/cancel'); assert.equal(env.timers.size, 0);
    if (mode === 'switch') { ui.state.loadProgress(8); env.reply(env.requests.shift(), progress(8, 2)); await flush(); }
    if (mode === 'hide') { env.hooks.hide.forEach(fn => fn()); await flush(); }
    if (mode === '401') env.reply(req, null, 401, 40100); else if (mode === 'unknown') req.fail(); else env.reply(req, cancellation());
    await flush(); assert.equal(env.requests.length, 0);
    if (mode === '401' || mode === 'hide') {
      assert.equal(ui.state.progress, null); assert.equal(ui.state.history.length, 0); assert.equal(env.timers.size, 0);
    } else if (mode === 'switch') {
      assert.equal(ui.state.progress.id, 8); assert.ok(!ui.text().includes('取消请求已登记')); assert.equal(env.timers.size, 1);
    } else {
      assert.equal(ui.state.progress.cancelRequested, false, '取消DTO不能冒充原任务GET');
      ui.click('核对原任务状态'); const get = env.requests.shift(); assert.equal(get.method, 'GET'); assert.equal(get.url, '/api/v1/ai/tasks/9');
      env.reply(get, { ...progress(9, 2), eventSeq: 9, cancelRequested: true }); await flush();
      assert.equal(ui.state.progress.cancelRequested, true); assert.equal(env.timers.size, 1);
      const poll = [...env.timers.values()][0]; env.timers.clear(); poll.fn(); await flush();
      env.reply(env.requests.shift(), { ...progress(9, 5), eventSeq: 10, errorCode: 'cancelled', cancelRequested: true }); await flush();
      assert.equal(ui.state.progress.status, 5); assert.equal(env.timers.size, 0); assert.ok(ui.text().includes('原任务当前状态：任务已取消'));
      assert.ok(!ui.text().includes('取消请求已登记，等待后台确认'), '不可逆取消标志留存不代表终态仍待确认');
    }
    env.hooks.unload.forEach(fn => fn()); ui.app.unmount(); assert.equal(env.timers.size, 0); assert.equal(env.intervals.size, 0);
  }
  console.log('通过：实际会员页嵌套取消组件暂停/恢复串行轮询、只核查原任务、确认终态及401/切任务/离页清理。');
}

async function shellTabs() {
  const env = environment(); env.storage.set('token', memberToken(42));
  const ui = mount(env.load('pages/ai/index.uvue').default);
  env.hooks.show.forEach(fn => fn());
  env.reply(env.requests.shift(), { items: [summary(9)], nextBeforeId: 0, hasMore: false }); await flush();
  assert.equal(ui.state.activeTab, 'conversation', '默认以会话为主入口');
  assert.ok(ui.text().includes('会话'));
  assert.ok(ui.text().includes('全部任务'));
  assert.ok(ui.text().includes('我的会话'), '会话面板默认挂载');
  ui.click('全部任务'); await flush();
  assert.equal(ui.state.activeTab, 'tasks');
  assert.ok(ui.text().includes('全部资料任务（历史）'), '历史任务仅在第二页签呈现');
  // v-show 只切换显示：会话面板不应卸载，切换回来无需重新请求。
  const before = env.requests.length;
  ui.click('会话'); await flush();
  assert.equal(ui.state.activeTab, 'conversation');
  assert.equal(env.requests.length, before, '切换页签不得触发新请求');
  ui.app.unmount();
  console.log('通过：实际首页默认会话主入口、会话/全部任务双页签、历史任务区归属与切换不重发请求。');
}

async function conversationFrontend() {
  console.log('DEBUG conversationFrontend start');
  const summary = (id=1,archived=false) => ({id,title:'测试会话',archived,version:1,updatedAt:now});
  const detail = (id=1) => ({...summary(id),lastSeq:0,createdAt:now});
  const msg = (seq=1) => ({seq,kind:'task',subjectType:'theme',subjectId:7,skill:'light',question:'',taskId:9,createdAt:now});
  const page = (items=[]) => ({items,hasMore:false,nextBeforeId:0});
  const history = (items=[]) => ({conversationId:1,items,upperSeq:items.length,nextAfterSeq:items.length,hasMore:false});
  {
    const e=environment(new Map(),true),r=e.load('pages/libs/ai-conversation.uts');
    assert.equal(r.isAIConversationTitle('测试会话😀'),true);assert.equal(r.isAIConversationTitle('😀'.repeat(40)),true);
    for(const title of ['', ' 题材', '题材\n记录','\ud800','字'.repeat(41)])assert.equal(r.isAIConversationTitle(title),false);
    assert.equal(r.isAIConversation(detail(),1),true);assert.equal(r.isAIConversation({...detail(),id:'1'},1),false);
    for(const items of [[summary(1),summary(2)],[summary(),summary()],[{...summary(),archived:1}]])assert.equal(r.isAIConversations(page(items),0,false),false);
    assert.equal(r.isAIConversationMessages(history([msg()]),1,0),true);
    for(const items of [[msg(2)],[{...msg(),question:'不应存在'}],[{...msg(),subjectId:0}],[{...msg(),taskId:'9'}],[{...msg(),kind:['task']}],[{...msg(),kind:'assistant',question:'测试自由输出'}],[{...msg(),kind:'question',skill:'material_qa',taskId:0,question:'字'.repeat(1001)}]])assert.equal(r.isAIConversationMessages(history(items),1,0),false);
    assert.equal(r.isAIConversationMessages({...history(),conversationId:'1'},1,0),false);
    const token=memberToken(1);e.storage.set('token',token);const partition=r.conversationPartition(token);
    e.storage.set(partition,0);assert.throws(()=>r.readPendingConversation(partition));e.storage.delete(partition);
    const reserved=await Promise.all([r.reserveConversation(partition,'测试会话'),r.reserveConversation(partition,'另一会话')]);assert.equal(reserved.filter(Boolean).length,1);
    assert.equal(await r.clearPendingConversation(partition,'different_original_key'),false);assert.equal(await r.clearPendingConversation(partition,reserved.find(Boolean).key),true);
    const api=e.load('pages/libs/ai-conversation-api.uts').AIConversationApi;
    const get=api.list(0,false);let req=e.requests.shift();assert.equal(req.url,'/api/v1/ai/conversations?limit=20&archived=0');assert.equal(req.header.Authorization,'Bearer '+token);e.reply(req,page());assert.equal((await get).code,0);
    assert.equal((await api.rename(0,1,'测试会话')).code,40001);assert.equal(e.requests.length,0);
    const patch=api.rename(1,1,'测试改名');req=e.requests.shift();assert.equal(req.method,'PATCH');assert.equal(req.header['Content-Type'],'application/json');assert.deepEqual(clone(req.data),{title:'测试改名',expectedVersion:1});e.reply(req,{...detail(),title:'测试改名',version:2});assert.equal((await patch).code,0);
    const bad=api.messages(1,0);e.reply(e.requests.shift(),history([msg(2)]));assert.equal((await bad).code,50001);
    assert.equal((await e.load('pages/libs/member-api.uts').MemberApi.aiConversation('delta',1,0,false,{},'')).code,40001);assert.equal(e.requests.length,0);
    e.uni.getStorageSync=()=>{throw new Error('测试存储不可读')};assert.equal(e.load('pages/libs/ai-session.uts').currentAIMemberToken(),'');
  }
  {
    const e=environment(new Map(),true),token=memberToken(1);e.storage.set('token',token);
    const tasks=[],ui=mount(e.load('pages/ai/components/AIConversationPanel.uvue').default,{sessionToken:token,onTask:id=>tasks.push(id)});await flush();e.reply(e.requests.shift(),page());await flush();ui.input('测试会话');await flush();ui.click('新建会话');ui.state.create();await flush();assert.equal(e.requests.length,1);
    let req=e.requests.shift();assert.equal(req.method,'POST');assert.equal(req.data.title,'测试会话');const key=req.header['Idempotency-Key'];assert.ok(key.startsWith('conversation_'));assert.ok([...e.storage.values()].some(v=>typeof v==='string'&&v.includes(key)),'先持久保存再POST');req.fail();await flush();assert.ok(ui.text().includes('待核查'));ui.click('核查原会话');req=e.requests.shift();assert.equal(req.method,'GET');assert.equal(req.header['Idempotency-Key'],key);e.reply(req,null,404,40403);await flush();assert.equal(e.requests.length,0);assert.ok(ui.state.pending);
    ui.click('核查原会话');e.reply(e.requests.shift(),{id:1,createdAt:now});await flush();e.reply(e.requests.shift(),detail());await flush();e.reply(e.requests.shift(),history([msg()]));await flush();assert.ok(ui.text().includes('当前会话：测试会话'));assert.equal(ui.state.pending,null);ui.click('查看关联任务 #9');assert.deepEqual(tasks,[9]);
    ui.state.renameTitle='测试改名';ui.click('保存会话标题');req=e.requests.shift();assert.equal(req.method,'PATCH');assert.equal(req.data.expectedVersion,1);e.reply(req,{...detail(),title:'测试改名',version:2,updatedAt:now+1});await flush();assert.ok(ui.text().includes('当前会话：测试改名'));
    ui.click('归档当前会话');assert.equal(e.requests.length,0);e.modals.shift().success({confirm:true});req=e.requests.shift();assert.equal(req.method,'POST');assert.equal(req.data.expectedVersion,2);e.reply(req,{...detail(),archived:true,version:3,updatedAt:now+2});await flush();assert.equal(ui.state.selected,null);assert.equal(e.requests.length,0,'归档不能取消任务或生成报告');
    ui.click('查看已归档');req=e.requests.shift();assert.ok(req.url.endsWith('archived=1'));e.reply(req,page([{...summary(),archived:true,version:3}]));await flush();ui.click('打开会话 #1');const late=e.requests.shift();e.storage.set('token',memberToken(2));ui.app._instance.props.sessionToken=memberToken(2);await flush();e.reply(late,detail());e.reply(e.requests.shift(),page());await flush();assert.equal(ui.state.selected,null);assert.equal(ui.state.rows.length,0);assert.ok(!ui.text().includes('测试改名'));ui.app.unmount();
  }
  console.log('DEBUG conversationFrontend end');
  console.log('通过：会话DTO/七个固定接口、标题/连续历史/分页拒绝、安全原键跨页签互斥、未知只查原键、新建/改名/归档/关联任务及换账号迟到隔离。');
}

async function conversationTaskFrontend() {
  const dto=(state='prepared')=>({id:3,conversationId:1,analysisType:'light',subjectType:'theme',subjectId:7,priceVersion:'test-v1',pricePoints:7,state,rejectionCode:state==='not_created'?'creation_disabled':'',taskId:state==='linked'?9:0,messageSeq:state==='linked'?1:0,createdAt:now,updatedAt:state==='prepared'?now:now+1});
  const item=(id=3)=>({id,state:'prepared',analysisType:'light',subjectType:'theme',subjectId:7,updatedAt:now});
  const page=(items=[])=>({items,nextBeforeId:0,hasMore:false});
  const partition='tm_ai_intent_v1_42',token=memberToken(42);
  const pending=()=>({key:'test_intent_original_key',conversationId:1,analysisType:'light',subjectId:7,priceVersion:'test-v1',pricePoints:7,intentId:3,confirmSent:false});
  {
    const e=environment(new Map(),true,true),r=e.load('pages/libs/ai-conversation-task.uts');e.storage.set('token',token);
    for(const state of ['prepared','unconfirmed','linked','not_created'])assert.equal(r.isAIConversationTask(dto(state),1),true);
    for(const change of [v=>v.id='3',v=>v.conversationId='1',v=>v.conversationId=2,v=>v.subjectId=0,v=>v.analysisType=['light'],v=>v.state=['prepared'],v=>v.pricePoints=9007199254740992,v=>v.taskId=9,v=>v.messageSeq=1,v=>v.rejectionCode='raw_secret',v=>v.updatedAt=now-1]){const v=dto();change(v);assert.equal(r.isAIConversationTask(v,1),false)}
    assert.equal(r.isAIConversationTask({...dto('linked'),messageSeq:0},1),false);assert.equal(r.isAIConversationTask({...dto('not_created'),rejectionCode:''},1),false);
    assert.equal(r.isAIConversationTasks(page([item(3),item(2)]),0),true);assert.equal(r.isAIConversationTasks(page([item(2),item(3)]),0),false);assert.equal(r.isAIConversationTasks({...page([item()]),hasMore:true,nextBeforeId:2},0),false);
    assert.equal(r.intentPartition(token),partition);assert.equal(r.intentMatchesPending(dto(),pending()),true);assert.equal(r.intentMatchesPending({...dto(),subjectId:8},pending()),false);
    e.storage.set(partition,0);assert.throws(()=>r.readPendingIntent(partition));e.storage.delete(partition);
    const reservations=await Promise.all([r.reserveIntent(partition,1,quote()),r.reserveIntent(partition,1,quote())]);assert.equal(reservations.filter(Boolean).length,1);const p=reservations.find(Boolean);
    assert.equal(await r.markPendingIntent(partition,'wrong_original_key',3,false),null);
    const marked=await r.markPendingIntent(partition,p.key,3,false);assert.equal(marked.confirmSent,false);
    const sent=await Promise.all([r.markPendingIntent(partition,p.key,3,true),r.markPendingIntent(partition,p.key,3,true)]);assert.equal(sent.filter(Boolean).length,1);assert.equal(r.readPendingIntent(partition).confirmSent,true);
    assert.equal(await r.clearPendingIntent(partition,'wrong_original_key'),false);assert.equal(await r.clearPendingIntent(partition,p.key),true);
    const legacy=e.load('pages/libs/ai-submission.uts'),legacyPartition='tm_ai_pending_v1_42';
    for(let turn=0;turn<6;turn++){
      const contenders=turn%2?[r.reserveIntent(partition,1,quote()),legacy.reserveAIPending(legacyPartition,quote())]:[legacy.reserveAIPending(legacyPartition,quote()),r.reserveIntent(partition,1,quote())];
      const won=await Promise.all(contenders);assert.equal(won.filter(Boolean).length,1,'两种入口不能同时保留新生成权限');
      assert.equal(Number(e.storage.has(partition))+Number(e.storage.has(legacyPartition)),1);e.storage.delete(partition);e.storage.delete(legacyPartition);
    }
    e.storage.set(partition,0);assert.equal(await legacy.reserveAIPending(legacyPartition,quote()),null);e.storage.delete(partition);
    e.storage.set(legacyPartition,0);await assert.rejects(()=>r.reserveIntent(partition,1,quote()));e.storage.delete(legacyPartition);
    delete e.globals.window.navigator.locks;assert.equal(await r.reserveIntent(partition,1,quote()),null);
  }
  {
    const e=environment(new Map(),true,true),a=e.load('pages/libs/ai-conversation-task-api.uts').AIConversationTaskApi;e.storage.set('token',token);
    for(const [call,method,suffix,body,key] of [
      [()=>a.list(1,0),'GET','/task-intents?limit=20',{},false],
      [()=>a.prepare({...pending(),intentId:0}),'POST','/task-intents',{analysis_type:'light',subject_type:'theme',subject_id:7,price_version:'test-v1'},true],
      [()=>a.receipt(1,pending().key),'GET','/task-intent-receipt',{},true],
      [()=>a.detail(1,3),'GET','/task-intents/3',{},false],
      [()=>a.confirm(1,3),'POST','/task-intents/3/confirm',{confirmed:true},false],
      [()=>a.reconcile(1,3),'POST','/task-intents/3/reconcile',{},false],
    ]){const result=call(),req=e.requests.shift();assert.equal(req.url,'/api/v1/ai/conversations/1'+suffix);assert.equal(req.method,method);assert.deepEqual(JSON.parse(JSON.stringify(req.data)),body);assert.equal(req.header.Authorization,'Bearer '+token);assert.equal(Boolean(req.header['Idempotency-Key']),key);e.reply(req,suffix.includes('?')?page():dto());assert.equal((await result).code,0)}
    assert.notEqual((await a.detail(0,3)).code,0);assert.notEqual((await a.prepare(pending())).code,0);assert.equal(e.requests.length,0);
    const bad=a.detail(1,3);e.reply(e.requests.shift(),{...dto(),id:4});assert.notEqual((await bad).code,0);
  }
  const start=async(options={})=>{const e=environment(options.storage||new Map(),true,true);e.storage.set('token',token);if(options.pending)e.storage.set(partition,JSON.stringify(options.pending));const props=vue.reactive({sessionToken:token,conversationId:1,analysisType:options.analysisType||'light',archived:options.archived||false});const tasks=[],component=e.load('pages/ai/components/AIConversationTaskPanel.uvue').default;const ui=mount({render:()=>vue.h(component,{...props,onTask:id=>tasks.push(id)})});e.reply(e.requests.shift(),page());await flush();return {e,props,tasks,ui,component}};
  // 完整实际组件点击：共用原搜索/报价，但仅准备会话意图，不走独立Create。
  {
    const {e,ui,tasks}=await start();ui.input('测试示例题材');ui.click('查询题材');e.reply(e.requests.shift(),[{id:7,name:'测试示例题材',level:1}]);await flush();ui.click('核对该题材报价');e.reply(e.requests.shift(),quote());await flush();assert.ok(!ui.text().includes('确认报价并创建'));
    ui.click('保存会话报价意图（不冻结积分）');await flush();const req=e.requests.shift();assert.ok(req.url.endsWith('/task-intents'));assert.equal(JSON.parse(e.storage.get(partition)).intentId,0);e.reply(req,dto());await flush();assert.equal(JSON.parse(e.storage.get(partition)).intentId,3);
    ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),quote());await flush();ui.click('确认会话报价并创建');assert.equal(e.requests.length,0);e.modals.shift().success({confirm:true});await flush();assert.equal(JSON.parse(e.storage.get(partition)).confirmSent,true);const confirm=e.requests.shift();assert.ok(confirm.url.endsWith('/3/confirm'));e.reply(confirm,dto('linked'));await flush();ui.click('查看会话关联任务 #9');assert.deepEqual(tasks,[9]);
    ui.click('核对终态并记录完成');assert.ok(e.storage.has(partition));e.reply(e.requests.shift(),dto('linked'));await flush();assert.equal(e.storage.has(partition),false);ui.app.unmount();
  }
  // Prepare/Confirm未知、重载、关闭、变价、跨用户、离页、归档和迟到回包均不补交。
  for(const mode of ['prepare_unknown','confirm_unknown','404','price_changed','wrong_scope','401','switch','unmount','archive','storage_fail','duplicate_tab']){
    const initial=mode==='prepare_unknown'?{...pending(),intentId:0}:pending();const {e,ui,props}=await start({pending:initial});
    ui.click('查询本机原意图');const original=e.requests.shift();assert.equal(original.method,'GET');assert.equal(original.header['Idempotency-Key'],initial.key);
    if(mode==='404'){e.reply(original,null,404,40403);await flush();assert.ok(e.storage.has(partition));assert.equal(e.requests.length,0);ui.app.unmount();continue}
    if(mode==='wrong_scope'){e.reply(original,{...dto(),subjectId:8});await flush();assert.ok(!ui.text().includes('重新核对报价与资料范围'));assert.equal(e.requests.length,0);ui.app.unmount();continue}
    e.reply(original,dto());await flush();
    if(mode==='prepare_unknown'){assert.equal(JSON.parse(e.storage.get(partition)).intentId,3);assert.equal(e.requests.length,0);ui.app.unmount();continue}
    ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),mode==='price_changed'?{...quote(),pricePoints:8}:quote());await flush();
    if(mode==='price_changed'){assert.ok(!ui.text().includes('确认会话报价并创建'));assert.equal(e.requests.length,0);ui.app.unmount();continue}
    ui.click('确认会话报价并创建');const modal=e.modals.shift();
    if(mode==='archive')props.archived=true;if(mode==='switch'){e.storage.set('token',memberToken(2));props.sessionToken=memberToken(2)}if(mode==='unmount')ui.app.unmount();await flush();
    if(mode==='storage_fail')e.uni.setStorageSync=()=>{throw new Error('测试存储故障')};
    if(mode==='duplicate_tab'){const r=e.load('pages/libs/ai-conversation-task.uts');assert.ok(await r.markPendingIntent(partition,initial.key,3,true))}
    modal.success({confirm:true});await flush();
    if(['archive','unmount','storage_fail','duplicate_tab','switch'].includes(mode)){assert.ok(!e.requests.some(x=>x.url.endsWith('/confirm')));if(mode!=='unmount')ui.app.unmount();continue}
    const sent=e.requests.shift();assert.ok(sent.url.endsWith('/confirm'));
    if(mode==='401'){e.reply(sent,null,401,40100);await flush();assert.ok(!ui.text().includes('查看会话关联任务'));assert.equal(JSON.parse(e.storage.get(partition)).confirmSent,true);ui.app.unmount();continue}
    sent.fail();await flush();assert.equal(e.requests.length,0);ui.app.unmount();
    const recovered=await start({storage:e.storage});recovered.ui.click('查询本机原意图');recovered.e.reply(recovered.e.requests.shift(),dto('unconfirmed'));await flush();assert.ok(!recovered.ui.text().includes('确认会话报价并创建'));
    recovered.ui.click('核查并恢复原任务关联（不创建）');const recon=recovered.e.requests.shift();assert.ok(recon.url.endsWith('/reconcile'));assert.deepEqual(JSON.parse(JSON.stringify(recon.data)),{});recovered.e.reply(recon,dto('linked'));await flush();recovered.ui.click('查看会话关联任务 #9');assert.deepEqual(recovered.tasks,[9]);recovered.ui.app.unmount();
  }
  {const {e,ui}=await start({archived:true});assert.ok(!ui.text().includes('查询题材'));assert.equal(e.requests.length,0);ui.app.unmount()}
  {const e=environment(new Map(),true,true);e.storage.set('token',token);e.storage.set(partition,JSON.stringify(pending()));const ui=mount(e.load('pages/ai/components/AICreatePanel.uvue').default,{sessionToken:token});assert.ok(ui.text().includes('独立入口暂不新建'));assert.ok(!ui.text().includes('查询题材'));assert.equal(e.requests.length,0);ui.app.unmount()}
  {
    const e=environment(new Map(),true,true);e.storage.set('token',token);const ui=mount(e.load('pages/ai/index.uvue').default);
    const take=path=>{const i=e.requests.findIndex(r=>r.url===path);assert.ok(i>=0,'缺少实际请求：'+path);return e.requests.splice(i,1)[0]};
    e.hooks.show.forEach(fn=>fn());e.reply(take('/api/v1/ai/tasks?limit=20'),page());await flush();
    const c={id:1,title:'测试会话纵向',archived:false,version:1,updatedAt:now,createdAt:now,lastSeq:0};
    e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),page([{id:1,title:c.title,archived:false,version:1,updatedAt:now}]));await flush();ui.click('打开会话 #1');e.reply(take('/api/v1/ai/conversations/1'),c);await flush();
    e.reply(take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0'),{conversationId:1,items:[],upperSeq:0,nextAfterSeq:0,hasMore:false});ui.click('使用该技能');await flush();e.reply(take('/api/v1/ai/conversations/1/task-intents?limit=20'),page());await flush();assert.equal(ui.state.conversationId,1);assert.ok(!ui.text().includes('新建轻度资料整理'));
    // 定位共用报价组件，避免会话标题输入框与题材输入框混用。
    const fields=[];function walk(n){if(n.tag==='x-input'&&n.props.placeholder==='输入题材名称，仅用于查询目录')fields.push(n);n.children.forEach(walk)}walk(ui.tree);assert.equal(fields.length,1);fields[0].props['on:update:modelValue']('测试题材');await flush();
    ui.click('查询题材');e.reply(e.requests.shift(),[{id:7,name:'测试示例题材'}]);await flush();ui.click('核对该题材报价');e.reply(e.requests.shift(),quote());await flush();ui.click('保存会话报价意图（不冻结积分）');await flush();e.reply(e.requests.shift(),dto());await flush();ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),quote());await flush();ui.click('确认会话报价并创建');e.modals.shift().success({confirm:true});await flush();e.reply(e.requests.shift(),dto('linked'));await flush();
    e.reply(take('/api/v1/ai/conversations/1'),{...c,version:2,lastSeq:1});await flush();e.reply(take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0'),{conversationId:1,items:[{seq:1,kind:'task',subjectType:'theme',subjectId:7,skill:'light',question:'',taskId:9,createdAt:now}],upperSeq:1,nextAfterSeq:1,hasMore:false});await flush();
    ui.click('查看会话关联任务 #9');e.reply(take('/api/v1/ai/tasks/9'),progress());await flush();ui.click('核对并查看报告');e.reply(take('/api/v1/ai/tasks/9/report'),report());await flush();assert.equal(ui.state.report.taskId,9);assert.equal(ui.state.conversationId,1);assert.equal(e.requests.length,0);
    e.hooks.hide.forEach(fn=>fn());await flush();assert.equal(ui.state.report,null);assert.equal(ui.state.conversationId,0);ui.app.unmount();
  }
  {
    const deepQuote=quote({analysisType:'deep',pricePoints:30,availablePoints:50});
    const deepDto=(state='prepared')=>({...dto(state),analysisType:'deep',pricePoints:30});
    const {e,ui,tasks}=await start({analysisType:'deep'});
    assert.ok(ui.text().includes('深度资料任务'));
    ui.input('测试示例题材');ui.click('查询题材');e.reply(e.requests.shift(),[{id:7,name:'测试示例题材',level:1}]);await flush();
    ui.click('核对该题材报价');e.reply(e.requests.shift(),deepQuote);await flush();
    assert.ok(ui.text().includes('30 积分'));
    ui.click('保存会话报价意图（不冻结积分）');await flush();
    const prepare=e.requests.shift();assert.ok(prepare.url.endsWith('/task-intents'));assert.equal(prepare.data.analysis_type,'deep');assert.equal(prepare.data.price_version,'test-v1');
    e.reply(prepare,deepDto());await flush();
    assert.equal(JSON.parse(e.storage.get(partition)).analysisType,'deep');
    ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),deepQuote);await flush();
    ui.click('确认会话报价并创建');e.modals.shift().success({confirm:true});await flush();
    const confirm=e.requests.shift();assert.ok(confirm.url.endsWith('/confirm'));e.reply(confirm,deepDto('linked'));await flush();
    ui.click('查看会话关联任务 #9');assert.deepEqual(tasks,[9]);ui.app.unmount();
  }
  {
    // 深度确认被准入/限流拒绝：只显示不可创建原因，不得出现关联任务或再次冻结。
    const deepQuote=quote({analysisType:'deep',pricePoints:30,availablePoints:50});
    const {e,ui}=await start({analysisType:'deep'});
    ui.input('测试示例题材');ui.click('查询题材');e.reply(e.requests.shift(),[{id:7,name:'测试示例题材',level:1}]);await flush();
    ui.click('核对该题材报价');e.reply(e.requests.shift(),deepQuote);await flush();
    ui.click('保存会话报价意图（不冻结积分）');await flush();e.reply(e.requests.shift(),{...dto(),analysisType:'deep',pricePoints:30});await flush();
    ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),deepQuote);await flush();
    ui.click('确认会话报价并创建');e.modals.shift().success({confirm:true});await flush();
    e.reply(e.requests.shift(),{...dto('not_created'),analysisType:'deep',pricePoints:30,rejectionCode:'admission_unavailable'});await flush();
    const text=ui.text();
    assert.ok(text.includes('当前资料或执行准入不满足'));
    assert.ok(!text.includes('查看会话关联任务'));
    assert.equal(JSON.parse(e.storage.get(partition)).confirmSent,true);
    ui.app.unmount();
  }
  console.log('通过：会话意图六接口/闭集DTO、先存原键与跨页签单次确认、实际搜索报价→准备→二次确认→关联任务→终态核查；深度按30积分准备并携带analysisType=deep。');
}

(async () => { await validators(); await conversationTaskFrontend(); await api(); await historyReportPermission(); await taskOutcomeFeedback(); await page(); await panel(); await reportPresentation(); await reportTools(); await feedbackContracts(); await feedbackPanel(); entry(); await submissionContracts(); await submissionPage(); await memberFlow(); await creationHistoryFlow(); await cancellationContracts(); await cancellationPanel(); await cancellationMemberFlow(); await shellTabs(); await conversationFrontend(); await conversationTaskFrontend(); console.log('本脚本不覆盖：真实 UTS/uni-app x 整体构建、TMUI实现与浏览器布局；须另做实际H5验收，不能因依赖已经存在而升级源码测试证据。'); })()
  .catch(error => { console.error(error); process.exitCode = 1; });
