// 直接执行实际问答 UTS 模块与 UVue 组件的模拟接口验收；不替代 HBuilderX 或真实浏览器/真实模型验收。
// 用法：node scripts/dev/verify-ai-qa-ui.cjs <隔离依赖目录>
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
const { webcrypto } = require('node:crypto');
const native = tag => ['view', 'text', 'scroll-view', 'x-sheet', 'x-button', 'x-navbar', 'x-input', 'x-drawer'].includes(tag);
const now = 1792281600000;
const token = 'test.' + Buffer.from(JSON.stringify({ ID: 1, UserType: 'client' })).toString('base64url') + '.test';
const excerpt = '测试示例题材：此处为自动化测试引文，非实际资料。';
const excerptBytes = Buffer.byteLength(excerpt, 'utf8');
const question = '该题材的公开定义是什么？';

const status = (over = {}) => ({ seq: 1, state: 'queued', subjectType: 'theme', subjectId: 7, skill: 'material_qa', question, createdAt: now, updatedAt: now, ...over });
const quote = (over = {}) => ({ snapshotId: 1, startByte: 0, endByte: excerptBytes, excerpt, sourceType: '公告', sourceUrl: 'https://example.test/source', collectedAt: now, ...over });
const answer = (over = {}) => ({ seq: 1, state: 'answered', template: 'AI 资料问答：以下为所选授权资料中经核验的原文引文，不是新的报告。', subjectName: '测试示例题材', aiGenerated: true, modelName: '测试模型（非真实调用）', filingNo: '测试编号（非备案证明）', quotes: [quote()], ...over });
const conversation = (over = {}) => ({ id: 3, title: '测试会话', archived: false, version: 5, lastSeq: 4, createdAt: now, updatedAt: now, ...over });

function environment() {
  const cache = new Map();
  const requests = [], modals = [], opened = [], storage = new Map(), hooks = { show: [], hide: [], unload: [] }, lockQueues = new Map();
  const uni = {
    getWindowInfo: () => ({ windowWidth: 1280 }), onWindowResize: () => {}, offWindowResize: () => {},
    getStorageSync: key => storage.get(key), setStorageSync: (key, value) => storage.set(key, value), removeStorageSync: key => storage.delete(key),
    request: options => requests.push(options), showModal: options => modals.push(options), showToast: () => {},
    setClipboardData: () => {}, navigateTo: () => {},
  };
  const globals = {
    uni, window: { atob, crypto: webcrypto, navigator: { locks: { request: (name, callback) => {
      const result = (lockQueues.get(name) || Promise.resolve()).then(callback);
      lockQueues.set(name, result.catch(() => {})); return result;
    } } }, open: url => { const target = { opener: '测试占位' }; opened.push({ url, target }); return target; } },
    onShow: fn => hooks.show.push(fn), onHide: fn => hooks.hide.push(fn), onUnload: fn => hooks.unload.push(fn),
    setTimeout: (fn, ms) => setTimeout(fn, ms), clearTimeout: id => clearTimeout(id),
    setInterval: (fn, ms) => setInterval(fn, ms), clearInterval: id => clearInterval(id),
  };
  function execute(source, filename) {
    const output = ts.transpileModule(source, { compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 }, reportDiagnostics: true });
    assert.equal(output.diagnostics.filter(d => d.category === ts.DiagnosticCategory.Error).length, 0, filename + ' 语法转换失败');
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
      assert.deepEqual(parsed.errors, [], filename + ' SFC 解析失败');
      const script = compiler.compileScript(parsed.descriptor, { id: filename });
      const template = compiler.compileTemplate({ source: parsed.descriptor.template.content, filename, id: filename, compilerOptions: { isCustomElement: native, bindingMetadata: script.bindings } });
      assert.deepEqual(template.errors, [], filename + ' 模板编译失败');
      exports = execute(script.content, filename);
      exports.default.render = execute(template.code, filename).render;
    } else exports = execute(source, filename);
    cache.set(filename, exports);
    return exports;
  }
  const reply = (request, data, statusCode = 200, code = 0) => request.success({ statusCode, data: JSON.stringify({ code, msg: '测试内部文字不得展示', data }) });
  return { load, requests, modals, opened, storage, hooks, reply, uni };
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
  return { app, tree, state: app._instance.setupState, nodes: () => all(tree), updateProps: values => Object.assign(app._instance.props, values), text: () => text(tree), input(value, placeholder) {
    const field = all(tree).find(n => n.tag === 'x-input' && !n.props.disabled && (placeholder == null || n.props.placeholder === placeholder));
    assert.ok(field, '找不到可编辑的实际输入绑定');
    field.props['on:update:modelValue'](value);
  }, click(label, occurrence = 0) {
    const button = all(tree).filter(n => n.tag === 'x-button' && text(n) === label)[occurrence];
    assert.ok(button, '找不到实际模板按钮：' + label);
    assert.ok(!button.props.disabled, '按钮被禁用：' + label);
    button.props.onClick();
  } };
}
const flush = async () => { for (let i = 0; i < 10; i++) { await Promise.resolve(); await vue.nextTick(); } };

