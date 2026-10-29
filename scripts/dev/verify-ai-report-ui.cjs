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
const native = tag => ['view', 'text', 'scroll-view', 'x-sheet', 'x-button', 'x-navbar', 'x-input', 'x-drawer'].includes(tag);
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
  let viewportWidth = 1280;
  let viewportHeight = 800;
  const resizeListeners = new Set();
  const uni = {
    getWindowInfo: () => ({ windowWidth: viewportWidth, windowHeight: viewportHeight }),
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
    onResize: fn => resizeListeners.add(fn),
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
  return { load, requests, timers, intervals, hooks, storage, modals, opened, navigation, clipboard, reply, globals, uni, resizeListeners,
    resize: (width, height = viewportHeight) => { viewportWidth = width; viewportHeight = height; for (const listener of resizeListeners) listener({size:{windowWidth:width,windowHeight:height}}); } };
}

function mount(component, props = {}) {
  const node = (tag, text = '') => ({ tag, text, children: [], props: {}, style: {}, parent: null });
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
  // 仅模拟抽屉show及v-show的可见性，不替代TMUI实际动画、层级和点击验收。
  const visibleText = n => n.style.display === 'none' || (n.tag === 'x-drawer' && !n.props.show) ? '' : (n.tag === '#comment' ? '' : n.text) + n.children.map(visibleText).join('');
  const all = n => [n, ...n.children.flatMap(all)];
  return { app, tree, state: app._instance.setupState, text: () => text(tree), visibleText: () => visibleText(tree), input(value, placeholder) {
    const field = all(tree).find(n => n.tag === 'x-input' && !n.props.disabled && (placeholder == null || n.props.placeholder === placeholder)); assert.ok(field, '找不到可编辑的实际输入绑定');
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
  const findGuestScroll = n => n.props.class === 'ai-guest-scroll' ? n : n.children.map(findGuestScroll).find(Boolean);
  const guestScroll = findGuestScroll(ui.tree); assert.ok(guestScroll);
  assert.ok(ui.visibleText().includes('登录后查看本人任务'));
  ui.click('前往登录'); assert.equal(env.navigation[0], '/pages/member/login');
  env.storage.set('token', 'test-member-one'); ui.click('重新核对登录状态'); await flush();
  assert.ok(ui.text().includes('正在读取任务记录')); env.reply(env.requests.shift(), { items: [], nextBeforeId: 0, hasMore: false }); await flush();
  assert.ok(ui.text().includes('当前账号暂无任务记录'));
  assert.equal(findGuestScroll(ui.tree), guestScroll, '登录后保留游客滚动实例，避免激活恢复访问销毁节点');
  assert.ok(!ui.visibleText().includes('登录后查看本人任务'), '已登录时隐藏游客入口');
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
  promise=api.quote({...scope,analysisType:'deep'}); request=env.requests.shift();
  assert.equal(request.method,'POST'); assert.equal(request.data.analysis_type,'deep');
  env.reply(request,quote({analysisType:'deep',pricePoints:30,availablePoints:50})); assert.equal((await promise).code,0); assert.equal(env.requests.length,0);
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
  const guidanceRequests = env.requests.length;
  const disclaimer = '本平台为公开信息聚合与历史数据统计，不构成任何投资建议。';
  assert.ok(ui.text().includes(disclaimer));
  assert.ok(!ui.visibleText().includes('默认进入会话：'));
  ui.click('使用说明'); await flush();
  assert.ok(ui.visibleText().includes('问答不扣积分，生成报告须另行报价确认'));
  assert.ok(ui.text().includes(disclaimer));
  ui.click('收起说明'); await flush();
  assert.ok(!ui.visibleText().includes('默认进入会话：'));
  assert.ok(ui.text().includes(disclaimer));
  assert.equal(env.requests.length, guidanceRequests, '使用说明开关不得发业务请求');
  const requestCount = env.requests.length;
  for (const width of [375, 960, 959, 1280]) {
    env.resize(width); await flush();
    assert.equal(ui.state.windowWidth, width, '页面接收原onResize尺寸');
    const scrolls = n => [...(n.tag === 'scroll-view' && n.props.class === 'conversation-history-scroll' ? [n] : []), ...n.children.flatMap(scrolls)];
    assert.equal(scrolls(ui.tree).length, width >= 960 ? 1 : 0, '原页面尺寸必须传到实际会话组件');
    assert.equal(env.requests.length, requestCount, '页面缩放不得新增业务请求');
  }
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

async function reportQuestionPageAccess() {
  {
    const e=environment(new Map(),true);e.storage.set('token',memberToken(42));
    const page=mount(e.load('pages/ai/index.uvue').default);
    e.hooks.show.forEach(fn=>fn());e.reply(e.requests.shift(),{items:[],nextBeforeId:0,hasMore:false});await flush();
    e.reply(e.requests.shift(),{items:[],nextBeforeId:0,hasMore:false});await flush();
    page.state.conversationId=3;page.state.loadProgress(9);e.reply(e.requests.shift(),progress());await flush();
    page.click('核对并查看报告');e.reply(e.requests.shift(),report());await flush();
    page.input('报告中的事实出处是什么？','输入针对本报告的事实问题，最多1000字');await flush();
    page.click('将报告追问固定到底部');await flush();
    assert.equal(page.state.reportInputDocked,true);
    assert.equal(e.requests.length,0,'固定报告输入不能重读或提交');
    const findInput = n => n.tag === 'x-input' && n.props.type === 'textarea' ? n : n.children.map(findInput).find(Boolean);
    const originalInput = findInput(page.tree);
    for (const [height, docked] of [[320,false],[519,false],[520,true],[568,true]]) {
      e.resize(375,height);await flush();
      assert.equal(page.state.allowDock,docked);
      assert.equal(page.state.reportInputDocked,true,'低高度仅切换布局，不丢用户固定偏好');
      assert.equal(findInput(page.tree),originalInput,'尺寸变化不得重新挂载输入或丢草稿');
      assert.equal(originalInput.props.fixed,docked);
      assert.equal(originalInput.props['model-value'],'报告中的事实出处是什么？');
      assert.equal(e.requests.length,0,'高度变化不得提交或重新读取报告');
    }
    page.click('恢复报告内输入');await flush();assert.equal(page.state.reportInputDocked,false);
    page.click('将报告追问固定到底部');await flush();
    page.click('全部任务');await flush();assert.equal(page.state.reportInputDocked,false);
    page.click('会话');await flush();
    page.click('将报告追问固定到底部');await flush();
    page.state.changeConversation();await flush();assert.equal(page.state.reportInputDocked,false);
    assert.equal(page.state.report,null);assert.equal(e.requests.length,0);page.app.unmount();
  }
  const env=environment(new Map(),true);env.storage.set('token',memberToken(42));
  const ui=mount(env.load('pages/ai/index.uvue').default);
  env.hooks.show.forEach(fn=>fn());env.reply(env.requests.shift(),{items:[],nextBeforeId:0,hasMore:false});await flush();
  env.reply(env.requests.shift(),{items:[],nextBeforeId:0,hasMore:false});await flush();
  ui.state.conversationId=3;
  ui.state.openReportQuestion(9,2);
  let request=env.requests.shift();assert.equal(request.method,'GET');assert.equal(request.url,'/api/v1/ai/tasks/9');
  env.reply(request,progress());await flush();
  assert.equal(ui.state.reportQuestionSeq,2);
  assert.ok(ui.text().includes('请核对并查看关联报告'));
  ui.click('核对并查看报告');request=env.requests.shift();assert.equal(request.method,'GET');
  env.reply(request,report());await flush();
  request=env.requests.shift();assert.equal(request.method,'GET');assert.equal(request.url,'/api/v1/ai/conversations/3/questions/2');
  env.reply(request,null,401,40100);await flush();
  assert.equal(ui.state.state,'guest','子追问鉴权失败必须清理父页面');
  assert.equal(ui.state.report,null);assert.equal(ui.state.progress,null);assert.equal(ui.state.conversationId,0);assert.equal(ui.state.reportQuestionSeq,0);
  assert.equal(env.requests.length,0,'历史查看或失效身份不得隐式发起任务/问答');
  ui.app.unmount();
  console.log('通过：历史报告追问绑定原任务/序号、先核报告再读取、无隐式POST；子追问401清理父页面全部私有状态。');
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
    const tasks=[],reportTargets=[],ui=mount(e.load('pages/ai/components/AIConversationPanel.uvue').default,{sessionToken:token,onTask:id=>tasks.push(id),onReportQuestion:(id,seq)=>reportTargets.push([id,seq])});await flush();e.reply(e.requests.shift(),page());await flush();ui.input('测试会话');await flush();ui.click('新建会话');ui.state.create();await flush();assert.equal(e.requests.length,1);
    let req=e.requests.shift();assert.equal(req.method,'POST');assert.equal(req.data.title,'测试会话');const key=req.header['Idempotency-Key'];assert.ok(key.startsWith('conversation_'));assert.ok([...e.storage.values()].some(v=>typeof v==='string'&&v.includes(key)),'先持久保存再POST');req.fail();await flush();assert.ok(ui.text().includes('待核查'));ui.click('核查原会话');req=e.requests.shift();assert.equal(req.method,'GET');assert.equal(req.header['Idempotency-Key'],key);e.reply(req,null,404,40403);await flush();assert.equal(e.requests.length,0);assert.ok(ui.state.pending);
    ui.click('核查原会话');e.reply(e.requests.shift(),{id:1,createdAt:now});await flush();e.reply(e.requests.shift(),detail());await flush();e.reply(e.requests.shift(),history([msg(),{...msg(2),kind:'question',skill:'report_qa',question:'报告引用了什么分类？'}]));await flush();
    const refreshed=e.requests.shift();assert.equal(refreshed.url,'/api/v1/ai/conversations?limit=20&archived=0');assert.equal(refreshed.method,'GET');e.reply(refreshed,page([summary()]));await flush();
    assert.ok(ui.text().includes('当前会话：测试会话'));assert.equal(ui.state.pending,null);ui.click('查看关联任务 #9');assert.deepEqual(tasks,[9]);
    ui.click('查询报告追问 #2');assert.deepEqual(reportTargets,[[9,2]],'历史入口必须绑定原报告和消息序号');
    ui.state.renameTitle='测试改名';ui.click('保存会话标题');req=e.requests.shift();assert.equal(req.method,'PATCH');assert.equal(req.data.expectedVersion,1);e.reply(req,{...detail(),title:'测试改名',version:2,updatedAt:now+1});await flush();assert.ok(ui.text().includes('当前会话：测试改名'));
    ui.click('归档当前会话');assert.equal(e.requests.length,0);e.modals.shift().success({confirm:true});req=e.requests.shift();assert.equal(req.method,'POST');assert.equal(req.data.expectedVersion,2);e.reply(req,{...detail(),archived:true,version:3,updatedAt:now+2});await flush();assert.equal(ui.state.selected,null);assert.equal(e.requests.length,0,'归档不能取消任务或生成报告');
    ui.click('查看已归档');req=e.requests.shift();assert.ok(req.url.endsWith('archived=1'));e.reply(req,page([{...summary(),archived:true,version:3}]));await flush();ui.click('打开会话 #1');const late=e.requests.shift();e.storage.set('token',memberToken(2));ui.app._instance.props.sessionToken=memberToken(2);await flush();e.reply(late,detail());e.reply(e.requests.shift(),page());await flush();assert.equal(ui.state.selected,null);assert.equal(ui.state.rows.length,0);assert.ok(!ui.text().includes('测试改名'));ui.app.unmount();
  }
  console.log('DEBUG conversationFrontend end');
  console.log('通过：会话DTO/七个固定接口、标题/连续历史/分页拒绝、安全原键跨页签互斥、未知只查原键、新建/改名/归档/关联任务及换账号迟到隔离。');
}

async function conversationLateMessages() {
  for (const outcome of ['success','network','unauthorized']) {
    const e=environment(new Map(),true,true),token=memberToken(1);e.storage.set('token',token);
    let unauthorized=0;
    const ui=mount(e.load('pages/ai/components/AIConversationPanel.uvue').default,{sessionToken:token,onUnauthorized:()=>unauthorized++});
    const first={id:1,title:'先前会话',archived:false,version:1,updatedAt:now,createdAt:now,lastSeq:0};
    const second={...first,id:2,title:'当前会话'};
    const take=path=>{const i=e.requests.findIndex(r=>r.url===path);assert.ok(i>=0,'缺少请求：'+path);return e.requests.splice(i,1)[0]};
    e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),{items:[second,first],hasMore:false,nextBeforeId:0});await flush();
    ui.click('打开会话 #1');e.reply(take('/api/v1/ai/conversations/1'),first);await flush();
    const late=take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0');
    ui.click('打开会话 #2');e.reply(take('/api/v1/ai/conversations/2'),second);await flush();
    e.reply(take('/api/v1/ai/conversations/2/messages?limit=20&after_seq=0'),{conversationId:2,items:[],upperSeq:0,nextAfterSeq:0,hasMore:false});await flush();
    if(outcome==='network')late.fail();
    else if(outcome==='unauthorized')e.reply(late,null,401,40100);
    else e.reply(late,{conversationId:1,items:[{seq:1,kind:'question',subjectType:'theme',subjectId:7,skill:'material_qa',question:'旧会话的私有问题',taskId:0,createdAt:now}],upperSeq:1,nextAfterSeq:1,hasMore:false});
    await flush();
    assert.equal(ui.state.selected.id,2,outcome+'：旧消息不能改变当前会话');
    assert.equal(ui.state.messages.length,0);assert.equal(ui.state.loadingMessages,false);
    assert.equal(ui.state.message,'');assert.equal(unauthorized,0,'过期请求不能使当前会话退出登录');
    assert.ok(!ui.text().includes('旧会话的私有问题'));assert.equal(e.requests.length,0);
    ui.app.unmount();
  }
  console.log('通过：同账号切换会话后，旧消息成功、网络失败及401迟到均不能覆盖新会话或触发退出。');
}

