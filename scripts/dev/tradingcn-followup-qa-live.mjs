// 仅问答的真实供应商定位入口；报告和Agent仍为合成替身，绝不宣称完整多Agent通过。
import http from 'node:http'
import fs from 'node:fs/promises'
import {createWriteStream} from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import {spawn} from 'node:child_process'
import {createHash} from 'node:crypto'
import {fileURLToPath} from 'node:url'
import {evaluationHTTPS,evaluationRoute} from './tradingcn-live-gateway.mjs'
import {validFollowupQA,translateEvaluationSchema,qaReplyMetadata} from './tradingcn-followup.mjs'

export function qaOnlyReceipt(calls,exit,proof) {
  const usageComplete=calls.length>0&&calls.every(row=>Number.isSafeInteger(row.input_tokens)&&row.input_tokens>0&&Number.isSafeInteger(row.output_tokens)&&row.output_tokens>0)
  const complete=calls.length===6&&usageComplete&&calls.every(row=>row.state==='confirmed'&&row.status===200)
  return {classification:'real-qa-only-synthetic-report-and-agent-not-full-chain',verified:exit===0&&proof===true&&complete,requests:calls.length,unknown:calls.some(row=>row.state!=='confirmed'),usage_complete:usageComplete,input_tokens:usageComplete?calls.reduce((n,row)=>n+row.input_tokens,0):null,output_tokens:usageComplete?calls.reduce((n,row)=>n+row.output_tokens,0):null,actual_cost:null}
}

