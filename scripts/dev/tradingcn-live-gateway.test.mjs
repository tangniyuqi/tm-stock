// 只测纯守卫，不启动子进程、不使用真实Key、不访问模型服务。
import test from 'node:test'
import assert from 'node:assert/strict'
import { validateEvaluationRequest, recordReceipt, evaluationRoute, evaluationBody, evaluationReportCalls, evaluationInputCall, evaluationInputBundle, createInputJournal, evaluationHTTPS } from './tradingcn-live-gateway.mjs'
import { EventEmitter } from 'node:events'
import { PassThrough, Readable } from 'node:stream'
import { randomBytes } from 'node:crypto'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'

test('输入发送前持久记录，失败/未知保留，写盘故障不得继续发送', async () => {
  const output=await fs.mkdtemp(path.join(os.tmpdir(),'tm-stock-tradingcn-live-'))
  const journal=createInputJournal(output)
  const first={phase:'dispatch_planned',call:{seq:1,request_json:'仅合成私有输入',state:'response_unknown',input_tokens:null,output_tokens:null}}
  const start=journal.append(first);first.call.request_json='后续变更不得改历史'
  const end=journal.append({phase:'response_unknown',seq:1,input_tokens:null,output_tokens:null})
  await Promise.all([start,end]);await journal.flush()
  const rows=(await fs.readFile(path.join(output,'input-events.jsonl'),'utf8')).trim().split('\n').map(JSON.parse)
  assert.equal(rows.length,2);assert.equal(rows[0].call.request_json,'仅合成私有输入');assert.equal(rows[1].input_tokens,null)
  let sent=0,writes=0
  const broken=createInputJournal(output,{appendFile:async()=>{writes++;throw new Error('仅测试写盘故障')}})
  await assert.rejects(async()=>{await broken.append({phase:'dispatch_planned'});sent++})
  await assert.rejects(()=>broken.append({phase:'another'}));assert.equal(sent,0);assert.equal(writes,1)
  assert.throws(()=>createInputJournal(path.join(os.tmpdir(),'unowned-folder')))
})

test('原Agent实际发送字节捕获绑定调用身份并拒绝凭据正文', async () => {
  const fixture=JSON.parse(await fs.readFile(new URL('../../eval/agents/inputs-synthetic-protocol.json',import.meta.url),'utf8'))
  const first=fixture.calls[0]
  const args={seq:1,node:first.node,callId:first.call_id,body:Buffer.from(first.request_json),model:first.model}
  const call=evaluationInputCall(args)
  assert.equal(call.request_json,first.request_json);assert.equal(call.request_digest,first.request_digest);assert.equal(call.prompt_digest,first.prompt_digest)
  assert.equal(call.state,'response_unknown');assert.equal(call.input_tokens,null);assert.equal(call.output_tokens,null)
  for(const edit of[x=>{x.seq=0},x=>{x.seq=129},x=>{x.node='other'},x=>{x.callId='bad'},x=>{x.model='other'},x=>{x.body=Buffer.alloc(512*1024+1)},x=>{x.secrets=['隔离提示私有标记']}]){const x={...args};edit(x);assert.throws(()=>evaluationInputCall(x))}
})

test('上游输入只接完整原工厂和实际网关调用的一一对应', async () => {
  const f=JSON.parse(await fs.readFile(new URL('../../eval/agents/inputs-synthetic-protocol.json',import.meta.url),'utf8'))
  const state={execute:true,child:'passed',unknown:false,route:{provider:'deepseek'},inputCalls:f.calls,summary:{verified:true,executed:true,model_calls:f.calls.length,node_model_calls:f.calls.map(c=>({node:c.node,call_id:c.call_id,success:true}))}}
  assert.equal(evaluationInputBundle(state,f.fixture_json).calls.length,14)
  for(const edit of[x=>{x.execute=false},x=>{x.child='running'},x=>{x.unknown=true},x=>{x.summary.verified=false},x=>{x.summary.executed=false},x=>{x.summary.model_calls--},x=>{x.inputCalls.pop()},x=>{x.inputCalls[0].state='response_unknown'},x=>{x.inputCalls[0].input_tokens=null},x=>{x.inputCalls[0].response_model='other'},x=>{x.inputCalls[1].call_id=x.inputCalls[0].call_id},x=>{x.summary.node_model_calls[1]=x.summary.node_model_calls[0]},x=>{x.summary.node_model_calls[0].node='other'},x=>{x.summary.node_model_calls[0].success=false}]){const x=structuredClone(state);edit(x);assert.throws(()=>evaluationInputBundle(x,f.fixture_json))}
})

