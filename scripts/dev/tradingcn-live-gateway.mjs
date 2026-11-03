// 仅本地联调：真模型Key只存于本进程内存，不传给上游源码、环境或命令行。
import http from 'node:http'
import https from 'node:https'
import fs from 'node:fs/promises'
import { createWriteStream } from 'node:fs'
import os from 'node:os'
import path from 'node:path'
import { fileURLToPath } from 'node:url'
import { randomBytes, createHash } from 'node:crypto'
import { spawn, execFileSync } from 'node:child_process'

const inputNodes = new Set(['bear_researcher_v2','bull_researcher_v2','fundamentals_analyst_v2','index_analyst_v2','market_analyst_v2','neutral_analyst_v2','news_analyst_v2','research_manager_v2','risk_manager_v2','risky_analyst_v2','safe_analyst_v2','sector_analyst_v2','social_analyst_v2','trader_v2'])
const digest = value => createHash('sha256').update(value).digest('hex')

// 请求体为父网关真正发出的字节，不使用_generate入参猜测。完整正文只进入隔离结果。
export function evaluationInputCall({ seq, node, callId, body, model, secrets = [] }) {
  if (!Number.isSafeInteger(seq) || seq < 1 || seq > 128 || !inputNodes.has(node) || !/^[a-f0-9]{32}$/.test(callId ?? '')
    || !Buffer.isBuffer(body) || body.length > 512 * 1024 || secrets.some(secret => secret && body.includes(Buffer.from(secret)))) throw new Error('输入审计身份/正文边界无效')
  const request = JSON.parse(body.toString('utf8'))
  if (request.model !== model || !Array.isArray(request.messages) || !request.messages.length) throw new Error('输入审计模型或消息无效')
  return { seq, call_id: callId, node, model, request_json: body.toString('utf8'), request_digest: digest(body),
    prompt_digest: digest(JSON.stringify(request.messages)), state: 'response_unknown', status: 0, response_model: '', input_tokens: null, output_tokens: null }
}

export function evaluationInputBundle(state, fixtureJSON) {
  const calls = state.inputCalls
  const observed = state.summary?.node_model_calls
  if (state.execute !== true || state.child !== 'passed' || state.summary?.verified !== true || state.summary?.executed !== true || state.unknown || !Array.isArray(calls) || !Array.isArray(observed) || calls.length !== observed.length || state.summary.model_calls !== calls.length
    || calls.length < inputNodes.size || calls.some(c => c.state !== 'confirmed' || c.status !== 200 || c.response_model !== c.model
      || !Number.isSafeInteger(c.input_tokens) || c.input_tokens < 0 || !Number.isSafeInteger(c.output_tokens) || c.output_tokens < 0)
    || new Set(calls.map(c => c.call_id)).size !== calls.length || new Set(observed.map(c => c.call_id)).size !== observed.length || new Set(calls.map(c => c.node)).size !== inputNodes.size
    || observed.some(c => !c.success || !calls.some(w => w.call_id === c.call_id && w.node === c.node))) throw new Error('原Agent和实际网关调用无法完整对应')
  const bundle = { schema: 'tm-stock-agent-inputs-v1', scope: 'synthetic-fixture-only', upstream_commit: '51060a7682cf51810b4bc3a704829438d18cd8dc',
    provider: state.route.provider, fixture_json: fixtureJSON, fixture_digest: digest(fixtureJSON), calls }
  if (Buffer.byteLength(JSON.stringify(bundle)) > 2 * 1024 * 1024) throw new Error('输入审计超出隔离上限')
  return bundle
}

// 串行追加，发送前必须确认输入已落盘；失败/未知也保留原身份，不覆盖旧记录。
export function createInputJournal(output, io = fs) {
  if (path.dirname(output) !== os.tmpdir() || !/^tm-stock-tradingcn-live-[A-Za-z0-9]{6}$/.test(path.basename(output))) throw new Error('输入日志只允许本次隔离目录')
  const file = path.join(output, 'input-events.jsonl')
  let pending = Promise.resolve()
  return { append(event) {
    const line = JSON.stringify(event) + '\n'
    pending = pending.then(() => io.appendFile(file, line, { mode: 0o600, flush: true }))
    return pending
  }, flush: () => pending }
}

