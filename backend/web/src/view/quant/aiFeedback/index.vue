<template>
  <section class="feedback-console">
    <header class="gva-search-box">
      <h2>报告反馈处理</h2>
      <p>只处理反馈记录，不调用模型、不更改报告或积分。已处理不代表更正、授权恢复或退款完成。</p>
      <div class="feedback-actions">
        <el-button :disabled="busy || pending != null || confirmation != null" @click="loadList(0)">查询首批反馈</el-button>
        <el-input v-model="lookupId" aria-label="受理单编号" placeholder="输入受理单编号" :disabled="busy || pending != null || confirmation != null" />
        <el-button :disabled="busy || pending != null || confirmation != null" @click="lookup">按编号查询</el-button>
      </div>
      <p v-if="busy" role="status">正在核对，请稍候…</p>
      <p v-if="message" role="status">{{ message }}</p>
    </header>

    <div v-if="pending" class="gva-table-box feedback-pending" role="alert">
      <h3>受理单 #{{ pending.id }} 需要核对</h3>
      <p>原操作版本 {{ pending.expectedVersion }} → {{ statusLabel(pending.status) }}。不会自动重发；请先核对原受理单及处理历史。</p>
      <el-button :disabled="busy" @click="loadDetail(pending.id)">核对原受理单</el-button>
    </div>

    <div v-if="listLoaded" class="gva-table-box">
      <h3>反馈列表（按受理单编号递增）</h3>
      <p v-if="rows.length === 0">本批暂无反馈记录。</p>
      <ul class="feedback-list">
        <li v-for="item in rows" :key="item.id" class="feedback-row">
          <span>#{{ item.id }} · 任务 #{{ item.taskId }} · {{ categoryLabel(item.category) }} · {{ statusLabel(item.status) }}</span>
          <el-button :disabled="busy || pending != null || confirmation != null" @click="loadDetail(item.id)">查看受理单 #{{ item.id }}</el-button>
        </li>
      </ul>
      <el-button v-if="hasMore" :disabled="busy || pending != null || confirmation != null" @click="loadList(nextAfterId)">查询下一批</el-button>
      <p>列表只反映查询时点，不代表独占分派；处理前须重新读取详情。</p>
    </div>

    <div v-if="detail" class="gva-table-box">
      <h3>受理单 #{{ detail.item.id }} · {{ statusLabel(detail.item.status) }}</h3>
      <p>任务 #{{ detail.item.taskId }} · {{ categoryLabel(detail.item.category) }} · 当前版本 {{ detail.item.version }}</p>
      <p>创建时间：{{ time(detail.item.createdAt) }}；更新时间：{{ time(detail.item.updatedAt) }}</p>
      <p>{{ resolutionLabel(detail.item.resolution) }}</p>
      <el-button :disabled="busy || confirmation != null" @click="loadDetail(detail.item.id)">重新查询详情</el-button>
      <h4>不可变处理历史</h4>
      <ol>
        <li v-for="event in detail.events" :key="event.version">版本 {{ event.version }} · {{ statusLabel(event.status) }} · {{ event.actorId === 0 ? '会员提交' : '内部处理人 #' + event.actorId }} · {{ time(event.at) }} · {{ resolutionLabel(event.resolution) }}</li>
      </ol>
      <div v-if="detail.item.status === 'received'">
        <el-button :disabled="busy || confirmation != null" @click="prepareStart">开始处理</el-button>
      </div>
      <div v-else-if="detail.item.status === 'processing'" class="feedback-actions">
        <el-select v-model="outcome" aria-label="处理决定" placeholder="选择处理决定" :disabled="busy || confirmation != null" @change="resolution = ''">
          <el-option label="已处理" value="resolved" /><el-option label="未确认" value="dismissed" />
        </el-select>
        <el-select v-model="resolution" aria-label="处理结果" placeholder="选择处理结果" :disabled="busy || confirmation != null || outcome === ''">
          <el-option v-for="value in resolutions" :key="value" :label="resolutionLabel(value)" :value="value" />
        </el-select>
        <el-button :disabled="busy || confirmation != null || !validDecision({ expectedVersion: 2, status: outcome, resolution })" @click="prepareResult">核对处理结果</el-button>
      </div>
      <p v-else>该受理单已结束，不能重开、删除或覆盖终态。</p>
    </div>

    <div v-if="confirmation" class="gva-table-box feedback-confirm" role="alertdialog" aria-label="确认处理反馈">
      <h3>确认处理受理单 #{{ confirmation.id }}</h3>
      <p>版本 {{ confirmation.expectedVersion }} → {{ statusLabel(confirmation.status) }}；{{ resolutionLabel(confirmation.resolution) }}</p>
      <p>仅记录本次核查，不自动更正报告、通知会员或退款。</p>
      <el-button :disabled="busy" @click="confirmation = null">暂不提交</el-button>
      <el-button type="primary" :disabled="busy" @click="submit">确认提交处理</el-button>
    </div>
  </section>
</template>

<script setup>
import { ref, computed, watch, onMounted, onActivated, onDeactivated, onBeforeUnmount } from 'vue'
import { useUserStore } from '@/pinia/modules/user'
import { feedbackList, feedbackDetail, feedbackDecision, safeId, validDecision, categoryLabel, statusLabel, resolutionLabel, feedbackError } from '@/api/quant/aiFeedback'