test('固定研究经理委派必须完整存证，不能顶替主节点或隐藏失败', async () => {
  const f=JSON.parse(await fs.readFile(new URL('../../eval/agents/inputs-synthetic-protocol.json',import.meta.url),'utf8'))
  const delegated={...f.calls[0],seq:15,node:'chip_distribution_analyst_v2',call_id:'f'.repeat(32)}
  assert.equal(evaluationInputCall({seq:15,node:delegated.node,callId:delegated.call_id,body:Buffer.from(delegated.request_json),model:delegated.model}).node,delegated.node)
  const calls=[...f.calls,delegated]
  const state={execute:true,child:'passed',unknown:false,route:{provider:'deepseek'},inputCalls:calls,summary:{verified:true,executed:true,model_calls:15,node_model_calls:calls.map(c=>({node:c.node,call_id:c.call_id,success:true}))}}
  assert.equal(evaluationInputBundle(state,f.fixture_json).calls.length,15)
  for(const edit of[
    x=>{x.inputCalls[0].node=delegated.node;x.summary.node_model_calls[0].node=delegated.node},
    x=>{x.inputCalls[14].node='unknown_delegate';x.summary.node_model_calls[14].node='unknown_delegate'},
    x=>{x.summary.node_model_calls[14].success=false},
    x=>{x.inputCalls[14].state='response_unknown'},
    x=>{x.inputCalls[14].input_tokens=null},
    x=>{x.inputCalls[14].call_id=x.inputCalls[0].call_id},
    x=>{x.summary.node_model_calls.pop()},
  ]){const changed=structuredClone(state);edit(changed);assert.throws(()=>evaluationInputBundle(changed,f.fixture_json))}
})

const valid = () => ({ execute: true, method: 'POST', url: '/v1/chat/completions',
  headers: { authorization: 'Bearer test-session', 'content-type': 'application/json' },
  session: 'test-session', requests: 0,
  data: { model: 'deepseek-flash', messages: [{ role: 'user', content: '仅测试' }], max_tokens: 8, stream: false } })

test('HTTPS单次传输区分响应头/正文超时及故障，写盘失败不确认、不重试', async () => {
  for (const mode of ['confirmed','headers_deadline','response_deadline','transport_error','response_error','response_aborted','response_closed','invalid_json','http_rejected','journal_error','response_too_large','local_transport_error','local_write_error']) {
    let sends=0;const events=[];const credential=randomBytes(32).toString('hex');const body=Buffer.from(JSON.stringify(valid().data))
    const request=(options,callback)=>{
      if(mode==='local_transport_error')throw new Error('仅测试本地错误')
      assert.equal(options.hostname,'api.deepseek.com');assert.equal(options.rejectUnauthorized,true);assert.equal(options.headers.Authorization,'Bearer '+credential)
      const req=new EventEmitter();req.destroy=()=>{}
      req.end=(wire,written)=>{
        sends++;assert.deepEqual(wire,body);written()
        if(mode==='local_write_error')throw new Error('仅测试已尝试写出的错误')
        if(mode==='headers_deadline')return
        if(mode==='transport_error'){queueMicrotask(()=>req.emit('error',new Error('仅测试敏感地址')));return}
        const status=mode==='http_rejected'?503:200
        const bytes=mode==='invalid_json'?Buffer.from('仅测试非法响应'):mode==='response_too_large'?Buffer.alloc(4*1024*1024+1):Buffer.from(JSON.stringify({model:'deepseek-flash',usage:{prompt_tokens:1,completion_tokens:2}}))
        const incoming=['response_deadline','response_error','response_aborted','response_closed'].includes(mode)?new PassThrough():Readable.from([bytes])
        incoming.statusCode=status;callback(incoming)
        if(mode==='response_error')queueMicrotask(()=>incoming.emit('error',new Error('仅测试私有正文')))
        if(mode==='response_aborted')queueMicrotask(()=>incoming.emit('aborted'))
        if(mode==='response_closed')queueMicrotask(()=>incoming.emit('close'))
      }
      return req
    }
    const result=await evaluationHTTPS({route:evaluationRoute(),credential,body,request,timeoutMS:30,onEvent:async(phase,reason,status,observedAtMS)=>{events.push({phase,reason,status,observedAtMS});if(mode==='journal_error'&&phase==='response_headers')throw new Error('仅测试写盘失败')}})
    assert.equal(sends,mode==='local_transport_error'?0:1,mode)
    assert.equal(result.state,['confirmed','http_rejected'].includes(mode)?'confirmed':mode==='local_transport_error'?'not_sent':'response_unknown',mode)
    assert.equal(result.reason,mode==='confirmed'?'http_ok':mode,mode)
    assert.equal(JSON.stringify(events).includes(credential),false)
    assert.ok(events.every(e=>!JSON.stringify(e).includes('敏感')&&!JSON.stringify(e).includes('私有')))
    assert.ok(events.every(e=>Number.isSafeInteger(e.observedAtMS)&&e.observedAtMS>0&&e.observedAtMS<=Date.now()),mode)
    if(result.state==='confirmed')assert.deepEqual(events.map(e=>e.phase),['request_dispatched','response_headers','response_complete'],mode)
    assert.equal(events.filter(e=>['response_complete','response_unknown'].includes(e.phase)).length,mode==='journal_error'?0:1,mode)
  }
  await assert.rejects(()=>evaluationHTTPS({route:{...evaluationRoute(),hostname:'example.test'},credential:randomBytes(32).toString('hex'),body:Buffer.from('{}')}))
})