async function validators() {
  const env = environment(), qa = env.load('pages/libs/ai-qa.uts');
  // 字段名检查不得接受带齐同名属性的数组/函数，或放过未知字段。
  const pendingShape={key:'question_0123456789abcdef',conversationId:3,version:5,subjectId:7,taskId:0,skill:'material_qa',question};
  for(const [check,value] of [[v=>qa.isAIQAStatus(v,1),status()],[v=>qa.isAIQAAnswer(v,1),answer()],[qa.isAIQAQuote,quote()],[qa.isAIPendingQA,pendingShape]]) {
    assert.equal(check(value),true);
    for(const invalid of [null,undefined,1,'test',Object.assign([],value),Object.assign(()=>{},value),{...value,unexpected:true}]) assert.equal(check(invalid),false);
    for(const key of Object.keys(value)){const missing={...value};delete missing[key];assert.equal(check(missing),false)}
  }
  assert.equal(qa.isAIQAStatus(status(), 1), true);
  assert.equal(qa.isAIQAStatus(status(), 0), true);
  const badStatus = [
    s => { s.seq = '1'; }, s => { s.seq = 9007199254740992; }, s => { s.state = 'done'; }, s => { s.subjectType = 'topic'; },
    s => { s.skill = 'other'; }, s => { s.question = ''; }, s => { s.question = ' 前后空白 '; },
    s => { s.question = '含控制字符\u0000'; }, s => { s.question = '字'.repeat(1001); }, s => { s.createdAt = -1; },
    s => { s.updatedAt = now - 1; }, s => { delete s.updatedAt; }, s => { s.extra = 1; }, s => { s.subjectId = 0; },
  ];
  for (const mutate of badStatus) { const value = status(); mutate(value); assert.equal(qa.isAIQAStatus(value, 1), false); }
  assert.equal(qa.isAIQAStatus(status({ seq: 2 }), 1), false);
  assert.equal(qa.isAIQAStatus(status({ skill: 'report_qa' }), 1), true);
  assert.equal(qa.isAIQAStatus(null, 0), false);

  assert.equal(qa.isAIQAAnswer(answer(), 1), true);
  assert.equal(qa.isAIQAAnswer(answer({ state: 'refused', aiGenerated: false, modelName: '', filingNo: '', template: qa.aiQARefusalTemplates[0], quotes: [] }), 1), true);
  const badAnswer = [
    a => { a.seq = 2; }, a => { a.template = '自由正文'; }, a => { a.aiGenerated = 'true'; }, a => { a.quotes = []; },
    a => { a.quotes = Array(5).fill(quote()); }, a => { a.modelName = ''; },
    a => { a.quotes = [quote({ endByte: excerptBytes + 1 })]; },
    a => { a.quotes = [quote({ sourceUrl: 'javascript:alert(1)' })]; },
    a => { a.quotes = [quote({ excerpt: '' })]; },
    a => { a.quotes = [quote({ startByte: -1 })]; },
    a => { a.quotes = [quote({ snapshotId: 2 }), quote({ snapshotId: 1 })]; },
    a => { a.quotes = [quote({ snapshotId: 1, startByte: 0, endByte: excerptBytes, excerpt: excerpt + 'x' })]; },
    a => { a.quotes = [quote(), quote({ snapshotId: 1 })]; },
    a => { a.state = 'failed'; },
    a => { delete a.quotes; },
  ];
  for (const mutate of badAnswer) { const value = answer(); mutate(value); assert.equal(qa.isAIQAAnswer(value, 1), false); }
  assert.equal(qa.isAIQAAnswer(answer({ state: 'refused', template: '自定义拒答' }), 1), false);

  assert.equal(qa.isAIQAQuestion(question), true);
  assert.equal(qa.isAIQAQuestion('   '), false);
  assert.equal(qa.isAIQAQuestion('字'.repeat(1001)), false);
  assert.equal(qa.qaMatchesPending(status(), { key: 'question_0123456789abcdef', conversationId: 3, version: 5, subjectId: 7, taskId: 0, skill: 'material_qa', question }), true);
  assert.equal(qa.qaMatchesPending(status(), { key: 'question_0123456789abcdef', conversationId: 3, version: 5, subjectId: 8, taskId: 0, skill: 'material_qa', question }), false);
  assert.equal(qa.qaMatchesPending(status(), { key: 'bad', conversationId: 3, version: 5, subjectId: 7, taskId: 0, skill: 'material_qa', question }), false);
  assert.equal(qa.qaMatchesPending(status({ skill: 'report_qa' }), { key: 'question_0123456789abcdef', conversationId: 3, version: 5, subjectId: 7, taskId: 9, skill: 'report_qa', question }), true);
  assert.equal(qa.qaMatchesPending(status({ skill: 'report_qa' }), { key: 'question_0123456789abcdef', conversationId: 3, version: 5, subjectId: 7, taskId: 0, skill: 'material_qa', question }), false);
  assert.equal(qa.aiSourceTypeLabel('annual_report'), '年报');
  assert.equal(qa.aiSourceTypeLabel('official_catalog'), '官方产业目录');
  assert.equal(qa.aiSourceTypeLabel('自定义'), '自定义');
  assert.equal(qa.aiQAError(40100), '登录状态失效，请重新登录后核查原问题');
  const taskLib = env.load('pages/libs/ai-conversation-task.uts');
  const deepIntent = { id: 5, conversationId: 3, analysisType: 'deep', subjectType: 'theme', subjectId: 7, priceVersion: 'v1', pricePoints: 30, state: 'prepared', rejectionCode: '', taskId: 0, messageSeq: 0, createdAt: now, updatedAt: now };
  const deepQuote = { analysisType: 'deep', subjectType: 'theme', subjectId: 7, subjectName: '示例题材', pricePoints: 30, priceVersion: 'v1', availablePoints: 100, quotedAt: now, holdExpireSec: 60, materialScope: [{ sourceType: 'announcement', sourceUrl: 'https://example.test/source', collectedAt: now }] };
  assert.equal(taskLib.intentMatchesQuote(deepIntent, deepQuote), true);
  assert.equal(taskLib.intentMatchesQuote(deepIntent, { ...deepQuote, pricePoints: 7 }), false);
  const taskApi = env.load('pages/libs/ai-conversation-task-api.uts').AIConversationTaskApi;
  const pendingDeep = { key: 'intent_0123456789abcdef', conversationId: 3, analysisType: 'deep', subjectId: 7, priceVersion: 'v1', pricePoints: 30, intentId: 0, confirmSent: false };
  const prepared = taskApi.prepare(pendingDeep);
  const prepareRequest = env.requests.shift();
  assert.equal(prepareRequest.url, '/api/v1/ai/conversations/3/task-intents');
  assert.equal(prepareRequest.data.analysis_type, 'deep');
  env.reply(prepareRequest, { code: 50001, data: null }); await prepared;
  assert.ok(!qa.aiQAError(50001).includes('内部'));
  console.log('通过：问答 DTO 闭集、字节区间、出处排序去重、固定模板与拒答模板、原问题匹配及错误脱敏。');
}

