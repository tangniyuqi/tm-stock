// 执行实际UTS解析器/订阅器；网络与时间可控，不冒充HBuilderX或浏览器验收。
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const vm = require('node:vm');
const { createRequire } = require('node:module');
if (!process.argv[2]) throw new Error('需要仓库外隔离依赖目录');
const requireDependency = createRequire(path.join(path.resolve(process.argv[2]), 'package.json'));
const ts = requireDependency('typescript');
const root = path.resolve(__dirname, '../..');
const event = (seq=2,type='claimed',status=2) => ({seq,type,status,step:0,errorCode:'',at:1792281600000});
const frame = value => `id: ${value.seq}\nevent: progress\ndata: ${JSON.stringify(value)}\n\n`;
const snapshot = (seq=2,status=2) => ({id:9,analysisType:1,subjectType:'theme',subjectId:7,status,currentStep:0,eventSeq:seq,cancelRequested:false,errorCode:'',deadlineAt:1792281600000,reportAvailable:false});
const flush = async () => { for(let n=0;n<30;n++) await Promise.resolve(); };
function environment() {
  const modules=new Map(),timers=new Map(),requests=[],connections=[],progress=[],events=[],modes=[],errors=[];
  let token='test-only-member',live=true,timerID=0;
  const hooks={current:()=>live,progress:v=>progress.push(v),event:v=>events.push(v),mode:v=>modes.push(v),error:c=>errors.push(c)};
  const globals={TextDecoder,AbortController,Uint8Array,uni:{getStorageSync:()=>token},window:{TextDecoder},
    setTimeout:(fn,ms)=>{const id=++timerID;timers.set(id,{fn,ms});return id},clearTimeout:id=>timers.delete(id)};
  function load(name){
    if(modules.has(name))return modules.get(name);
    if(path.basename(name)==='ai-api.uts') return {AiApi:{progress:id=>new Promise(resolve=>requests.push({id,resolve}))}};
    const output=ts.transpileModule(fs.readFileSync(name,'utf8'),{compilerOptions:{module:ts.ModuleKind.CommonJS,target:ts.ScriptTarget.ES2020},reportDiagnostics:true});
    assert.equal(output.diagnostics.filter(d=>d.category===ts.DiagnosticCategory.Error).length,0);
    const box={exports:{},...globals,require:p=>load(path.resolve(path.dirname(name),p))};vm.runInNewContext(output.outputText,box,{filename:name,timeout:2000});modules.set(name,box.exports);return box.exports;
  }
  globals.window.fetch=(url,options)=>new Promise(resolve=>connections.push({url,options,resolve}));
  const parser=load(path.join(root,'pages/libs/ai-task-stream.uts')),feed=load(path.join(root,'pages/libs/ai-task-feed.uts'));
  const runTimer=ms=>{const next=[...timers].find(([,t])=>t.ms===ms);assert.ok(next,`缺少${ms}ms调度`);timers.delete(next[0]);next[1].fn()};
  const response=(connection,status=200,contentType='text/event-stream; charset=utf-8')=>{
    let source;const body=new ReadableStream({start(c){source=c}});
    connection.options.signal.addEventListener('abort',()=>{try{source.error(new Error('测试终止'))}catch{}});
    connection.resolve({status,headers:{get:()=>contentType},body});return source;
  };
  return {parser,feed,globals,hooks,timers,requests,connections,progress,events,modes,errors,runTimer,response,stop:()=>{live=false},switch:()=>{token='test-only-other-member'}};
}
function parsers(){
  const {parser:{AITaskStreamParser:P,isValidAIPublicEvent:valid}}=environment();
  const types=[event(1,'created',1),event(2,'claimed',2),event(3,'retried',1),{...event(4,'step_completed'),step:1},event(5,'cancel_requested'),event(6,'succeeded',3),{...event(7,'failed',4),errorCode:'guard_rejected'},{...event(8,'cancelled',5),errorCode:'cancelled'},{...event(9,'expired',6),errorCode:'expired'}];
  for(const e of types)assert.equal(valid(e),true);
  for(const change of [v=>v.seq=0,v=>v.seq=9007199254740992,v=>v.at=-1,v=>v.status=3,v=>v.status='2',v=>v.type=['claimed'],v=>v.type='delta',v=>v.step=65,v=>v.errorCode='测试内部详情',v=>v.text='测试模型原文']){const e=event();change(e);assert.equal(valid(e),false)}
  const wire=': 已连接（仅测试）\r\n\r\n'+frame(types[0]).replaceAll('\n','\r\n')+frame(types[1]);
  // 每个字符/每个UTF-8字节边界（含中文注释）都须保留完整帧。
  for(let split=0;split<=wire.length;split++){const p=new P(0);const got=[...p.push(wire.slice(0,split)),...p.push(wire.slice(split))];p.finish();assert.equal(got.length,2)}
  const p=new P(0),decoder=new TextDecoder('utf-8',{fatal:true}),got=[];for(const byte of new TextEncoder().encode(wire))got.push(...p.push(decoder.decode(new Uint8Array([byte]),{stream:true})));p.push(decoder.decode());p.finish();assert.equal(got.length,2);
  const replay=new P(1);assert.equal(replay.push(frame(types[0])+frame(types[1])+frame(types[1])).length,1);
  for(const text of [frame(event(3)),frame({...event(),seq:'2'}),frame(event()).replace('id: 2','id: 02'),frame(event()).replace('id: 2','id: 3'),frame(event()).replace('event: progress','event: delta'),frame(event()).replace('event: progress','event: progress\nevent: progress'),frame(event()).replace('id: 2','id: 2\nid: 2'),'retry: 1\n\n','data: {}\n\n','id: 2\n\n','event: error\ndata: {"code":40100,"text":"测试内部详情"}\n\n','event: error\ndata: {"code":9}\n\n','\u0000', 'x'.repeat(8193), ':'.repeat(65537)]){const q=new P(1);assert.throws(()=>{q.push(text);q.finish()});assert.throws(()=>q.push(''), '损坏后不能复用解析器')}
  const partial=new P(1);assert.equal(partial.push(frame(event()).slice(0,-1)).length,0);assert.throws(()=>partial.finish());
  const error=new P(1).push('event: error\ndata: {"code":40100}\n\n');assert.equal(error[0].code,40100);
  assert.throws(()=>new P(-1));assert.equal(valid(null),false);
  console.log('通过：公开事件闭集、UTF-8逐字节/中文/CRLF拆帧、重放去重、坏帧/缺序/超限/EOF拒绝。');
}
async function feeds(){
  {
    const e=environment(),control=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),c=e.connections.shift();assert.equal(c.url,'/api/v1/ai/tasks/9/stream?after_seq=1');assert.equal(c.options.credentials,'omit');assert.equal(c.options.redirect,'error');assert.equal(c.options.headers.Authorization,'Bearer test-only-member');
    const body=e.response(c);await flush();body.enqueue(new TextEncoder().encode(frame(event())));await flush();assert.equal(e.events.length,1);assert.equal(e.progress.length,0,'事件不能授报告许可');assert.equal(e.requests.length,1);e.requests.shift().resolve({code:0,data:snapshot(2)});await flush();body.close();await flush();e.runTimer(1000);await flush();e.requests.shift().resolve({code:0,data:snapshot(2)});await flush();const again=e.connections.shift();assert.equal(again.url,'/api/v1/ai/tasks/9/stream?after_seq=2');control.close();assert.equal(again.options.signal.aborted,true);assert.equal(e.timers.size,0);again.resolve({status:401});await flush();assert.equal(e.errors.length,0);
  }
  for(const mode of ['404','429','503','wrong_type','gap','utf8','partial']){
    const e=environment(),control=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),c=e.connections.shift();const b=e.response(c,/^\d+$/.test(mode)?Number(mode):200,mode==='wrong_type'?'application/json':'text/event-stream');await flush();
    if(mode==='gap')b.enqueue(new TextEncoder().encode(frame(event(3))));if(mode==='utf8')b.enqueue(new Uint8Array([0xc0,0xaf]));if(mode==='partial'){b.enqueue(new TextEncoder().encode(frame(event()).slice(0,-1)));b.close()};await flush();assert.ok(e.modes.includes('poll'),mode);assert.equal(e.events.length,0);e.runTimer(3000);await flush();assert.equal(e.requests.length,1);e.requests.shift().resolve({code:0,data:snapshot(2,3)});await flush();assert.equal(e.progress.length,1);assert.equal(e.timers.size,0);assert.equal(e.connections.length,0);control.close();
  }
  for(const status of [401,403]){const e=environment();e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks);e.response(e.connections.shift(),status);await flush();assert.deepEqual(e.errors,[status===401?40100:40301]);assert.equal(e.timers.size,0)}
  {const e=environment(),ctl=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),c=e.connections.shift();e.response(c);await flush();e.runTimer(18000);await flush();assert.equal(c.options.signal.aborted,true,'看门狗必须中断实际读取');assert.ok(e.modes.includes('reconnecting'));assert.equal(e.events.length,0);e.runTimer(1000);await flush();e.requests.shift().resolve({code:0,data:snapshot(1)});await flush();assert.equal(e.connections.shift().url,'/api/v1/ai/tasks/9/stream?after_seq=1');ctl.close();assert.equal(e.timers.size,0)}
  for(const code of [40100,40403,42901,50302]){const e=environment(),ctl=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),b=e.response(e.connections.shift());await flush();b.enqueue(new TextEncoder().encode(`event: error\ndata: {"code":${code}}\n\n`));await flush();assert.equal(e.events.length,0);if(code===40100){assert.deepEqual(e.errors,[40100]);assert.equal(e.timers.size,0)}else{assert.ok(e.modes.includes('poll'));e.runTimer(3000);await flush();e.requests.shift().resolve({code:40403,data:null});await flush();assert.deepEqual(e.errors,[40403]);assert.equal(e.timers.size,0)}ctl.close()}
  {const e=environment(),ctl=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),b=e.response(e.connections.shift());await flush();b.enqueue(new TextEncoder().encode(frame(event())));await flush();e.requests.shift().resolve({code:0,data:snapshot(1)});await flush();assert.deepEqual(e.errors,[50001],'当前详情不能倒退越过确认游标');assert.equal(e.progress.length,0);assert.equal(e.timers.size,0);ctl.close()}
  for(const leave of ['close','switch','hide']){const e=environment(),ctl=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),b=e.response(e.connections.shift());await flush();b.enqueue(new TextEncoder().encode(frame(event())));await flush();assert.equal(e.requests.length,1);if(leave==='close')ctl.close();if(leave==='switch')e.switch();if(leave==='hide')e.stop();e.requests.shift().resolve({code:0,data:snapshot(2,3)});await flush();assert.equal(e.progress.length,0,'旧详情迟到不能恢复私有状态');assert.equal(e.timers.size,0);ctl.close()}
  for(const mode of ['close','switch','hide']){const e=environment(),ctl=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),c=e.connections.shift();const b=e.response(c);await flush();if(mode==='close')ctl.close();if(mode==='switch')e.switch();if(mode==='hide')e.stop();try{b.enqueue(new TextEncoder().encode(frame(event())))}catch{}await flush();assert.equal(e.events.length,0);assert.equal(e.progress.length,0);assert.equal(e.timers.size,0);ctl.close()}
  {const e=environment();delete e.globals.window.fetch;const c=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks);assert.equal(e.connections.length,0);e.runTimer(3000);await flush();e.switch();e.requests.shift().resolve({code:0,data:snapshot()});await flush();assert.equal(e.progress.length,0);assert.equal(e.timers.size,0);c.close()}
  {const e=environment(),c=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks),b=e.response(e.connections.shift());await flush();b.enqueue(new TextEncoder().encode(frame(event(2,'succeeded',3))));await flush();assert.equal(e.events.length,1);e.requests.shift().resolve({code:0,data:snapshot(2,3)});await flush();assert.equal(e.progress.length,1);assert.equal(e.timers.size,0);c.close()}
  {const e=environment(),c=e.feed.watchAITaskFeed(9,1,'test-only-member',e.hooks);for(const ms of [1000,2000,4000]){e.connections.shift().resolve(Promise.reject(new Error('测试断网')));await flush();e.runTimer(ms);await flush();e.requests.shift().resolve({code:0,data:snapshot()});await flush()};e.connections.shift().resolve(Promise.reject(new Error('测试断网')));await flush();assert.ok(e.modes.includes('poll'));assert.equal(e.timers.size,1);c.close()}
  console.log('通过：同源Bearer、18秒看门狗、有界原游标重连、GET降级、终态独立核查、401/403/切账号/离页清理。');
}
(async()=>{parsers();await feeds()})().catch(e=>{console.error(e);process.exitCode=1});
