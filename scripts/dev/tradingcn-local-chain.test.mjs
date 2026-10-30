// 无真实Key、无供应商网络；只验证异步桥契约和持久状态，不是V2验收证据。
import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { createAsyncBridge, validSubmission, finishChainVerification, chainConversationOptions, hasConversationProof, hasDeepProof, hasProcessHTTPProof, hasProcessH5Proof, startChainVerification } from './tradingcn-local-chain.mjs'

test('实际H5进程证明独立于HTTP，入口在启动子进程前拒绝错误模式和外部源', () => {
  assert.deepEqual(chainConversationOptions({processes:true,processBrowser:true,conversation:true}),{TM_TEST_AI_LIVE_PROCESS:'local-fixture-only',TM_TEST_AI_H5_LOCAL_DEPTH:'light',TM_TEST_AI_LIVE_PROCESS_H5:'local-fixture-only'})
  for (const processBrowser of [true,'true',1]) assert.throws(()=>chainConversationOptions({processBrowser}))
  assert.equal(hasProcessH5Proof('    runner.go:12: TM_STOCK_LIVE_PROCESS_H5_VERIFIED\r\n'),true)
  for (const raw of ['TM_STOCK_LIVE_PROCESS_H5_VERIFIED','    runner.go:12: TM_STOCK_LIVE_PROCESS_HTTP_VERIFIED\n','    runner.go:12: TM_STOCK_LIVE_PROCESS_H5_VERIFIED','    runner.go:12: TM_STOCK_LIVE_PROCESS_H5_VERIFIED_extra\n']) assert.equal(hasProcessH5Proof(raw),false)
  for (const origin of [undefined,'https://example.test','http://localhost:1234','http://127.0.0.1','http://127.0.0.1:1234/app','http://user:pass@127.0.0.1:1234','http://127.0.0.1:1234/?x=1','http://127.0.0.1:1234/#x']) {
    assert.throws(()=>startChainVerification({evaluation:{child:'waiting'}},{processes:true,processBrowser:true,conversation:true,origin}))
  }
})

test('正式进程模式单独声明HTTP证据，不能沿用旧会话证明冒充', async () => {
  assert.deepEqual(chainConversationOptions({processes:true,conversation:true,analysisType:'deep'}),{TM_TEST_AI_LIVE_PROCESS:'local-fixture-only',TM_TEST_AI_H5_LOCAL_DEPTH:'deep'})
  for(const options of [{processes:true},{processes:'true'},{processes:true,conversation:true,loginRunner:'old.exe'},{processes:true,conversation:true,analysisType:'other'}]) assert.throws(()=>chainConversationOptions(options))
  assert.equal(hasProcessHTTPProof('    x.go:12: TM_STOCK_LIVE_PROCESS_HTTP_VERIFIED\n'),true)
  for(const output of ['TM_STOCK_LIVE_PROCESS_HTTP_VERIFIED','    x.go:12: TM_STOCK_LIVE_CONVERSATION_VERIFIED\n','    x.go:12: TM_STOCK_LIVE_PROCESS_HTTP_VERIFIED_extra\n'])assert.equal(hasProcessHTTPProof(output),false)
  const output=await fs.mkdtemp(path.join(os.tmpdir(),'tm-stock-tradingcn-live-'))
  const bridge={evaluation:{output,execute:true,child:'passed',summary:{verified:true,executed:true,model_calls:1},requests:4,usage:[{phase:'core',status:200,input:1,output:1},...Array.from({length:3},()=>({phase:'report',status:200,input:1,output:1}))]},counts:{submissions:1,polls:1,results:1},async close(){}}
  const verification={status:'passed',exit:0,finished:Promise.resolve(),conversation:true,conversationVerified:true,processes:true,analysisType:'light'}
  try{
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.processHTTPVerified=true
    const receipt=await finishChainVerification(bridge,verification)
    assert.equal(receipt.verified,true);assert.equal(receipt.browser_verified,false);assert.equal(receipt.process_http_verified,true)
    assert.equal(receipt.scope,'local_http_process_conversation_async_real_v2_test_ledger')
    verification.processBrowser=true
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.processH5Verified=true
    verification.processHTTPVerified=false
    const h5=await finishChainVerification(bridge,verification)
    assert.equal(h5.verified,true);assert.equal(h5.browser_verified,true);assert.equal(h5.process_h5_verified,true);assert.equal(h5.process_http_verified,false)
    assert.equal(h5.scope,'local_h5_process_conversation_async_real_v2_test_ledger')
    verification.processes=false
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.processes=true
    verification.conversation=false
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
  }finally{assert.equal(path.dirname(output),os.tmpdir());assert.match(path.basename(output),/^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/);await fs.rm(output,{recursive:true})}
})