async function pointsBalanceContract() {
  for(const mode of ['ok','zero','negative','fraction','overflow','time','null','array','failure','network']) {
    const e=environment();e.storage.set('token',memberToken(1));
    const api=e.load('pages/libs/ai-balance-api.uts').AIBalanceApi;
    const promise=api.read();const req=e.requests.shift();assert.equal(req.url,'/api/v1/ai/points/balance');assert.equal(req.method,'GET');assert.equal(req.header.Authorization,'Bearer '+memberToken(1));
    let data={availablePoints:13,frozenPoints:7,checkedAt:now};
    if(mode==='zero')data={...data,availablePoints:0,frozenPoints:0};
    if(mode==='negative')data.availablePoints=-1;if(mode==='fraction')data.frozenPoints=0.1;if(mode==='overflow')data.availablePoints=9007199254740992;if(mode==='time')data.checkedAt=0;
    if(mode==='null')data=null;if(mode==='array')data=[];
    if(mode==='network')req.fail();else if(mode==='failure')e.reply(req,data,503,50302);else e.reply(req,data);
    const result=await promise;
    if(mode==='ok'||mode==='zero'){assert.equal(result.code,0);assert.deepEqual(clone(result.data),data)}else{assert.notEqual(result.code,0);assert.equal(result.data,null);assert.equal(result.message,'积分余额暂不可用')}
    assert.equal(e.requests.length,0);
  }
  console.log('通过：新余额固定只读接口、Bearer、零余额、非法数字/时点/响应与网络失败拒绝。');
}

