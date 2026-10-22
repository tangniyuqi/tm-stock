// 只测纯守卫，不启动子进程、不使用真实Key、不访问模型服务。
import test from 'node:test'
import assert from 'node:assert/strict'
import { validateEvaluationRequest, recordReceipt } from './tradingcn-live-gateway.mjs'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'

const valid = () => ({ execute: true, method: 'POST', url: '/v1/chat/completions',
  headers: { authorization: 'Bearer test-session', 'content-type': 'application/json' },
  session: 'test-session', requests: 0,
  data: { model: 'deepseek-flash', messages: [{ role: 'user', content: '仅测试' }], max_tokens: 8, stream: false } })

test('正常请求保留原模型且不修改输入', () => {
  const input = valid()
  const original = structuredClone(input)
  assert.deepEqual(validateEvaluationRequest(input), { ...input.data, thinking: { type: 'disabled' } })
  assert.deepEqual(input, original)
})

for (const [name, change] of Object.entries({
  '关闭拒绝': x => { x.execute = false },
  'GET拒绝': x => { x.method = 'GET' },
  '任意路径拒绝': x => { x.url = '/other' },
  '查询参数拒绝': x => { x.url += '?url=https://example.com' },
  '无凭据拒绝': x => { delete x.headers.authorization },
  '错误凭据拒绝': x => { x.headers.authorization = 'Bearer wrong' },
  '浏览器来源拒绝': x => { x.headers.origin = 'http://127.0.0.1' },
  '空来源也拒绝': x => { x.headers.origin = '' },
  '编码拒绝': x => { x.headers['content-encoding'] = 'gzip' },
  '内容类型拒绝': x => { x.headers['content-type'] = 'text/plain' },
  '其它模型拒绝': x => { x.data.model = 'deepseek-v4-pro' },
  '流式拒绝': x => { x.data.stream = true },
  '无消息拒绝': x => { x.data.messages = [] },
  '对象消息拒绝': x => { x.data.messages = {} },
  '角色覆盖拒绝': x => { x.data.messages[0].role = 'developer' },
  '远程图片拒绝': x => { x.data.messages[0].content = [{ type: 'image_url', image_url: { url: 'https://example.com' } }] },
  '负输出数拒绝': x => { x.data.max_tokens = -1 },
  '零输出数拒绝': x => { x.data.max_tokens = 0 },
  '字符串输出数拒绝': x => { x.data.max_tokens = '8' },
  '非整数输出数拒绝': x => { x.data.max_tokens = 1.1 },
  '达到次数上限拒绝': x => { x.requests = 128 },
  '非法计数拒绝': x => { x.requests = -1 },
})) test(name, () => { const input = valid(); change(input); assert.equal(validateEvaluationRequest(input), null) })

test('明确输出上限、关闭思考且删除冲突参数', () => {
  const input = valid()
  input.data.max_tokens = 10000
  input.data.reasoning_effort = 'high'
  const result = validateEvaluationRequest(input)
  assert.equal(result.max_tokens, 2048)
  assert.deepEqual(result.thinking, { type: 'disabled' })
  assert.equal(Object.hasOwn(result, 'reasoning_effort'), false)
})

async function receiptFixture(run) {
  const output = await fs.mkdtemp(path.join(os.tmpdir(), 'tm-stock-tradingcn-live-'))
  try { await run(output) } finally {
    // 仅清理本测试刚创建且核对过父目录/名称的独占目录。
    assert.equal(path.dirname(output), os.tmpdir())
    assert.match(path.basename(output), /^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/)
    await fs.rm(output, { recursive: true })
  }
}

test('零请求预检不能记为完整真模型实跑', () => receiptFixture(async output => {
  const receipt = await recordReceipt({ output, child: 'passed', execute: false, summary: { verified: true, executed: false }, usage: [], requests: 0 })
  assert.equal(receipt.workflow_verified, false)
  assert.equal(receipt.requests, 0)
}))

test('真实回执须用量齐全、数量相符且无未知响应', () => receiptFixture(async output => {
  const state = { output, child: 'passed', execute: true, summary: { verified: true, executed: true, model_calls: 1 }, usage: [{ status: 200, input: 1, output: 1 }], requests: 1, unknown: false }
  assert.equal((await recordReceipt(state)).workflow_verified, true)
  state.unknown = true
  assert.equal((await recordReceipt(state)).workflow_verified, false)
  state.unknown = false
  state.usage[0].input = null
  assert.equal((await recordReceipt(state)).workflow_verified, false)
  state.usage = []
  assert.equal((await recordReceipt(state)).usage_complete, false)
}))

test('核心与报告用量分别计数，报告回包不能补足缺失的核心调用', () => receiptFixture(async output => {
  const state = { output, child: 'passed', execute: true, summary: { verified: true, executed: true, model_calls: 1 },
    usage: [{ phase:'core', status:200,input:2,output:1 },{phase:'report',status:200,input:3,output:1}], requests:2,unknown:false }
  const receipt = await recordReceipt(state)
  assert.equal(receipt.workflow_verified,true); assert.equal(receipt.report_model_calls,1); assert.equal(receipt.input_tokens,5)
  state.summary.model_calls=2
  assert.equal((await recordReceipt(state)).workflow_verified,false)
  state.usage[0].phase='report'; state.summary.model_calls=0
  assert.equal((await recordReceipt(state)).workflow_verified,false)
  state.child='waiting'
  await assert.rejects(recordReceipt(state),/不是已结束/)
}))
