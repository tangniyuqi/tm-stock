// 仅本地一次性评估异步桥：真实V2引擎，显式测试资料；不是社区版Web/Redis部署。
import http from 'node:http'
import fs from 'node:fs/promises'
import path from 'node:path'
import { randomBytes, randomUUID } from 'node:crypto'
import { spawn } from 'node:child_process'
import { createWriteStream } from 'node:fs'
import os from 'node:os'
import { fileURLToPath } from 'node:url'
import { isDeepStrictEqual } from 'node:util'
import { startEvaluation, recordReceipt } from './tradingcn-live-gateway.mjs'

export function validSubmission(body) {
  return !!body && Object.keys(body).sort().join(',') === 'parameters,symbol' && body.symbol === '600001'
    && isDeepStrictEqual(body.parameters, { engine: 'v2', market_type: 'A股', research_depth: '快速',
      selected_analysts: ['news', 'fundamentals'], include_sentiment: false, include_risk: false,
      language: 'zh-CN', quick_analysis_model: 'deepseek-flash', deep_analysis_model: 'deepseek-flash', enable_trace: false })
}

export async function startLocalChain(options) {
  const evaluation = await startEvaluation({ ...options, execute: true, deferred: true, keepAlive: true })
  try { return await createAsyncBridge(evaluation) } catch (error) {
    evaluation.child = 'failed'; await evaluation.close(); throw error
  }
}

// 子进程只收到临时桥凭据，绝不继承真正模型Key或产品配置。
export function chainConversationOptions({ conversation = false, loginRunner, analysisType = 'light', processes = false } = {}) {
  if (typeof processes !== 'boolean') throw new Error('进程模式必须显式布尔值')
  if (processes) {
    if (conversation !== true || loginRunner !== undefined || !['light','deep'].includes(analysisType)) throw new Error('进程验收必须明确会话及轻深，不复用预编译登录程序')
    return {TM_TEST_AI_LIVE_PROCESS:'local-fixture-only',TM_TEST_AI_H5_LOCAL_DEPTH:analysisType}
  }
  if (!['light', 'deep'].includes(analysisType) || (analysisType === 'deep' && !conversation)) throw new Error('深度验收必须使用显式会话模式')
  if (typeof conversation !== 'boolean' || (!conversation && loginRunner !== undefined)
    || (conversation && (typeof loginRunner !== 'string' || !path.isAbsolute(loginRunner) || !/\.exe$/i.test(loginRunner)))) {
    throw new Error('会话验收必须显式启用并提供原会员登录测试程序绝对路径')
  }
  return conversation ? { TM_TEST_AI_LIVE_CONVERSATION: 'local-fixture-only', TM_TEST_AI_H5_INTENTS: 'local-fixture-only',
    TM_TEST_AI_H5_LOGIN: 'local-fixture-only', TM_TEST_AI_H5_LOGIN_RUNNER: loginRunner,
    ...(analysisType === 'deep' ? { TM_TEST_AI_LIVE_DEPTH: 'deep' } : {}) } : {}
}

export function hasConversationProof(output) {
  return /^[ \t]+\S+\.go:\d+: TM_STOCK_LIVE_CONVERSATION_VERIFIED\r?\n/m.test(output)
}

export function hasDeepProof(output) {
  return /^[ \t]+\S+\.go:\d+: TM_STOCK_LIVE_DEEP_VERIFIED\r?\n/m.test(output)
}

export function hasProcessHTTPProof(output) {
  return /^[ \t]+\S+\.go:\d+: TM_STOCK_LIVE_PROCESS_HTTP_VERIFIED\r?\n/m.test(output)
}

