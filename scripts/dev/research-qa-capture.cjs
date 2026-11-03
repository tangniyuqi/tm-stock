// 只读当前会员问答回执；不提供提交、重试、模型、凭据落盘或外部主机入口。
const fs = require('node:fs');
const http = require('node:http');
const crypto = require('node:crypto');
const {dataset,materials,parseJSON,score} = require('./research-eval.cjs');
const fail = code => { throw new Error(code); };
const keys = (v,a)=>v!==null&&typeof v==='object'&&!Array.isArray(v)&&Object.keys(v).length===a.length&&a.every(k=>Object.hasOwn(v,k));
const positive = n=>Number.isSafeInteger(n)&&n>0;
const plain = s=>typeof s==='string'&&s.trim()===s&&s.length>0&&s.length<=128&&!/[\u0000-\u001f\u007f]/.test(s);
const diagnostics=new Set(['invalid_arguments','invalid_binding','invalid_origin','incomplete_binding','duplicate_question','invalid_locator','duplicate_locator','capture_timeout','capture_unavailable','response_too_large','invalid_response','invalid_status','invalid_answer','invalid_quote','unmapped_quote','missing_token','receipt_changed','invalid_capture','invalid_message','invalid_report','invalid_report_section','invalid_report_quote','invalid_report_url','invalid_report_binding']);
const skill=b=>b.skill??'material_qa';
const categories={'公告':'announcement','年报':'annual_report','招股书':'prospectus','官方产业目录':'official_catalog','互动易问答':'interactive_qa'};
const stable = v=>JSON.stringify(Array.isArray(v)?v.map(x=>JSON.parse(stable(x))):v&&typeof v==='object'?Object.fromEntries(Object.keys(v).sort().map(k=>[k,JSON.parse(stable(v[k]))])):v);
function binding(d,c) {
  if (!keys(c,['schema','datasetDigest','split','baseURL','modelVersion','promptVersion','bindings']) || !['tm-stock-qa-capture-v1','tm-stock-qa-capture-v2'].includes(c.schema) || c.datasetDigest!==d.digest || !plain(c.modelVersion) || !plain(c.promptVersion)) fail('invalid_binding');
  let base; try { base=new URL(c.baseURL); } catch { fail('invalid_origin'); }
  if (base.protocol!=='http:'||base.hostname!=='127.0.0.1'||!positive(Number(base.port))||base.username||base.password||base.pathname!=='/'||base.search||base.hash) fail('invalid_origin');
  const m=materials(d,c.split), seen=new Set(), locators=new Set(), questions=new Set();
  if (!Array.isArray(c.bindings)||c.bindings.length!==m.cases.length) fail('incomplete_binding');
  for(const b of c.bindings){
    const item=m.cases.find(v=>v.id===b.id);
    const fields=['id','conversationId','seq','subjectId','sources'];
    if(c.schema==='tm-stock-qa-capture-v2')fields.push('skill','taskId');
    if(!keys(b,fields)||!item||seen.has(b.id)||![b.conversationId,b.seq,b.subjectId].every(positive)||!Array.isArray(b.sources)||b.sources.length!==item.sources.length)fail('invalid_binding');
    if(c.schema==='tm-stock-qa-capture-v2'&&(!['material_qa','report_qa'].includes(b.skill)||(b.skill==='report_qa'?!positive(b.taskId):b.taskId!==0)))fail('invalid_binding');
    const q=b.conversationId+':'+b.seq;if(questions.has(q))fail('duplicate_question');questions.add(q);seen.add(b.id);
    const refs=new Set();
    for(const s of b.sources){
      const source=item.sources.find(v=>v.id===s.ref);
      if(!keys(s,['ref','snapshotId','startByte','endByte','sourceType','sourceUrl','collectedAt'])||!source||refs.has(s.ref)||!positive(s.snapshotId)||!Number.isSafeInteger(s.startByte)||s.startByte<0||!positive(s.endByte)||s.endByte-s.startByte!==Buffer.byteLength(source.text)||!positive(s.collectedAt)||!['announcement','annual_report','prospectus','official_catalog','interactive_qa'].includes(s.sourceType))fail('invalid_locator');
      let u;try{u=new URL(s.sourceUrl);}catch{fail('invalid_locator');}
      if(u.protocol!=='https:'||!u.hostname||u.username||u.password||u.href!==s.sourceUrl)fail('invalid_locator');
      const key=b.id+':'+s.snapshotId+':'+s.startByte+':'+s.endByte;
      if(locators.has(key))fail('duplicate_locator');locators.add(key);refs.add(s.ref);
    }
  }
  return {m,base};
}
function read(base,path,token,deadline,timeout=5000){
  return new Promise((resolve,reject)=>{
    const remaining=deadline-Date.now();if(remaining<=0)return reject(new Error('capture_timeout'));
    // 不使用代理/域名解析或自动跳转，固定回环IPv4和原只读路径。
    const req=http.request({hostname:'127.0.0.1',port:base.port,family:4,path,method:'GET',agent:false,headers:{Authorization:'Bearer '+token,Accept:'application/json'}},res=>{
      const parts=[];let bytes=0;
      res.on('data',p=>{bytes+=p.length;if(bytes>256*1024){req.destroy();reject(new Error('response_too_large'));}else parts.push(p);});
      res.on('error',()=>reject(new Error('capture_unavailable')));
      res.on('end',()=>{
        if(res.statusCode!==200||!/^application\/json(?:;|$)/.test(String(res.headers['content-type']||'')))return reject(new Error('capture_unavailable'));
        try{const v=parseJSON(Buffer.concat(parts).toString('utf8'));if(!keys(v,['code','msg','data'])||v.code!==0||typeof v.msg!=='string')fail('capture_unavailable');resolve(v.data);}catch{reject(new Error('invalid_response'));}
      });
    });
    const timer=setTimeout(()=>req.destroy(new Error('capture_timeout')),Math.min(timeout,remaining));
    req.once('close',()=>clearTimeout(timer));
    req.on('error',()=>reject(new Error('capture_unavailable')));req.end();
  });
}
function status(v,b,item){
  if(!keys(v,['seq','state','subjectType','subjectId','skill','question','createdAt','updatedAt'])||v.seq!==b.seq||v.subjectId!==b.subjectId||v.subjectType!=='theme'||v.skill!==skill(b)||v.question!==item.question||!['answered','refused','failed'].includes(v.state)||!positive(v.createdAt)||!positive(v.updatedAt)||v.updatedAt<v.createdAt||v.updatedAt-v.createdAt>86400000)fail('invalid_status');
}
function message(v,s,b,item){
  if(!keys(v,['conversationId','items','upperSeq','nextAfterSeq','hasMore'])||v.conversationId!==b.conversationId||!Array.isArray(v.items)||v.items.length!==1||!positive(v.upperSeq)||v.upperSeq<b.seq||v.nextAfterSeq!==b.seq||v.hasMore!==(v.upperSeq>b.seq))fail('invalid_message');
  const m=v.items[0];
  if(!keys(m,['seq','kind','subjectType','subjectId','skill','question','taskId','createdAt'])||m.seq!==b.seq||m.kind!=='question'||m.subjectType!=='theme'||m.subjectId!==b.subjectId||m.skill!=='report_qa'||m.question!==item.question||m.taskId!==b.taskId||!positive(m.createdAt)||m.createdAt>s.updatedAt)fail('invalid_message');
  return m;
}
function report(v,m,b,item){
  const fields=['taskId','subjectType','subjectId','subjectName','generatedAt','disclosure','sections'];
  if(v&&Object.hasOwn(v,'dossierReview'))fields.push('dossierReview');
  if(!keys(v,fields)||v.taskId!==b.taskId||v.subjectType!=='theme'||v.subjectId!==b.subjectId||v.subjectName!==item.subject||!positive(v.generatedAt)||v.generatedAt>m.createdAt||!keys(v.disclosure,['aiGenerated','modelName','filingNo'])||v.disclosure.aiGenerated!==true||!plain(v.disclosure.modelName)||!plain(v.disclosure.filingNo)||!Array.isArray(v.sections)||v.sections.length!==4)fail('invalid_report');
  const sections=new Set(),seen=new Set(),quotes=[];let count=0;
  for(const section of v.sections){
    const fields=['key','title','aiGenerated','quotes'];if(section&&Object.hasOwn(section,'companies'))fields.push('companies');
    if(!keys(section,fields)||!['definition','timeline','segment','companies'].includes(section.key)||sections.has(section.key)||!plain(section.title)||typeof section.aiGenerated!=='boolean'||section.key!=='companies'&&!section.aiGenerated||!Array.isArray(section.quotes))fail('invalid_report_section');
    sections.add(section.key);if(section.key==='companies')continue;
    for(const q of section.quotes){
      if(++count>16||!keys(q,['text','sourceType','sourceUrl','collectedAt'])||typeof q.text!=='string'||!q.text.length||q.text.trim()!==q.text||q.text.normalize('NFKC')!==q.text||/[\u0000-\u0008\u000b-\u001f\u007f]/.test(q.text)||Buffer.byteLength(q.text)>65536||!Object.hasOwn(categories,q.sourceType)||!positive(q.collectedAt)||q.collectedAt>v.generatedAt)fail('invalid_report_quote');
      let url;try{url=new URL(q.sourceUrl);}catch{fail('invalid_report_url');}
      if(url.protocol!=='https:'||!url.hostname||url.username||url.password||url.href!==q.sourceUrl)fail('invalid_report_url');
      const key=q.sourceUrl+'\u0000'+q.text;if(seen.has(key))continue;seen.add(key);quotes.push(q);
    }
  }
  if(!quotes.length)fail('invalid_report');
  for(const loc of b.sources){
    const q=quotes[loc.snapshotId-1],source=item.sources.find(s=>s.id===loc.ref);
    if(!q||categories[q.sourceType]!==loc.sourceType||q.sourceUrl!==loc.sourceUrl||q.collectedAt!==loc.collectedAt||loc.endByte>Buffer.byteLength(q.text)||Buffer.from(q.text).subarray(loc.startByte,loc.endByte).toString('utf8')!==source.text)fail('invalid_report_binding');
  }
  return crypto.createHash('sha256').update(stable(v)).digest('hex');
}
function answer(v,s,b,item){
  if(!keys(v,['seq','state','template','subjectName','aiGenerated','modelName','filingNo','quotes'])||v.seq!==b.seq||v.state!==s.state||v.subjectName!==item.subject||typeof v.aiGenerated!=='boolean'||typeof v.modelName!=='string'||typeof v.filingNo!=='string'||!Array.isArray(v.quotes)||v.quotes.length>4)fail('invalid_answer');
  const refusal=['所选授权资料中没有可直接支持本问题的证据，暂不能回答。','本问题超出当前所选题材或专题的授权资料范围，暂不能回答。','仅提供所选资料的客观事实引文，不能回答投资意见或执行范围外指令。','现有证据未通过问答核验，暂不能回答。'];
  if(v.aiGenerated? !plain(v.modelName)||!plain(v.filingNo):v.modelName!==''||v.filingNo!=='')fail('invalid_answer');
  if(v.state==='refused'){if(!refusal.includes(v.template)||v.quotes.length)fail('invalid_answer');return [];}
  if(!v.aiGenerated||!plain(v.modelName)||!plain(v.filingNo)||v.template!=='AI 资料问答：以下为所选授权资料中经核验的原文引文，不是新的报告。'||!v.quotes.length)fail('invalid_answer');
  const refs=[];
  for(const q of v.quotes){
    if(!keys(q,['snapshotId','startByte','endByte','excerpt','sourceType','sourceUrl','collectedAt']))fail('invalid_quote');
    const mapped=b.sources.find(r=>r.snapshotId===q.snapshotId&&r.startByte===q.startByte&&r.endByte===q.endByte&&r.sourceType===q.sourceType&&r.sourceUrl===q.sourceUrl&&r.collectedAt===q.collectedAt);
    if(!mapped||item.sources.find(v=>v.id===mapped.ref)?.text!==q.excerpt||refs.includes(mapped.ref))fail('unmapped_quote');refs.push(mapped.ref);
  }
  return refs;
}
async function capture(d,c,token){
  const {m,base}=binding(d,c);
  if(typeof token!=='string'||token.length<16||token.length>8192||/[\s\u0000-\u001f\u007f]/.test(token))fail('missing_token');
  const deadline=Date.now()+60000;
  const once=async b=>{
    const item=m.cases.find(v=>v.id===b.id), path='/api/v1/ai/conversations/'+b.conversationId+'/questions/'+b.seq;
    const s=await read(base,path,token,deadline);status(s,b,item);
    const originalMessage=skill(b)==='report_qa'?message(await read(base,'/api/v1/ai/conversations/'+b.conversationId+'/messages?after_seq='+(b.seq-1)+'&limit=1',token,deadline),s,b,item):null;
    const a=s.state==='failed'?null:await read(base,path+'/answer',token,deadline,skill(b)==='report_qa'?9000:5000);
    const citations=a===null?[]:answer(a,s,b,item);
    const reportDigest=originalMessage===null?null:report(await read(base,'/api/v1/ai/tasks/'+b.taskId+'/report',token,deadline,11000),originalMessage,b,item);
    return {s,a,citations,...(originalMessage===null?{}:{message:originalMessage,reportDigest})};
  };
  const first=[];
  for(const b of c.bindings)first.push(await once(b));
  // 不边读边打印；较早的问题在整批完成前也必须再核当前权限和原结果。
  for(let i=0;i<c.bindings.length;i++){if(stable(await once(c.bindings[i]))!==stable(first[i]))fail('receipt_changed');}
  const run={schema:d.schema,datasetDigest:d.digest,split:c.split,mode:'stored_qa',kind:'api_capture',modelVersion:c.modelVersion,promptVersion:c.promptVersion,
    receiptDigest:crypto.createHash('sha256').update(stable({binding:c,receipts:first})).digest('hex'),
    results:first.map((v,i)=>({id:c.bindings[i].id,state:v.s.state,citations:v.citations,elapsedMs:v.s.updatedAt-v.s.createdAt,costMicros:null,costRef:null,workflow:null}))};
  score(d,run);
  if(Date.now()>deadline||JSON.stringify(run).includes(token))fail('invalid_capture');
  return run;
}
module.exports={capture,binding};
if(require.main===module){(async()=>{
  if(process.argv.length!==4)fail('invalid_arguments');
  const d=dataset(fs.readFileSync(process.argv[2],'utf8')),c=parseJSON(fs.readFileSync(process.argv[3],'utf8'));
  const run=await capture(d,c,process.env.TM_STOCK_EVAL_TOKEN);process.stdout.write(JSON.stringify(run,null,2)+'\n');
})().catch(err=>{const code=diagnostics.has(err?.message)?err.message:'capture_failed';process.stderr.write('问答回执采集失败（'+code+'），未输出评测记录。\n');process.exitCode=1;});}