async function conversationRefreshDuringEdit() {
  for(const mode of ['draft_before_refresh','pristine_refresh','archived_refresh','rename_ok','rename_error','rename_network','rename_unauthorized','archive_cancel','archive_ok','archive_network','switch_account','unmount']) {
  const token=memberToken(1),e=environment(new Map(),true,true);e.storage.set('token',token);
  const archiveStates=[];
  const props=vue.reactive({sessionToken:token,refreshKey:0,onArchived:value=>archiveStates.push(value)});
  const component=e.load('pages/ai/components/AIConversationPanel.uvue').default;
  const ui=mount(component,props),c={id:1,title:'测试刷新竞争',archived:false,version:1,updatedAt:now,createdAt:now,lastSeq:0};
  const take=path=>{const i=e.requests.findIndex(r=>r.url===path);assert.ok(i>=0,'缺少实际请求：'+path);return e.requests.splice(i,1)[0]};
  const messages={conversationId:1,items:[],upperSeq:0,nextAfterSeq:0,hasMore:false};
  e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),{items:[c],hasMore:false,nextBeforeId:0});await flush();
  ui.click('打开会话 #1');e.reply(take('/api/v1/ai/conversations/1'),c);await flush();
  e.reply(take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0'),messages);await flush();
  if(mode.endsWith('_refresh')) {
    const draft='尚未保存的会话标题';
    if(mode==='draft_before_refresh')ui.input(draft,'新的会话标题');
    ui.app._instance.props.refreshKey=1;await flush();
    const refresh=take('/api/v1/ai/conversations/1');assert.equal(refresh.method,'GET');
    assert.throws(()=>ui.input('刷新中不允许编辑','新的会话标题'),/找不到可编辑/);
    e.reply(refresh,{...c,title:'另一页已保存的标题',archived:mode==='archived_refresh',version:2,updatedAt:now+1});await flush();
    e.reply(take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0'),messages);await flush();
    assert.equal(ui.state.selected.title,'另一页已保存的标题');assert.equal(ui.state.selected.version,2);
    assert.equal(ui.state.renameTitle,mode==='draft_before_refresh'?draft:'另一页已保存的标题');
    assert.equal(archiveStates.at(-1),mode==='archived_refresh','历史刷新必须同步父页面的报告追问只读状态');
    assert.equal(e.requests.length,0,'同步历史不能自动保存草稿');
    const next=memberToken(2);e.storage.set('token',next);ui.app._instance.props.sessionToken=next;await flush();
    e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),{items:[],hasMore:false,nextBeforeId:0});await flush();
    assert.equal(ui.state.renameTitle,'');assert.equal(ui.state.selected,null);ui.app.unmount();continue;
  }
  const archiving=mode.startsWith('archive');let request;
  if(archiving){ui.click('归档当前会话');assert.equal(e.modals.length,1)}
  else{ui.state.renameTitle='测试刷新后的标题';ui.click('保存会话标题');request=take('/api/v1/ai/conversations/1');assert.equal(request.method,'PATCH')}
  // 多次保存通知只合并成一次只读刷新，不能抢占当前写请求或归档确认。
  for(const key of [1,2]){ui.app._instance.props.refreshKey=key;await flush();assert.equal(e.requests.length,0,mode+'：交互结束前不发历史GET')}
  const renamed={...c,title:'测试刷新后的标题',version:2,updatedAt:now+1};
  if(mode==='unmount'){ui.app.unmount();e.reply(request,renamed);await flush();assert.equal(e.requests.length,0);continue}
  if(mode==='switch_account') {
    const next=memberToken(2);e.storage.set('token',next);ui.app._instance.props.sessionToken=next;await flush();
    e.reply(request,renamed);e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),{items:[],hasMore:false,nextBeforeId:0});await flush();
    assert.equal(ui.state.selected,null);assert.equal(ui.state.busy,false);assert.equal(e.requests.length,0);ui.app.unmount();continue;
  }
  if(archiving){
    e.modals.shift().success({confirm:mode!=='archive_cancel'});await flush();
    if(mode!=='archive_cancel'){request=take('/api/v1/ai/conversations/1/archive');assert.equal(request.method,'POST');if(mode==='archive_network')request.fail();else e.reply(request,{...c,archived:true,version:2,updatedAt:now+1})}
  } else if(mode==='rename_network')request.fail();
  else if(mode==='rename_error')e.reply(request,null,409,40901);
  else if(mode==='rename_unauthorized')e.reply(request,null,401,40100);
  else e.reply(request,renamed);
  await flush();assert.equal(ui.state.busy,false,mode+'：不能永久锁住会话');
  if(mode==='rename_ok'||mode==='archive_cancel') {
    assert.equal(e.requests.length,1,mode+'：通知合并为一次GET');
    const late=take('/api/v1/ai/conversations/1');assert.equal(late.method,'GET');
    assert.throws(()=>ui.click('保存会话标题'),/按钮被禁用/);assert.throws(()=>ui.click('归档当前会话'),/按钮被禁用/);
    ui.state.rename();ui.state.archive();await flush();assert.equal(e.requests.length,0,'刷新期间直接调用也不能发写请求');
    e.reply(late,renamed);await flush();
    e.reply(take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0'),messages);await flush();
    assert.equal(ui.state.selected.title,renamed.title);
  } else assert.equal(ui.state.selected,null,mode+'：失败/归档/无权后不能恢复旧详情');
  assert.equal(e.requests.length,0);ui.app.unmount();
  }
  console.log('通过：会话历史刷新与改名/归档交错，合并只读通知、反向写入门禁、错误/取消/401/换账号/卸载不锁死或恢复旧状态。');
}