test('真实会话入口显式原登录配置，不接受隐式或不完整模式', () => {
  assert.deepEqual(chainConversationOptions(), {})
  const loginRunner = path.join(os.tmpdir(), 'test-member-login.exe')
  const env = chainConversationOptions({ conversation: true, loginRunner })
  assert.equal(env.TM_TEST_AI_H5_LOGIN_RUNNER, loginRunner)
  for (const key of ['TM_TEST_AI_LIVE_CONVERSATION','TM_TEST_AI_H5_INTENTS','TM_TEST_AI_H5_LOGIN']) assert.equal(env[key], 'local-fixture-only')
  assert.equal(Object.keys(env).length, 4)
  assert.equal(chainConversationOptions({conversation:true,loginRunner,analysisType:'deep'}).TM_TEST_AI_LIVE_DEPTH,'deep')
  assert.throws(()=>chainConversationOptions({analysisType:'deep'}),/深度验收/)
  assert.throws(()=>chainConversationOptions({conversation:true,loginRunner,analysisType:'unknown'}),/深度验收/)
  for (const input of [{ loginRunner }, { conversation: 'true', loginRunner }, { conversation: true },
    { conversation: true, loginRunner: 'relative.exe' }, { conversation: true, loginRunner: path.join(os.tmpdir(), 'file.txt') }]) {
    assert.throws(() => chainConversationOptions(input), /会话验收/)
  }
})

test('会话证明必须是Go完成断言日志而非运行提示或残缺片段', () => {
  assert.equal(hasDeepProof('    x.go:12: TM_STOCK_LIVE_DEEP_VERIFIED\n'),true)
  assert.equal(hasDeepProof('    x.go:12: TM_STOCK_LIVE_CONVERSATION_VERIFIED\n'),false)
  assert.equal(hasDeepProof('    x.go:12: TM_STOCK_LIVE_DEEP_VERIFIED'),false)
  assert.equal(hasConversationProof('    report_live_agents_integration_test.go:200: TM_STOCK_LIVE_CONVERSATION_VERIFIED\r\n'), true)
  for (const text of ['', 'TM_STOCK_LIVE_CONVERSATION_VERIFIED', '=== RUN TM_STOCK_LIVE_CONVERSATION_VERIFIED',
    '    x.go:12: TM_STOCK_LIVE_CONVERSATION_VERIFIED_extra\n', '    x.go:12: TM_STOCK_LIVE_CONVERSATION_VERIFIED',
    '    x.go:12: TM_STOCK_LIVE_CONVERSATION_']) assert.equal(hasConversationProof(text), false)
})

const submission = () => ({ symbol: '600001', parameters: { engine: 'v2', market_type: 'A股', research_depth: '快速',
  selected_analysts: ['news', 'fundamentals'], include_sentiment: false, include_risk: false,
  language: 'zh-CN', quick_analysis_model: 'deepseek-flash', deep_analysis_model: 'deepseek-flash', enable_trace: false } })

test('只接收固定本地格式占位和原适配器参数', () => {
  assert.equal(validSubmission(submission()), true)
  const reordered = submission(); reordered.parameters = Object.fromEntries(Object.entries(reordered.parameters).reverse())
  assert.equal(validSubmission(reordered), true)
  for (const change of [x => { x.symbol = '000001' }, x => { x.prompt = '任意内容' }, x => { x.parameters.engine = 'legacy' },
    x => { x.parameters.deep_analysis_model = 'other' }, x => { x.parameters.include_risk = true }, x => { x.parameters.extra = 1 }]) {
    const body = submission(); change(body); assert.equal(validSubmission(body), false)
  }
})