defineOptions({ name: 'AiFeedbackConsole' })
const user = useUserStore()
const rows = ref([]), listLoaded = ref(false), hasMore = ref(false), nextAfterId = ref(0)
const detail = ref(null), lookupId = ref(''), busy = ref(false), message = ref('')
const outcome = ref(''), resolution = ref(''), confirmation = ref(null), pending = ref(null)
let epoch = 0, active = true, alive = true
const resolutions = computed(() => outcome.value === 'resolved' ? ['reviewed', 'referred'] : outcome.value === 'dismissed' ? ['unconfirmed', 'not_applicable'] : [])
const time = value => new Date(value).toLocaleString('zh-CN', { hour12: false })
const identity = () => String(user.token || '') + ':' + String(user.userInfo?.ID || '')
const allowed = () => alive && active && typeof user.token === 'string' && user.token.length > 0 && safeId(user.userInfo?.ID)
const clear = () => { epoch++; rows.value = []; listLoaded.value = false; hasMore.value = false; nextAfterId.value = 0; detail.value = null; lookupId.value = ''; busy.value = false; message.value = ''; outcome.value = ''; resolution.value = ''; confirmation.value = null; pending.value = null }
const begin = () => {
  const round = ++epoch, account = identity()
  busy.value = true; message.value = ''; confirmation.value = null
  return () => alive && active && round === epoch && account === identity() && allowed()
}
const handleFailure = code => { message.value = feedbackError(code); if (code === 40100 || code === 40301) { rows.value = []; listLoaded.value = false; detail.value = null; confirmation.value = null } }
async function loadList(after) {
  if (busy.value || pending.value || confirmation.value || !allowed()) return
  const current = begin(); rows.value = []; listLoaded.value = false; detail.value = null
  const result = await feedbackList(after)
  if (!current()) return
  busy.value = false
  if (result.code !== 0) { handleFailure(result.code); return }
  rows.value = result.data.items; hasMore.value = result.data.hasMore; nextAfterId.value = result.data.nextAfterId; listLoaded.value = true
}
async function loadDetail(id) {
  if (busy.value || confirmation.value || !allowed() || !safeId(id) || pending.value && pending.value.id !== id) return
  const current = begin(); detail.value = null; outcome.value = ''; resolution.value = ''
  const result = await feedbackDetail(id)
  if (!current()) return
  busy.value = false
  if (result.code !== 0) { handleFailure(result.code); return }
  detail.value = result.data; lookupId.value = id.toString()
  if (pending.value) { pending.value = null; message.value = '已查询当前记录；请核对处理人、版本和历史，必要时重新确认操作。' }
}
function lookup() {
  if (!/^[1-9][0-9]*$/.test(lookupId.value) || !safeId(Number(lookupId.value))) { message.value = feedbackError(40001); return }
  loadDetail(Number(lookupId.value))
}
function prepare(decision) {
  if (busy.value || pending.value || !allowed() || !detail.value || !validDecision(decision) || detail.value.item.version !== decision.expectedVersion) return
  confirmation.value = { id: detail.value.item.id, ...decision }
}
const prepareStart = () => prepare({ expectedVersion: 1, status: 'processing', resolution: '' })
const prepareResult = () => prepare({ expectedVersion: 2, status: outcome.value, resolution: resolution.value })
async function submit() {
  if (busy.value || !confirmation.value || !allowed() || !detail.value || confirmation.value.id !== detail.value.item.id || confirmation.value.expectedVersion !== detail.value.item.version) return
  const selected = { ...confirmation.value }, current = begin()
  pending.value = selected; detail.value = null; rows.value = []; listLoaded.value = false
  const result = await feedbackDecision(selected.id, { expectedVersion: selected.expectedVersion, status: selected.status, resolution: selected.resolution })
  if (!current()) return
  busy.value = false
  message.value = result.code === 0 ? '处理请求已有回执，请重新查询受理单确认当前状态。' : feedbackError(result.code)
  if (result.code === 40100 || result.code === 40301) handleFailure(result.code)
}
watch(identity, () => clear(), { flush: 'sync' })
onMounted(() => { if (allowed()) loadList(0); else message.value = '请先登录已获准的后台处理账号。' })
onActivated(() => { active = true })
onDeactivated(() => { active = false; clear() })
onBeforeUnmount(() => { alive = false; active = false; clear() })
</script>

<style scoped>
.feedback-console { color: #30343c; }
.feedback-console p, .feedback-console li { line-height: 1.7; }
.feedback-actions { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; }
.feedback-actions :deep(.el-input), .feedback-actions :deep(.el-select) { width: 260px; max-width: 100%; }
.feedback-list { padding: 0; list-style: none; }
.feedback-row { display: flex; flex-wrap: wrap; gap: 12px; align-items: center; justify-content: space-between; padding: 12px 0; border-bottom: 1px solid #e4e7ed; }
.feedback-console :deep(.el-button) { min-height: 44px; white-space: normal; }
.feedback-pending { border-left: 4px solid #c47d18; }
.feedback-confirm { border-left: 4px solid #409eff; }
</style>