async function conversationResponsiveLayout() {
  {
    const e=environment(new Map(),true,true),token=memberToken(1);e.storage.set('token',token);
    const ui=mount(e.load('pages/ai/components/AIConversationPanel.uvue').default,{sessionToken:token});
    const c={id:1,title:'测试历史问题入口',archived:false,version:2,updatedAt:now,createdAt:now,lastSeq:1};
    e.reply(e.requests.shift(),{items:[c],hasMore:false,nextBeforeId:0});await flush();
    ui.click('打开会话 #1');e.reply(e.requests.shift(),c);await flush();
    e.reply(e.requests.shift(),{conversationId:1,items:[{seq:1,kind:'question',subjectType:'theme',subjectId:7,skill:'material_qa',question:'资料有哪些分类？',taskId:0,createdAt:now}],upperSeq:1,nextAfterSeq:1,hasMore:false});await flush();
    ui.click('深度资料整理');await flush();assert.equal(ui.state.activeSkill,'deep');
    const intents=e.requests.shift();assert.ok(intents.url.includes('/task-intents'));e.reply(intents,{items:[],hasMore:false,nextBeforeId:0});await flush();
    ui.click('查询问题状态 #1');await flush();
    assert.equal(ui.state.activeSkill,'material_qa','历史问题入口必须展示对应问答区，不能只在隐藏面板读取');
    assert.equal(e.requests.length,1);const read=e.requests.shift();assert.equal(read.method,'GET');assert.equal(read.url,'/api/v1/ai/conversations/1/questions/1');
    e.reply(read,{seq:1,state:'queued',subjectType:'theme',subjectId:7,skill:'material_qa',question:'资料有哪些分类？',createdAt:now,updatedAt:now});await flush();
    assert.equal(e.requests.length,0);assert.ok(ui.text().includes('问题 #1：资料有哪些分类？'));ui.app.unmount();
  }
  const e=environment(new Map(),true,true),token=memberToken(1);e.storage.set('token',token);
  const ui=mount(e.load('pages/ai/components/AIConversationPanel.uvue').default,{sessionToken:token});
  const c={id:1,title:'测试布局会话',archived:false,version:1,updatedAt:now,createdAt:now,lastSeq:0};
  e.reply(e.requests.shift(),{items:[c],hasMore:false,nextBeforeId:0});await flush();
  ui.input('未提交的标题');await flush();
  assert.equal(ui.state.wide,true);assert.equal(e.resizeListeners.size,0);
  ui.app._instance.props.windowWidth=375;await flush();ui.click('会话历史 / 新建');await flush();
  assert.equal(ui.state.historyVisible,true);assert.equal(ui.state.title,'未提交的标题');assert.equal(e.requests.length,0);
  ui.click('打开会话 #1');e.reply(e.requests.shift(),c);await flush();
  assert.equal(ui.state.historyVisible,false);
  e.reply(e.requests.shift(),{conversationId:1,items:[],upperSeq:0,nextAfterSeq:0,hasMore:false});await flush();
  ui.input('尚未保存的改名','新的会话标题');await flush();
  const original=ui.state.selected;
  const readingNodes=n=>[...(n.props?.class==='conversation-main'?[n]:[]),...n.children.flatMap(readingNodes)];
  for(const docked of [true,false,true,false]) {
    ui.app._instance.props.reportInputActive=docked;await flush();
    const reading=readingNodes(ui.tree)[0];
    assert.ok(reading,'固定输入仍复用原阅读区');
    assert.equal(JSON.stringify(reading.props.style).includes('240px'),docked,'固定输入须缩小可滚动视口而不是只加内容尾部留白');
    assert.equal(ui.state.selected,original);assert.equal(ui.state.renameTitle,'尚未保存的改名');
    assert.equal(e.requests.length,0,'输入避让不得重新请求任务或报告');
  }
  ui.click('回到底部');assert.equal(ui.state.scrollTarget,'');await flush();assert.equal(ui.state.scrollTarget,'ai-conversation-bottom');
  ui.click('回到底部');assert.equal(ui.state.scrollTarget,'');await flush();assert.equal(ui.state.scrollTarget,'ai-conversation-bottom');
  assert.equal(ui.state.selected,original);assert.equal(ui.state.renameTitle,'尚未保存的改名');assert.equal(e.requests.length,0,'回到底部不得发送接口或丢失草稿');
  ui.click('技能与范围');await flush();assert.equal(ui.state.skillsVisible,true);
  assert.equal(e.requests.length,0,'展开技能抽屉不得请求报价或创建任务');
  ui.app._instance.props.windowWidth=1280;await flush();assert.equal(ui.state.wide,true);assert.equal(ui.state.selected,original);
  assert.equal(ui.state.renameTitle,'尚未保存的改名');assert.equal(e.requests.length,0,'布局切换不得重发详情、问答或意图请求');
  ui.app._instance.props.windowWidth=375;await flush();ui.click('会话历史 / 新建');await flush();
  ui.app._instance.props.activeTab='tasks';await flush();assert.equal(ui.state.historyVisible,false);assert.equal(ui.state.selected,original);
  assert.equal(ui.state.skillsVisible,false,'离开会话页签关闭技能抽屉');
  assert.equal(ui.state.scrollTarget,'','切换页签清理滚动目标');
  ui.app._instance.props.activeTab='conversation';await flush();ui.click('会话历史 / 新建');await flush();
  ui.click('技能与范围');await flush();assert.equal(ui.state.skillsVisible,true);
  const next=memberToken(2);e.storage.set('token',next);ui.app._instance.props.sessionToken=next;await flush();
  assert.equal(ui.state.historyVisible,false);assert.equal(ui.state.title,'');assert.equal(ui.state.selected,null);
  assert.equal(ui.state.skillsVisible,false,'切换会员关闭技能抽屉');
  assert.equal(ui.state.scrollTarget,'','切换会员清理滚动目标');
  e.reply(e.requests.shift(),{items:[],hasMore:false,nextBeforeId:0});await flush();
  assert.ok(!ui.text().includes(c.title));assert.equal(e.requests.length,0);
  ui.app.unmount();assert.equal(e.resizeListeners.size,0);e.resize(1280);assert.equal(e.requests.length,0);
  // 进度/报告经插槽保留同一个组件实例，改变历史展示不能重建订阅或丢失草稿。
  const slotsEnv=environment(new Map(),true,true);slotsEnv.storage.set('token',token);
  const panel=slotsEnv.load('pages/ai/components/AIConversationPanel.uvue').default;
  let mounted=0,unmounted=0;
  const resultProbe={setup(){vue.onMounted(()=>mounted++);vue.onBeforeUnmount(()=>unmounted++);return()=>vue.h('text','仅测试报告实例')}};
  const tab=vue.ref('conversation');
  const layoutWidth=vue.ref(1280);
  const slotsUI=mount({render:()=>vue.h(panel,{sessionToken:token,activeTab:tab.value,windowWidth:layoutWidth.value},{result:()=>vue.h(resultProbe),tasks:()=>vue.h('text','仅测试全部任务')})});
  slotsEnv.reply(slotsEnv.requests.shift(),{items:[],hasMore:false,nextBeforeId:0});await flush();
  const scrollNodes=n=>[...(n.tag==='scroll-view'?[n]:[]),...n.children.flatMap(scrollNodes)];
  for(const width of [375,960,959,1280]){
    layoutWidth.value=width;await flush();assert.equal(mounted,1);assert.equal(unmounted,0);
    const scrolls=scrollNodes(slotsUI.tree);
    assert.equal(scrolls.filter(n=>n.props.class==='conversation-main').length,1,'消息和报告始终共用一个独立滚动区');
    assert.equal(scrolls.filter(n=>n.props.class==='conversation-history-scroll').length,width>=960?1:0,'桌面历史独立滚动，窄屏改用原抽屉');
  }
  tab.value='tasks';await flush();tab.value='conversation';await flush();
  assert.equal(mounted,1);assert.equal(unmounted,0);assert.equal(slotsEnv.requests.length,0);
  slotsUI.app.unmount();assert.equal(unmounted,1);assert.equal(slotsEnv.resizeListeners.size,0);
  // POST在途和结果未知时切换断点，仍只允许核查同一原键；不把展示组件重建当作再次创建许可。
  const pendingEnv=environment(new Map(),true,true);pendingEnv.storage.set('token',token);
  const pendingUI=mount(pendingEnv.load('pages/ai/components/AIConversationPanel.uvue').default,{sessionToken:token});
  pendingEnv.reply(pendingEnv.requests.shift(),{items:[],hasMore:false,nextBeforeId:0});await flush();
  pendingUI.input('测试断点未知收据');await flush();pendingUI.click('新建会话');await flush();
  assert.equal(pendingEnv.requests.length,1);const originalPost=pendingEnv.requests.shift();assert.equal(originalPost.method,'POST');
  const originalKey=pendingUI.state.pending.key;
  const partition=pendingEnv.load('pages/libs/ai-conversation.uts').conversationPartition(token);
  const stored=pendingEnv.storage.get(partition);
  for(const width of [375,1280,375]){pendingUI.app._instance.props.windowWidth=width;await flush();assert.equal(pendingUI.state.pending.key,originalKey);assert.equal(pendingEnv.storage.get(partition),stored)}
  assert.equal(pendingEnv.requests.length,0);originalPost.fail();await flush();
  assert.equal(pendingUI.state.busy,false);assert.equal(pendingUI.state.pending.key,originalKey);
  pendingUI.click('核查原会话');await flush();const lookup=pendingEnv.requests.shift();assert.equal(lookup.method,'GET');
  assert.equal(lookup.url,'/api/v1/ai/conversation-receipt');assert.equal(lookup.header['Idempotency-Key'],originalKey);assert.equal(originalPost.header['Idempotency-Key'],originalKey);
  pendingEnv.reply(lookup,null,404,40403);await flush();
  assert.equal(pendingUI.state.pending.key,originalKey);assert.equal(pendingEnv.storage.get(partition),stored);assert.equal(pendingEnv.requests.length,0);
  pendingUI.app.unmount();assert.equal(pendingEnv.resizeListeners.size,0);
  console.log('通过：375/1280布局切换保留标题、当前会话和未保存改名，零重复请求；开会话/换页签/换账号关闭抽屉，卸载解绑窗口监听。');
}