export async function runFollowupQATrial({mysqlBase,powershell,root=process.cwd()}) {
  if(![mysqlBase,powershell,root].every(value=>typeof value==='string'&&path.isAbsolute(value)))throw new Error('仅问答实调须明确绝对路径')
  const raw=await fs.readFile(path.join(os.homedir(),'.tmstock-credentials'),'utf8')
  const match=raw.match(/^\s*(?:export\s+)?DEEPSEEK_API_KEY\s*=\s*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|([^\s#]+))\s*(?:#.*)?$/m)
  let credential=match?.[1]??match?.[2]??match?.[3]??''
  if(credential.length<32||credential.length>8192||/\s/.test(credential))throw new Error('现有安全配置不可用')
  const output=await fs.mkdtemp(path.join(os.tmpdir(),'tm-stock-tradingcn-live-')),calls=[]
  const append=event=>fs.appendFile(path.join(output,'qa-only-input-events.jsonl'),JSON.stringify(event)+'\n',{mode:0o600,flush:true})
  let active=false,exit=-1
  const server=http.createServer(async(req,res)=>{
    const deny=()=>{if(!res.writableEnded&&!res.destroyed){res.writeHead(403,{'Content-Type':'application/json'});res.end('{"error":{"message":"qa_only_denied"}}')}}
    if(req.headers.host!==`127.0.0.1:${server.address().port}`||req.method!=='POST'||req.url!=='/v1/chat/completions'||Object.hasOwn(req.headers,'origin')||Object.hasOwn(req.headers,'authorization')||Object.hasOwn(req.headers,'content-encoding')||!/^application\/json(?:;\s*charset=utf-8)?$/i.test(req.headers['content-type']??'')||active||calls.length>=6||calls.some(row=>row.state!=='confirmed'||row.status!==200))return deny()
    try{
      const chunks=[];let size=0
      for await(const chunk of req){size+=chunk.length;if(size>65536)return deny();chunks.push(chunk)}
      const incoming=Buffer.concat(chunks),data=JSON.parse(incoming.toString('utf8'))
      if(!validFollowupQA(data)||data.model!=='deepseek-flash'||incoming.includes(Buffer.from(credential))||active)return deny()
      active=true
      const forwarded=translateEvaluationSchema({...data,thinking:{type:'disabled'}},'qa')
      const body=Buffer.from(JSON.stringify(forwarded)),row={seq:calls.length+1,state:'response_unknown',status:0,input_tokens:null,output_tokens:null}
      calls.push(row)
      await append({phase:'dispatch_planned',at_ms:Date.now(),seq:row.seq,incoming_json:incoming.toString('utf8'),request_json:body.toString('utf8'),request_digest:createHash('sha256').update(body).digest('hex')})
      const result=await evaluationHTTPS({route:evaluationRoute(),credential,body,timeoutMS:20000,onEvent:async(phase,reason,status,at_ms)=>append({phase,reason,status,at_ms,seq:row.seq})})
      const answer=result.answer
      Object.assign(row,{state:result.state,status:result.status,input_tokens:answer?.usage?.prompt_tokens??null,output_tokens:answer?.usage?.completion_tokens??null})
      await append({phase:'response_received',at_ms:Date.now(),call:row,...qaReplyMetadata(answer?.choices?.[0]?.message?.content??'')})
      if(result.state!=='confirmed'||result.status!==200)return deny()
      if(!res.destroyed){res.writeHead(200,{'Content-Type':'application/json','Cache-Control':'no-store'});res.end(JSON.stringify(answer))}
    }catch{deny()}finally{active=false}
  })
  await new Promise((resolve,reject)=>{server.once('error',reject);server.listen(0,'127.0.0.1',resolve)})
  try{
    const log=createWriteStream(path.join(output,'qa-only-go.log'),{mode:0o600})
    const env={SystemRoot:process.env.SystemRoot,WINDIR:process.env.WINDIR,TEMP:os.tmpdir(),TMP:os.tmpdir(),USERPROFILE:os.homedir(),APPDATA:process.env.APPDATA,LOCALAPPDATA:process.env.LOCALAPPDATA,PATH:process.env.PATH,PATHEXT:process.env.PATHEXT,ComSpec:process.env.ComSpec,GOMAXPROCS:'2',GOFLAGS:'-p=1',TM_TEST_AI_LIVE:'local-fixture-only',TM_TEST_AI_QA_ONLY:'local-fixture-only',TM_TEST_AI_REPORT_MODEL:'deepseek-flash',TM_TEST_AI_MODEL_URL:`http://127.0.0.1:${server.address().port}/v1`,TM_TEST_AI_FOLLOWUP_OUTPUT:output}
    const child=spawn(powershell,['-NoProfile','-File',path.join(root,'scripts/dev/verify-ai-source-native.ps1'),'-MySqlBaseDir',mysqlBase,'-BashPath','D:/git/bin/bash.exe','-ReportsOnly','-LiveAgents','-ReportPattern','^TestIntegrationAIReportFollowupQALive$','-RunCount','1'],{cwd:root,env,windowsHide:true,stdio:['ignore','pipe','pipe']})
    child.stdout.pipe(log,{end:false});child.stderr.pipe(log,{end:false})
    exit=await new Promise(resolve=>{child.once('error',()=>resolve(-1));child.once('close',code=>resolve(code??-1))})
    await new Promise(resolve=>log.end(resolve))
  }finally{credential='';await new Promise(resolve=>server.close(resolve))}
  const log=await fs.readFile(path.join(output,'qa-only-go.log'),'utf8')
  const receipt=qaOnlyReceipt(calls,exit,/^[ \t]+\S+\.go:\d+: TM_STOCK_QA_ONLY_LIVE_VERIFIED\r?\n/m.test(log))
  await fs.writeFile(path.join(output,'qa-only-receipt.json'),JSON.stringify({...receipt,native_exit:exit,calls},null,2),{mode:0o600,flag:'wx'})
  return {output,...receipt}
}

if(process.argv[1]&&path.resolve(process.argv[1])===fileURLToPath(import.meta.url)){
  try{
    if(process.argv.length!==4)throw new Error('需要MySQL与pwsh两个绝对路径')
    const result=await runFollowupQATrial({mysqlBase:process.argv[2],powershell:process.argv[3]})
    process.stdout.write(JSON.stringify(result)+'\n');process.exitCode=result.verified?0:1
  }catch{process.stderr.write('仅问答实调未完成；未输出凭据、正文或供应商原文。\n');process.exitCode=1}
}
