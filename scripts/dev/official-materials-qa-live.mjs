// 正式机关节选的单次开发问答实跑。真Key仅在父进程内存，原请求字节直接转发，绝不补齐模型结果。
import http from 'node:http'
import https from 'node:https'
import fs from 'node:fs/promises'
import { createWriteStream } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { createHash } from 'node:crypto'
import { spawn } from 'node:child_process'
import { fileURLToPath } from 'node:url'

export function validOfficialQARequest(data) {
  const allowed = new Set(['model', 'messages', 'max_tokens', 'temperature', 'seed', 'response_format', 'reasoning_effort'])
  return !!data && Object.keys(data).every(key => allowed.has(key)) && data.model === 'deepseek-flash'
    && data.max_tokens === 1024 && data.reasoning_effort === 'none'
    && data.response_format?.type === 'json_object' && Object.keys(data.response_format).length === 1
    && Array.isArray(data.messages) && data.messages.length === 2
    && data.messages[0]?.role === 'system' && typeof data.messages[0].content === 'string'
    && data.messages[0].content.startsWith('你是受限资料问答的内部校验器。')
    && data.messages[1]?.role === 'user' && typeof data.messages[1].content === 'string'
    && data.messages[1].content.includes('只返回一个符合以下 JSON Schema 的 JSON 对象')
    && data.messages.every(m => Object.keys(m).sort().join(',') === 'content,role')
}

export function officialQAReceipt(calls, rejected = 0) {
  const complete = calls.length === 10 && calls.every(c => c.status === 200 && c.state === 'confirmed'
    && Number.isSafeInteger(c.input_tokens) && c.input_tokens >= 0 && Number.isSafeInteger(c.output_tokens) && c.output_tokens >= 0)
  return { classification: 'real-provider-transport-not-human-gold', provider: 'deepseek', model: 'deepseek-flash',
    requests: calls.length, rejected, usage_complete: complete, unknown: calls.some(c => c.state !== 'confirmed'),
    input_tokens: complete ? calls.reduce((n, c) => n + c.input_tokens, 0) : null,
    output_tokens: complete ? calls.reduce((n, c) => n + c.output_tokens, 0) : null,
    cost_micros: null, calls }
}

export async function startOfficialQARelay(credential, request = https.request) {
  if (typeof credential !== 'string' || credential.length < 32 || credential.length > 8192 || /\s/.test(credential)) throw new Error('缺少进程内凭据')
  let key = credential
  const calls = []; let rejected = 0; let active = false
  const server = http.createServer(async (req, res) => {
    const deny = (status = 403) => { rejected++; if (!res.destroyed && !res.writableEnded) { res.writeHead(status, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); res.end('{"error":{"message":"official_qa_development_denied"}}') } }
    if (req.headers.host !== `127.0.0.1:${server.address().port}` || req.method !== 'POST' || req.url !== '/v1/chat/completions'
      || Object.hasOwn(req.headers, 'origin') || Object.hasOwn(req.headers, 'authorization') || Object.hasOwn(req.headers, 'content-encoding')
      || !/^application\/json(?:;\s*charset=utf-8)?$/i.test(req.headers['content-type'] ?? '') || calls.length >= 10 || active || calls.some(c => c.state !== 'confirmed' || c.status !== 200)) return deny()
    try {
      const chunks = []; let bytes = 0
      for await (const chunk of req) { bytes += chunk.length; if (bytes > 65536) return deny(413); chunks.push(chunk) }
      const body = Buffer.concat(chunks)
      if (!validOfficialQARequest(JSON.parse(body.toString('utf8'))) || active || calls.length >= 10) return deny()
      active = true
      const started = Date.now()
      const row = { request_body_sha256: createHash('sha256').update(body).digest('hex'), state: 'response_unknown', status: null, input_tokens: null, output_tokens: null, elapsed_ms: null }
      calls.push(row)
      let ended = false
      const finish = () => { if (ended) return false; ended = true; active = false; row.elapsed_ms = Date.now() - started; clearTimeout(timer); return true }
      const upstream = request({ hostname: 'api.deepseek.com', port: 443, path: '/chat/completions', method: 'POST', agent: false,
        rejectUnauthorized: true, minVersion: 'TLSv1.2', headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json', 'Content-Length': body.length } }, incoming => {
        const parts = []; let size = 0
        incoming.on('data', chunk => { size += chunk.length; if (size > 1048576) upstream.destroy(); else parts.push(chunk) })
        incoming.on('end', () => {
          if (!finish()) return
          row.status = incoming.statusCode
          let answer
          try { answer = JSON.parse(Buffer.concat(parts).toString('utf8')) } catch { return deny(502) }
          row.state = 'confirmed'
          row.input_tokens = Number.isSafeInteger(answer.usage?.prompt_tokens) && answer.usage.prompt_tokens >= 0 ? answer.usage.prompt_tokens : null
          row.output_tokens = Number.isSafeInteger(answer.usage?.completion_tokens) && answer.usage.completion_tokens >= 0 ? answer.usage.completion_tokens : null
          if (incoming.statusCode !== 200) return deny(502)
          if (!res.destroyed) { res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); res.end(Buffer.concat(parts)) }
        })
        incoming.on('error', () => { if (finish()) deny(502) })
      })
      const timer = setTimeout(() => { upstream.destroy(); if (finish()) deny(504) }, 22000)
      upstream.on('error', () => { if (finish()) deny(502) })
      res.once('close', () => { if (!res.writableEnded && finish()) upstream.destroy() })
      upstream.end(body)
    } catch { deny(400) }
  })
  server.requestTimeout = 25000; server.headersTimeout = 10000
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(0, '127.0.0.1', resolve) })
  return { endpoint: `http://127.0.0.1:${server.address().port}/v1`, receipt: () => officialQAReceipt(calls, rejected),
    close: async () => { key = ''; server.closeAllConnections(); await new Promise(resolve => server.close(resolve)) } }
}