async function conversationCreatedList() {
  for(const mode of ['success','list_error','unauthorized','switch_account','unmount']) {
  const token=memberToken(1),e=environment(new Map(),true,true);e.storage.set('token',token);
  const tasks=[],ui=mount(e.load('pages/ai/components/AIConversationPanel.uvue').default,{sessionToken:token,onTask:id=>tasks.push(id)});
  const take=path=>{const i=e.requests.findIndex(r=>r.url===path);assert.ok(i>=0,'缺少实际请求：'+path);return e.requests.splice(i,1)[0]};
  const c={id:1,title:'服务端核实的新会话',archived:false,version:2,updatedAt:now,createdAt:now,lastSeq:1};
  assert.equal(e.load('pages/libs/ai-conversation.uts').isAIConversation(c,1),true,'先验证测试详情满足原版本/序号契约');
  e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),{items:[],hasMore:false,nextBeforeId:0});await flush();
  ui.input('待创建标题');await flush();ui.click('新建会话');await flush();const created=take('/api/v1/ai/conversations');assert.equal(created.method,'POST');
  e.reply(created,{id:1,createdAt:now});await flush();e.reply(take('/api/v1/ai/conversations/1'),c);await flush();
  assert.ok(!ui.text().includes('暂无会话，可填写标题新建。'),'当前会话已由服务端核实，不能同时显示暂无会话');
  const listing=take('/api/v1/ai/conversations?limit=20&archived=0');assert.equal(listing.method,'GET');
  assert.ok(!ui.text().includes('打开会话 #1'),'不能仅凭创建收据补造列表摘要');
  const history=take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0');
  const messages={conversationId:1,items:[{seq:1,kind:'task',subjectType:'theme',subjectId:7,skill:'light',question:'',taskId:9,createdAt:now}],upperSeq:1,nextAfterSeq:1,hasMore:false};
  if(mode==='unmount'){ui.app.unmount();e.reply(listing,{items:[c],hasMore:false,nextBeforeId:0});e.reply(history,messages);await flush();assert.equal(e.requests.length,0);continue}
  if(mode==='switch_account') {
    const next=memberToken(2);e.storage.set('token',next);ui.app._instance.props.sessionToken=next;await flush();
    e.reply(listing,{items:[c],hasMore:false,nextBeforeId:0});e.reply(history,messages);e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),{items:[],hasMore:false,nextBeforeId:0});await flush();
    assert.equal(ui.state.selected,null);assert.equal(ui.state.rows.length,0);assert.ok(!ui.text().includes(c.title));assert.equal(e.requests.length,0);ui.app.unmount();continue;
  }
  if(mode==='unauthorized')e.reply(listing,null,401,40100);
  else if(mode==='list_error')e.reply(listing,null,503,50302);
  else e.reply(listing,{items:[c],hasMore:false,nextBeforeId:0});
  e.reply(history,messages);await flush();
  if(mode==='unauthorized'){assert.equal(ui.state.selected,null);assert.equal(ui.state.rows.length,0);assert.ok(!ui.text().includes(c.title))}
  else{assert.equal(ui.state.selected.id,1);assert.equal(ui.state.rows.length,mode==='success'?1:0);assert.ok(!ui.text().includes('暂无会话，可填写标题新建。'));ui.click('查看关联任务 #9');assert.deepEqual(tasks,[9])}
  assert.equal(e.requests.length,0);ui.app.unmount();
  }
  console.log('通过：新建原收据→独立详情/真实列表刷新，不补造摘要、不显示矛盾空态；列表失败保留当前会话，401/换账号/卸载拒绝迟到摘要及消息。');
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
  // 弹窗未出现时可退出等待；旧弹窗的迟到成功/失败不能干扰下一次确认。
  {
    const {e,ui}=await start({pending:pending()});
    ui.click('查询本机原意图');e.reply(e.requests.shift(),dto());await flush();
    ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),quote());await flush();
    const original=e.storage.get(partition);
    ui.click('确认会话报价并创建');const stale=e.modals.shift();await flush();
    ui.click('取消本次确认等待');await flush();
    assert.equal(e.storage.get(partition),original);assert.equal(e.requests.length,0);
    assert.ok(!ui.text().includes('确认会话报价并创建'));
    stale.success({confirm:true});stale.fail();await flush();assert.equal(e.requests.length,0);
    ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),quote());await flush();
    ui.click('确认会话报价并创建');const fresh=e.modals.shift();await flush();
    stale.success({confirm:true});stale.fail();await flush();
    assert.ok(ui.text().includes('取消本次确认等待'));assert.equal(e.requests.length,0);
    fresh.success({confirm:true});fresh.success({confirm:true});await flush();
    assert.equal(e.requests.length,1);assert.ok(e.requests[0].url.endsWith('/3/confirm'));
    e.reply(e.requests.shift(),dto('linked'));await flush();ui.app.unmount();
  }
  // Prepare/Confirm未知、重载、关闭、变价、跨用户、离页、归档和迟到回包均不补交。
  // 原详情同步列表，分页重叠不重复、不覆盖较新终态，游标仍使用服务端响应。
  {
    const {e,ui,props}=await start({pending:pending()});
    ui.click('查询会话任务意图');e.reply(e.requests.shift(),{items:[item(5)],hasMore:true,nextBeforeId:5});await flush();
    ui.click('查询本机原意图');e.reply(e.requests.shift(),dto('linked'));await flush();
    assert.ok(!ui.text().includes('当前会话暂无任务意图'));assert.ok(ui.text().includes('核对意图 #3'));assert.equal(e.requests.length,0);
    ui.click('加载更多意图');const next=e.requests.shift();assert.ok(next.url.includes('before_id=5'));
    e.reply(next,page([item(3),item(2)]));await flush();
    assert.equal(ui.text().split('核对意图 #3').length-1,1);
    // 清除本机保护后，列表仍保留真实关联状态。
    ui.click('核对终态并记录完成');e.reply(e.requests.shift(),dto('linked'));await flush();
    assert.ok(ui.text().includes('核对意图 #3'));assert.equal(ui.text().split('原任务已关联；报告仍须单独核对').length-1,1);
    props.conversationId=2;await flush();e.reply(e.requests.shift(),page());await flush();
    assert.ok(!ui.text().includes('核对意图 #3'));assert.ok(ui.text().includes('当前会话暂无任务意图'));ui.app.unmount();
  }
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
  for (const opening of ['after_messages','before_detail','before_messages']) {
    const e=environment(new Map(),true,true);e.storage.set('token',token);const ui=mount(e.load('pages/ai/index.uvue').default);
    const take=path=>{const i=e.requests.findIndex(r=>r.url===path);assert.ok(i>=0,'缺少实际请求：'+path);return e.requests.splice(i,1)[0]};
    e.hooks.show.forEach(fn=>fn());e.reply(take('/api/v1/ai/tasks?limit=20'),page());await flush();
    const c={id:1,title:'测试会话纵向',archived:false,version:1,updatedAt:now,createdAt:now,lastSeq:0};
    e.reply(take('/api/v1/ai/conversations?limit=20&archived=0'),page([{id:1,title:c.title,archived:false,version:1,updatedAt:now}]));await flush();ui.click('打开会话 #1');e.reply(take('/api/v1/ai/conversations/1'),c);await flush();
    e.reply(take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0'),{conversationId:1,items:[],upperSeq:0,nextAfterSeq:0,hasMore:false});ui.click('轻度资料整理');await flush();e.reply(take('/api/v1/ai/conversations/1/task-intents?limit=20'),page());await flush();assert.equal(ui.state.conversationId,1);assert.ok(!ui.text().includes('新建轻度资料整理'));
    // 定位共用报价组件，避免会话标题输入框与题材输入框混用。
    const fields=[];function walk(n){if(n.tag==='x-input'&&n.props.placeholder==='输入题材名称，仅用于查询目录')fields.push(n);n.children.forEach(walk)}walk(ui.tree);assert.equal(fields.length,1);fields[0].props['on:update:modelValue']('测试题材');await flush();
    ui.click('查询题材');e.reply(e.requests.shift(),[{id:7,name:'测试示例题材'}]);await flush();ui.click('核对该题材报价');e.reply(e.requests.shift(),quote());await flush();ui.click('保存会话报价意图（不冻结积分）');await flush();e.reply(e.requests.shift(),dto());await flush();ui.click('重新核对报价与资料范围');e.reply(e.requests.shift(),quote());await flush();ui.click('确认会话报价并创建');e.modals.shift().success({confirm:true});await flush();e.reply(e.requests.shift(),dto('linked'));await flush();
    if(opening==='before_detail'){ui.click('查看会话关联任务 #9');e.reply(take('/api/v1/ai/tasks/9'),progress());await flush();assert.equal(ui.state.selectedId,9)}
    e.reply(take('/api/v1/ai/conversations/1'),{...c,version:2,lastSeq:1});await flush();
    if(opening==='before_messages'){ui.click('查看会话关联任务 #9');e.reply(take('/api/v1/ai/tasks/9'),progress());await flush();assert.equal(ui.state.selectedId,9)}
    e.reply(take('/api/v1/ai/conversations/1/messages?limit=20&after_seq=0'),{conversationId:1,items:[{seq:1,kind:'task',subjectType:'theme',subjectId:7,skill:'light',question:'',taskId:9,createdAt:now}],upperSeq:1,nextAfterSeq:1,hasMore:false});await flush();
    if(opening==='after_messages'){ui.click('查看会话关联任务 #9');e.reply(take('/api/v1/ai/tasks/9'),progress());await flush()}
    assert.equal(ui.state.selectedId,9,opening+'：刷新历史不能抹除已选择任务');assert.equal(ui.state.progress.id,9);
    ui.click('核对并查看报告');e.reply(take('/api/v1/ai/tasks/9/report'),report());await flush();assert.equal(ui.state.report.taskId,9);assert.equal(ui.state.conversationId,1);assert.equal(e.requests.length,0);
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
  // 切换技能后恢复原意图：展示、复核报价及确认都绑定原类型，不按当前技能改写收费任务。
  for (const originalType of ['light','deep']) {
    const selectedType=originalType==='light'?'deep':'light';
    const originalPoints=originalType==='light'?7:37;
    const stored={...pending(),analysisType:originalType,pricePoints:originalPoints};
    const originalDto={...dto(),analysisType:originalType,pricePoints:originalPoints};
    const {e,ui}=await start({analysisType:selectedType,pending:stored});
    const originalLabel=originalType==='deep'?'深度资料整理':'轻度资料整理';
    assert.ok(ui.text().includes('待核查原意图：'+originalLabel));
    assert.ok(!ui.text().includes('30 积分报价'));
    ui.click('查询本机原意图'); e.reply(e.requests.shift(),originalDto); await flush();
    assert.ok(ui.text().includes('原意图类型：'+originalLabel));
    ui.click('重新核对报价与资料范围'); const review=e.requests.shift();
    assert.equal(review.data.analysis_type,originalType);
    e.reply(review,quote({analysisType:originalType,pricePoints:originalPoints,availablePoints:50}));await flush();
    ui.click('确认会话报价并创建');const modal=e.modals.shift();
    assert.ok(modal.title.includes(originalType==='deep'?'深度':'轻度'));
    assert.ok(modal.content.includes('冻结 '+originalPoints+' 积分'));
    modal.success({confirm:true});await flush();const req=e.requests.shift();
    assert.ok(req.url.endsWith('/3/confirm'));assert.equal(e.requests.length,0);
    e.reply(req,{...originalDto,state:'linked',taskId:9,messageSeq:1,updatedAt:now+1});await flush();
    assert.equal(JSON.parse(e.storage.get(partition)).analysisType,originalType);
    assert.equal(JSON.parse(e.storage.get(partition)).confirmSent,true);
    ui.app.unmount();
  }
  console.log('通过：会话意图六接口/闭集DTO、原键单次确认、搜索报价到关联任务；轻深技能切换恢复仍绑定原类型及实际报价。');
}

let completed = false, currentCase = '';
process.once('beforeExit', () => {
  if (!completed) { console.error('源码验收尚未完成，不得以退出0计通过：'+currentCase); process.exitCode=1; }
});
(async () => {
  for (const check of [validators, conversationResponsiveLayout, conversationCreatedList, conversationTaskFrontend, api, historyReportPermission, taskOutcomeFeedback, page, panel, reportPresentation, reportTools, feedbackContracts, feedbackPanel, entry, submissionContracts, submissionPage, memberFlow, creationHistoryFlow, cancellationContracts, cancellationPanel, cancellationMemberFlow, shellTabs, reportQuestionPageAccess, conversationFrontend, conversationRefreshDuringEdit]) {
    currentCase=check.name; await check();
  }
  currentCase='conversationLateMessages'; await conversationLateMessages();
  currentCase='pointsBalanceContract'; await pointsBalanceContract();
  completed=true;
  console.log('本脚本不覆盖：真实 UTS/uni-app x 整体构建、TMUI实现与浏览器布局；须另做实际H5验收，不能因依赖已经存在而升级源码测试证据。');
})().catch(error => { completed=true; console.error(error); process.exitCode = 1; });
