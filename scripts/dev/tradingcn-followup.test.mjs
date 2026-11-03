import test from 'node:test'
import assert from 'node:assert/strict'
import fs from 'node:fs/promises'
import os from 'node:os'
import path from 'node:path'
import {evaluationFollowupCalls,validFollowupQA,followupIsReady,followupProfile,translateEvaluationSchema,qaReplyMetadata} from './tradingcn-followup.mjs'
import {chainConversationOptions,finishChainVerification,hasFollowupProof,hasFollowupReadyProof} from './tradingcn-local-chain.mjs'
import {createInputJournal} from './tradingcn-live-gateway.mjs'
import {qaOnlyReceipt} from './tradingcn-followup-qa-live.mjs'

const report='dossier-comparison-report-v1'
test('问答结构并入首条系统消息且保留原数据，报告翻译与原协议相同',async()=>{
  const requests=JSON.parse(await fs.readFile(new URL('../../eval/agents/local-followup-wire.json',import.meta.url),'utf8'))
  for(const request of requests){
    const original=structuredClone(request),qa=translateEvaluationSchema(request,'qa'),report=translateEvaluationSchema(request)
    assert.deepEqual(request,original)
    assert.equal(qa.messages.length,2);assert.equal(qa.messages[1].content,request.messages[1].content)
    assert.ok(qa.messages[0].content.startsWith(request.messages[0].content+'\n'))
    assert.ok(qa.messages[0].content.includes(JSON.stringify(request.response_format.json_schema.schema)))
    if(request.response_format.json_schema.schema.required.includes('verdict'))for(const verdict of ['pass','block','uncertain'])assert.ok(qa.messages[0].content.includes(`"verdict":"${verdict}"`))
    assert.deepEqual(report.messages,[...request.messages,{role:'system',content:'只返回符合此JSON schema的JSON对象：'+JSON.stringify(request.response_format.json_schema.schema)}])
    assert.deepEqual(qa.response_format,{type:'json_object'})
  }
  assert.throws(()=>translateEvaluationSchema(requests[0],'other'))
})

test('问答诊断只输出结构和闭集枚举，空白与额外字段不冒充合格判定',()=>{
  assert.equal(qaReplyMetadata('   ').blank_content,true)
  assert.equal(qaReplyMetadata('[]').json_kind,'array')
  assert.equal(qaReplyMetadata('{"verdict":"pass","reason":"verified"}').closed_check_reply,true)
  const meta=qaReplyMetadata('{"verdict":"private-value","reason":"private-value","private-field":"private-content"}')
  assert.equal(meta.closed_check_reply,false);assert.equal(meta.verdict,null);assert.equal(meta.reason,null)
  assert.equal(JSON.stringify(meta).includes('private-'),false)
})
test('仅问答回执明确报告和Agent为替身，六次确认及原进程证明缺一不可',()=>{
  const calls=Array.from({length:6},()=>({state:'confirmed',status:200,input_tokens:1,output_tokens:1}))
  assert.equal(qaOnlyReceipt(calls,0,true).verified,true)
  assert.equal(qaOnlyReceipt(calls,0,true).classification,'real-qa-only-synthetic-report-and-agent-not-full-chain')
  assert.equal(qaOnlyReceipt(calls.slice(1),1,false).input_tokens,5)
  for(const [rows,exit,proof] of [[calls.slice(1),0,true],[calls,1,true],[calls,0,false],[calls.map((row,i)=>i?row:{...row,state:'response_unknown'}),0,true],[calls.map((row,i)=>i?row:{...row,input_tokens:null}),0,true]])assert.equal(qaOnlyReceipt(rows,exit,proof).verified,false)
})
test('联合范围必须显式七步深度HTTP，不接受隐式、浏览器或任意调用额度',()=>{
  assert.equal(evaluationFollowupCalls('',report,true),0)
  assert.equal(evaluationFollowupCalls(followupProfile,report,true),6)
  for(const args of [[null,report,true],[false,report,true],['other',report,true],[followupProfile,'',true],[followupProfile,report,false]])assert.throws(()=>evaluationFollowupCalls(...args))
  const valid={followup:followupProfile,reportProfile:report,processes:true,conversation:true,analysisType:'deep'}
  assert.equal(chainConversationOptions(valid).TM_TEST_AI_LIVE_FOLLOWUP,followupProfile)
  for(const patch of [{processBrowser:true},{processes:false},{conversation:false},{analysisType:'light'},{followup:'other'}])assert.throws(()=>chainConversationOptions({...valid,...patch}))
})

test('原SQL报告就绪与五次确认之前不能消费问答出口',()=>{
  const valid={profile:followupProfile,ready:true,child:'passed',reportCompleted:5,reportExpected:5,authorizationPresent:false}
  assert.equal(followupIsReady(valid),true)
  for(const patch of [{profile:''},{ready:false},{ready:1},{child:'running'},{child:'failed'},{reportCompleted:4},{reportCompleted:6},{reportExpected:3},{authorizationPresent:true},{authorizationPresent:undefined}])assert.equal(followupIsReady({...valid,...patch}),false)
})

