// 无真实Key、无供应商网络；只验证异步桥契约和持久状态，不是V2验收证据。
import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import { createAsyncBridge, validSubmission, finishChainVerification } from './tradingcn-local-chain.mjs'

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
    assert.equal(closed,7)
  } finally {
    assert.equal(path.dirname(output),os.tmpdir()); assert.match(path.basename(output),/^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/)
    await fs.rm(output,{recursive:true})
  }
})

for (const outcome of ['success', 'failed', 'unknown']) test(`异步桥${outcome}状态、重复提交、鉴权和回收`, async () => {
  const output = await fs.mkdtemp(path.join(os.tmpdir(), 'tm-stock-tradingcn-live-'))
  let release
  const gate = new Promise(resolve => { release = resolve })
  let launches = 0
  const evaluation = { output, execute: true, child: 'waiting', requests: 1, usage: [{ phase: 'core', status: 200, input: 1, output: 1 }],
    summary: { verified: true, executed: true, model_calls: 1 }, unknown: outcome === 'unknown',
    async launch() { launches++; this.child = 'running'; await gate; this.child = outcome === 'failed' ? 'failed' : 'passed' },
    async close() {} }
  await fs.writeFile(path.join(output, 'quarantined-result.json'), JSON.stringify({ final_report: '明确本地测试原报告', internal: '不转交' }))
  const bridge = await createAsyncBridge(evaluation)
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
    for (let i = 0; i < 30 && launches === 0; i++) await new Promise(resolve => setTimeout(resolve, 5))
    assert.equal(launches, 1)
    assert.equal((await (await call(base + '/status')).json()).data.status, 'running')
    assert.equal((await call(base + '/result')).status, 404)
    release()
    let state
    for (let i = 0; i < 50; i++) {
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
    release(); await bridge.close()
    assert.equal(path.dirname(output), os.tmpdir()); assert.match(path.basename(output), /^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/)
    await fs.rm(output, { recursive: true })
  }
})