export function startChainVerification(bridge, { powershell, mysql, bash, goBin, gccBin, origin, conversation = false, loginRunner, analysisType = 'light', processes = false }) {
  const conversationEnv = chainConversationOptions({ conversation, loginRunner, analysisType, processes })
  if (bridge.evaluation.child !== 'waiting') throw new Error('只能启动尚未执行的单次评估')
  const root = fileURLToPath(new URL('../../', import.meta.url))
  const env = { SystemRoot: 'C:\\Windows', WINDIR: 'C:\\Windows', PATH: `${goBin};${gccBin};C:\\Windows\\System32`, PATHEXT: '.COM;.EXE;.BAT;.CMD',
    USERPROFILE: os.homedir(), APPDATA: path.join(os.homedir(), 'AppData', 'Roaming'), LOCALAPPDATA: path.join(os.homedir(), 'AppData', 'Local'),
    TEMP: os.tmpdir(), TMP: os.tmpdir(), GOMAXPROCS: '2', GOFLAGS: '-p=1', CGO_ENABLED: '1',
    TM_TEST_AI_LIVE: 'local-fixture-only', TM_TEST_AI_AGENT_URL: bridge.url, TM_TEST_AI_AGENT_TOKEN: bridge.token,
    TM_TEST_AI_MODEL_URL: bridge.evaluation.reportEndpoint.url, TM_TEST_AI_MODEL_TOKEN: bridge.evaluation.reportEndpoint.token,
    TM_TEST_AI_H5_ORIGIN: origin, TM_TEST_AI_REPORT_MODEL: bridge.evaluation.route?.model ?? 'deepseek-flash', LANG: 'zh_CN.UTF-8', ...conversationEnv }
  const child = spawn(powershell, ['-NoProfile', '-File', path.join(root, 'scripts/dev/verify-ai-source-native.ps1'),
    '-MySqlBaseDir', mysql, '-RunCount', '1', '-BashPath', bash, '-ReportsOnly', '-LiveAgents', '-ReportPattern', processes ? '^TestIntegrationAIReportLocalAPIWorkerLive$' : '^TestIntegrationAIReportLiveAgents$'],
  { cwd: root, env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] })
  const log = createWriteStream(path.join(bridge.evaluation.output, 'local-chain-go.log'), { mode: 0o600 })
  const state = { status: 'running', lines: '', conversation, conversationVerified: false, analysisType, deepVerified: false, processes, processHTTPVerified:false }
  child.stdout.pipe(log, { end: false }); child.stderr.pipe(log, { end: false })
  child.stdout.on('data', bytes => {
    const output = state.lines + bytes.toString('utf8')
    state.conversationVerified ||= hasConversationProof(output)
    state.deepVerified ||= hasDeepProof(output)
    state.processHTTPVerified ||= hasProcessHTTPProof(output)
    state.lines = output.slice(-16000)
  })
  state.finished = new Promise(resolve => {
    child.once('error', () => { state.status = 'failed'; log.end(); resolve() })
    child.once('close', code => { state.exit = code; state.status = code === 0 ? 'passed' : 'failed'; log.end(); resolve() })
  })
  return state
}

export async function finishChainVerification(bridge, verification) {
  try {
    await verification.finished
    // Go启动失败时原引擎仍未启动；这是已知零调用失败，不是待执行成功回执。
    if (verification.status === 'failed' && bridge.evaluation.child === 'waiting') bridge.evaluation.child = 'failed'
    const provider = await recordReceipt(bridge.evaluation)
    const verified = verification.status === 'passed' && verification.exit === 0 && provider.workflow_verified && provider.report_model_calls === 3
      && bridge.counts.submissions === 1 && bridge.counts.polls > 0 && bridge.counts.results === 1
      && (!verification.conversation || verification.conversationVerified === true)
      && (!verification.processes || verification.conversation === true && verification.processHTTPVerified === true)
      && (verification.analysisType !== 'deep' || (verification.conversation === true && verification.deepVerified === true))
    const receipt = { scope: verification.processes ? 'local_http_process_conversation_async_real_v2_test_ledger' : verification.conversation ? 'local_conversation_async_real_v2_test_ledger' : 'local_web_async_real_v2_test_ledger', analysis_type: verification.analysisType ?? 'light', verified, go_exit: verification.exit,
      ...(verification.processes ? {browser_verified:false, process_http_verified:verification.processHTTPVerified===true} : {}),
      async: { ...bridge.counts }, provider, production_deployed: false, real_user_points_changed: false,
      materials: 'explicit_synthetic_fixture', upstream_web_mongo_redis_verified: false }
    await fs.writeFile(path.join(bridge.evaluation.output, 'local-chain-receipt.json'), JSON.stringify(receipt, null, 2), { mode: 0o600 })
    return receipt
  } finally { await bridge.close() }
}

