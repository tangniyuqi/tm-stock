// 执行实际后台Vue/API源码；模拟网络/Element标签不替代真实浏览器联调。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
const root = path.resolve(__dirname, '../..');
if (!process.argv[2]) throw new Error('需要隔离依赖目录');
const dep = createRequire(path.join(path.resolve(process.argv[2]), 'package.json'));
const ts = dep('typescript'), compiler = dep('@vue/compiler-sfc'), vue = dep('vue');
const stamp = 1792480000000;
const record = (version = 1) => ({ id: 1, taskId: 9, category: 'fact', status: ['received', 'processing', 'resolved'][version - 1], resolution: version === 3 ? 'reviewed' : '', version, createdAt: stamp, updatedAt: stamp });
const detail = (version = 1) => ({ item: record(version), events: Array.from({ length: version }, (_, i) => ({ version: i + 1, actorId: i === 0 ? 0 : 7, status: record(i + 1).status, resolution: record(i + 1).resolution, at: stamp })) });
function environment(guest = false, realTransport = false) {
  const requests = [], cache = new Map();
  const notices = [], redirects = [], tokens = [];
  const user = vue.reactive({ token: guest ? '' : 'test-only-admin', userInfo: { ID: guest ? 0 : 7 } });
  user.setToken = value => { tokens.push(value); user.token = value; };
  user.ClearStorage = () => { user.token = ''; };
  function execute(source, filename) {
    if (filename === 'backend/web/src/utils/request.js') source = source.replaceAll('import.meta.env.VITE_BASE_API', JSON.stringify('/test-admin-api'));
    const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText;
    const sandbox = { exports: {}, setTimeout, clearTimeout, console,
      document: { getElementById: () => null }, window: { addEventListener() {} }, require(name) {
      if (name === '@/utils/request' && !realTransport) return { default: options => new Promise((resolve, reject) => requests.push({ ...options, resolve, reject })) };
      if (name === '@/pinia/modules/user') return { useUserStore: () => user };
      if (name === '@/router/index') return { default: { push: value => redirects.push(value) } };
      if (name === '@/utils/bus') return { emitter: { emit: (kind, value) => notices.push({ kind, ...value }) } };
      if (name === 'element-plus') return { ElMessage: value => { notices.push(value); return { close() {} }; }, ElLoading: { service: () => { throw new Error('后台反馈请求不应启动全局Loading'); } } };
      if (name.startsWith('@/')) return load('backend/web/src/' + name.slice(2) + (path.extname(name) ? '' : '.js'));
      return dep(name);
    } };
    vm.runInNewContext(output, sandbox, { filename, timeout: 2000 }); return sandbox.exports;
  }
  function load(file) {
    if (cache.has(file)) return cache.get(file);
    const source = fs.readFileSync(path.join(root, file), 'utf8'); let exports;
    if (file.endsWith('.vue')) {
      const parsed = compiler.parse(source, { filename: file }); assert.deepEqual(parsed.errors, []);
      const script = compiler.compileScript(parsed.descriptor, { id: file });
      const template = compiler.compileTemplate({ source: parsed.descriptor.template.content, filename: file, id: file, compilerOptions: { bindingMetadata: script.bindings } });
      assert.deepEqual(template.errors, []); exports = execute(script.content, file); exports.default.render = execute(template.code, file).render;
    } else exports = execute(source, file);
    cache.set(file, exports);
    if (realTransport && file === 'backend/web/src/utils/request.js') {
      // 实际Axios拦截器保留，只替换最底层adapter，禁止任何测试请求访问网络。
      exports.default.defaults.adapter = options => new Promise((resolve, reject) => requests.push({ ...options, resolve, reject, axios: true }));
    }
    return exports;
  }
  return { load, user, requests, notices, redirects, tokens,
    reply(req, data, code = 0, headers = {}) {
      assert.ok(req); const envelope = { code, data, msg: code === 7 ? '权限不足' : '不可展示的自由错误文本' };
      req.resolve(req.axios ? { status: 200, statusText: 'OK', config: req, headers, data: envelope } : envelope);
    },
    httpError(req, status, code) { assert.ok(req?.axios); req.reject(new (dep('axios').AxiosError)('测试HTTP拒绝', 'ERR_BAD_RESPONSE', req, null, { status, config: req, headers: {}, data: { code, msg: '固定错误' } })); }
  };
}
function mount(component, cachePage = false) {
  const node = (tag, text = '') => ({ tag, text, children: [], props: {}, parent: null });
  const renderer = vue.createRenderer({ createElement: tag => node(tag), createText: text => node('#text', text), createComment: text => node('#comment', text),
    insert(child, parent, anchor) { if (child.parent) child.parent.children.splice(child.parent.children.indexOf(child), 1); const index = anchor ? parent.children.indexOf(anchor) : -1; parent.children.splice(index < 0 ? parent.children.length : index, 0, child); child.parent = parent; },
    remove(n) { if (n.parent) n.parent.children.splice(n.parent.children.indexOf(n), 1); n.parent = null; }, setText(n, text) { n.text = text; }, setElementText(n, text) { n.text = text; n.children = []; },
    parentNode: n => n.parent, nextSibling: n => n.parent?.children[n.parent.children.indexOf(n) + 1] || null, patchProp(n, key, old, value) { n.props[key] = value; }
  });
  const visible = vue.ref(true);
  const shell = cachePage ? { setup: () => () => vue.h(vue.KeepAlive, null, { default: () => visible.value ? vue.h(component) : vue.h('div', '已离开反馈后台') }) } : component;
  const tree = node('root'), app = renderer.createApp(shell);
  for (const tag of ['el-button', 'el-input', 'el-select', 'el-option']) app.component(tag, { inheritAttrs: false, setup(_, ctx) { return () => vue.h(tag, ctx.attrs, ctx.slots.default?.()); } });
  app.mount(tree);
  const text = n => (n.tag === '#comment' ? '' : n.text) + n.children.map(text).join(''); const all = n => [n, ...n.children.flatMap(all)];
  return { app, state: app._instance.setupState, activate: value => { visible.value = value; }, text: () => text(tree), click(label) { const button = all(tree).find(n => n.tag === 'el-button' && text(n) === label); assert.ok(button, label); assert.ok(!button.props.disabled, '按钮被禁用：' + label); button.props.onClick(); },
    model(label, value) { const input = all(tree).find(n => n.props['aria-label'] === label); assert.ok(input, label); assert.ok(!input.props.disabled); input.props['onUpdate:modelValue'](value); input.props.onChange?.(value); } };
}
const flush = async () => { for (let i = 0; i < 8; i++) { await Promise.resolve(); await vue.nextTick(); } };
async function contracts() {
  const routes = fs.readFileSync(path.join(root, 'backend/web/src/router/index.js'), 'utf8');
  assert.ok(routes.includes("path: '/ai-feedback'")); assert.ok(routes.includes("@/view/quant/aiFeedback/index.vue"));
  const permission = fs.readFileSync(path.join(root, 'backend/web/src/permission.js'), 'utf8');
  assert.ok(permission.includes("const WHITE_LIST = ['Login', 'Init']"), '不能将反馈后台加入免登录白名单');
  const retired = fs.readFileSync(path.join(root, 'backend/web/src/view/superAdmin/authority/components/datas.vue'), 'utf8');
  assert.ok(retired.includes('旧版角色关系设置已停用'));
  assert.ok(!retired.includes('from \'@/api/authority\'')); assert.ok(!retired.includes('<el-button'));
  const e = environment(), api = e.load('backend/web/src/api/quant/aiFeedback.js');
  assert.equal(typeof e.load('backend/web/src/view/superAdmin/authority/components/datas.vue').default, 'object');
  for (let i = 1; i <= 3; i++) assert.equal(api.validDetail(detail(i), 1), true);
  for (const mutate of [d => d.item.id = 2, d => d.item.id = '1', d => d.item.category = 'bad', d => d.item.version = '3', d => d.item.updatedAt--, d => d.events[0].actorId = 7, d => d.events[1].status = 'resolved', d => d.events[2].resolution = 'referred', d => d.events.pop()]) { const d = detail(3); mutate(d); assert.equal(api.validDetail(d, 1), false); }
  assert.equal(api.validPage({ items: [record(), record()], nextAfterId: 0, hasMore: false }, 0), false);
  for (const id of [0, -1, '1', 9007199254740992]) assert.notEqual((await api.feedbackDetail(id)).code, 0);
  assert.notEqual((await api.feedbackDecision(1, { expectedVersion: 1, status: 'processing', resolution: '', actorId: 7 })).code, 0); assert.equal(e.requests.length, 0);
  const p = api.feedbackDecision(1, { expectedVersion: 1, status: 'processing', resolution: '' }), req = e.requests.shift(); assert.equal(req.url, '/quant/ai-feedback/1/decisions'); assert.equal(req.method, 'post'); assert.equal(JSON.stringify(req.data), '{"expectedVersion":1,"status":"processing","resolution":""}'); e.reply(req, { ...record(2), secret: '不要留在页面状态' }); const receipt = await p; assert.equal(receipt.code, 0); assert.equal(receipt.data.secret, undefined);
  const invalid = api.feedbackDetail(1); e.reply(e.requests.shift(), detail(2), 40301); assert.equal((await invalid).code, 40301);
  const error = api.feedbackDetail(1); e.requests.shift().reject({ response: { status: 401, data: { code: 7 } } }); assert.equal((await error).code, 40100);
  console.log('通过：后台反馈闭集DTO、审计历史、固定请求端点、参数拒绝、字段投影与错误脱敏。');
}
async function page() {
  for (const mode of ['pass', 'conflict', 'unknown', 'malformed', 'switch', 'unmount', 'forbidden', 'list_error']) {
    const e = environment(), ui = mount(e.load('backend/web/src/view/quant/aiFeedback/index.vue').default);
    await flush(); const get = e.requests.shift(); assert.equal(get.method, 'get'); assert.ok(ui.text().includes('正在核对'));
    e.reply(get, { items: [record()], hasMore: false, nextAfterId: 0 }, mode === 'list_error' ? 50306 : 0); await flush();
    if (mode === 'list_error') { assert.ok(!ui.text().includes('本批暂无反馈')); ui.app.unmount(); continue; }
    ui.click('查看受理单 #1'); e.reply(e.requests.shift(), detail()); await flush(); ui.click('开始处理'); await flush();
    assert.equal(e.requests.length, 0, '确认前不得写入'); assert.ok(ui.text().includes('确认处理受理单 #1'));
    ui.click('确认提交处理'); await flush(); const post = e.requests.shift(); assert.equal(post.method, 'post'); assert.equal(post.data.expectedVersion, 1);
    assert.throws(() => ui.click('核对原受理单'), /按钮被禁用/);
    if (mode === 'switch') { e.user.token = 'test-only-other'; e.user.userInfo.ID = 8; await flush(); }
    if (mode === 'unmount') ui.app.unmount();
    if (mode === 'unknown') post.reject(new Error('不得展示的传输详情')); else e.reply(post, mode === 'malformed' ? { ...record(2), id: 3 } : record(2), mode === 'conflict' ? 40904 : mode === 'forbidden' ? 40301 : 0);
    await flush(); assert.equal(e.requests.length, 0, '不得自动重试或补GET'); assert.ok(!ui.text().includes('不可展示的自由错误文本'));
    if (mode === 'unmount') continue;
    if (mode === 'switch') { assert.ok(!ui.text().includes('受理单 #1')); ui.app.unmount(); continue; }
    assert.ok(ui.text().includes('受理单 #1 需要核对')); assert.throws(() => ui.click('查询首批反馈'), /按钮被禁用/);
    ui.click('核对原受理单'); e.reply(e.requests.shift(), detail(2)); await flush(); assert.ok(ui.text().includes('当前版本 2'));
    ui.model('处理决定', 'resolved'); await flush(); ui.model('处理结果', 'reviewed'); await flush(); ui.click('核对处理结果'); await flush();
    ui.click('暂不提交'); await flush(); assert.equal(e.requests.length, 0);
    ui.click('核对处理结果'); await flush(); ui.click('确认提交处理'); e.reply(e.requests.shift(), record(3)); await flush(); assert.equal(e.requests.length, 0);
    ui.click('核对原受理单'); e.reply(e.requests.shift(), detail(3)); await flush(); assert.ok(ui.text().includes('不能重开、删除或覆盖终态')); assert.ok(ui.text().includes('不代表报告已更正')); ui.app.unmount();
  }
  const e = environment(true), ui = mount(e.load('backend/web/src/view/quant/aiFeedback/index.vue').default); await flush(); assert.equal(e.requests.length, 0); assert.ok(ui.text().includes('请先登录')); ui.app.unmount();
  console.log('通过：后台列表→详情→确认→原单查询→处理终态；冲突/超时/畸形/无权限、换账号/离页及迟到响应隔离，无自动POST。');
}
async function paginationAndLateReads() {
  const e = environment(), ui = mount(e.load('backend/web/src/view/quant/aiFeedback/index.vue').default);
  await flush();
  e.reply(e.requests.shift(), { items: [record()], hasMore: true, nextAfterId: 1 }); await flush();
  ui.click('查询下一批'); await flush();
  const next = e.requests.shift(); assert.equal(next.params.after_id, 1); assert.equal(next.params.limit, 20);
  e.reply(next, { items: [{ ...record(), id: 2 }], hasMore: false, nextAfterId: 0 }); await flush();
  assert.ok(ui.text().includes('查看受理单 #2')); assert.ok(!ui.text().includes('查看受理单 #1'));
  for (const id of ['0', '01', '-1', '1e2', '9007199254740992']) {
    ui.model('受理单编号', id); ui.click('按编号查询'); await flush(); assert.equal(e.requests.length, 0);
  }
  ui.click('查询首批反馈'); await flush(); assert.equal(e.requests[0].params.after_id, undefined);
  e.reply(e.requests.shift(), { items: [record()], hasMore: false, nextAfterId: 0 }); await flush();
  ui.click('查看受理单 #1'); await flush(); const delayedDetail = e.requests.shift();
  e.user.token = 'test-only-other'; e.user.userInfo.ID = 8; await flush();
  e.reply(delayedDetail, detail(3)); await flush();
  assert.ok(!ui.text().includes('受理单 #1')); assert.ok(!ui.text().includes('不可变处理历史')); assert.equal(e.requests.length, 0);
  ui.click('查询首批反馈'); await flush(); const delayedList = e.requests.shift(); ui.app.unmount();
  e.reply(delayedList, { items: [record()], hasMore: false, nextAfterId: 0 }); await flush(); assert.equal(e.requests.length, 0);
  console.log('通过：实际游标翻页/重查首批、非法编号零请求、换账号后的迟到详情及卸载后的迟到列表隔离。');
}
async function realRequestChain() {
  const e = environment(false, true), api = e.load('backend/web/src/api/quant/aiFeedback.js');
  let result = api.feedbackList(); await flush(); let req = e.requests.shift();
  assert.equal(req.baseURL, '/test-admin-api'); assert.equal(req.headers.get('x-token'), 'test-only-admin');
  assert.equal(req.timeout, 10000); assert.equal(req.donNotShowLoading, true);
  e.reply(req, { items: [record()], hasMore: false, nextAfterId: 0 }); assert.equal((await result).code, 0);
  result = api.feedbackList(); await flush(); e.reply(e.requests.shift(), {}, 7);
  assert.equal((await result).code, 40301, '实际Casbin的HTTP200/code7必须映射无权限，而不是提交未知');
  result = api.feedbackDetail(1); await flush(); e.httpError(e.requests.shift(), 401, 7); assert.equal((await result).code, 40100);
  const authNotice = e.notices.find(item => item.code === '401'); assert.ok(authNotice); authNotice.fn();
  assert.equal(e.user.token, ''); assert.equal(e.redirects.at(-1).name, 'Login');
  e.user.token = 'test-only-admin';
  result = api.feedbackDecision(1, { expectedVersion: 1, status: 'processing', resolution: '' }); await flush(); req = e.requests.shift();
  assert.equal(JSON.parse(req.data).expectedVersion, 1); e.httpError(req, 409, 40904); assert.equal((await result).code, 40904);
  assert.equal(e.requests.length, 0, '实际Axios链不能自动补写');
  const ui = mount(e.load('backend/web/src/view/quant/aiFeedback/index.vue').default); await flush();
  e.reply(e.requests.shift(), { items: [record()], hasMore: false, nextAfterId: 0 }); await flush();
  ui.click('查看受理单 #1'); await flush(); e.reply(e.requests.shift(), detail(), 0, { 'new-token': 'test-only-refreshed' }); await flush();
  assert.equal(e.user.token, 'test-only-refreshed'); assert.equal(e.tokens.length, 1);
  assert.ok(!ui.text().includes('受理单 #1'), '统一续签后旧轮次结果也必须丢弃');
  ui.click('查询首批反馈'); await flush(); req = e.requests.shift(); assert.equal(req.headers.get('x-token'), 'test-only-refreshed');
  e.reply(req, {}, 7); await flush(); assert.ok(ui.text().includes('未获准处理反馈')); assert.ok(!ui.text().includes('本批暂无反馈')); ui.app.unmount();
  console.log('通过：实际统一Axios请求/响应拦截器、后台令牌/前缀、Casbin拒绝、401登录清理、409无重试及new-token迟到隔离。');
}
async function actualRouterGuard() {
  const vr = dep('vue-router'), user = { token: '', userInfo: { authority: {} }, async GetUserInfo() {} };
  const menu = { path: 'aiFeedback', name: 'AiFeedbackConsole', component: { render: () => null }, meta: { title: '报告反馈处理', keepAlive: false } };
  const routerStore = { asyncRouterFlag: 0, asyncRouters: [{ path: '/layout', name: 'layout', component: { render: () => null }, children: [menu] }],
    async SetAsyncRouter() { this.asyncRouterFlag = 1; }, async handleKeepAlive() {} };
  let router, guard;
  const execute = (file, dependencies) => {
    const output = ts.transpileModule(fs.readFileSync(path.join(root, file), 'utf8'), { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 } }).outputText;
    const sandbox = { exports: {}, console, sessionStorage: { getItem: () => null, removeItem() {} }, document: { title: '', getElementById: () => null, querySelector: () => null },
      require(name) { assert.ok(name in dependencies, '未声明的路由测试依赖：' + name); return dependencies[name]; } };
    vm.runInNewContext(output, sandbox, { filename: file, timeout: 2000 }); return sandbox.exports;
  };
  router = execute('backend/web/src/router/index.js', { 'vue-router': { ...vr, createWebHashHistory: vr.createMemoryHistory } }).default;
  const beforeEach = router.beforeEach.bind(router); router.beforeEach = fn => { guard = fn; return beforeEach(fn); };
  execute('backend/web/src/permission.js', {
    '@/pinia/modules/user': { useUserStore: () => user }, '@/pinia/modules/router': { useRouterStore: () => routerStore },
    '@/utils/page': { default: title => title }, '@/router': { default: router },
    nprogress: { default: { configure() {}, start() {}, done() {}, remove() {} } }, 'nprogress/nprogress.css': {}
  });
  assert.equal(typeof guard, 'function');
  const to = () => router.resolve('/ai-feedback'), from = { name: 'Login' };
  let decision = await guard(to(), from); assert.equal(decision.name, 'Login'); assert.equal(decision.query.redirect, '/ai-feedback');
  user.token = 'test-only-admin'; decision = await guard(to(), { name: 'Other' }); assert.equal(decision.path, '/ai-feedback');
  assert.equal(router.hasRoute('AiFeedbackDirect'), true); assert.equal(router.hasRoute('AiFeedbackConsole'), true);
  assert.equal(router.resolve('/layout/aiFeedback').name, 'AiFeedbackConsole');
  assert.equal(router.resolve('/ai-feedback').name, 'AiFeedbackDirect'); assert.equal(await guard(to(), from), true);
  assert.equal(router.resolve('/layout/aiFeedback').meta.keepAlive, false);
  user.token = ''; decision = await guard(router.resolve('/layout/aiFeedback'), from); assert.equal(decision.name, 'Login');
  console.log('通过：实际Vue Router内存路由与原permission守卫，固定/侧栏入口共存、游客重定向、登录回跳和登出拒绝。');
}
async function keepAliveIsolation() {
  const e = environment(), ui = mount(e.load('backend/web/src/view/quant/aiFeedback/index.vue').default, true);
  await flush(); e.reply(e.requests.shift(), { items: [record()], hasMore: false, nextAfterId: 0 }); await flush();
  ui.click('查看受理单 #1'); await flush(); const delayed = e.requests.shift(); ui.activate(false); await flush();
  e.reply(delayed, detail()); await flush(); ui.activate(true); await flush();
  assert.ok(!ui.text().includes('受理单 #1')); assert.equal(e.requests.length, 0);
  ui.model('受理单编号', '1'); ui.click('按编号查询'); e.reply(e.requests.shift(), detail()); await flush();
  ui.click('开始处理'); await flush(); ui.activate(false); await flush(); ui.activate(true); await flush();
  assert.ok(!ui.text().includes('确认处理受理单')); assert.equal(e.requests.length, 0);
  ui.model('受理单编号', '1'); ui.click('按编号查询'); e.reply(e.requests.shift(), detail()); await flush();
  ui.click('开始处理'); await flush(); ui.click('确认提交处理'); await flush(); const post = e.requests.shift();
  ui.activate(false); await flush(); e.reply(post, record(2)); await flush(); ui.activate(true); await flush();
  assert.ok(!ui.text().includes('受理单 #1')); assert.equal(e.requests.length, 0); ui.app.unmount();
  console.log('通过：真实Vue KeepAlive停用/激活清空详情、确认草稿和未知操作；迟到GET/POST无状态恢复或自动重发。');
}
(async () => { await contracts(); await page(); await paginationAndLateReads(); await realRequestChain(); await actualRouterGuard(); await keepAliveIsolation(); console.log('仅源码宿主验收，实际Element Plus/Vite/Go/SQL闭环须另验。'); })().catch(e => { console.error(e); process.exitCode = 1; });
