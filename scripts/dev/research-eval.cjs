// 资料研究评测的离线后处理：只读指定文件，不调用模型、资料服务或账本。
const fs = require('node:fs');
const crypto = require('node:crypto');
const schema = 'tm-stock-research-eval-v1';
const fail = code => { throw new Error(code); };
const object = v => v !== null && typeof v === 'object' && !Array.isArray(v);
const keys = (v, expected) => object(v) && Object.keys(v).length === expected.length && expected.every(k => Object.hasOwn(v, k));
const id = v => typeof v === 'string' && /^[a-z][a-z0-9_-]{0,63}$/.test(v);
const text = (v, max) => typeof v === 'string' && v.trim() === v && v.length > 0 && v.length <= max && !/[\u0000-\u0008\u000b-\u001f\u007f]/.test(v);
const count = v => Number.isSafeInteger(v) && v >= 0;
const unique = a => new Set(a).size === a.length;
const canonical = v => JSON.stringify(sort(v));
function sort(v) {
  if (Array.isArray(v)) return v.map(sort);
  if (object(v)) return Object.fromEntries(Object.keys(v).sort().map(k => [k, sort(v[k])]));
  return v;
}

// JSON.parse会吞掉重复键，先按已验证语法遍历并检查解码后的键，避免歧义摘要。
function parseJSON(raw) {
  if (typeof raw !== 'string' || Buffer.byteLength(raw) > 8 * 1024 * 1024) fail('invalid_json');
  let value;
  try { value = JSON.parse(raw); } catch { fail('invalid_json'); }
  let p = 0;
  const space = () => { while (/\s/.test(raw[p] || '') && p < raw.length) p++; };
  const string = () => {
    const start = p++;
    while (p < raw.length) { if (raw[p] === '\\') p += 2; else if (raw[p++] === '"') break; }
    return JSON.parse(raw.slice(start, p));
  };
  const walk = depth => {
    if (depth > 32) fail('invalid_json');
    space();
    if (raw[p] === '"') { string(); return; }
    if (raw[p] === '{') {
      p++; space(); const seen = new Set();
      if (raw[p] === '}') { p++; return; }
      while (p < raw.length) {
        space(); const key = string();
        if (seen.has(key)) fail('duplicate_json_key');
        seen.add(key); space(); p++; walk(depth + 1); space();
        if (raw[p++] === '}') return;
      }
    } else if (raw[p] === '[') {
      p++; space(); if (raw[p] === ']') { p++; return; }
      while (p < raw.length) { walk(depth + 1); space(); if (raw[p++] === ']') return; }
    } else { while (p < raw.length && !/[\s,}\]]/.test(raw[p])) p++; }
  };
  walk(0);
  return value;
}

function dataset(raw) {
  if (typeof raw !== 'string' || Buffer.byteLength(raw) > 2 * 1024 * 1024) fail('invalid_dataset');
  const normalized = raw.replace(/\r\n/g, '\n');
  const lines = (normalized.endsWith('\n') ? normalized.slice(0,-1) : normalized).split('\n');
  if (lines.length < 1 || lines.length > 1000 || lines.some(l => !l.trim())) fail('invalid_dataset');
  const cases = lines.map(l => parseJSON(l));
  const seen = new Set();
  for (const c of cases) {
    if (!keys(c, ['id','split','tags','subject','question','sources','expected','basis','annotation']) || !id(c.id) || seen.has(c.id) || !['development','holdout'].includes(c.split) || !Array.isArray(c.tags) || !c.tags.length || c.tags.length > 8 || !c.tags.every(id) || !unique(c.tags) || !text(c.subject,128) || !text(c.question,1000) || !text(c.basis,2000) || c.annotation !== 'synthetic-engineering-unreviewed' || !Array.isArray(c.sources) || c.sources.length > 32) fail('invalid_case');
    seen.add(c.id);
    const refs = new Map(), units = new Map();
    for (const s of c.sources) {
      if (!keys(s,['id','unit','text','authorized']) || !id(s.id) || !id(s.unit) || !text(s.text,4000) || typeof s.authorized !== 'boolean' || refs.has(s.id) || units.has(s.unit) && units.get(s.unit) !== s.text) fail('invalid_source');
      refs.set(s.id,s); units.set(s.unit,s.text);
    }
    const e = c.expected;
    if (!keys(e,['state','answers']) || !['answered','refused'].includes(e.state) || !Array.isArray(e.answers) || e.answers.length > 8 || (e.state === 'answered') !== (e.answers.length > 0)) fail('invalid_gold');
    for (const a of e.answers) {
      if (!Array.isArray(a) || !a.length || a.length > 4 || !unique(a) || a.some(k => !refs.get(k)?.authorized)) fail('invalid_gold');
    }
  }
  cases.sort((a,b)=>a.id<b.id?-1:a.id>b.id?1:0);
  const digest = crypto.createHash('sha256').update(canonical({schema,cases})).digest('hex');
  return {schema,digest,cases};
}

