const test = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');
const path = require('node:path');
const os = require('node:os');
const {spawnSync} = require('node:child_process');
const {dataset,materials,score,compare,parseJSON} = require('./research-eval.cjs');
const {validTAP} = require('./verify-research-eval.cjs');
const corpus = path.resolve(__dirname,'../../eval/research/cases.jsonl');
const raw = fs.readFileSync(corpus,'utf8'), d = dataset(raw);
const clone = x=>JSON.parse(JSON.stringify(x));
// 本测试录制只校验评分器算术，不能当模型答案或质量成绩。
function replay() {
  return {schema:d.schema,datasetDigest:d.digest,split:'development',mode:'same_materials',kind:'replay',modelVersion:'synthetic-recording-v1',promptVersion:'scorer-test-v1',results:d.cases.map(c=>({id:c.id,state:c.expected.state,citations:c.expected.answers[0]||[],elapsedMs:100,costMicros:null,costRef:null,workflow:null}))};
}
const byID = (r,id)=>r.results.find(v=>v.id===id);

test('固定工程样例覆盖必要类别，规范摘要与物料边界稳定',()=>{
  assert.equal(d.cases.length,17);
  for(const tag of ['subject','historical_current','sample_production','planned_actual','unit','basis','scope','reprint','correction','withdrawal','injection','no_evidence'])assert.ok(d.cases.some(c=>c.tags.includes(tag)),tag);
  const reverse=d.cases.map(c=>JSON.stringify(c)).reverse().join('\n')+'\n';
  assert.equal(dataset(reverse).digest,d.digest);
  const m=materials(d,'development');assert.equal(m.cases.length,17);
  assert.ok(!JSON.stringify(m).includes('七批'));
  for(const c of m.cases){assert.ok(!Object.hasOwn(c,'expected'));assert.ok(!Object.hasOwn(c,'basis'));for(const s of c.sources)assert.deepEqual(Object.keys(s),['id','text']);}
  assert.throws(()=>materials(d,'holdout'),/empty_split/);
});
test('录制回放证明计分算术，未知费用保留为空',()=>{
  const report=score(d,replay());
  assert.equal(report.counts.answerable,7);assert.equal(report.counts.mustRefuse,10);
  assert.equal(report.metrics.strictAccuracy,1);assert.equal(report.metrics.answerCoverage,1);
  assert.equal(report.metrics.refusalRecall,1);assert.equal(report.metrics.supportPrecision,1);
  assert.equal(report.measurement.completeCostMicros,null);assert.equal(report.measurement.totalKnownCostMicros,null);
  assert.equal(report.measurement.unknownCostSamples,17);assert.equal(report.kind,'replay');assert.equal(report.annotation,'synthetic-engineering-unreviewed');
  assert.match(report.runDigest,/^[a-f0-9]{64}$/);assert.equal(report.runDigest,score(d,replay()).runDigest);
  const changed=replay();changed.results[0].elapsedMs++;assert.notEqual(score(d,changed).runDigest,report.runDigest);
});
test('全拒答不能得到高覆盖，全失败不能记为正确拒答',()=>{
  const r=replay();for(const v of r.results){v.state='refused';v.citations=[];}
  let s=score(d,r);assert.equal(s.metrics.answerCoverage,0);assert.equal(s.metrics.falseRefusalRate,1);assert.equal(s.metrics.supportPrecision,null);assert.equal(s.counts.falseRefusals,7);
  for(const v of r.results)v.state='failed';s=score(d,r);assert.equal(s.metrics.refusalRecall,0);assert.equal(s.metrics.strictAccuracy,0);assert.equal(s.counts.failed,17);
});
test('转载重复不加分，相同引用重复属于输出错误',()=>{
  const r=replay(), v=byID(r,'duplicate-reprint');v.citations=['original','copy'];
  const before=score(d,r);assert.equal(before.metrics.answerCoverage,1);
  v.citations=['original'];const one=score(d,r);assert.equal(one.counts.citedUnits,before.counts.citedUnits);assert.equal(one.counts.supportedUnits,before.counts.supportedUnits);
  v.citations=['original','original'];const bad=score(d,r);assert.ok(bad.rows.find(v=>v.id==='duplicate-reprint').errors.includes('duplicate_citation'));assert.equal(bad.counts.citedUnits,one.counts.citedUnits);
});
test('主体误读、无证据和未知引用均计错误，关键词不是支持',()=>{
  const r=replay();Object.assign(byID(r,'subject-explicit'),{citations:['b']});
  Object.assign(byID(r,'group-to-subsidiary'),{state:'answered',citations:['a']});
  byID(r,'period-versus-publication').citations=['not-present'];
  const s=score(d,r);assert.equal(s.counts.incorrectAnswers,3);assert.equal(s.tags.subject.errors,2);
  assert.ok(s.rows.find(v=>v.id==='period-versus-publication').errors.includes('unknown_citation'));
  assert.ok(s.metrics.supportPrecision<1);assert.ok(s.metrics.answerCoverage<1);
});
test('越权内容不得进入资料或贡献支持分',()=>{
  const r=replay();Object.assign(byID(r,'withdrawn-source'),{state:'answered',citations:['withdrawn']});
  const s=score(d,r);assert.ok(s.rows.find(v=>v.id==='withdrawn-source').errors.includes('unauthorized_citation'));assert.equal(s.counts.correctRefusals,9);assert.ok(s.metrics.supportPrecision<1);
});
test('漏题、重题、资料或分组替换和非法元数据必须拒绝评分',()=>{
  for(const mutate of [r=>r.results.pop(),r=>r.results[1]=clone(r.results[0]),r=>r.datasetDigest='0'.repeat(64),r=>r.split='holdout',r=>r.results[0].extra=true,r=>r.results[0].costMicros=1,r=>r.results[0].elapsedMs=-1,r=>r.results[0].elapsedMs=.5,r=>r.results[0].state='unknown',r=>r.results[0].workflow={}]){
    const r=replay();mutate(r);assert.throws(()=>score(d,r));
  }
});
test('当前授权或标注变化会改变数据集摘要，坏标注不能作为金标',()=>{
  // 选无引用答案的拒答题修改授权，避免把合法标注改成结构错误。
  const c=clone(d.cases);const target=c.find(v=>v.id==='withdrawn-source');target.sources[0].authorized=true;
  assert.notEqual(dataset(c.map(JSON.stringify).join('\n')).digest,d.digest);
  for(const mutate of [v=>v[1]=clone(v[0]),v=>v[0].annotation='human-reviewed',v=>v[0].expected={state:'answered',answers:[['missing']]},v=>v.find(c=>c.id==='duplicate-reprint').sources[1].text='被替换的正文']){
    const v=clone(d.cases);mutate(v);assert.throws(()=>dataset(v.map(JSON.stringify).join('\n')));
  }
  assert.throws(()=>dataset(raw+'\n'),/invalid_dataset/);
});
test('JSON重复键、转义键名与过深输入不接受',()=>{
  assert.throws(()=>parseJSON('{"id":1,"\\u0069d":2}'),/duplicate_json_key/);
  assert.throws(()=>parseJSON('{"outer":[{"id":1,"id":2}]}'),/duplicate_json_key/);
  assert.throws(()=>parseJSON('['.repeat(34)+'0'+']'.repeat(34)),/invalid_json/);
  assert.deepEqual(parseJSON('{"text":"括号[不是结构]，转义\\\"也不是结束","v":[true,false,null,2.3]}'),{text:'括号[不是结构]，转义"也不是结束',v:[true,false,null,2.3]});
});
test('完整任务必须计入找资料等耗时，同资料记录不能混比',()=>{
  const r=replay();r.mode='whole_task';assert.throws(()=>score(d,r),/invalid_workflow/);
  for(const v of r.results)v.workflow={searchMs:20,verifyMs:30,generationMs:40,interactionMs:10,operations:4};
  assert.equal(score(d,r).measurement.p95Operations,4);
  r.results[0].workflow.searchMs=200;assert.throws(()=>score(d,r),/invalid_workflow/);r.results[0].workflow.searchMs=20;
  assert.throws(()=>compare(d,replay(),r),/incomparable_runs/);
  const real=replay();real.kind='real_model';assert.throws(()=>compare(d,replay(),real),/incomparable_runs/);
  const both=compare(d,replay(),replay());assert.equal(both.left.kind,'replay');assert.equal(both.right.kind,'replay');assert.ok(!Object.hasOwn(both,'winner'));
});
test('费用分开记录已知未知，零值须有实际费用依据，和溢出拒绝',()=>{
  const r=replay();r.results[0].costMicros=0;r.results[0].costRef='synthetic:test-only-zero';
  let s=score(d,r);assert.equal(s.measurement.knownCostSamples,1);assert.equal(s.measurement.totalKnownCostMicros,0);assert.equal(s.measurement.completeCostMicros,null);
  for(const v of r.results){v.costMicros=1;v.costRef='synthetic:test-only';}s=score(d,r);assert.equal(s.measurement.completeCostMicros,17);
  r.results[0].costMicros=Number.MAX_SAFE_INTEGER;assert.throws(()=>score(d,r),/metric_overflow/);
});
test('原型属性标签不污染对象，命令行只离线输出且拒绝缺文件和混合计数',()=>{
  const modified=clone(d.cases);modified[0].tags=['constructor'];const dd=dataset(modified.map(JSON.stringify).join('\n')),rr=replay();rr.datasetDigest=dd.digest;
  assert.equal(score(dd,rr).tags.constructor.samples,1);assert.equal(Object.samples,undefined);
  const dir=fs.mkdtempSync(path.join(os.tmpdir(),'tm-stock-research-eval-'));
  const record=path.join(dir,'run.json');fs.writeFileSync(record,JSON.stringify(replay()));
  const cli=path.join(__dirname,'research-eval.cjs');
  for(const args of [['materials',corpus,'development'],['score',corpus,record],['compare',corpus,record,record]]){
    const r=spawnSync(process.execPath,[cli,...args],{encoding:'utf8'});assert.equal(r.status,0,r.stderr);assert.equal(JSON.parse(r.stdout).schema,d.schema);
  }
  for(const args of [[],['materials',corpus,'holdout'],['score',corpus,path.join(dir,'missing.json')],['materials',corpus,'development','unexpected']])assert.notEqual(spawnSync(process.execPath,[cli,...args],{encoding:'utf8'}).status,0);
});
test('门禁拒绝零用例、缺少通过项、跳过、取消和重复摘要',()=>{
  const tap=Array.from({length:21},(_,i)=>'ok '+(i+1)+' - 自测'+i).join('\n')+'\n# tests 21\n# pass 21\n# fail 0\n# cancelled 0\n# skipped 0\n# todo 0\n';
  assert.equal(validTAP(tap),true);
  for(const bad of ['',tap.replace('ok 1 - 自测0\n',''),tap.replace('# tests 21','# tests 0'),tap.replace('# pass 21','# pass 20'),tap.replace('# fail 0','# fail 1'),tap.replace('# skipped 0','# skipped 1'),tap.replace('# cancelled 0','# cancelled 1'),tap.replace('# todo 0','# todo 1'),tap+'# tests 21\n'])assert.equal(validTAP(bad),false);
});