test('整链路回执必须同时具备Go成功、核心实调、三次报告模型及唯一异步任务', async () => {
  const output = await fs.mkdtemp(path.join(os.tmpdir(), 'tm-stock-tradingcn-live-'))
  let closed = 0
  const bridge = { evaluation:{ output,execute:true,child:'passed',summary:{verified:true,executed:true,model_calls:1},
    requests:4,usage:[{phase:'core',status:200,input:1,output:1},...Array.from({length:3},()=>({phase:'report',status:200,input:1,output:1}))]},
    counts:{submissions:1,polls:1,results:1},async close(){closed++} }
  const verification = {status:'passed',exit:0,finished:Promise.resolve()}
  try {
    assert.equal((await finishChainVerification(bridge,verification)).verified,true)
    verification.conversation=true
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.conversationVerified=true
    const conversationReceipt=await finishChainVerification(bridge,verification)
    assert.equal(conversationReceipt.verified,true)
    assert.equal(conversationReceipt.scope,'local_conversation_async_real_v2_test_ledger')
    verification.analysisType='deep'
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.deepVerified=true
    const deepReceipt=await finishChainVerification(bridge,verification)
    assert.equal(deepReceipt.verified,true)
    assert.equal(deepReceipt.analysis_type,'deep')
    verification.conversation=false
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.analysisType='light'
    verification.conversation=false
    verification.exit=1
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.exit=0
    verification.status='failed'
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    verification.status='passed'; bridge.counts.submissions=2
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    bridge.counts.submissions=1; bridge.evaluation.unknown=true
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    bridge.evaluation.unknown=false; bridge.evaluation.usage.pop(); bridge.evaluation.requests--
    assert.equal((await finishChainVerification(bridge,verification)).verified,false)
    bridge.evaluation.child='waiting'
    await assert.rejects(finishChainVerification(bridge,verification),/不是已结束/)
    verification.status='failed'; verification.exit=-4058
    const notStarted=await finishChainVerification(bridge,verification)
    assert.equal(notStarted.verified,false)
    assert.equal(notStarted.go_exit,-4058)
    assert.equal(bridge.evaluation.child,'failed')
    assert.equal(closed,13)
  } finally {
    assert.equal(path.dirname(output),os.tmpdir()); assert.match(path.basename(output),/^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/)
    await fs.rm(output,{recursive:true})
  }
})

for (const outcome of ['success', 'failed', 'unknown']) test(`异步桥${outcome}状态、重复提交、鉴权和回收`, async () => {
  const output = await fs.mkdtemp(path.join(os.tmpdir(), 'tm-stock-tradingcn-live-'))
  let release
  const gate = new Promise(resolve => { release = resolve })
  let releaseJournal, enteredJournal
  const journalGate = new Promise(resolve => { releaseJournal = resolve })
  const journalEntered = new Promise(resolve => { enteredJournal = resolve })
  let launches = 0
  const evaluation = { output, execute: true, child: 'waiting', requests: 1, usage: [{ phase: 'core', status: 200, input: 1, output: 1 }],
    summary: { verified: true, executed: true, model_calls: 1 }, unknown: outcome === 'unknown',
    async launch() { launches++; this.child = 'running'; await gate; this.child = outcome === 'failed' ? 'failed' : 'passed' },
    async close() {} }
  await fs.writeFile(path.join(output, 'quarantined-result.json'), JSON.stringify({ final_report: '明确本地测试原报告', internal: '不转交' }))
  // 真实文件系统只在终态写入前受控暂停，证明HTTP不能早于持久记录发布终态。
  const journalIO = { ...fs, async writeFile(file, data, options) {
    if (file === path.join(output, 'async-job.json.new') && ['completed', 'failed'].includes(JSON.parse(data).status)) {
      enteredJournal(); await journalGate
    }
    return fs.writeFile(file, data, options)
  } }
  const bridge = await createAsyncBridge(evaluation, journalIO)
  const call = (url, method = 'GET', body, headers = {}) => fetch(bridge.url + url, { method, headers: { Authorization: `Bearer ${bridge.token}`,
    'Content-Type': 'application/json', ...headers }, body: body === undefined ? undefined : JSON.stringify(body), redirect: 'error' })
  try {
    assert.equal((await call('/api/analysis/single', 'POST', submission(), { Origin: '' })).status, 403)
    assert.equal((await call('/api/analysis/single', 'POST', submission(), { Authorization: 'Bearer wrong' })).status, 403)
    assert.equal((await call('/api/analysis/single', 'POST', { ...submission(), symbol: '000001' })).status, 400)
    assert.equal(launches, 0)
    const [one, two] = await Promise.all([call('/api/analysis/single', 'POST', submission()), call('/api/analysis/single', 'POST', submission())])
    assert.deepEqual([one.status, two.status].sort(), [200, 409])
    const accepted = await (one.status === 200 ? one : two).json()
    assert.equal(accepted.data.status, 'queued')
    const base = '/api/analysis/tasks/' + accepted.data.task_id
    const launchDeadline = Date.now() + 5000
    while (launches === 0 && Date.now() < launchDeadline) await new Promise(resolve => setTimeout(resolve, 5))
    assert.equal(launches, 1)
    assert.equal((await (await call(base + '/status')).json()).data.status, 'running')
    assert.equal((await call(base + '/result')).status, 404)
    release()
    let journalWatchdog
    try {
      await Promise.race([journalEntered, new Promise((_, reject) => { journalWatchdog = setTimeout(() => reject(new Error('未进入受控终态持久写入')), 5000) })])
    } finally { clearTimeout(journalWatchdog) }
    assert.equal((await (await call(base + '/status')).json()).data.status, 'running', '终态持久化前不得对外发布')
    assert.equal(JSON.parse(await fs.readFile(path.join(output, 'async-job.json'), 'utf8')).status, 'running')
    assert.equal((await call(base + '/result')).status, 404)
    releaseJournal()
    let state
    // 完成回执包含异步磁盘写入；按有界真实状态等候，不把固定250ms调度预算当业务期限。
    const terminalDeadline = Date.now() + 5000
    while (Date.now() < terminalDeadline) {
      state = (await (await call(base + '/status')).json()).data.status
      if (state !== 'running') break
      await new Promise(resolve => setTimeout(resolve, 5))
    }
    assert.equal(state, outcome === 'success' ? 'completed' : 'failed')
    const persisted = JSON.parse(await fs.readFile(path.join(output, 'async-job.json'), 'utf8'))
    assert.equal(persisted.engine_subject, 'TMEVAL'); assert.equal(persisted.status, state)
    const result = await call(base + '/result')
    assert.equal(result.status, outcome === 'success' ? 200 : 404)
    if (outcome === 'success') assert.deepEqual((await result.json()).data.reports, { final_report: '明确本地测试原报告' })
    assert.equal((await call('/api/analysis/single', 'POST', submission())).status, 409)
    assert.equal((await call(base + '/cancel', 'POST')).status, 404)
    assert.equal((await call('/api/analysis/tasks/foreign/status')).status, 404)
    await assert.rejects(createAsyncBridge(evaluation), /不得接管/)
    assert.equal(launches, 1)
  } finally {
    release(); releaseJournal(); await bridge.close()
    assert.equal(path.dirname(output), os.tmpdir()); assert.match(path.basename(output), /^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/)
    await fs.rm(output, { recursive: true })
  }
})