// 纯函数守卫供无网络自测调用；不接受任意目的地址、模型或流式出口。
export function validateEvaluationRequest({ execute, method, url, headers, data, session, requests, model = 'deepseek-flash' }) {
  if (!execute || method !== 'POST' || !['/v1/chat/completions', '/chat/completions'].includes(url)
    || headers.authorization !== `Bearer ${session}` || Object.hasOwn(headers, 'origin')
    || Object.hasOwn(headers, 'content-encoding') || !/^application\/json(?:;\s*charset=utf-8)?$/i.test(headers['content-type'] ?? '')
    || !['deepseek-flash', 'qwen3.7-plus'].includes(model) || !data || data.model !== model || data.stream === true
    || !Array.isArray(data.messages) || data.messages.length < 1 || data.messages.length > 128
    || !Number.isSafeInteger(requests) || requests < 0 || requests >= 128) return null
  const supplied = [data.max_tokens, data.max_completion_tokens].filter(value => value != null)
  if (supplied.some(value => !Number.isSafeInteger(value) || value < 1) || supplied.length === 2 && supplied[0] !== supplied[1]) return null
  const max = supplied[0] ?? 2048
  if (data.messages.some(message => !message || !['system', 'user', 'assistant', 'tool'].includes(message.role)
    || message.content !== null && typeof message.content !== 'string')) return null
  const body = { ...data, thinking: { type: 'disabled' }, max_tokens: Math.min(max, 2048) }
  // 现代SDK把max_tokens改名；受控出口只发送供应商支持的单一字段。
  delete body.max_completion_tokens
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

// 调用数由显式编排确定，不能用任意数字扩大测试桥出口。
export function evaluationReportCalls(reportProfile = '') {
  if (reportProfile === '') return 3
  if (reportProfile === 'dossier-comparison-report-v1') return 5
  throw new Error('真实联调报告编排不受支持')
}

// 单次HTTPS出口：只返回封闭失败原因，不重试、不把供应商错误正文带入日志。
// request/timeoutMS供无网络故障注入；真实入口沿固定路由与原90秒上限。
export async function evaluationHTTPS({ route, credential, body, onEvent = async () => {}, request = https.request, timeoutMS = 90000 }) {
  if (JSON.stringify(evaluationRoute(route?.provider)) !== JSON.stringify(route) || !Buffer.isBuffer(body) || body.length > 512 * 1024
    || typeof credential !== 'string' || credential.length < 32 || credential.length > 8192 || /\s/.test(credential)
    || !Number.isSafeInteger(timeoutMS) || timeoutMS < 1 || timeoutMS > 90000) throw new Error('单次模型出口参数无效')
  return new Promise(resolve => {
    let done = false, headers = false, writeAttempted = false, timer, upstream
    let observations = Promise.resolve()
    const finish = async (state, reason, status = 0, answer = null) => {
      if (done) return
      const observedAtMS = Date.now()
      done = true; clearTimeout(timer)
      try { await observations; await onEvent(state === 'confirmed' ? 'response_complete' : 'response_unknown', reason, status, observedAtMS) }
      catch { state = 'response_unknown'; reason = 'journal_error'; answer = null }
      resolve({ state, reason, status, answer })
    }
    const observe = (phase, reason, status = 0) => {
      if (done) return
      const observedAtMS = Date.now()
      observations = observations.then(() => onEvent(phase, reason, status, observedAtMS))
      observations.catch(() => { finish('response_unknown', 'journal_error'); upstream?.destroy() })
    }
    try {
      upstream = request({ hostname: route.hostname, port: 443, path: route.path, method: 'POST', agent: false,
        rejectUnauthorized: true, minVersion: 'TLSv1.2', headers: { Authorization: `Bearer ${credential}`, 'Content-Type': 'application/json', 'Content-Length': body.length } }, incoming => {
        headers = true; observe('response_headers', 'http_' + incoming.statusCode, incoming.statusCode)
        let size = 0; const parts = []
        incoming.on('data', chunk => { if (done) return; size += chunk.length; if (size > 4 * 1024 * 1024) { finish('response_unknown', 'response_too_large', incoming.statusCode); upstream.destroy() } else parts.push(chunk) })
        incoming.on('end', () => {
          if (done) return
          let answer
          try { answer = JSON.parse(Buffer.concat(parts).toString('utf8')) } catch { finish('response_unknown', 'invalid_json', incoming.statusCode); return }
          if (!answer || typeof answer !== 'object' || Array.isArray(answer)) { finish('response_unknown', 'invalid_json', incoming.statusCode); return }
          finish('confirmed', incoming.statusCode === 200 ? 'http_ok' : 'http_rejected', incoming.statusCode, answer)
        })
        incoming.on('error', () => finish('response_unknown', 'response_error', incoming.statusCode))
        incoming.on('aborted', () => finish('response_unknown', 'response_aborted', incoming.statusCode))
        incoming.on('close', () => { if (!done) finish('response_unknown', 'response_closed', incoming.statusCode) })
      })
      timer = setTimeout(() => { finish('response_unknown', headers ? 'response_deadline' : 'headers_deadline'); upstream.destroy() }, timeoutMS)
      upstream.on('error', () => finish('response_unknown', 'transport_error'))
      writeAttempted = true
      upstream.end(body, () => observe('request_dispatched', 'local_write_complete'))
    } catch { finish(writeAttempted ? 'response_unknown' : 'not_sent', writeAttempted ? 'local_write_error' : 'local_transport_error'); upstream?.destroy() }
  })
}

export async function startEvaluation({ credential, source, python, execute = false, deferred = false, keepAlive = false, provider = 'deepseek', reportProfile = '' }) {
  const reportCallLimit = evaluationReportCalls(reportProfile)
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
  const state = { output, execute, route, reportProfile, requests: 0, usage: [], inputCalls: [], child: 'waiting', summary: null, rejected: 0, unknown: false }
  const inputJournal = createInputJournal(output)
  const persistInput = async (phase, call, reason = '', observed_at_ms = Date.now()) => {
    if (!call) return
    try {
      const event = phase === 'dispatch_planned' ? { phase, observed_at_ms, call } : { phase, observed_at_ms, reason, seq: call.seq, call_id: call.call_id, state: call.state, status: call.status, response_model: call.response_model, input_tokens: call.input_tokens, output_tokens: call.output_tokens }
      await inputJournal.append(event)
    } catch { state.unknown = true; state.inputJournalError = true; throw new Error('输入日志未确认落盘') }
  }
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
      if (report && reportRequests >= reportCallLimit) return reject(429)
      // DeepSeek Chat API提供JSON对象模式，不提供json_schema模式。仅本地桥显式翻译协议；
      // 原schema作为系统约束原样传入，Go端仍逐项严格验收，绝不改写或补齐模型响应。
      if (report && data.response_format?.type === 'json_schema') {
        const schema = data.response_format.json_schema?.schema
        if (!schema || typeof schema !== 'object') return reject()
        data.messages = [...data.messages, { role: 'system', content: `只返回符合此JSON schema的JSON对象：${JSON.stringify(schema)}` }]
        data.response_format = { type: 'json_object' }
      }
      const body = Buffer.from(JSON.stringify(evaluationBody(data, route)))
      let inputCall = null
      if (!report) {
        inputCall = evaluationInputCall({ seq: state.inputCalls.length + 1, node: req.headers['x-tm-evaluation-node'], callId: req.headers['x-tm-evaluation-call'], body, model: route.model, secrets: [key, session, reportSession] })
        if (state.inputCalls.some(c => c.call_id === inputCall.call_id)) return reject()
        if (Buffer.byteLength(JSON.stringify([...state.inputCalls, inputCall])) > 2 * 1024 * 1024 - 16384) return reject(413)
        state.inputCalls.push(inputCall)
        await persistInput('dispatch_planned', inputCall)
      }
      if (report) reportRequests++
      state.requests++
      const result = await evaluationHTTPS({ route, credential: key, body, onEvent: async (phase, reason, status, observedAtMS) => {
        if (inputCall && status) inputCall.status = status
        await persistInput(phase, inputCall, reason, observedAtMS)
      } })
      if (result.state !== 'confirmed') { state.unknown = true; return reject(502) }
      const answer = result.answer
      state.usage.push({ phase: report ? 'report' : 'core', status: result.status, input: answer.usage?.prompt_tokens ?? null, output: answer.usage?.completion_tokens ?? null, finish: answer.choices?.[0]?.finish_reason ?? null })
      if (inputCall) {
        Object.assign(inputCall, { state: 'confirmed', status: result.status, response_model: answer.model ?? '', input_tokens: answer.usage?.prompt_tokens ?? null, output_tokens: answer.usage?.completion_tokens ?? null })
        await persistInput('response_received', inputCall, result.reason)
      }
      if (result.status !== 200) return reject(result.status)
      if (!res.destroyed && !res.writableEnded) { res.writeHead(200, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' }); res.end(JSON.stringify(answer)) }
    } catch { reject(400) }
  })
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  state.port = server.address().port
  state.reportEndpoint = { url: `http://127.0.0.1:${state.port}/v1`, token: reportSession }
  state.inputBundle = async () => {
    const bundle = evaluationInputBundle(state, await fs.readFile(path.join(output, 'input-fixture.json'), 'utf8'))
    const text = JSON.stringify(bundle)
    const file = path.join(output, 'input-audit.json')
    try { await fs.writeFile(file, text, { mode: 0o600, flag: 'wx', flush: true }) } catch (error) {
      if (error.code !== 'EEXIST' || await fs.readFile(file, 'utf8') !== text) throw new Error('原输入档案已存在且不一致，禁止覆盖')
    }
    return bundle
  }
  state.close = async () => {
    key = ''; await new Promise(resolve => server.close(resolve))
    try { await inputJournal.flush() } catch { state.unknown = true; state.inputJournalError = true }
    const partial = { schema: 'tm-stock-agent-inputs-partial-v1', scope: 'synthetic-fixture-only', complete: false, child: state.child, unknown: state.unknown, calls: state.inputCalls }
    let archiveError = false
    try { await fs.writeFile(path.join(output, 'input-audit-partial.json'), JSON.stringify(partial), { mode: 0o600, flag: 'wx', flush: true }) } catch { state.unknown = true; state.inputJournalError = true; archiveError = true }
    await recordReceipt(state)
    if (archiveError) throw new Error('原输入部分档案未确认保存，禁止成功回执')
  }
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
    report_profile: state.reportProfile ?? '',
    requests: state.requests, confirmed_responses: state.usage.length,
    http_successes: state.usage.filter(item => item.status === 200).length,
    unknown: state.unknown === true, usage_complete: complete,
    input_tokens: complete ? state.usage.reduce((n, item) => n + item.input, 0) : null,
    output_tokens: complete ? state.usage.reduce((n, item) => n + item.output, 0) : null }
  await fs.writeFile(path.join(state.output, 'provider-receipt.json'), JSON.stringify(receipt, null, 2), { mode: 0o600 })
  return receipt
}
