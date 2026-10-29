// 仅本地联调：真模型Key只存于本进程内存，不传给上游源码、环境或命令行。
import http from 'node:http'
import https from 'node:https'
import fs from 'node:fs/promises'
import { createWriteStream } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { randomBytes } from 'node:crypto'
import { spawn, execFileSync } from 'node:child_process'

// 纯函数守卫供无网络自测调用；不接受任意目的地址、模型或流式出口。
export function validateEvaluationRequest({ execute, method, url, headers, data, session, requests, model = 'deepseek-flash' }) {
  if (!execute || method !== 'POST' || !['/v1/chat/completions', '/chat/completions'].includes(url)
    || headers.authorization !== `Bearer ${session}` || Object.hasOwn(headers, 'origin')
    || Object.hasOwn(headers, 'content-encoding') || !/^application\/json(?:;\s*charset=utf-8)?$/i.test(headers['content-type'] ?? '')
    || !['deepseek-flash', 'qwen3.7-plus'].includes(model) || !data || data.model !== model || data.stream === true
    || !Array.isArray(data.messages) || data.messages.length < 1 || data.messages.length > 128
    || !Number.isSafeInteger(requests) || requests < 0 || requests >= 128) return null
  const max = data.max_tokens ?? 2048
  if (!Number.isSafeInteger(max) || max < 1) return null
  if (data.messages.some(message => !message || !['system', 'user', 'assistant', 'tool'].includes(message.role)
    || message.content !== null && typeof message.content !== 'string')) return null
  const body = { ...data, thinking: { type: 'disabled' }, max_tokens: Math.min(max, 2048) }
  delete body.reasoning_effort
  return body
}

// 路由仅由父进程显式选择，子进程仍使用隔离协议别名；回执保留实际供应商与模型。
export function evaluationRoute(provider = 'deepseek') {
  if (provider === 'deepseek') return { provider, hostname: 'api.deepseek.com', path: '/chat/completions', model: 'deepseek-flash' }
  if (provider === 'qwen-token-plan') return { provider, hostname: 'token-plan.maas.qianwenaiapi.com', path: '/compatible-mode/v1/chat/completions', model: 'qwen3.7-plus' }
  throw new Error('隔离评估供应商不在白名单中')
}

export function evaluationBody(data, route) {
  if (JSON.stringify(evaluationRoute(route.provider)) !== JSON.stringify(route)) throw new Error('隔离路由配置无效')
  const body = { ...data, model: route.model }
  if (route.provider === 'qwen-token-plan') { delete body.thinking; body.enable_thinking = false }
  return body
}

