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
export function startChainVerification(bridge, { powershell, mysql, bash, goBin, gccBin, origin }) {
  if (bridge.evaluation.child !== 'waiting') throw new Error('只能启动尚未执行的单次评估')
  const root = fileURLToPath(new URL('../../', import.meta.url))
  const env = { SystemRoot: 'C:\\Windows', WINDIR: 'C:\\Windows', PATH: `${goBin};${gccBin};C:\\Windows\\System32`, PATHEXT: '.COM;.EXE;.BAT;.CMD',
    USERPROFILE: os.homedir(), APPDATA: path.join(os.homedir(), 'AppData', 'Roaming'), LOCALAPPDATA: path.join(os.homedir(), 'AppData', 'Local'),
    TEMP: os.tmpdir(), TMP: os.tmpdir(), GOMAXPROCS: '2', GOFLAGS: '-p=1', CGO_ENABLED: '1',
    TM_TEST_AI_LIVE: 'local-fixture-only', TM_TEST_AI_AGENT_URL: bridge.url, TM_TEST_AI_AGENT_TOKEN: bridge.token,
    TM_TEST_AI_MODEL_URL: bridge.evaluation.reportEndpoint.url, TM_TEST_AI_MODEL_TOKEN: bridge.evaluation.reportEndpoint.token,
    TM_TEST_AI_H5_ORIGIN: origin, LANG: 'zh_CN.UTF-8' }
  const child = spawn(powershell, ['-NoProfile', '-File', path.join(root, 'scripts/dev/verify-ai-source-native.ps1'),
    '-MySqlBaseDir', mysql, '-RunCount', '1', '-BashPath', bash, '-ReportsOnly', '-LiveAgents', '-ReportPattern', '^TestIntegrationAIReportLiveAgents$'],
  { cwd: root, env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] })
  const log = createWriteStream(path.join(bridge.evaluation.output, 'local-chain-go.log'), { mode: 0o600 })
  const state = { status: 'running', lines: '' }
  child.stdout.pipe(log, { end: false }); child.stderr.pipe(log, { end: false })
  child.stdout.on('data', bytes => { state.lines = (state.lines + bytes.toString('utf8')).slice(-16000) })
  state.finished = new Promise(resolve => {
    child.once('error', () => { state.status = 'failed'; log.end(); resolve() })
    child.once('close', code => { state.exit = code; state.status = code === 0 ? 'passed' : 'failed'; log.end(); resolve() })
  })
  return state
}

export async function finishChainVerification(bridge, verification) {
  try {
    await verification.finished
    const provider = await recordReceipt(bridge.evaluation)
    const verified = verification.status === 'passed' && verification.exit === 0 && provider.workflow_verified && provider.report_model_calls === 3
      && bridge.counts.submissions === 1 && bridge.counts.polls > 0 && bridge.counts.results === 1
    const receipt = { scope: 'local_web_async_real_v2_test_ledger', verified, go_exit: verification.exit,
      async: { ...bridge.counts }, provider, production_deployed: false, real_user_points_changed: false,
      materials: 'explicit_synthetic_fixture', upstream_web_mongo_redis_verified: false }
    await fs.writeFile(path.join(bridge.evaluation.output, 'local-chain-receipt.json'), JSON.stringify(receipt, null, 2), { mode: 0o600 })
    return receipt
  } finally { await bridge.close() }
}

// runner注入只用于无模型网络自测。真实入口只能由startLocalChain构造。
export async function createAsyncBridge(evaluation) {
  const token = randomBytes(32).toString('hex')
  let job = null
  let submitting = false
  const counts = { submissions: 0, polls: 0, results: 0 }
  const journal = path.join(evaluation.output, 'async-job.json')
  const save = async () => {
    await fs.writeFile(journal + '.new', JSON.stringify(job), { mode: 0o600, flag: 'w' })
    await fs.rename(journal + '.new', journal)
  }
  // 目录由父网关独占创建；已有日志禁止接管或重跑未知任务。
  try { await fs.access(journal); throw new Error('不得接管已有评估任务') } catch (error) { if (error.code !== 'ENOENT') throw error }
  const respond = (res, code, data) => {
    res.writeHead(code, { 'Content-Type': 'application/json', 'Cache-Control': 'no-store' })
    res.end(JSON.stringify({ success: code === 200, data }))
  }
  const run = async () => {
    try {
      job.status = 'running'; await save()
      await evaluation.launch()
      const receipt = await recordReceipt(evaluation)
      if (!receipt.workflow_verified) throw new Error('真实引擎未取得完整验收回执')
      job.status = 'completed'; await save()
    } catch {
      job.status = 'failed'
      try { await save() } catch { /* 保持失败，不能返回completed */ }
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
          void run()
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
      await new Promise(resolve => server.close(resolve))
      await evaluation.close()
    } }
}
