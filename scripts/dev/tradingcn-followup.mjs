// 单次本地联合试跑的固定范围；不授予生产模型或资料用途。
export const followupProfile = 'report-followup-feedback-v1'

// 报告沿原翻译；问答把结构约束合入首条系统消息，避免末尾系统消息歧义。
export function translateEvaluationSchema(data, phase='report') {
  if(!['qa','report'].includes(phase)||data?.response_format?.type!=='json_schema'||!data.response_format.json_schema?.schema||typeof data.response_format.json_schema.schema!=='object'||Array.isArray(data.response_format.json_schema.schema)||!Array.isArray(data.messages)||data.messages[0]?.role!=='system'||typeof data.messages[0].content!=='string')throw new Error('本地JSON结构翻译参数无效')
  const schema=data.response_format.json_schema.schema
  const instruction=`只返回符合此JSON schema的JSON对象：${JSON.stringify(schema)}`
  if(phase==='report')return{...data,messages:[...data.messages,{role:'system',content:instruction}],response_format:{type:'json_object'}}
  let constraint='只输出一个合法json对象，不输出Markdown、解释或schema本身。'+instruction
  if(Array.isArray(schema.required)&&schema.required.length===2&&schema.required.includes('verdict')&&schema.required.includes('reason'))constraint+='\n结构示例（不是本次判定）：{"verdict":"pass","reason":"verified"}；{"verdict":"block","reason":"unsupported"}；{"verdict":"uncertain","reason":"uncertain"}。必须依据原问题和资料作出本次判定，不能照抄示例结论。'
  return{...data,messages:data.messages.map((row,i)=>i===0?{...row,content:row.content+'\n'+constraint}:row),response_format:{type:'json_object'}}
}

// 只保留结构和闭集分类，不能把供应商解释或未知字段名写入诊断。
export function qaReplyMetadata(content) {
  const result={json_kind:'invalid_json',blank_content:typeof content==='string'&&content.trim()==='',closed_check_reply:false,field_names:[],verdict:null,reason:null}
  let reply
  try{reply=JSON.parse(content)}catch{return result}
  result.json_kind=reply===null?'null':Array.isArray(reply)?'array':typeof reply
  if(result.json_kind!=='object')return result
  const keys=Object.keys(reply).sort()
  result.closed_check_reply=keys.join(',')==='reason,verdict'
  result.field_names=keys.map(key=>['verdict','reason','action','citations','data','results','status','message','error','explanation','analysis','answer'].includes(key)?key:'other')
  result.verdict=['pass','block','uncertain'].includes(reply.verdict)?reply.verdict:null
  result.reason=['none','no_evidence','verified','unsupported','out_of_scope','request_not_allowed','uncertain'].includes(reply.reason)?reply.reason:null
  return result
}

export function evaluationFollowupCalls(profile = '', report = '', keepAlive = false) {
  if (profile === '') return 0
  if (profile !== followupProfile || report !== 'dossier-comparison-report-v1' || keepAlive !== true) throw new Error('报告后联合验收须显式七步存活会话')
  return 6
}

export function followupIsReady({profile,ready,child,reportCompleted,reportExpected,authorizationPresent}) {
  return profile === followupProfile && ready === true && child === 'passed' && reportExpected === 5 && reportCompleted === 5 && authorizationPresent === false
}

export function validFollowupQA(data) {
  if (!data || Object.keys(data).some(key => !['model','messages','max_tokens','temperature','seed','response_format'].includes(key))
    || data.max_tokens !== 1024 || data.response_format?.type !== 'json_schema' || data.response_format.json_schema?.strict !== true
    || !Array.isArray(data.messages) || data.messages.length !== 2 || data.messages[0]?.role !== 'system' || data.messages[1]?.role !== 'user'
    || typeof data.messages[0].content !== 'string' || !data.messages[0].content.startsWith('你是受限资料问答的内部校验器。')
    || typeof data.messages[1].content !== 'string' || data.messages.some(row => Object.keys(row).sort().join(',') !== 'content,role')) return false
  let body
  try { body = JSON.parse(data.messages[1].content) } catch { return false }
  const v = body?.data
  return !!body && Object.keys(body).join(',') === 'data' && !!v && Object.keys(v).sort().join(',') === 'picks,question,sources,subject_id,subject_name,subject_type'
    && v.subject_type === 'theme' && v.subject_id === 1 && v.subject_name === '仅测试:合成题材'
    && v.question === '所选资料的主题是什么？' && Array.isArray(v.sources) && v.sources.length >= 1 && v.sources.length <= 4
    && v.sources.every(s => !!s && Object.keys(s).sort().join(',') === 'hash,sentences,snapshot_id' && Number.isSafeInteger(s.snapshot_id) && s.snapshot_id > 0
      && typeof s.hash === 'string' && /^[a-f0-9]{64}$/.test(s.hash) && Array.isArray(s.sentences) && s.sentences.length >= 1 && s.sentences.length <= 64
      && s.sentences.every(r => !!r && Object.keys(r).sort().join(',') === 'id,text' && Number.isSafeInteger(r.id) && r.id > 0 && typeof r.text === 'string'))
    && (v.picks === null || Array.isArray(v.picks) && v.picks.length <= 4 && v.picks.every(p => !!p && Object.keys(p).sort().join(',') === 'sentence_ids,snapshot_id'
      && Number.isSafeInteger(p.snapshot_id) && p.snapshot_id > 0 && Array.isArray(p.sentence_ids) && p.sentence_ids.length >= 1 && p.sentence_ids.length <= 8
      && p.sentence_ids.every((id,i) => Number.isSafeInteger(id) && id > 0 && (i === 0 || id === p.sentence_ids[i-1]+1))))
}
