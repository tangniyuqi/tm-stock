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
const quote = () => ({ analysisType: 'light', subjectType: 'theme', subjectId: 7, subjectName: '测试示例题材', pricePoints: 7, priceVersion: 'test-v1', availablePoints: 20, quotedAt: now, holdExpireSec: 90,
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

function environment(sharedStorage = new Map()) {
  const cache = new Map();
  const requests = [], timers = new Map(), intervals = new Map(), hooks = { show: [], hide: [], unload: [] };
  const storage = sharedStorage, modals = [], opened = [], navigation = [], lockQueues = new Map();
  let timerID = 0;
  const uni = {
    getStorageSync: key => storage.get(key), request: options => requests.push(options),
    setStorageSync: (key, value) => storage.set(key, value), removeStorageSync: key => storage.delete(key),
    showModal: options => modals.push(options), showToast: () => {},
    setClipboardData: () => {}, navigateTo: options => navigation.push(options.url),
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
  return { load, requests, timers, intervals, hooks, storage, modals, opened, navigation, reply, globals, uni };
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
    const field = all(tree).find(n => n.tag === 'x-input'); assert.ok(field, '找不到实际输入绑定');
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
    ui.click('申请取消任务'); await flush(); assert.equal(env.timers.size, 0); assert.equal(ui.state.cancelInteracting, true);
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

(async () => { await validators(); await api(); await historyReportPermission(); await taskOutcomeFeedback(); await page(); await panel(); entry(); await submissionContracts(); await submissionPage(); await memberFlow(); await creationHistoryFlow(); await cancellationContracts(); await cancellationPanel(); await cancellationMemberFlow(); console.log('本脚本不覆盖：真实 UTS/uni-app x 整体构建、TMUI实现与浏览器布局；须另做实际H5验收，不能因依赖已经存在而升级源码测试证据。'); })()
  .catch(error => { console.error(error); process.exitCode = 1; });