export async function startEvaluation({ credential, source, python, execute = false, deferred = false, keepAlive = false, provider = 'deepseek' }) {
  const route = evaluationRoute(provider)
  if (execute && (typeof credential !== 'string' || credential.length < 32 || credential.length > 8192 || /\s/.test(credential))) throw new Error('缺少有效的进程内模型凭据')
  const commit = execFileSync('git', ['-C', source, 'rev-parse', 'HEAD'], { encoding: 'utf8' }).trim()
  if (commit !== '51060a7682cf51810b4bc3a704829438d18cd8dc') throw new Error('固定社区版提交不匹配')
  execFileSync('git', ['-C', source, 'diff', '--quiet', 'HEAD', '--'])
  const tracked = new Set(execFileSync('git', ['-C', source, 'ls-files', '-z'], { encoding: 'utf8' }).split('\0'))
  const output = await fs.mkdtemp(path.join(os.tmpdir(), 'tm-stock-tradingcn-live-'))
  const copy = path.join(output, 'source')
  const excluded = new Set(['.git', '.venv', '__pycache__', 'node_modules', 'data', 'data_cache', 'cache', 'logs', 'results', 'dist'])
  await fs.cp(source, copy, { recursive: true, filter: async filename => {
    const relative = path.relative(source, filename)
    const names = relative.split(path.sep)
    if (names.some(name => excluded.has(name) || name.startsWith('.env'))) return false
    const stat = await fs.lstat(filename)
    return !stat.isSymbolicLink() && (stat.isDirectory() || tracked.has(names.join('/')))
  } })
  const session = randomBytes(32).toString('hex')
  const state = { output, execute, route, requests: 0, usage: [], child: 'waiting', summary: null, rejected: 0, unknown: false }
  const reportSession = randomBytes(32).toString('hex')
  let key = credential
  let reportRequests = 0
  const server = http.createServer(async (req, res) => {
    const reject = (status = 403) => { if (res.writableEnded || res.destroyed) return; state.rejected++; res.writeHead(status, { 'Content-Type': 'application/json' }); res.end('{"error":{"message":"local_evaluation_denied"}}') }
    const report = keepAlive && state.child === 'passed' && req.headers.authorization === `Bearer ${reportSession}`
    if (req.headers.host !== `127.0.0.1:${state.port}` || !execute || req.method !== 'POST' || !['/v1/chat/completions', '/chat/completions'].includes(req.url)
      || (!report && (state.child !== 'running' || req.headers.authorization !== `Bearer ${session}`)) || Object.hasOwn(req.headers, 'origin')) return reject()
    let size = 0; const chunks = []
    try {
      for await (const chunk of req) { size += chunk.length; if (size > 512 * 1024) return reject(413); chunks.push(chunk) }
      const data = validateEvaluationRequest({ execute, method: req.method, url: req.url, headers: req.headers,
        data: JSON.parse(Buffer.concat(chunks).toString('utf8')), session: report ? reportSession : session, requests: state.requests, model: report ? route.model : 'deepseek-flash' })
      if (!data || state.unknown) return reject()
      if (report && reportRequests >= 3) return reject(429)
      // DeepSeek Chat API提供JSON对象模式，不提供json_schema模式。仅本地桥显式翻译协议；
      // 原schema作为系统约束原样传入，Go端仍逐项严格验收，绝不改写或补齐模型响应。
      if (report && data.response_format?.type === 'json_schema') {
        const schema = data.response_format.json_schema?.schema
        if (!schema || typeof schema !== 'object') return reject()
        data.messages = [...data.messages, { role: 'system', content: `只返回符合此JSON schema的JSON对象：${JSON.stringify(schema)}` }]
        data.response_format = { type: 'json_object' }
      }
      const body = Buffer.from(JSON.stringify(evaluationBody(data, route)))
      if (report) reportRequests++
      state.requests++
      const upstream = https.request({ hostname: route.hostname, port: 443, path: route.path, method: 'POST', agent: false, rejectUnauthorized: true, minVersion: 'TLSv1.2', headers: { Authorization: `Bearer ${key}`, 'Content-Type': 'application/json', 'Content-Length': body.length } }, incoming => {
        const parts = []; let bytes = 0
        incoming.on('data', chunk => { bytes += chunk.length; if (bytes > 4 * 1024 * 1024) upstream.destroy(); else parts.push(chunk) })
        incoming.on('end', () => {
          clearTimeout(timer)
          let answer
          try { answer = JSON.parse(Buffer.concat(parts).toString('utf8')) } catch { state.unknown = true; return reject(502) }
          state.usage.push({ phase: report ? 'report' : 'core', status: incoming.statusCode, input: answer.usage?.prompt_tokens ?? null, output: answer.usage?.completion_tokens ?? null, finish: answer.choices?.[0]?.finish_reason ?? null })
          // 错误不转发供应商回显；成功响应仅供隔离Agent读取，不打印内容。
          if (incoming.statusCode !== 200) return reject(incoming.statusCode)
          res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); res.end(JSON.stringify(answer))
        })
        incoming.on('error', () => { state.unknown = true; reject(502) })
      })
      const timer = setTimeout(() => { state.unknown = true; upstream.destroy() }, 90000)
      upstream.on('error', () => { clearTimeout(timer); state.unknown = true; reject(502) })
      upstream.end(body)
    } catch { reject(400) }
  })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  state.port = server.address().port
  state.reportEndpoint = { url: `http://127.0.0.1:${state.port}/v1`, token: reportSession }
  state.close = async () => { key = ''; await new Promise(resolve => server.close(resolve)); await recordReceipt(state) }
  state.launch = () => {
    if (state.child !== 'waiting') throw new Error('评估只允许启动一次，不自动重试')
    // 使用独立的Windows最小环境；不继承宿主任何API Key、代理或产品配置。
    const env = { SystemRoot: 'C:\\Windows', WINDIR: 'C:\\Windows',
      PATH: 'C:\\Windows\\System32', TEMP: os.tmpdir(), TMP: os.tmpdir(),
      USERPROFILE: os.homedir(), APPDATA: path.join(os.homedir(), 'AppData', 'Roaming'),
      LOCALAPPDATA: path.join(os.homedir(), 'AppData', 'Local') }
    env.PYTHONIOENCODING = 'utf-8'; env.PYTHONDONTWRITEBYTECODE = '1'
    const log = createWriteStream(path.join(output, 'isolated-upstream.log'), { mode: 0o600 })
    const script = fileURLToPath(new URL('./verify-tradingcn-live.py', import.meta.url))
    const child = spawn(python, ['-X', 'utf8', script], { cwd: output, env, windowsHide: true, stdio: ['pipe', 'pipe', 'pipe'] })
    state.child = 'running'
    child.stdout.pipe(log, { end: false }); child.stderr.pipe(log, { end: false })
    let text = ''
    child.stdout.on('data', chunk => {
      text = (text + chunk.toString('utf8')).slice(-256 * 1024)
      for (const line of text.split(/\r?\n/)) if (line.startsWith('TM_TRADINGCN_EVAL=')) { try { state.summary = JSON.parse(line.slice(18)) } catch { /* 完整行到达后再解析 */ } }
    })
    child.stdin.end(JSON.stringify({ source: copy, output, port: state.port, session, execute }))
    const watchdog = setTimeout(() => { state.child = 'timeout_unknown'; state.unknown = true; child.kill() }, 15 * 60000)
    state.cancel = () => { state.child = 'cancelled_unknown'; state.unknown = true; child.kill() }
    state.finished = new Promise(resolve => {
      let closed = false
      const finish = async code => {
        if (closed) return
        closed = true
        clearTimeout(watchdog); state.exit = code; if (!['timeout_unknown', 'cancelled_unknown'].includes(state.child)) state.child = code === 0 && !state.unknown ? 'passed' : 'failed'
        if (!keepAlive) { key = ''; server.close() }
        log.end()
        try { await recordReceipt(state) } catch { state.receipt_error = true }
        resolve()
      }
      child.once('error', () => finish(-1)); child.once('exit', code => finish(code))
    })
    return state.finished
  }
  if (!deferred) state.launch()
  return state
}

