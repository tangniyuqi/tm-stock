import request from '@/utils/request'

export const safeId = value => Number.isSafeInteger(value) && value > 0
const timestamp = value => safeId(value) && value <= 8640000000000000
const categories = ['fact', 'source', 'display', 'other']
export const validRecord = value => value != null && safeId(value.id) && safeId(value.taskId) && categories.includes(value.category)
  && timestamp(value.createdAt) && timestamp(value.updatedAt) && value.updatedAt >= value.createdAt
  && ((value.status === 'received' && value.version === 1 && value.resolution === '' && value.createdAt === value.updatedAt)
    || (value.status === 'processing' && value.version === 2 && value.resolution === '')
    || (value.status === 'resolved' && value.version === 3 && ['reviewed', 'referred'].includes(value.resolution))
    || (value.status === 'dismissed' && value.version === 3 && ['unconfirmed', 'not_applicable'].includes(value.resolution)))
const copyRecord = value => ({ id: value.id, taskId: value.taskId, category: value.category, status: value.status, resolution: value.resolution, version: value.version, createdAt: value.createdAt, updatedAt: value.updatedAt })
export const validPage = (page, after) => {
  if (!page || !Array.isArray(page.items) || page.items.length > 20 || typeof page.hasMore !== 'boolean') return false
  let prev = after
  for (const item of page.items) { if (!validRecord(item) || item.id <= prev) return false; prev = item.id }
  return page.hasMore ? page.items.length > 0 && page.nextAfterId === prev : page.nextAfterId === 0
}
export const validDetail = (value, id) => {
  if (!value || !validRecord(value.item) || value.item.id !== id || !Array.isArray(value.events) || value.events.length !== value.item.version) return false
  const item = value.item
  for (let i = 0; i < value.events.length; i++) {
    const event = value.events[i]
    if (!event || event.version !== i + 1 || !timestamp(event.at) || event.at < item.createdAt || event.at > item.updatedAt) return false
    if (i === 0 && (event.actorId !== 0 || event.status !== 'received' || event.resolution !== '' || event.at !== item.createdAt)) return false
    if (i > 0 && (!safeId(event.actorId) || event.at < value.events[i - 1].at)) return false
    if (i === 1 && (event.status !== 'processing' || event.resolution !== '')) return false
    if (i === 2 && (event.status !== item.status || event.resolution !== item.resolution)) return false
  }
  const last = value.events[value.events.length - 1]
  return last.status === item.status && last.resolution === item.resolution && last.at === item.updatedAt
}
export const validDecision = value => value != null && Object.keys(value).sort().join(',') === 'expectedVersion,resolution,status'
  && ((value.expectedVersion === 1 && value.status === 'processing' && value.resolution === '')
    || (value.expectedVersion === 2 && value.status === 'resolved' && ['reviewed', 'referred'].includes(value.resolution))
    || (value.expectedVersion === 2 && value.status === 'dismissed' && ['unconfirmed', 'not_applicable'].includes(value.resolution)))
export const categoryLabel = value => ({ fact: '事实或归属依据', source: '来源或授权', display: '页面显示', other: '其他问题' }[value] || '分类暂不可用')
export const statusLabel = value => ({ received: '已受理', processing: '处理中', resolved: '已处理', dismissed: '未确认' }[value] || '状态暂不可用')
export const resolutionLabel = value => ({ reviewed: '已完成核查记录（不代表报告已更正）', referred: '已记录转交（不代表更正或退款已完成）', unconfirmed: '本次核查未确认所选问题', not_applicable: '所选分类不适用于本报告', '': '尚无处理结果' }[value] || '结果暂不可用')
export const feedbackError = code => ({ 40100: '登录状态已失效，请重新登录。', 40301: '当前账号或内部凭据未获准处理反馈。', 40403: '受理单不存在。', 40904: '受理单已被其他操作更新，请查询当前状态。', 40001: '查询或处理参数无效。' }[code] || '服务或提交结果未确认，请查询原受理单，不要重复提交。')
const failure = code => ({ code, data: null })
async function call(options) {
  try {
    const response = await request({ ...options, timeout: 10000, donNotShowLoading: true })
    // 原GVA Casbin中间件用HTTP200/code7拒绝；仅在反馈专用封装归一化，不改全站契约。
    if (response?.code === 7) return failure(40301)
    return response && Number.isInteger(response.code) ? response : failure(50306)
  } catch (error) {
    return failure(error?.response?.status === 401 ? 40100 : ([40001, 40301, 40403, 40904].includes(error?.response?.data?.code) ? error.response.data.code : 50306))
  }
}
export async function feedbackList(after = 0) {
  if (!Number.isSafeInteger(after) || after < 0) return failure(40001)
  const response = await call({ url: '/quant/ai-feedback', method: 'get', params: after > 0 ? { after_id: after, limit: 20 } : { limit: 20 } })
  if (response.code !== 0) return failure(response.code)
  return validPage(response.data, after) ? { code: 0, data: { items: response.data.items.map(copyRecord), hasMore: response.data.hasMore, nextAfterId: response.data.nextAfterId } } : failure(50306)
}
export async function feedbackDetail(id) {
  if (!safeId(id)) return failure(40001)
  const response = await call({ url: '/quant/ai-feedback/' + id, method: 'get' })
  if (response.code !== 0) return failure(response.code)
  return validDetail(response.data, id) ? { code: 0, data: { item: copyRecord(response.data.item), events: response.data.events.map(e => ({ version: e.version, actorId: e.actorId, status: e.status, resolution: e.resolution, at: e.at })) } } : failure(50306)
}
export async function feedbackDecision(id, decision) {
  if (!safeId(id) || !validDecision(decision)) return failure(40001)
  const response = await call({ url: '/quant/ai-feedback/' + id + '/decisions', method: 'post', data: { ...decision } })
  if (response.code !== 0) return failure(response.code)
  return validRecord(response.data) && response.data.id === id && response.data.version >= decision.expectedVersion + 1 ? { code: 0, data: copyRecord(response.data) } : failure(50306)
}