test('SDK两种输出上限只归一成max_tokens，冲突或非法别名拒绝', () => {
  const input=valid();delete input.data.max_tokens;input.data.max_completion_tokens=32
  const result=validateEvaluationRequest(input)
  assert.equal(result.max_tokens,32);assert.equal(Object.hasOwn(result,'max_completion_tokens'),false)
  assert.equal(input.data.max_completion_tokens,32)
  input.data.max_tokens=32;assert.equal(validateEvaluationRequest(input).max_tokens,32)
  input.data.max_tokens=31;assert.equal(validateEvaluationRequest(input),null)
  for(const value of[-1,0,1.5,'32',true]){const x=valid();delete x.data.max_tokens;x.data.max_completion_tokens=value;assert.equal(validateEvaluationRequest(x),null)}
  input.data.max_tokens=4096;input.data.max_completion_tokens=4096;assert.equal(validateEvaluationRequest(input).max_tokens,2048)
})

test('报告调用数只能选择旧三次或七步五次', () => {
  assert.equal(evaluationReportCalls(), 3)
  assert.equal(evaluationReportCalls('dossier-comparison-report-v1'), 5)
  for (const profile of [null, 5, true, {}, 'dossier-report-v1', 'other']) assert.throws(() => evaluationReportCalls(profile))
})

test('千问测试路由固定目的地址与模型，拒绝任意目的地且不改输入', () => {
  assert.throws(() => evaluationRoute('https://example.com'))
  const route = evaluationRoute('qwen-token-plan')
  assert.equal(route.hostname, 'token-plan.maas.qianwenaiapi.com')
  const data = validateEvaluationRequest(valid()), original = structuredClone(data)
  const body = evaluationBody(data, route)
  assert.equal(body.model, 'qwen3.7-plus')
  assert.equal(body.enable_thinking, false)
  assert.equal(Object.hasOwn(body, 'thinking'), false)
  assert.deepEqual(data, original)
  assert.throws(() => evaluationBody(data, { ...route, hostname: 'example.com' }))
  assert.equal(evaluationBody(data, evaluationRoute()).model, 'deepseek-flash')
})

test('报告模型使用实际身份，不能沿用Agent协议别名或接受任意模型', () => {
  const input=valid()
  input.model='qwen3.7-plus'
  assert.equal(validateEvaluationRequest(input),null)
  input.data.model='qwen3.7-plus'
  assert.equal(validateEvaluationRequest(input).model,'qwen3.7-plus')
  input.model='other'; input.data.model='other'
  assert.equal(validateEvaluationRequest(input),null)
})

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