// 仅保存闭集元数据，不保存Key、请求体或模型正文；可补记已结束的旧会话。
export async function recordReceipt(state) {
  if (['waiting', 'running'].includes(state.child) || path.dirname(state.output) !== os.tmpdir() || !/^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/.test(path.basename(state.output))) throw new Error('不是已结束的本次隔离评估目录')
  const complete = state.requests === state.usage.length && state.usage.every(item => Number.isSafeInteger(item.input) && item.input >= 0 && Number.isSafeInteger(item.output) && item.output >= 0)
  const coreCalls = state.usage.filter(item => item.phase !== 'report').length
  const receipt = { scope: 'real_provider_transport_receipt', child: state.child, execute: state.execute,
    provider: state.route?.provider ?? 'deepseek', model: state.route?.model ?? 'deepseek-flash', protocol_model_alias: 'deepseek-flash',
    workflow_verified: state.execute === true && state.child === 'passed' && state.summary?.verified === true && state.summary?.executed === true
      && coreCalls > 0 && coreCalls === state.summary.model_calls && complete && state.unknown !== true && state.usage.every(item => item.status === 200),
    report_model_calls: state.usage.filter(item => item.phase === 'report').length,
    requests: state.requests, confirmed_responses: state.usage.length,
    http_successes: state.usage.filter(item => item.status === 200).length,
    unknown: state.unknown === true, usage_complete: complete,
    input_tokens: complete ? state.usage.reduce((n, item) => n + item.input, 0) : null,
    output_tokens: complete ? state.usage.reduce((n, item) => n + item.output, 0) : null }
  await fs.writeFile(path.join(state.output, 'provider-receipt.json'), JSON.stringify(receipt, null, 2), { mode: 0o600 })
  return receipt
}
