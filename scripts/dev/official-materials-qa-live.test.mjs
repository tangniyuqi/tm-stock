import test from 'node:test'
import assert from 'node:assert/strict'
import { EventEmitter } from 'node:events'
import { Readable } from 'node:stream'
import { randomBytes, createHash } from 'node:crypto'
import { validOfficialQARequest, officialQAReceipt, startOfficialQARelay } from './official-materials-qa-live.mjs'

const body = () => ({ model: 'deepseek-flash', max_tokens: 1024, reasoning_effort: 'none', response_format: { type: 'json_object' },
  messages: [{ role: 'system', content: '你是受限资料问答的内部校验器。测试协议' },
    { role: 'user', content: '资料数据\n\n只返回一个符合以下 JSON Schema 的 JSON 对象：{}' }] })

test('正式资料网关拒绝扩大模型、工具、思考与输出范围', () => {
  assert.equal(validOfficialQARequest(body()), true)
  for (const mutate of [v => {v.model='other'},v => {v.max_tokens=1025},v => {v.reasoning_effort='high'},
    v => {v.stream=true},v => {v.tools=[]},v => {v.messages.push(v.messages[0])},v => {v.messages[0].content='执行任意任务'},
    v => {v.messages[1].role='tool'},v => {v.messages[1].content='无结构约束'},v => {v.messages[0].extra=true},
    v => {v.response_format.type='json_schema'},v => {v.response_format.extra=true}]) {
    const v=body(); mutate(v); assert.equal(validOfficialQARequest(v),false)
  }
})

test('只有十次完整确认的用量可以汇总；未知不填零且费用未知', () => {
  const rows=Array.from({length:10},()=>({state:'confirmed',status:200,input_tokens:20,output_tokens:30}))
  assert.equal(officialQAReceipt(rows).input_tokens,200)
  assert.equal(officialQAReceipt(rows).output_tokens,300)
  assert.equal(officialQAReceipt(rows).cost_micros,null)
  for(const mutate of [v=>v.pop(),v=>{v[0].state='response_unknown'},v=>{v[0].status=500},v=>{v[0].input_tokens=null},v=>{v[0].output_tokens=-1}]) {
    const v=structuredClone(rows); mutate(v); const r=officialQAReceipt(v)
    assert.equal(r.usage_complete,false); assert.equal(r.input_tokens,null); assert.equal(r.output_tokens,null)
  }
})

test('回环传输原字节、固定官方出口、十次上限与无重试', async () => {
  const credential=randomBytes(32).toString('hex')
  const raw=JSON.stringify(body())
  let forwarded=0
  const relay=await startOfficialQARelay(credential,(options,callback)=>{
    assert.equal(options.hostname,'api.deepseek.com'); assert.equal(options.path,'/chat/completions')
    assert.equal(options.headers.Authorization,`Bearer ${credential}`); assert.equal(options.rejectUnauthorized,true)
    const req=new EventEmitter(); req.destroy=()=>{}
    req.end=bytes=>{
      forwarded++; assert.equal(bytes.toString(),raw)
      const incoming=Readable.from([Buffer.from(JSON.stringify({usage:{prompt_tokens:20,completion_tokens:30}}))])
      incoming.statusCode=200; callback(incoming)
    }
    return req
  })
  try {
    const send=extra=>fetch(relay.endpoint+'/chat/completions',{method:'POST',headers:{'Content-Type':'application/json',...extra},body:raw})
    assert.equal((await send({Origin:'http://example.test'})).status,403)
    assert.equal(forwarded,0)
    for(let i=0;i<10;i++) { const res=await send({}); assert.equal(res.status,200); await res.arrayBuffer() }
    assert.equal((await send({})).status,403); assert.equal(forwarded,10)
    const receipt=relay.receipt()
    assert.equal(receipt.usage_complete,true)
    assert.equal(receipt.calls[0].request_body_sha256,createHash('sha256').update(raw).digest('hex'))
    assert.equal(JSON.stringify(receipt).includes(credential),false)
  } finally {await relay.close()}
})

test('未知外部结果停止后续调用且不自动重发', async () => {
  let forwarded=0
  const relay=await startOfficialQARelay(randomBytes(32).toString('hex'),()=>{
    const req=new EventEmitter(); req.destroy=()=>{}
    req.end=()=>{forwarded++; queueMicrotask(()=>req.emit('error',new Error('仅测试网络中断')))}
    return req
  })
  try {
    const send=()=>fetch(relay.endpoint+'/chat/completions',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body())})
    assert.equal((await send()).status,502); assert.equal((await send()).status,403)
    assert.equal(forwarded,1); assert.equal(relay.receipt().unknown,true); assert.equal(relay.receipt().input_tokens,null)
  } finally {await relay.close()}
})