async function skillCatalog() {
  const env = environment(), m = env.load('pages/libs/ai-skills.uts');
  const list = m.aiSkills();
  assert.equal(list.length, 4);
  assert.deepEqual(Array.from(list, s => s.key), ['material_qa', 'light', 'deep', 'report_qa']);
  assert.equal(m.isAISkillUsable(m.aiSkillByKey('material_qa')), true);
  assert.equal(m.isAISkillUsable(m.aiSkillByKey('light')), true);
  assert.equal(m.isAISkillUsable(m.aiSkillByKey('deep')), true);
  assert.equal(m.isAISkillUsable(m.aiSkillByKey('report_qa')), false);
	assert.equal(m.aiSkillByKey('report_qa').state, 'report');
  assert.equal(m.isAISkillKey('deep'), true);
  assert.equal(m.isAISkillKey('other'), false);
  assert.equal(m.aiSkillByKey('other'), null);
  assert.equal(m.aiSkillStateLabel('quote'), '需报价');
  assert.equal(m.aiSkillStateLabel('unavailable'), '暂不可用');
  let picked = '';
  const ui = mount(env.load('pages/ai/components/AISkillPanel.uvue').default, { active: 'material_qa', onSelect: key => { picked = key } });
  const text = ui.text();
  assert.ok(text.includes('选择技能'));
  assert.ok(text.includes('资料问答'));
  assert.ok(text.includes('轻度资料整理'));
  assert.ok(text.includes('深度资料整理'));
  assert.ok(text.includes('报告追问'));
  assert.ok(text.includes('报告内使用'));
  assert.ok(text.includes('需报价确认'));
  assert.ok(!text.includes('30积分/次'), '技能目录不能把开发报价当作固定售价');
  assert.ok(text.includes('从报告下方发起追问'));
  const buttons = [];
  const walk = n => { if (n.tag === 'x-button') buttons.push(n); n.children.forEach(walk); };
  walk(ui.tree);
  assert.equal(buttons.filter(b => b.props.disabled === true).length, 1, '仅报告追问不可提交');
  assert.ok(buttons.some(b => b.props.disabled !== true && ui.text().includes('当前技能')));
  ui.click('轻度资料整理');
  assert.equal(picked, 'light', '可用技能应发出选择事件');
  ui.updateProps({active:'deep'}); await flush();
  assert.ok(ui.text().includes('当前技能：深度资料整理'));
  assert.ok(ui.text().includes('先核对当前题材报价，确认后冻结'));
  assert.ok(!ui.text().includes('当前技能：资料问答'));
  picked=''; ui.click('深度资料整理'); assert.equal(picked,'','当前技能不能重复发选择事件');
  ui.state.choose(m.aiSkillByKey('report_qa')); assert.equal(picked,'','报告追问不能绕过按钮进入通用提交');
  ui.updateProps({active:'unknown'}); await flush();
  assert.ok(ui.text().includes('尚未选择有效技能'));
  assert.equal(env.requests.length,0); assert.equal(env.storage.size,0);
  ui.app.unmount();
  console.log('通过：技能目录闭集、可用/需报价/暂不可用状态、不可用技能不可提交、可用技能发出选择。');
}

async function reportHistoryEntry() {
  const env = environment(); env.storage.set('token', token);
  const ui = mount(env.load('pages/ai/components/AIReportQuestionPanel.uvue').default, {sessionToken:token,conversationId:3,taskId:9,subjectId:7,archived:true,questionSeq:1});
  await flush();
  const request=env.requests.shift();
  assert.ok(request,'历史追问必须按服务端消息序号查询，不能依赖本机原键');
  assert.equal(request.method,'GET'); assert.equal(request.url,'/api/v1/ai/conversations/3/questions/1');
  env.reply(request,status({skill:'report_qa',state:'answered'})); await flush();
  assert.ok(!ui.text().includes('提交报告追问（不扣积分）'));
  ui.click('核对当前答案'); const read=env.requests.shift();
  assert.equal(read.method,'GET'); env.reply(read,answer()); await flush();
  assert.ok(ui.text().includes(excerpt));
  assert.equal(env.requests.length,0); assert.equal(env.storage.size,1,'历史读取不能写原键或令牌副本');
  ui.app.unmount();
}