// runner与journalIO注入只用于无模型网络自测。真实入口固定使用默认文件系统，由startLocalChain构造。
export async function createAsyncBridge(evaluation, journalIO = fs) {
  const token = randomBytes(32).toString('hex')
  let job = null
  let submitting = false
  let running = null
  const counts = { submissions: 0, polls: 0, results: 0 }
  const journal = path.join(evaluation.output, 'async-job.json')
  const save = async (next = job) => {
    await journalIO.writeFile(journal + '.new', JSON.stringify(next), { mode: 0o600, flag: 'w' })
    await journalIO.rename(journal + '.new', journal)
  }
  const transition = async status => {
    const next = { ...job, status }
    await save(next)
    // 原子替换日志成功之后才发布状态；持久化失败继续保留最后已确认事实。
    job = next
  }
  // 目录由父网关独占创建；已有日志禁止接管或重跑未知任务。
  try { await fs.access(journal); throw new Error('不得接管已有评估任务') } catch (error) { if (error.code !== 'ENOENT') throw error }
  const respond = (res, code, data) => {
    res.writeHead(code, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' })
    res.end(JSON.stringify({ success: code === 200, data }))
  }
  const run = async () => {
    try {
      await transition('running')
      await evaluation.launch()
      const receipt = await recordReceipt(evaluation)
      if (!receipt.workflow_verified) throw new Error('真实引擎未取得完整验收回执')
      await transition('completed')
    } catch {
      try { await transition('failed') } catch { /* 未确认落盘，保留原非终态，不假报失败或完成。 */ }
    }
  }
  const server = http.createServer(async (req, res) => {
    try {
      if (req.headers.host !== `127.0.0.1:${server.address().port}` || Object.hasOwn(req.headers, 'origin')
        || req.headers.authorization !== `Bearer ${token}`) return respond(res, 403, {})
      if (req.url === '/api/analysis/single' && req.method === 'POST') {
        if (job || submitting) return respond(res, 409, {})
        submitting = true
        try {
          if (!/^application\/json(?:;\s*charset=utf-8)?$/i.test(req.headers['content-type'] ?? '') || req.headers['content-encoding']) return respond(res, 400, {})
          let size = 0; const parts = []
          for await (const chunk of req) { size += chunk.length; if (size > 8192) return respond(res, 413, {}); parts.push(chunk) }
          if (!validSubmission(JSON.parse(Buffer.concat(parts).toString('utf8')))) return respond(res, 400, {})
          // 600001只是原Go契约的格式占位；原引擎始终分析明确的非证券TMEVAL测试主体。
          job = { task_id: randomUUID(), status: 'queued', scope: 'local_fixture_v2_engine', contract_symbol: '600001', engine_subject: 'TMEVAL' }
          await save(); counts.submissions++
          respond(res, 200, { task_id: job.task_id, status: 'queued' })
          running = run()
        } finally { submitting = false }
        return
      }
      if (!job) return respond(res, 404, {})
      const base = `/api/analysis/tasks/${job.task_id}`
      if (req.method === 'GET' && req.url === base + '/status') {
        counts.polls++; return respond(res, 200, { task_id: job.task_id, status: job.status })
      }
      if (req.method === 'GET' && req.url === base + '/result' && job.status === 'completed') {
        const raw = await fs.readFile(path.join(evaluation.output, 'quarantined-result.json'))
        if (raw.length > 4 * 1024 * 1024) return respond(res, 502, {})
        const result = JSON.parse(raw.toString('utf8'))
        // 正文原样保留在隔离结果中，只转交原引擎的报告字段作不可信线索，非公开报告。
        const reports = Object.fromEntries(Object.entries(result).filter(([key, value]) => key.endsWith('_report') && typeof value === 'string'))
        if (!Object.keys(reports).length) return respond(res, 502, {})
        counts.results++; return respond(res, 200, { analysis_id: job.task_id, reports, evaluation_scope: 'synthetic_fixture_only' })
      }
      // 取消请求未实现远端安全停机，明确拒绝，不伪称已退款或停止。
      return respond(res, 404, {})
    } catch { if (!res.headersSent) respond(res, 500, {}) }
  })
  server.requestTimeout = 10000; server.headersTimeout = 5000
  await new Promise(resolve => server.listen(0, '127.0.0.1', resolve))
  const timeout = setTimeout(() => {
    if (evaluation.child === 'waiting') evaluation.child = 'failed'
    else if (evaluation.child === 'running') evaluation.cancel?.()
    server.close()
    void evaluation.close().catch(() => { evaluation.receipt_error = true })
  }, 20 * 60000)
  return { evaluation, counts, url: `http://127.0.0.1:${server.address().port}`, token,
    async close() {
      clearTimeout(timeout)
      if (evaluation.child === 'running') { evaluation.cancel(); await evaluation.finished }
      if (evaluation.child === 'waiting') evaluation.child = 'failed'
      await running
      await new Promise(resolve => server.close(resolve))
      await evaluation.close()
    } }
}