export async function runOfficialQATrial({ mysqlBase, powershell, root = process.cwd() }) {
  if (![mysqlBase, powershell, root].every(p => typeof p === 'string' && path.isAbsolute(p))) throw new Error('本地运行路径须为绝对路径')
  let credential = ''
  try {
    const raw = await fs.readFile(path.join(os.homedir(), '.tmstock-credentials'), 'utf8')
    const match = raw.match(/^\s*(?:export\s+)?DEEPSEEK_API_KEY\s*=\s*(?:"([^"\r\n]*)"|'([^'\r\n]*)'|([^\s#]+))\s*(?:#.*)?$/m)
    credential = match?.[1] ?? match?.[2] ?? match?.[3] ?? ''
    if (credential.length < 32 || credential.length > 8192 || /\s/.test(credential)) throw new Error()
  } catch { throw new Error('既有安全文件中未找到有效DeepSeek配置；内容未输出') }
  const output = await fs.mkdtemp(path.join(os.tmpdir(), 'tm-stock-official-qa-live-'))
  const relay = await startOfficialQARelay(credential); credential = ''
  let code = -1
  try {
    const log = createWriteStream(path.join(output, 'native-go.log'), { mode: 0o600 })
    const env = { SystemRoot: process.env.SystemRoot, WINDIR: process.env.WINDIR, TEMP: os.tmpdir(), TMP: os.tmpdir(), USERPROFILE: os.homedir(),
      APPDATA: process.env.APPDATA, LOCALAPPDATA: process.env.LOCALAPPDATA, PATH: process.env.PATH, PATHEXT: process.env.PATHEXT, ComSpec: process.env.ComSpec, GOMAXPROCS: '2', GOFLAGS: '-p=1',
      TM_TEST_AI_LIVE: 'local-fixture-only', TM_TEST_AI_OFFICIAL_QA_ENDPOINT: relay.endpoint, TM_TEST_AI_OFFICIAL_QA_OUTPUT: output }
    const child = spawn(powershell, ['-NoProfile', '-File', path.join(root, 'scripts/dev/verify-ai-source-native.ps1'), '-MySqlBaseDir', mysqlBase,
      '-BashPath', 'D:\\git\\bin\\bash.exe', '-ReportsOnly', '-LiveAgents', '-ReportPattern', '^TestIntegrationAIReportOfficialMaterialQALive$', '-RunCount', '1'],
    { cwd: root, env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] })
    child.stdout.pipe(log, { end: false }); child.stderr.pipe(log, { end: false })
    code = await new Promise(resolve => { child.once('error', () => resolve(-1)); child.once('close', value => resolve(value ?? -1)) })
    await new Promise(resolve => log.end(resolve))
  } finally { await relay.close() }
  const receipt = relay.receipt()
  const text = await fs.readFile(path.join(output, 'native-go.log'), 'utf8')
  const verified = code === 0 && receipt.usage_complete && !receipt.unknown && /TM_STOCK_OFFICIAL_QA_LIVE_VERIFIED/.test(text)
  await fs.writeFile(path.join(output, 'provider-receipt.json'), JSON.stringify({ ...receipt, native_exit: code, api_trial_verified: verified }, null, 2), { mode: 0o600 })
  return { output, native_exit: code, api_trial_verified: verified, requests: receipt.requests, input_tokens: receipt.input_tokens, output_tokens: receipt.output_tokens, cost_micros: null }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  try {
    if (process.argv.length !== 4) throw new Error('需要MySQL目录和pwsh程序两个绝对路径')
    const result = await runOfficialQATrial({ mysqlBase: process.argv[2], powershell: process.argv[3] })
    process.stdout.write(JSON.stringify(result) + '\n'); process.exitCode = result.api_trial_verified ? 0 : 1
  } catch { process.stderr.write('正式资料真实问答实跑未完成；未输出凭据或供应商原文。\n'); process.exitCode = 1 }
}