async function reportPanel() {
  const env = environment(); env.storage.set('token', token);
  let saved=0;
  const ui = mount(env.load('pages/ai/components/AIReportQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, taskId: 9, subjectId: 7, archived: false, onSaved:()=>{saved++} });
  await flush();
  assert.ok(ui.text().includes('报告追问 · 任务 #9'));
  ui.input(question, '输入针对本报告的事实问题，最多1000字');
  await flush();
  for (const layout of [{allowDock:true,dockInput:true,dockLeft:324},{dockLeft:16},{dockInput:false},{allowDock:false,dockInput:true},{allowDock:true}]) {
    ui.updateProps(layout); await flush();
    assert.equal(ui.state.question,question,'报告输入布局切换保留唯一草稿');
    assert.equal(env.requests.length,0,'布局操作不请求报告、不提交问题');
  }
  assert.ok(ui.nodes().some(n=>n.props.class==='report-qa-composer-reserve'));
  assert.equal(ui.nodes().filter(n=>n.tag==='x-input'&&n.props.type==='textarea').length,1);
  ui.click('提交报告追问（不扣积分）');
  let request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3'); assert.equal(request.method, 'GET');
  env.reply(request, conversation()); await flush();
  const submit = env.requests.shift();
  assert.equal(submit.url, '/api/v1/ai/conversations/3/questions'); assert.equal(submit.method, 'POST');
  assert.equal(submit.header['Idempotency-Key'], ui.state.pending.key);
  assert.deepEqual(JSON.parse(JSON.stringify(submit.data)), { version: 5, subjectType: 'theme', subjectId: 7, skill: 'report_qa', taskId: 9, question });
  const stored = JSON.parse(env.storage.get(ui.state.partition));
  assert.equal(stored.skill, 'report_qa'); assert.equal(stored.taskId, 9);
  env.reply(submit, status({ skill: 'report_qa', state: 'answered', updatedAt: now + 1000 })); await flush();
  assert.equal(ui.state.status.skill, 'report_qa');
  assert.equal(saved,1,'提交已保存后必须通知父会话刷新消息');
  assert.ok(ui.text().includes('原问题已完成'));
  ui.click('核对当前答案');
  request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/questions/1/answer');
  const reportAnswer = () => answer({ quotes: [{ snapshotId: 1, startByte: 0, endByte: excerptBytes, excerpt, sourceType: 'annual_report', sourceUrl: 'https://example.test/source', collectedAt: now }] });
  env.reply(request, reportAnswer()); await flush();
  assert.ok(ui.text().includes(excerpt));
  assert.ok(ui.text().includes('年报'), '来源枚举应转换为中文');

  ui.click('重新核对并打开出处 #1');
  request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/questions/1/answer', '打开出处前必须重新读取并核对');
  env.reply(request, reportAnswer()); await flush();
  assert.equal(env.modals.length, 1);
  env.modals[0].success({ confirm: true }); await flush();
  assert.equal(env.opened.length, 1); assert.equal(env.opened[0].url, 'https://example.test/source');
  assert.equal(env.opened[0].target.opener, null, '外部链接不得保留 opener');
  ui.click('核实终态并清理本地原键');
  request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/question-receipt');
  env.reply(request, status({ skill: 'report_qa', state: 'answered', updatedAt: now + 1000 })); await flush();
  assert.equal(ui.state.pending, null); assert.equal(env.storage.has('tm_ai_question_v1_1'), false);
  ui.app.unmount();

  // 归档会话只允许核查；本机若另存资料问答原键，不得在报告面板另建第二个。
  const env2 = environment(); env2.storage.set('token', token);
  env2.storage.set('tm_ai_question_v1_1', JSON.stringify({ key: 'question_0123456789abcdef', conversationId: 3, version: 5, subjectId: 7, taskId: 0, skill: 'material_qa', question }));
  const ui2 = mount(env2.load('pages/ai/components/AIReportQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, taskId: 9, subjectId: 7, archived: true });
  await flush();
  assert.ok(ui2.text().includes('当前会话已归档'));
  assert.ok(!ui2.text().includes('提交报告追问（不扣积分）'));
  assert.ok(ui2.text().includes('另有一条待核查问题'));
  assert.ok(ui2.text().includes('待核查的是会话 #3 的资料问答'));
  assert.ok(!ui2.text().includes('任务 #0'));
  assert.ok(!ui2.text().includes('原报告问题'));
  assert.equal(env2.requests.length, 0, '跨技能提示不能代替用户核查或提交');
  assert.equal(JSON.parse(env2.storage.get('tm_ai_question_v1_1')).skill, 'material_qa', '提示不清除原键');
  ui2.app.unmount();

  // 401：清空私有状态并通知外层，不继续提交。
  const env3 = environment(); env3.storage.set('token', token); let unauthorized = 0;
  const ui3 = mount(env3.load('pages/ai/components/AIReportQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, taskId: 9, subjectId: 7, archived: false, onUnauthorized: () => { unauthorized++; } });
  await flush();
  ui3.input(question, '输入针对本报告的事实问题，最多1000字'); await flush();
  ui3.click('提交报告追问（不扣积分）');
  env3.reply(env3.requests.shift(), null, 401, 40101); await flush();
  assert.equal(unauthorized, 1); assert.equal(ui3.state.status, null); assert.equal(ui3.state.pending, null);
  assert.equal(env3.requests.length, 0);
  ui3.app.unmount();

  // 详情回包前切换账号：不得继续发出问题提交或展示旧账号结果。
  const env4 = environment(); env4.storage.set('token', token);
  const ui4 = mount(env4.load('pages/ai/components/AIReportQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, taskId: 9, subjectId: 7, archived: false });
  await flush();
  ui4.input(question, '输入针对本报告的事实问题，最多1000字'); await flush();
  ui4.click('提交报告追问（不扣积分）');
  const stale = env4.requests.shift();
  env4.storage.set('token', 'test.other.member');
  env4.reply(stale, conversation()); await flush();
  assert.equal(env4.requests.length, 0, '切账号后不得继续提交');
  assert.equal(ui4.state.status, null);
  ui4.app.unmount();

  // 卸载后迟到回包不得继续写页面或发起提交。
  const env5 = environment(); env5.storage.set('token', token);
  const ui5 = mount(env5.load('pages/ai/components/AIReportQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, taskId: 9, subjectId: 7, archived: false });
  await flush();
  ui5.input(question, '输入针对本报告的事实问题，最多1000字'); await flush();
  ui5.click('提交报告追问（不扣积分）');
  const late = env5.requests.shift();
  ui5.app.unmount();
  env5.reply(late, conversation()); await flush();
  assert.equal(env5.requests.length, 0, '卸载后不得继续提交');

  // 本机存储故障：失败关闭，不渲染提交入口。
  const env6 = environment(); env6.storage.set('token', token);
  const getItem = env6.uni.getStorageSync;
  env6.uni.getStorageSync = key => { if (String(key).startsWith('tm_ai_question_v1_')) throw new Error('测试存储故障'); return getItem(key); };
  const ui6 = mount(env6.load('pages/ai/components/AIReportQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, taskId: 9, subjectId: 7, archived: false });
  await flush();
  assert.ok(ui6.text().includes('本机原问题记录不可读取'));
  assert.ok(!ui6.text().includes('提交报告追问（不扣积分）'));
  assert.equal(env6.requests.length, 0);
  ui6.app.unmount();
  console.log('通过：报告追问面板固定任务、持久原键含技能/任务号、提交体与答案读取、终态清理；归档只读、401清状态、切账号/卸载丢弃迟到回包、存储故障失败关闭。');
}

async function api() {
  const env = environment(), { AIQAApi } = env.load('pages/libs/ai-qa-api.uts');
  env.storage.set('token', token);
  const key = 'question_0123456789abcdef';
  const pending = { key, conversationId: 3, version: 5, subjectId: 7, taskId: 0, skill: 'material_qa', question };
  assert.equal((await AIQAApi.submit({ ...pending, question: '' })).data, null);
  assert.equal(env.requests.length, 0);

  let promise = AIQAApi.submit(pending), request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/questions'); assert.equal(request.method, 'POST');
  assert.equal(request.header['Idempotency-Key'], key); assert.equal(request.header.Authorization, 'Bearer ' + token);
  assert.equal(request.timeout, 28000);
  assert.deepEqual(JSON.parse(JSON.stringify(request.data)), { version: 5, subjectType: 'theme', subjectId: 7, skill: 'material_qa', taskId: 0, question });
  env.reply(request, status());
  assert.equal((await promise).data.seq, 1);

  promise = AIQAApi.submit(pending); request = env.requests.shift();
  env.reply(request, status({ question: '另一个问题' }));
  assert.equal((await promise).data, null, '回执与原问题不一致须整笔拒绝');

  promise = AIQAApi.receipt(3, key); request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/question-receipt'); assert.equal(request.method, 'GET');
  assert.equal(request.header['Idempotency-Key'], key); assert.equal(request.timeout, 10000);
  env.reply(request, status({ state: 'running' })); assert.equal((await promise).data.state, 'running');

  promise = AIQAApi.status(3, 1); request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/questions/1'); assert.equal(request.header.Authorization, 'Bearer ' + token);
  env.reply(request, status({ seq: 2 })); assert.equal((await promise).data, null, '状态编号不一致须拒绝');

  promise = AIQAApi.answer(3, 1); request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/questions/1/answer');
  env.reply(request, answer()); assert.equal((await promise).data.quotes[0].excerpt, excerpt);

  for (const bad of [0, -1, 1.5, 9007199254740992]) {
    assert.equal((await AIQAApi.status(3, bad)).data, null);
    assert.equal((await AIQAApi.answer(bad, 1)).data, null);
  }
  assert.equal((await AIQAApi.receipt(3, 'short')).data, null);
  promise = AIQAApi.answer(3, 1); env.requests.shift().success({ statusCode: 200, data: '<html>测试错误</html>' });
  assert.equal((await promise).data, null);
  console.log('通过：问答四固定端点、原键仅首次提交/收据使用、28秒提交期限、当前会员令牌及回执不一致拒绝。');
}

async function panelSubmit() {
  const env = environment(); env.storage.set('token', token);
  const ui = mount(env.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: false, questionSeq: 0 });
  await flush();
  assert.ok(ui.text().includes('会话资料问答'));
  assert.ok(!ui.text().includes('测试模型'));

  ui.input('示例题材', '查找本次问答的题材');
  ui.click('查询问答题材');
  let request = env.requests.shift();
  assert.equal(request.url, '/api/v1/theme/search'); assert.deepEqual(JSON.parse(JSON.stringify(request.data)), { kw: '示例题材', limit: 50 });
  env.reply(request, [{ id: 7, name: '测试示例题材', description: '测试占位' }]);
  await flush();
  assert.ok(ui.text().includes('测试示例题材'));

  ui.click('选择问答题材 #7');
  await flush();
  ui.input(question, '输入所选资料的事实问题，最多1000字');
  await flush();
  ui.click('提交本次资料问题（不扣积分）');
  await flush();
  request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3'); assert.equal(request.method, 'GET');
  env.reply(request, conversation());
  await flush();
  const submit = env.requests.shift();
  assert.equal(submit.url, '/api/v1/ai/conversations/3/questions'); assert.equal(submit.method, 'POST');
  assert.equal(submit.header['Idempotency-Key'], ui.state.pending.key);
  assert.equal(submit.data.question, question); assert.equal(submit.data.subjectId, 7); assert.equal(submit.data.version, 5);
  const stored = JSON.parse(env.storage.get(ui.state.partition));
  assert.equal(stored.key, submit.header['Idempotency-Key'], '发请求前必须先持久化原键');
  env.reply(submit, status());
  await flush();
  assert.equal(ui.state.status.state, 'queued');
  assert.ok(ui.text().includes('本机保留有原问题'));
  assert.ok(ui.text().includes('原问题已保存，执行结果待核查'));

  // 待核查期间不得出现第二条提交入口。
  assert.ok(!ui.text().includes('提交本次资料问题'));

  ui.click('核查本机原问题');
  request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/question-receipt'); assert.equal(request.header['Idempotency-Key'], stored.key);
  env.reply(request, status({ state: 'answered', updatedAt: now + 1000 }));
  await flush();
  assert.equal(ui.state.status.state, 'answered');
  assert.equal(ui.state.pending.key, stored.key, '仅状态成功不代表可以清原键');

  const before = env.requests.length;
  ui.state.submit(); await flush();
  assert.equal(env.requests.length, before, '已有待核查记录时不得再次提交');
  console.log('通过：问答面板选题→详情核版本→持久原键→提交→状态回执；待核查期间无重复提交。');
}

async function controlledComposer() {
  const env = environment(); let changed = '', submitted = 0;
  const ui = mount(env.load('pages/ai/components/AIQuestionComposer.uvue').default, {
    question: '', scopeLabel: '测试所选题材', placeholder: '测试受控输入', submitLabel: '测试提交', busy: false,
    onChange: value => { changed = value; }, onSubmit: () => { submitted++; },
  });
  const input = () => ui.nodes().find(n => n.tag === 'x-input');
  const button = () => ui.nodes().find(n => n.tag === 'x-button');
  ui.input(question); await flush();
  assert.equal(changed, question); assert.equal(input().props['model-value'], '', '子组件不得自建草稿');
  button().props.onClick(); assert.equal(submitted, 0, '非法输入即使直接触发事件也不得提交');
  ui.updateProps({question, docked: true, dockLeft: 324}); await flush();
  assert.equal(input().props.fixed, true);
  assert.equal(input().props.height, '64', '固定输入保留两行编辑空间，避免占满低高度正文');
  assert.ok(ui.nodes().some(n => n.props.style === 'left:324px;bottom:0px;'));
  ui.updateProps({dockBottom:60}); await flush();
  assert.ok(ui.nodes().some(n => n.props.style === 'left:324px;bottom:60px;'));
  assert.equal(input().props['model-value'],question);
  assert.ok(!ui.nodes().some(n => n.props.class === 'composer-reserve'), '留白由面板末尾统一放置，不能位于错误提示之前');
  ui.click('测试提交'); assert.equal(submitted, 1);
  ui.updateProps({busy:true}); await flush();
  input().props['on:update:modelValue']('迟到输入'); button().props.onClick();
  assert.equal(changed, question); assert.equal(submitted, 1); assert.equal(button().props.disabled, true);
  ui.updateProps({busy:false, docked:false}); await flush();
  assert.equal(input().props['model-value'], question); assert.equal(input().props.fixed, false);
  assert.equal(input().props.height, '112', '恢复内联只改变尺寸，不重建草稿');
  assert.ok(!ui.nodes().some(n => n.props.class === 'composer-reserve'));
  assert.equal(env.requests.length,0); assert.equal(env.storage.size,0);
  ui.app.unmount();

  env.storage.set('token', token);
  const panel = mount(env.load('pages/ai/components/AIQuestionPanel.uvue').default, {
    sessionToken:token, conversationId:3, archived:false, questionSeq:0, dockInput:true, dockLeft:324,
  });
  const textarea = () => panel.nodes().find(n => n.tag === 'x-input' && n.props.type === 'textarea');
  await flush(); assert.equal(textarea(), undefined, '未选题材不得出现通用聊天输入');
  panel.input('示例题材','查找本次问答的题材'); panel.click('查询问答题材');
  env.reply(env.requests.shift(),[{id:7,name:'测试示例题材',description:''}]); await flush();
  panel.click('选择问答题材 #7'); await flush();
  // 同一轮连续输入事件必须逐次同步到唯一父草稿，不能节流成首字。
  const deliverInput = textarea().props['on:update:modelValue'];
  for (const value of ['资', '资料有哪些', question]) {
    deliverInput(value);
    assert.equal(panel.state.question,value,'每次输入事件须同步更新父草稿');
  }
  assert.equal(env.requests.length,0,'输入事件本身不能自动提交');
  panel.input(question,'输入所选资料的事实问题，最多1000字'); await flush();
  for (const props of [{dockLeft:16,dockBottom:60},{dockInput:false},{dockInput:true,dockLeft:324}]) {
    panel.updateProps(props); await flush();
    assert.equal(panel.state.question,question); assert.equal(textarea().props['model-value'],question);
    assert.equal(env.requests.length,0); assert.equal(env.storage.size,1,'布局变化不得持久化新原键');
  }
  panel.click('提交本次资料问题（不扣积分）'); await flush();
  assert.equal(textarea().props.disabled,true);
  env.reply(env.requests.shift(),null,503,50301); await flush();
  assert.equal(env.requests.length,0,'详情核验失败不得发送问题POST');
  assert.equal(panel.state.question,question); assert.equal(textarea().props.disabled,false);
  const nodes=panel.nodes(), reserve=nodes.find(n=>n.props.class==='qa-composer-reserve');
  const error=nodes.find(n=>n.tag==='text' && n.text.includes('当前会话不可提问'));
  assert.ok(error); assert.ok(reserve,'固定输入必须在问答面板末尾留白');
  assert.ok(nodes.indexOf(reserve)>nodes.indexOf(error),'失败提示必须位于留白之前，以便滚动出固定输入遮挡区');
  panel.click('提交本次资料问题（不扣积分）'); await flush();
  panel.updateProps({dockLeft:16}); await flush();
  assert.equal(panel.state.question,question);
  env.reply(env.requests.shift(),conversation()); await flush();
  const request=env.requests.shift(), key=request.header['Idempotency-Key'];
  assert.equal(request.method,'POST'); assert.equal(textarea(),undefined,'持久原键后固定输入必须消失');
  request.fail({errMsg:'测试未知结果'}); await flush();
  panel.updateProps({dockInput:false}); await flush(); panel.updateProps({dockInput:true}); await flush();
  assert.equal(textarea(),undefined); assert.equal(env.requests.length,0);
  assert.equal(panel.state.pending.key,key,'未知结果切布局必须保留唯一原键');
  panel.click('核查本机原问题');
  const receipt=env.requests.shift(); assert.equal(receipt.method,'GET'); assert.equal(receipt.header['Idempotency-Key'],key);
  env.reply(receipt,status()); await flush();
  panel.updateProps({sessionToken:'',conversationId:0}); await flush();
  assert.equal(textarea(),undefined); assert.equal(panel.state.question,''); assert.equal(panel.state.subject,null);
  panel.app.unmount();
  console.log('通过：受控输入无副本、忙态与非法输入禁发、固定/内联切换保留草稿、原键持久后收起输入、未知结果只核原键、退出清除范围。');
}

async function panelAnswer() {
  const env = environment(); env.storage.set('token', token);
  const key = 'question_0123456789abcdef';
  // 本地分区按令牌身份编号（1）派生，与会话编号无关；会话编号只在记录内部校验。
  env.storage.set('tm_ai_question_v1_1', JSON.stringify({ key, conversationId: 3, version: 5, subjectId: 7, taskId: 0, skill: 'material_qa', question }));
  const ui = mount(env.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: false, questionSeq: 0 });
  await flush();
  assert.equal(ui.state.pending.key, key);
  assert.ok(ui.text().includes('本机保留有原问题'));

  ui.click('核查本机原问题');
  env.reply(env.requests.shift(), status({ state: 'answered', updatedAt: now + 1000 })); await flush();
  ui.click('核对当前答案');
  let request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/questions/1/answer');
  env.reply(request, answer()); await flush();
  assert.ok(ui.text().includes(excerpt));
  assert.ok(ui.text().includes('AI 生成'));
  assert.ok(ui.text().includes('测试模型（非真实调用）'));
  assert.ok(ui.text().includes('https://example.test/source'));

  ui.click('重新核对并打开出处 #1');
  request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/questions/1/answer', '打开出处前必须重新读取并核对');
  env.reply(request, answer()); await flush();
  assert.equal(env.modals.length, 1);
  env.modals[0].success({ confirm: true }); await flush();
  assert.equal(env.opened.length, 1); assert.equal(env.opened[0].url, 'https://example.test/source');
  assert.equal(env.opened[0].target.opener, null, '外部链接不得保留 opener');

  ui.click('重新核对并打开出处 #1');
  request = env.requests.shift();
  env.reply(request, answer({ quotes: [quote({ excerpt: excerpt + '（已更新）', endByte: excerptBytes + Buffer.byteLength('（已更新）', 'utf8') })] })); await flush();
  assert.ok(ui.text().includes('当前出处未通过重新核对'));
  assert.equal(env.modals.length, 1, '出处变化时不得打开旧链接');

  ui.click('核实终态并清理本地原键');
  request = env.requests.shift();
  assert.equal(request.url, '/api/v1/ai/conversations/3/question-receipt');
  env.reply(request, status({ state: 'answered', updatedAt: now + 2000 })); await flush();
  assert.equal(ui.state.pending, null); assert.equal(env.storage.has('tm_ai_question_v1_1'), false);
  assert.ok(ui.text().includes('本地原键已清理'));

  // 终态不符不得清键。
  const env2 = environment(); env2.storage.set('token', token);
  env2.storage.set('tm_ai_question_v1_1', JSON.stringify({ key, conversationId: 3, version: 5, subjectId: 7, taskId: 0, skill: 'material_qa', question }));
  const ui2 = mount(env2.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: false, questionSeq: 0 });
  await flush(); ui2.click('核查本机原问题');
  env2.reply(env2.requests.shift(), status({ state: 'running', updatedAt: now + 3000 })); await flush();
  assert.equal(ui2.state.pending.key, key);
  assert.equal(env2.storage.get('tm_ai_question_v1_1').includes(key), true, '未到终态不得清理原键');
  console.log('通过：答案引文/模型标注展示、出处二次核对与 opener 隔离、变化拒绝；终态核查后才清原键。');
}

async function isolation() {
  // 401 清私有状态并通知外层。
  const env = environment(); env.storage.set('token', token);
  let unauthorized = 0;
  const ui = mount(env.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: false, questionSeq: 0, onUnauthorized: () => { unauthorized++; } });
  await flush();
  ui.input('示例题材', '查找本次问答的题材'); ui.click('查询问答题材');
  env.reply(env.requests.shift(), null, 401, 40101); await flush();
  assert.equal(unauthorized, 1);
  assert.deepEqual(Array.from(ui.state.themes), []);
  assert.equal(ui.state.busy, false);

  // 题目响应回包前登录已切换：不得把旧账号结果写入当前页面。
  const env2 = environment(); env2.storage.set('token', token);
  const ui2 = mount(env2.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: false, questionSeq: 0 });
  await flush();
  ui2.input('示例题材', '查找本次问答的题材'); ui2.click('查询问答题材');
  env2.storage.set('token', 'test.other.member');
  env2.reply(env2.requests.shift(), [{ id: 7, name: '旧账号题材', description: '' }]); await flush();
  assert.ok(!ui2.text().includes('旧账号题材'));

  // 已归档只允许核查，不出现新问题入口。
  const env3 = environment(); env3.storage.set('token', token);
  const ui3 = mount(env3.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: true, questionSeq: 0 });
  await flush();
  assert.ok(ui3.text().includes('当前会话已归档'));
  assert.ok(!ui3.text().includes('查询问答题材'));
  assert.ok(!ui3.text().includes('提交本次资料问题'));

  // 存储不可用：失败关闭，不发请求。
  const env4 = environment(); env4.storage.set('token', token);
  env4.uni.getStorageSync = key => { if (String(key).startsWith('tm_ai_question_v1_')) throw new Error('测试存储不可用'); return env4.storage.get(key); };
  const ui4 = mount(env4.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: false, questionSeq: 0 });
  await flush();
  assert.ok(ui4.text().includes('本机原问题记录不可读取'));
  assert.ok(!ui4.text().includes('查询问答题材'));
  assert.equal(env4.requests.length, 0);

  // 迟到回包在卸载后不得继续写入。
  const env5 = environment(); env5.storage.set('token', token);
  const ui5 = mount(env5.load('pages/ai/components/AIQuestionPanel.uvue').default, { sessionToken: token, conversationId: 3, archived: false, questionSeq: 0 });
  await flush();
  ui5.input('示例题材', '查找本次问答的题材'); ui5.click('查询问答题材');
  const late = env5.requests.shift(); ui5.app.unmount();
  env5.reply(late, [{ id: 7, name: '卸载后题材', description: '' }]); await flush();
  assert.ok(!ui5.text().includes('卸载后题材'));
  console.log('通过：401 清状态、切换登录丢弃迟到回包、归档只读、存储故障失败关闭、卸载后不写页面。');
}

function dossierFixture(version=1,compare=0) {
  const q = {text:'仅测试：甲资料。',context:'仅测试：甲资料。',sourceType:'official_catalog',sourceUrl:'https://evidence.example.test/1',collectedAt:now,snapshotId:1,sentence:1,startByte:0,endByte:Buffer.byteLength('仅测试：甲资料。')};
  const fields=['subject','publishedDate','effectiveDate','period','scope','stage','basis','unit'].map(key=>({key,state:key==='subject'?'known':'unknown',...(key==='subject'?{quote:{...q}}:{})}));
  const row=v=>({version:v,compiledAt:now+v,cards:[{key:'a',section:'progress',quote:{...q},fields:JSON.parse(JSON.stringify(fields))}],gaps:[]});
  return {themeId:7,notice:'以下为所选版本的资料摘录；引用存在不等于已核实表述，未知字段表示资料不足。',versions:compare?[row(version),row(compare)]:[row(version)],...(compare?{comparison:{notice:'仅比较档案中已引用的原句、字段和资料缺口；本版未出现不表示事实撤回，摘录调整不等于事实更正。',added:[],notPresent:[],changed:[],gaps:[],quotedSentenceOrderChanged:false}}:{})};
}
const dossierList=()=>({themeId:7,items:[{version:2,compiledAt:now+2},{version:1,compiledAt:now+1}],hasMore:false,nextBeforeVersion:0});
async function dossierContracts() {
  const env=environment(),types=env.load('pages/libs/ai-dossier.uts');
  assert.equal(types.isDossierList(dossierList(),7,0),true);
  assert.equal(types.isDossierView(dossierFixture(),7,1,0),true);
  assert.equal(types.isDossierView(dossierFixture(1,2),7,1,2),true);
  for(const mutate of [v=>v.themeId=8,v=>v.privateLicense='hidden',v=>v.versions[0].cards[0].quote.endByte++,v=>v.versions[0].cards[0].fields[1].quote=v.versions[0].cards[0].quote,v=>v.versions[0].cards[0].fields.reverse(),v=>v.versions[0].cards[0].quote.sourceUrl='javascript:alert(1)',v=>v.versions[0].compiledAt=0,v=>v.notice='模型生成的自由文本',v=>v.versions[0].cards[0].quote=null]) {
    const value=dossierFixture();mutate(value);assert.equal(types.isDossierView(value,7,1,0),false);
  }
  const diff=dossierFixture(1,2);diff.comparison.added=['missing'];assert.equal(types.isDossierView(diff,7,1,2),false);
  const duplicate=dossierList();duplicate.items[1].version=2;assert.equal(types.isDossierList(duplicate,7,0),false);
  const cursor=dossierList();cursor.hasMore=true;cursor.nextBeforeVersion=1;assert.equal(types.isDossierList(cursor,7,0),false);
  const api=env.load('pages/libs/ai-dossier-api.uts').AIDossierApi;env.storage.set('token',token);
  const p=api.list(7,0);assert.equal(env.requests.length,1);assert.ok(env.requests[0].url.endsWith('/api/v1/ai/themes/7/dossiers'));env.reply(env.requests.shift(),dossierList());assert.equal((await p).code,0);
  assert.equal((await api.view(7,2,1)).code,40001);assert.equal(env.requests.length,0);
  const r=api.view(7,1,2);assert.ok(env.requests[0].url.endsWith('/dossiers/1?compareVersion=2'));env.reply(env.requests.shift(),dossierFixture(1,2));assert.equal((await r).code,0);
  console.log('通过：专题列表/原文/字段/对照DTO白名单与固定请求边界。');
}
async function dossierPanel() {
  const env=environment();env.storage.set('token',token);let copied=[];env.uni.setClipboardData=v=>copied.push(v.data);
  const ui=mount(env.load('pages/ai/components/AIDossierPanel.uvue').default,{sessionToken:token,themeId:7,themeName:'仅测试题材',enabled:true});await flush();
  assert.equal(env.requests.length,0,'收起状态不自动请求');
  ui.click('查询可读档案版本');assert.equal(env.requests.length,1);env.reply(env.requests.shift(),dossierList());await flush();
  assert.ok(ui.text().includes('版本 2'));ui.click('阅读此版本',1);env.reply(env.requests.shift(),dossierFixture());await flush();
  assert.ok(ui.text().includes('仅测试：甲资料。'));assert.ok(ui.text().includes('未知，资料不足'));
  ui.click('核对并复制原文链接');assert.equal(copied.length,0,'未重新核验前不能复制旧来源');env.reply(env.requests.shift(),null,404,40404);await flush();
  assert.equal(copied.length,0);assert.ok(!ui.text().includes('仅测试：甲资料。'));
  assert.ok(!ui.text().includes('阅读此版本'));ui.click('重新核对可读版本');env.reply(env.requests.shift(),dossierList());await flush();
  ui.click('阅读此版本',1);env.reply(env.requests.shift(),dossierFixture());await flush();ui.click('核对并复制原文链接');env.reply(env.requests.shift(),dossierFixture());await flush();assert.equal(copied[0],'https://evidence.example.test/1');
  ui.click('加入对照',0);await flush();ui.click('加入对照',0);await flush();ui.click('对照所选的两个版本（2/2）');env.reply(env.requests.shift(),dossierFixture(1,2));await flush();assert.ok(ui.text().includes('档案摘录变化'));
  ui.click('重新核对可读版本');const late=env.requests.shift();ui.updateProps({enabled:false});await flush();env.reply(late,dossierList());await flush();assert.ok(!ui.text().includes('阅读此版本'));
  ui.updateProps({enabled:true});await flush();ui.click('查询可读档案版本');env.reply(env.requests.shift(),dossierList());await flush();ui.click('阅读此版本',1);const stale=env.requests.shift();ui.updateProps({themeId:8});await flush();env.reply(stale,dossierFixture());await flush();assert.ok(!ui.text().includes('仅测试：甲资料。'));
  ui.app.unmount();
  const env2=environment();env2.storage.set('token',token);let unauthorized=0;const ui2=mount(env2.load('pages/ai/components/AIDossierPanel.uvue').default,{sessionToken:token,themeId:7,themeName:'测试题材',enabled:true,onUnauthorized:()=>unauthorized++});await flush();ui2.click('查询可读档案版本');env2.reply(env2.requests.shift(),null,401,40100);await flush();assert.equal(unauthorized,1);assert.ok(!ui2.text().includes('阅读此版本'));ui2.app.unmount();
  console.log('通过：专题展开/单版/双版对照、出处复验、拒读清空、页签/题材切换迟到响应与401清理。');
}

let completed=false;
process.once('beforeExit',()=>{if(!completed){console.error('问答源码验收尚未全部完成');process.exitCode=1;}});
(async () => {
  await validators();
  await skillCatalog();
  await reportHistoryEntry();
  await reportPanel();
  await api();
  await panelSubmit();
  await controlledComposer();
  await panelAnswer();
  await isolation();
  await dossierContracts();
  await dossierPanel();
  completed=true;
  console.log('本脚本不覆盖：真实 UTS/uni-app x 整体构建、真实模型质量、浏览器布局与生产开关；须另做实际 H5 与浏览器验收。');
})().catch(error => { completed=true; console.error(error); process.exitCode=1; });