function selected(d, split) {
  if (!['development','holdout'].includes(split)) fail('invalid_split');
  const cases = d.cases.filter(c=>c.split===split);
  if (!cases.length) fail('empty_split');
  return cases;
}
function materials(d, split) {
  return {schema,datasetDigest:d.digest,split,annotation:'synthetic-engineering-unreviewed',cases:selected(d,split).map(c=>({id:c.id,subject:c.subject,question:c.question,sources:c.sources.filter(s=>s.authorized).map(s=>({id:s.id,text:s.text}))}))};
}
function validateRun(d, run) {
  const capture = run?.kind === 'api_capture';
  const fields = ['schema','datasetDigest','split','mode','kind','modelVersion','promptVersion','results'];
  if (capture) fields.push('receiptDigest');
  if (!keys(run,fields) || run.schema !== schema || run.datasetDigest !== d.digest || !['same_materials','whole_task','stored_qa'].includes(run.mode) || !['replay','real_model','human','api_capture'].includes(run.kind) || capture !== (run.mode === 'stored_qa') || capture && (typeof run.receiptDigest !== 'string' || !/^[a-f0-9]{64}$/.test(run.receiptDigest)) || !text(run.modelVersion,128) || !text(run.promptVersion,128) || !Array.isArray(run.results)) fail('invalid_run');
  const cases = selected(d,run.split), ids = new Set(cases.map(c=>c.id)), seen = new Set();
  if (run.results.length !== cases.length) fail('incomplete_run');
  for (const r of run.results) {
    if (!keys(r,['id','state','citations','elapsedMs','costMicros','costRef','workflow']) || !ids.has(r.id) || seen.has(r.id) || !['answered','refused','failed'].includes(r.state) || !Array.isArray(r.citations) || r.citations.length > 32 || !r.citations.every(id) || !count(r.elapsedMs) || r.elapsedMs > 86400000 || r.costMicros !== null && !count(r.costMicros) || (r.costMicros === null ? r.costRef !== null : !text(r.costRef,128)) || r.state !== 'answered' && r.citations.length) fail('invalid_result');
    if ((run.mode === 'same_materials' || capture) && r.workflow !== null || capture && (r.costMicros !== null || r.costRef !== null)) fail('mixed_measurement');
    if (run.mode === 'whole_task') {
      const w = r.workflow;
      if (!keys(w,['searchMs','verifyMs','generationMs','interactionMs','operations']) || Object.values(w).some(v=>!count(v)) || w.searchMs+w.verifyMs+w.generationMs+w.interactionMs > r.elapsedMs) fail('invalid_workflow');
    }
    seen.add(r.id);
  }
  return cases;
}
const ratio = (n,d) => d ? n/d : null;
const p95 = a => a.length ? [...a].sort((a,b)=>a-b)[Math.ceil(a.length*.95)-1] : null;
function score(d, run) {
  const cases = validateRun(d,run), results = new Map(run.results.map(r=>[r.id,r]));
  const n = {samples:cases.length,answerable:0,mustRefuse:0,answered:0,refused:0,failed:0,correctAnswers:0,correctRefusals:0,falseRefusals:0,incorrectAnswers:0,citedUnits:0,supportedUnits:0};
  const rows = [], tags = Object.create(null), costs = [], elapsed = [], operations = [];
  for (const c of cases) {
    const r = results.get(c.id), refs = new Map(c.sources.map(s=>[s.id,s])), expected = c.expected;
    n[expected.state === 'answered' ? 'answerable' : 'mustRefuse']++;
    n[r.state]++;
    const errors = [], cited = new Set();
    if (r.state === 'answered') {
      if (!r.citations.length) errors.push('empty_answer');
      if (!unique(r.citations)) errors.push('duplicate_citation');
      for (const id of new Set(r.citations)) {
        const s = refs.get(id);
        if (!s) { cited.add('unknown:'+id); errors.push('unknown_citation'); }
        else if (!s.authorized) { cited.add('denied:'+s.unit); errors.push('unauthorized_citation'); }
        else cited.add(s.unit);
      }
      const alternatives = expected.answers.map(a=>new Set(a.map(id=>refs.get(id).unit)));
      const supported = new Set(alternatives.flatMap(a=>[...a]));
      n.citedUnits += cited.size;
      n.supportedUnits += [...cited].filter(u=>supported.has(u)).length;
      if (expected.state !== 'answered') errors.push('should_refuse');
      else if (!alternatives.some(a=>a.size===cited.size && [...a].every(u=>cited.has(u)))) errors.push('unsupported_or_incomplete');
      if (errors.length) n.incorrectAnswers++; else n.correctAnswers++;
    } else if (r.state === 'refused') {
      if (expected.state === 'refused') n.correctRefusals++;
      else { n.falseRefusals++; errors.push('false_refusal'); }
    } else errors.push('execution_failed');
    elapsed.push(r.elapsedMs);
    if (r.costMicros !== null) costs.push(r.costMicros);
    if (r.workflow !== null) operations.push(r.workflow.operations);
    for (const tag of c.tags) { if (!tags[tag]) tags[tag]={samples:0,errors:0}; tags[tag].samples++; if (errors.length) tags[tag].errors++; }
    rows.push({id:c.id,correct:errors.length===0,errors:[...new Set(errors)]});
  }
  const total = costs.reduce((a,b)=>a+b,0);
  if (!Number.isSafeInteger(total)) fail('metric_overflow');
  return {schema,datasetDigest:d.digest,runDigest:crypto.createHash('sha256').update(canonical(run)).digest('hex'),split:run.split,mode:run.mode,kind:run.kind,modelVersion:run.modelVersion,promptVersion:run.promptVersion,annotation:'synthetic-engineering-unreviewed',counts:n,
    metrics:{supportPrecision:ratio(n.supportedUnits,n.citedUnits),answerCoverage:ratio(n.correctAnswers,n.answerable),refusalRecall:ratio(n.correctRefusals,n.mustRefuse),falseRefusalRate:ratio(n.falseRefusals,n.answerable),strictAccuracy:ratio(n.correctAnswers+n.correctRefusals,n.samples)},
    ...(run.kind==='api_capture'?{receiptDigest:run.receiptDigest}:{}),
    measurement:{source:run.kind==='api_capture'?'stored_question_timestamps':'input_records',p95ElapsedMs:p95(elapsed),p95Operations:p95(operations),p95KnownCostMicros:p95(costs),costUnit:'CNY_micro',knownCostSamples:costs.length,unknownCostSamples:n.samples-costs.length,totalKnownCostMicros:costs.length?total:null,completeCostMicros:costs.length===n.samples?total:null},tags,rows};
}
function compare(d, left, right) {
  const a = score(d,left), b = score(d,right);
  if (a.mode === 'stored_qa' || b.mode === 'stored_qa' || a.mode !== b.mode || a.split !== b.split || (a.kind === 'replay') !== (b.kind === 'replay')) fail('incomparable_runs');
  return {schema,datasetDigest:d.digest,mode:a.mode,split:a.split,left:a,right:b};
}
function main(args) {
  const [command, corpus, ...files] = args;
  if (!['materials','score','compare'].includes(command) || !corpus || files.length !== (command==='compare'?2:1)) fail('usage: materials dataset.jsonl split | score dataset.jsonl run.json | compare dataset.jsonl left.json right.json');
  const d = dataset(fs.readFileSync(corpus,'utf8'));
  const out = command === 'materials' ? materials(d,files[0]) : command === 'score' ? score(d,parseJSON(fs.readFileSync(files[0],'utf8'))) : compare(d,...files.map(f=>parseJSON(fs.readFileSync(f,'utf8'))));
  process.stdout.write(JSON.stringify(out,null,2)+'\n');
}
module.exports = {dataset,materials,score,compare,parseJSON};
if (require.main === module) { try { main(process.argv.slice(2)); } catch (err) { process.stderr.write('离线评测失败：'+err.message+'\n'); process.exitCode=1; } }
