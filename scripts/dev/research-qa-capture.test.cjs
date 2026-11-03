const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),os=require('node:os'),http=require('node:http');
const {execFile}=require('node:child_process'),{promisify}=require('node:util');
const {dataset,score,compare}=require('./research-eval.cjs'),{capture,binding}=require('./research-qa-capture.cjs');
const corpus=path.resolve(__dirname,'../../eval/research/cases.jsonl'),d=dataset(fs.readFileSync(corpus,'utf8'));
const token=require('node:crypto').randomBytes(24).toString('base64url'),clone=x=>JSON.parse(JSON.stringify(x));
const template='AI 资料问答：以下为所选授权资料中经核验的原文引文，不是新的报告。';
async function fixture(t){
  const responses=new Map(),requests=[];let hook=null,ordinal=0;
  const c={schema:'tm-stock-qa-capture-v1',datasetDigest:d.digest,split:'development',baseURL:'',modelVersion:'synthetic-http-only',promptVersion:'capture-unit-v1',bindings:[]};
  for(const item of d.cases){
    const b={id:item.id,conversationId:100+ordinal,seq:1,subjectId:200+ordinal,sources:[]};ordinal++;
    for(const [i,s]of item.sources.filter(s=>s.authorized).entries())b.sources.push({ref:s.id,snapshotId:ordinal*10+i+1,startByte:0,endByte:Buffer.byteLength(s.text),sourceType:'official_catalog',sourceUrl:'https://example.test/'+item.id+'/'+s.id,collectedAt:1793620000000});
    c.bindings.push(b);
    const status={seq:1,state:item.expected.state,subjectType:'theme',subjectId:b.subjectId,skill:'material_qa',question:item.question,createdAt:1793620001000,updatedAt:1793620001025};
    const answer={seq:1,state:status.state,template:status.state==='answered'?template:'所选授权资料中没有可直接支持本问题的证据，暂不能回答。',subjectName:item.subject,aiGenerated:status.state==='answered',modelName:status.state==='answered'?'synthetic-http-only':'',filingNo:status.state==='answered'?'only-test-filing':'',quotes:(item.expected.answers[0]||[]).map(ref=>{const loc=b.sources.find(s=>s.ref===ref);return {snapshotId:loc.snapshotId,startByte:loc.startByte,endByte:loc.endByte,excerpt:item.sources.find(s=>s.id===ref).text,sourceType:loc.sourceType,sourceUrl:loc.sourceUrl,collectedAt:loc.collectedAt};})};
    const base='/api/v1/ai/conversations/'+b.conversationId+'/questions/1';responses.set(base,status);responses.set(base+'/answer',answer);
  }
  const server=http.createServer((req,res)=>{
    requests.push({method:req.method,path:req.url});
    if(hook&&hook(req,res))return;
    if(req.method!=='GET'||req.headers.authorization!=='Bearer '+token||!responses.has(req.url)){res.writeHead(403);res.end();return;}
    res.setHeader('Content-Type','application/json');res.end(JSON.stringify({code:0,msg:'',data:responses.get(req.url)}));
  });
  await new Promise(r=>server.listen(0,'127.0.0.1',r));c.baseURL='http://127.0.0.1:'+server.address().port;
  t.after(()=>new Promise(r=>{server.closeAllConnections();server.close(r);}));
  return {c,responses,requests,setHook:f=>hook=f};
}
test('绑定覆盖全题全物料，非回环、越权、重复与缺凭据在联网前拒绝',async t=>{
  const f=await fixture(t);assert.equal(binding(d,f.c).m.cases.length,17);
  const authority=fs.readFileSync(path.resolve(__dirname,'../../server/internal/sources/source.go'),'utf8');
  const categories=[...authority.matchAll(/^\s*Category(Announcement|AnnualReport|Prospectus|OfficialList|Interactive)\s*=\s*"([^"]+)"/gm)].map(v=>v[2]);
  assert.equal(categories.length,5,'来源枚举必须核当前服务端，不能由HTTP替身自行定义');
  for(const category of categories){const c=clone(f.c);c.bindings[0].sources[0].sourceType=category;assert.doesNotThrow(()=>binding(d,c));}
  for(const mutate of [c=>c.baseURL='http://localhost:1234',c=>c.baseURL='https://example.test',c=>c.baseURL+='/?x=1',c=>c.bindings.pop(),c=>c.bindings[1]=clone(c.bindings[0]),c=>c.bindings[0].sources.pop(),c=>c.bindings[0].sources[0].endByte++,c=>c.bindings[0].sources[0].sourceUrl='https://user:pass@example.test/a',c=>c.bindings[0].sources[0].ref='withdrawn',c=>c.bindings[0].sources[0].sourceType='official_list']){
    const c=clone(f.c);mutate(c);assert.throws(()=>binding(d,c));
  }
  await assert.rejects(capture(d,f.c,''));assert.equal(f.requests.length,0);
});
test('双遍实际HTTP读取只归一化真实回包，不输出身份、正文、定位或令牌',async t=>{
  const f=await fixture(t),run=await capture(d,f.c,token);
  assert.equal(f.requests.length,68);assert.ok(f.requests.every(r=>r.method==='GET'));
  assert.equal(run.kind,'api_capture');assert.equal(run.mode,'stored_qa');assert.match(run.receiptDigest,/^[a-f0-9]{64}$/);
  const result=score(d,run);assert.equal(result.metrics.answerCoverage,1);assert.equal(result.measurement.source,'stored_question_timestamps');assert.equal(result.measurement.completeCostMicros,null);
  assert.throws(()=>compare(d,run,run),/incomparable_runs/);
  const raw=JSON.stringify(run);for(const secret of [token,'conversationId','subjectId','snapshotId','excerpt','sourceUrl',d.cases[0].question])assert.ok(!raw.includes(secret),secret);
  const forged=clone(run);forged.costRef='not-allowed';assert.throws(()=>score(d,forged));
});
test('第二遍撤权或变化导致整批拒绝，不能保留先前成功结果',async t=>{
  const f=await fixture(t);
  f.setHook((req,res)=>{if(f.requests.length>34&&req.url.endsWith('/answer')){res.writeHead(403);res.end('不应输出的私有正文');return true;}});
  await assert.rejects(capture(d,f.c,token));
  f.setHook(null);f.requests.length=0;
  f.setHook((req,res)=>{if(f.requests.length===35){const original=f.responses.get(req.url);f.responses.set(req.url,{...original,updatedAt:original.updatedAt+1});}});
  await assert.rejects(capture(d,f.c,token),/receipt_changed/);
});
test('原问题、对象、技能、时间和引用变动不能靠关键词相同导出',async t=>{
  const f=await fixture(t),statusPath=f.c.bindings[0].conversationId,base='/api/v1/ai/conversations/'+statusPath+'/questions/1';
  const status=clone(f.responses.get(base)),answer=clone(f.responses.get(base+'/answer'));
  for(const mutate of [s=>s.question+='不同问题',s=>s.subjectId++,s=>s.skill='report_qa',s=>s.state='queued',s=>s.updatedAt=s.createdAt-1,s=>s.internal='未知字段']){
    const s=clone(status);mutate(s);f.responses.set(base,s);await assert.rejects(capture(d,f.c,token));
  }
  f.responses.set(base,status);
  for(const mutate of [a=>a.subjectName+='不同对象',a=>a.template='模型自由解释',a=>a.quotes[0].startByte++,a=>a.quotes[0].excerpt+='改写',a=>a.quotes[0].sourceUrl='https://example.test/other',a=>a.quotes[0].collectedAt++,a=>a.quotes.push(clone(a.quotes[0])),a=>a.raw='内部输出']){
    const a=clone(answer);mutate(a);f.responses.set(base+'/answer',a);await assert.rejects(capture(d,f.c,token));
  }
});
test('实际失败保留failed且不读不存在的答案，不把失败算成拒答',async t=>{
  const f=await fixture(t),base='/api/v1/ai/conversations/'+f.c.bindings[0].conversationId+'/questions/1';f.responses.get(base).state='failed';
  const modelRefusal=[...f.responses.values()].find(v=>v.state==='refused'&&Object.hasOwn(v,'aiGenerated'));
  modelRefusal.aiGenerated=true;modelRefusal.modelName='synthetic-refusal-model';modelRefusal.filingNo='only-test-filing';
  const run=await capture(d,f.c,token);assert.equal(f.requests.length,66);assert.equal(run.results[0].state,'failed');assert.equal(score(d,run).counts.failed,1);
  assert.equal(f.requests.filter(r=>r.path===base+'/answer').length,0);
  run.results[0].costMicros=0;run.results[0].costRef='caller-invented';assert.throws(()=>score(d,run),/mixed_measurement/);
  modelRefusal.modelName='';await assert.rejects(capture(d,f.c,token),/invalid_answer/);
});
test('重定向、错误包装、坏JSON、过大响应和慢流均拒绝，不向第二主机传令牌',async t=>{
  const f=await fixture(t);let redirects=0;
  const other=http.createServer((req,res)=>{redirects++;res.end();});await new Promise(r=>other.listen(0,'127.0.0.1',r));t.after(()=>new Promise(r=>other.close(r)));
  for(const hook of [(req,res)=>{res.writeHead(302,{Location:'http://127.0.0.1:'+other.address().port});res.end();return true;},(req,res)=>{res.setHeader('Content-Type','application/json');res.end('{"code":40100,"msg":"私有内容","data":null}');return true;},(req,res)=>{res.setHeader('Content-Type','application/json');res.end('{"code":0,"code":1,"msg":"","data":null}');return true;},(req,res)=>{res.setHeader('Content-Type','application/json');res.end('x'.repeat(256*1024+1));return true;}]){
    f.setHook(hook);await assert.rejects(capture(d,f.c,token));
  }
  assert.equal(redirects,0);
  f.setHook((req,res)=>{res.writeHead(200,{'Content-Type':'application/json'});res.write('{');return true;});
  await assert.rejects(capture(d,f.c,token));
});
test('命令行使用环境凭据，任一读取失败时标准输出必须为空',async t=>{
  const f=await fixture(t),dir=fs.mkdtempSync(path.join(os.tmpdir(),'tm-stock-qa-capture-')),file=path.join(dir,'binding.json'),cli=path.join(__dirname,'research-qa-capture.cjs');
  fs.writeFileSync(file,JSON.stringify(f.c),{mode:0o600});
  const env={...process.env,TM_STOCK_EVAL_TOKEN:token};
  const first=await promisify(execFile)(process.execPath,[cli,corpus,file],{env});assert.equal(JSON.parse(first.stdout).kind,'api_capture');assert.ok(!first.stdout.includes(token));
  f.setHook((req,res)=>{res.writeHead(401);res.end('private:'+token);return true;});
  try{await promisify(execFile)(process.execPath,[cli,corpus,file],{env});assert.fail('错误读取仍输出');}catch(e){assert.equal(e.stdout,'');assert.equal(e.stderr,'问答回执采集失败（capture_unavailable），未输出评测记录。\n');}
});
test('采集记录类型不可冒充真实模型、回放或完整任务成绩',async t=>{
  const f=await fixture(t),run=await capture(d,f.c,token);
  for(const mutate of [r=>r.kind='real_model',r=>r.mode='same_materials',r=>r.mode='whole_task',r=>delete r.receiptDigest,r=>r.receiptDigest='not-a-digest',r=>r.results.pop()]){const r=clone(run);mutate(r);assert.throws(()=>score(d,r));}
});