test('异步桥终态持久化失败保留已确认运行态，不交付结果或重交任务', { timeout: 5000 }, async () => {
  const output = await fs.mkdtemp(path.join(os.tmpdir(), 'tm-stock-tradingcn-live-'))
  let observedFailure, attempts = 0
  const failedJournal = new Promise(resolve => { observedFailure = resolve })
  const evaluation = { output, execute: true, child: 'waiting', requests: 1,
    usage: [{ phase: 'core', status: 200, input: 1, output: 1 }],
    summary: { verified: true, executed: true, model_calls: 1 }, unknown: false,
    async launch() { this.child = 'passed' }, async close() {} }
  const journalIO = { ...fs, async writeFile(file, data, options) {
    const status = JSON.parse(data).status
    if (status === 'completed' || status === 'failed') {
      attempts++
      if (status === 'failed') observedFailure()
      throw new Error('仅测试：终态日志写入失败')
    }
    return fs.writeFile(file, data, options)
  } }
  const bridge = await createAsyncBridge(evaluation, journalIO)
  const call = (url, method = 'GET', body) => fetch(bridge.url + url, { method,
    headers: { Authorization: `Bearer ${bridge.token}`, 'Content-Type': 'application/json' },
    body: body === undefined ? undefined : JSON.stringify(body), redirect: 'error' })
  try {
    const response = await call('/api/analysis/single', 'POST', submission())
    assert.equal(response.status, 200)
    const base = '/api/analysis/tasks/' + (await response.json()).data.task_id
    await failedJournal
    assert.equal(attempts, 2)
    assert.equal((await (await call(base + '/status')).json()).data.status, 'running')
    assert.equal(JSON.parse(await fs.readFile(path.join(output, 'async-job.json'), 'utf8')).status, 'running')
    assert.equal((await call(base + '/result')).status, 404)
    assert.equal((await call('/api/analysis/single', 'POST', submission())).status, 409)
    assert.equal(bridge.counts.submissions, 1)
  } finally {
    await bridge.close()
    assert.equal(path.dirname(output), os.tmpdir()); assert.match(path.basename(output), /^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/)
    await fs.rm(output, { recursive: true })
  }
})