function request(){return{model:'deepseek-flash',max_tokens:1024,messages:[{role:'system',content:'你是受限资料问答的内部校验器。仅测试'},{role:'user',content:JSON.stringify({data:{subject_type:'theme',subject_id:1,subject_name:'仅测试:合成题材',question:'所选资料的主题是什么？',sources:[{snapshot_id:1,hash:'a'.repeat(64),sentences:[{id:1,text:'示例题材是本测试资料的主题。'}]}],picks:null}})}],response_format:{type:'json_schema',json_schema:{name:'closed_output',strict:true,schema:{type:'object'}}}}}
test('问答请求仅接受固定资料对象、原问题、闭集来源和受限模型输出',()=>{
  assert.equal(validFollowupQA(request()),true)
  for(const edit of [x=>{x.messages[0].content='other'},x=>{x.messages[1].role='assistant'},x=>{x.max_tokens=2048},x=>{x.tools=[]},x=>{x.response_format.type='json_object'},x=>{x.response_format.json_schema.strict=false},x=>{x.messages[1].content='not-json'}]){const value=request();edit(value);assert.equal(validFollowupQA(value),false)}
  for(const edit of [v=>{v.subject_id='1'},v=>{v.subject_name='真实资料'},v=>{v.question='other'},v=>{v.sources=[]},v=>{v.sources[0].hash=['a'.repeat(64)]},v=>{v.sources[0].sentences[0].id='1'},v=>{v.picks=[{snapshot_id:1,sentence_ids:[1,3]}]},v=>{v.extra=true}]){const value=request(),body=JSON.parse(value.messages[1].content);edit(body.data);value.messages[1].content=JSON.stringify(body);assert.equal(validFollowupQA(value),false)}
  for(const patch of [{subject_name:'示例题材'},{question:'示例题材资料记载什么？'}]){const value=request(),body=JSON.parse(value.messages[1].content);Object.assign(body.data,patch);value.messages[1].content=JSON.stringify(body);assert.equal(validFollowupQA(value),false)}
})

test('原Go实际发送的六次问答HTTP请求与JS出口守卫一致',async()=>{
  const requests=JSON.parse(await fs.readFile(new URL('../../eval/agents/local-followup-wire.json',import.meta.url),'utf8'))
  assert.equal(requests.length,6)
  for(const request of requests)assert.equal(validFollowupQA(request),true)
})

test('联合成功必须有原进程最终证明及分别五/六次确认，不允许核心计数借用问答',async()=>{
  const output=await fs.mkdtemp(path.join(os.tmpdir(),'tm-stock-tradingcn-live-'))
  const usage=[{phase:'core',status:200,input:1,output:1},...Array.from({length:5},()=>({phase:'report',status:200,input:1,output:1})),...Array.from({length:6},()=>({phase:'qa',status:200,input:1,output:1}))]
  const bridge={evaluation:{output,reportProfile:report,followup:followupProfile,followupReady:true,execute:true,child:'passed',summary:{verified:true,executed:true,model_calls:1},requests:12,usage},counts:{submissions:1,polls:1,results:1},async close(){}}
  const v={status:'passed',exit:0,finished:Promise.resolve(),reportProfile:report,followup:followupProfile,followupVerified:true,sevenStepVerified:true,processes:true,processBrowser:false,conversation:true,conversationVerified:true,analysisType:'deep',deepVerified:true,processHTTPVerified:true}
  assert.equal((await finishChainVerification(bridge,v)).followup_verified,true)
  bridge.evaluation.summary.model_calls=7;assert.equal((await finishChainVerification(bridge,v)).verified,false);bridge.evaluation.summary.model_calls=1
  for(const patch of [{followupVerified:false},{processBrowser:true},{followup:''}])assert.equal((await finishChainVerification(bridge,{...v,...patch})).verified,false)
  bridge.evaluation.followupReady=false;assert.equal((await finishChainVerification(bridge,v)).verified,false);bridge.evaluation.followupReady=true
  bridge.evaluation.usage.pop();bridge.evaluation.requests--;assert.equal((await finishChainVerification(bridge,v)).verified,false)
})

test('报告和问答输入单独追加存证，拒绝其它文件名及写盘失败后继续发送',async()=>{
  const output=await fs.mkdtemp(path.join(os.tmpdir(),'tm-stock-tradingcn-live-'))
  const core=createInputJournal(output),model=createInputJournal(output,fs,'model-input-events.jsonl')
  await core.append({phase:'dispatch_planned',call:{seq:1,node:'news_analyst_v2'}})
  await model.append({phase:'dispatch_planned',call:{seq:1,phase:'report',state:'response_unknown'}})
  await model.append({phase:'response_received',call:{seq:1,phase:'report',state:'confirmed'}})
  await Promise.all([core.flush(),model.flush()])
  assert.equal((await fs.readFile(path.join(output,'input-events.jsonl'),'utf8')).trim().split('\n').length,1)
  const rows=(await fs.readFile(path.join(output,'model-input-events.jsonl'),'utf8')).trim().split('\n').map(row=>JSON.parse(row))
  assert.deepEqual(rows.map(row=>row.phase),['dispatch_planned','response_received'])
  for(const filename of ['../other','receipt.json','',null])assert.throws(()=>createInputJournal(output,fs,filename))
  let sent=0
  const broken=createInputJournal(output,{appendFile:async()=>{throw new Error('仅测试写盘故障')}},'model-input-events.jsonl')
  await assert.rejects(async()=>{await broken.append({phase:'dispatch_planned'});sent++})
  await assert.rejects(broken.flush())
  assert.equal(sent,0)
})

test('联合及就绪证明必须是原Go完整断言行',()=>{
  for(const [read,marker] of [[hasFollowupProof,'TM_STOCK_LIVE_FOLLOWUP_FEEDBACK_VERIFIED'],[hasFollowupReadyProof,'TM_STOCK_LIVE_REPORT_READY_FOR_FOLLOWUP']]){
    assert.equal(read(`    source.go:12: ${marker}\n`),true)
    for(const value of [marker,`    source.go:12: ${marker}`,`    source.go:12: ${marker}_extra\n`])assert.equal(read(value),false)
  }
})
